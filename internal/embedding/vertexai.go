package embedding

import (
	"context"
	"fmt"

	"github.com/nlink-jp/slack-personal-agent/internal/config"
	"google.golang.org/genai"
)

// VertexAIEmbedder implements Embedder using Vertex AI text embeddings.
type VertexAIEmbedder struct {
	client *genai.Client
	model  string
}

// NewVertexAIEmbedder creates a new Vertex AI embedding backend.
func NewVertexAIEmbedder(vertexCfg config.VertexAIConfig, embCfg config.EmbeddingVertexAIConfig) (*VertexAIEmbedder, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  vertexCfg.Project,
		Location: vertexCfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create Vertex AI client: %w", err)
	}

	model := embCfg.Model
	if model == "" {
		model = "text-embedding-005"
	}

	return &VertexAIEmbedder{
		client: client,
		model:  model,
	}, nil
}

func (e *VertexAIEmbedder) ModelID() string {
	return fmt.Sprintf("vertex_ai:%s:768", e.model)
}

func (e *VertexAIEmbedder) Dimensions() int {
	return 768 // text-embedding-005 default
}

// Embed generates embeddings via Vertex AI.
func (e *VertexAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	var contents []*genai.Content
	for _, t := range texts {
		contents = append(contents, genai.NewContentFromText(t, ""))
	}

	result, err := e.client.Models.EmbedContent(ctx, e.model, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("Vertex AI embed: %w", err)
	}

	embeddings := make([][]float32, len(result.Embeddings))
	for i, emb := range result.Embeddings {
		embeddings[i] = emb.Values
	}
	return embeddings, nil
}
