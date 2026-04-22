package embedding

import (
	"context"
	"fmt"
)

// MockEmbedder is an in-memory Embedder for testing.
// Generates deterministic embeddings based on text length.
type MockEmbedder struct {
	dims    int
	modelID string
}

// NewMockEmbedder creates a mock embedder with the given dimensions.
func NewMockEmbedder(dims int) *MockEmbedder {
	return &MockEmbedder{
		dims:    dims,
		modelID: fmt.Sprintf("mock:test:%d", dims),
	}
}

func (m *MockEmbedder) Dimensions() int  { return m.dims }
func (m *MockEmbedder) ModelID() string   { return m.modelID }

// Embed generates deterministic mock embeddings.
// Each dimension is filled with a value derived from the text length.
func (m *MockEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	result := make([][]float32, len(texts))
	for i, t := range texts {
		vec := make([]float32, m.dims)
		seed := float32(len(t)) / 100.0
		for j := range vec {
			vec[j] = seed + float32(j)*0.001
		}
		result[i] = vec
	}
	return result, nil
}
