package embedding

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
)

const (
	builtinModel    = "sentence-transformers/all-MiniLM-L6-v2"
	builtinDims     = 384
	builtinModelDir = "models"
)

// BuiltinEmbedder uses Hugot with all-MiniLM-L6-v2 for local embedding.
// Pure Go backend — no external dependencies or API calls.
type BuiltinEmbedder struct {
	session  *hugot.Session
	pipeline *pipelines.FeatureExtractionPipeline
	modelDir string
	mu       sync.Mutex
}

// NewBuiltinEmbedder creates a new builtin embedder.
// modelDir is the directory where the ONNX model is cached.
// If empty, defaults to ~/Library/Application Support/slack-personal-agent/models.
func NewBuiltinEmbedder(modelDir string) (*BuiltinEmbedder, error) {
	if modelDir == "" {
		home, _ := os.UserHomeDir()
		modelDir = filepath.Join(home, "Library", "Application Support", "slack-personal-agent", builtinModelDir)
	}

	if err := os.MkdirAll(modelDir, 0o700); err != nil {
		return nil, fmt.Errorf("create model dir: %w", err)
	}

	return &BuiltinEmbedder{
		modelDir: modelDir,
	}, nil
}

func (e *BuiltinEmbedder) ModelID() string {
	return fmt.Sprintf("builtin:%s:%d", builtinModel, builtinDims)
}

func (e *BuiltinEmbedder) Dimensions() int {
	return builtinDims
}

// Embed generates embeddings using the built-in model.
// On first call, downloads and initializes the model (may take a few seconds).
func (e *BuiltinEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	if err := e.ensureInitialized(ctx); err != nil {
		return nil, fmt.Errorf("init builtin embedder: %w", err)
	}

	// Truncate texts to fit within model's max sequence length (512 tokens).
	// Conservative limit: ~1500 chars covers most 512-token inputs for mixed JP/EN.
	truncated := make([]string, len(texts))
	for i, t := range texts {
		truncated[i] = truncateForEmbedding(t, 1500)
	}

	output, err := e.pipeline.RunPipeline(ctx, truncated)
	if err != nil {
		return nil, fmt.Errorf("run embedding: %w", err)
	}

	results := make([][]float32, len(output.Embeddings))
	for i, emb := range output.Embeddings {
		results[i] = emb
	}
	return results, nil
}

// ensureInitialized lazily initializes the session and pipeline.
func (e *BuiltinEmbedder) ensureInitialized(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.pipeline != nil {
		return nil
	}

	// Download model if not cached
	modelPath := filepath.Join(e.modelDir, "all-MiniLM-L6-v2")
	if _, err := os.Stat(filepath.Join(modelPath, "tokenizer.json")); os.IsNotExist(err) {
		log.Printf("Downloading embedding model %s...", builtinModel)
		opts := hugot.NewDownloadOptions()
		opts.OnnxFilePath = "onnx/model.onnx" // Select base model (repo has multiple variants)
		downloaded, err := hugot.DownloadModel(ctx, builtinModel, e.modelDir, opts)
		if err != nil {
			return fmt.Errorf("download model: %w", err)
		}
		modelPath = downloaded
		log.Printf("Model downloaded to %s", modelPath)
	}

	// Create Go session (pure Go, no ONNX runtime needed)
	session, err := hugot.NewGoSession(ctx)
	if err != nil {
		return fmt.Errorf("create Go session: %w", err)
	}
	e.session = session

	// Create feature extraction pipeline
	// Explicitly select the base ONNX model (the repo contains multiple variants)
	pipelineConfig := hugot.FeatureExtractionConfig{
		ModelPath:    modelPath,
		Name:         "embedding",
		OnnxFilename: "onnx/model.onnx",
	}
	pipeline, err := hugot.NewPipeline(session, pipelineConfig)
	if err != nil {
		return fmt.Errorf("create pipeline: %w", err)
	}
	e.pipeline = pipeline

	log.Printf("Builtin embedder initialized: %s (%d dims)", builtinModel, builtinDims)
	return nil
}

// truncateForEmbedding truncates text to approximately maxChars characters,
// cutting at a word/rune boundary. MiniLM's max sequence length is 512 tokens;
// ~1500 chars is a safe limit for mixed Japanese/English text.
func truncateForEmbedding(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	// Cut at rune boundary
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text
	}
	return string(runes[:maxChars])
}

// Close releases the session resources.
func (e *BuiltinEmbedder) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.session != nil {
		e.session.Destroy()
		e.session = nil
		e.pipeline = nil
	}
}
