package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/nlink-jp/slack-personal-agent/internal/config"
	"google.golang.org/genai"
)

// VertexAIBackend implements Backend for Vertex AI (Gemini).
type VertexAIBackend struct {
	client *genai.Client
	model  string
}

// NewVertexAIBackend creates a new Vertex AI backend using ADC.
func NewVertexAIBackend(cfg config.VertexAIConfig) (*VertexAIBackend, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  cfg.Project,
		Location: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create Vertex AI client: %w", err)
	}
	return &VertexAIBackend{
		client: client,
		model:  cfg.Model,
	}, nil
}

func (b *VertexAIBackend) Name() string              { return "vertex_ai:" + b.model }
func (b *VertexAIBackend) EstimateTokens(text string) int { return EstimateTokenCount(text) }

// Chat sends a non-streaming request to Vertex AI.
func (b *VertexAIBackend) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	gcConfig := b.buildConfig(req)
	contents := b.buildContents(req)

	resp, err := b.client.Models.GenerateContent(ctx, b.model, contents, gcConfig)
	if err != nil {
		return nil, fmt.Errorf("Vertex AI generate: %w", err)
	}

	text := extractText(resp)
	result := &ChatResponse{Content: text}
	if resp.UsageMetadata != nil {
		result.Usage = &Usage{
			InputTokens:  int(resp.UsageMetadata.PromptTokenCount),
			OutputTokens: int(resp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:  int(resp.UsageMetadata.TotalTokenCount),
		}
	}
	return result, nil
}

// ChatStream sends a streaming request to Vertex AI.
func (b *VertexAIBackend) ChatStream(ctx context.Context, req *ChatRequest, cb StreamCallback) error {
	gcConfig := b.buildConfig(req)
	contents := b.buildContents(req)

	iter := b.client.Models.GenerateContentStream(ctx, b.model, contents, gcConfig)
	for resp, err := range iter {
		if err != nil {
			return fmt.Errorf("Vertex AI stream: %w", err)
		}
		text := extractText(resp)
		if text != "" {
			cb(text, false)
		}
	}

	cb("", true)
	return nil
}

func (b *VertexAIBackend) buildConfig(req *ChatRequest) *genai.GenerateContentConfig {
	cfg := &genai.GenerateContentConfig{}

	if req.SystemPrompt != "" {
		cfg.SystemInstruction = genai.NewContentFromText(req.SystemPrompt, "")
	}
	if req.Temperature != nil {
		t := float32(*req.Temperature)
		cfg.Temperature = &t
	}
	if req.MaxTokens > 0 {
		cfg.MaxOutputTokens = int32(req.MaxTokens)
	}
	if req.ResponseJSON {
		cfg.ResponseMIMEType = "application/json"
	}
	return cfg
}

func (b *VertexAIBackend) buildContents(req *ChatRequest) []*genai.Content {
	var contents []*genai.Content
	for _, m := range req.Messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, genai.NewContentFromText(m.Content, genai.Role(role)))
	}
	return contents
}

func extractText(resp *genai.GenerateContentResponse) string {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return ""
	}
	var parts []string
	for _, p := range resp.Candidates[0].Content.Parts {
		if p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "")
}
