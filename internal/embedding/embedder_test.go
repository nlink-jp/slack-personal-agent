package embedding

import (
	"context"
	"testing"

	"github.com/nlink-jp/slack-personal-agent/internal/config"
)

func TestMockEmbedder(t *testing.T) {
	emb := NewMockEmbedder(384)

	if emb.Dimensions() != 384 {
		t.Errorf("expected 384 dimensions, got %d", emb.Dimensions())
	}
	if emb.ModelID() != "mock:test:384" {
		t.Errorf("expected 'mock:test:384', got %q", emb.ModelID())
	}

	ctx := context.Background()
	vecs, err := emb.Embed(ctx, []string{"hi", "hello world"})
	if err != nil {
		t.Fatal(err)
	}

	if len(vecs) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vecs))
	}
	if len(vecs[0]) != 384 {
		t.Errorf("expected 384-dim vector, got %d", len(vecs[0]))
	}

	// Different text lengths should produce different embeddings
	if vecs[0][0] == vecs[1][0] {
		t.Error("expected different embeddings for different text lengths")
	}
}

func TestMockEmbedderEmptyInput(t *testing.T) {
	emb := NewMockEmbedder(384)

	vecs, err := emb.Embed(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if vecs != nil {
		t.Errorf("expected nil for empty input, got %v", vecs)
	}
}

func TestNewEmbedderLocal(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Embedding.Backend = "local"

	emb, err := NewEmbedder(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if emb == nil {
		t.Fatal("expected non-nil embedder")
	}
}

func TestNewEmbedderBuiltinNotYetImplemented(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Embedding.Backend = "builtin"

	_, err := NewEmbedder(cfg)
	if err == nil {
		t.Error("expected error for not-yet-implemented builtin")
	}
}

func TestNewEmbedderUnknown(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Embedding.Backend = "unknown"

	_, err := NewEmbedder(cfg)
	if err == nil {
		t.Error("expected error for unknown backend")
	}
}

func TestLocalEmbedderModelID(t *testing.T) {
	emb := NewLocalEmbedder(config.EmbeddingLocalConfig{
		Endpoint: "http://localhost:1234/v1",
		Model:    "nomic-embed-text",
	})

	id := emb.ModelID()
	if id != "local:nomic-embed-text:0" {
		t.Errorf("expected model ID with 0 dims (auto-detect), got %q", id)
	}
}

// Compile-time interface checks
var _ Embedder = (*LocalEmbedder)(nil)
var _ Embedder = (*MockEmbedder)(nil)
