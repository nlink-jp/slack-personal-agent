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

	output, err := e.pipeline.RunPipeline(ctx, texts)
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
		downloaded, err := hugot.DownloadModel(ctx, builtinModel, e.modelDir, hugot.NewDownloadOptions())
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
	pipelineConfig := hugot.FeatureExtractionConfig{
		ModelPath: modelPath,
		Name:      "embedding",
	}
	pipeline, err := hugot.NewPipeline(session, pipelineConfig)
	if err != nil {
		return fmt.Errorf("create pipeline: %w", err)
	}
	e.pipeline = pipeline

	log.Printf("Builtin embedder initialized: %s (%d dims)", builtinModel, builtinDims)
	return nil
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
