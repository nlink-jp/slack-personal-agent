// Package embedding provides the text embedding interface, independent of the LLM backend.
// Switching the LLM backend (local/vertex_ai) does NOT affect embeddings.
// Switching the embedding backend requires re-indexing — ModelID tracks this.
package embedding

import (
	"context"
	"fmt"

	"github.com/nlink-jp/slack-personal-agent/internal/config"
)

// Embedder generates vector embeddings from text.
// Independent of the LLM Backend used for chat/summarization.
type Embedder interface {
	// Embed generates embeddings for the given texts.
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the embedding vector dimensionality.
	Dimensions() int

	// ModelID returns a unique identifier for the embedding model.
	// Used to detect model changes that require re-indexing.
	// Format: "<backend>:<model-name>:<dimensions>"
	ModelID() string
}

// NewEmbedder creates the appropriate embedder from config.
func NewEmbedder(cfg *config.Config) (Embedder, error) {
	switch cfg.Embedding.Backend {
	case "builtin":
		// TODO: Implement builtin embedder (Hugot + all-MiniLM-L6-v2)
		return nil, fmt.Errorf("builtin embedder not yet implemented; use 'local' or 'vertex_ai'")
	case "local":
		return NewLocalEmbedder(cfg.Embedding.Local), nil
	case "vertex_ai":
		return NewVertexAIEmbedder(cfg.VertexAI, cfg.Embedding.VertexAI)
	default:
		return nil, fmt.Errorf("unknown embedding backend: %q", cfg.Embedding.Backend)
	}
}
