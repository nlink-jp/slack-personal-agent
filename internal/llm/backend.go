// Package llm provides the unified LLM interface for chat and summarization.
// Embedding is handled separately in the embedding package.
package llm

import (
	"context"
	"fmt"

	"github.com/nlink-jp/slack-personal-agent/internal/config"
)

// Backend is the unified interface for LLM backends.
// Used for chat, summarization, and intent detection.
// Embedding is NOT part of this interface (see internal/embedding).
type Backend interface {
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req *ChatRequest, cb StreamCallback) error
	EstimateTokens(text string) int
	Name() string
}

// StreamCallback receives streaming tokens from the LLM.
// done is true on the final call.
type StreamCallback func(token string, done bool)

// ChatRequest holds a request to the LLM.
type ChatRequest struct {
	SystemPrompt string
	Messages     []Message
	Temperature  *float32
	MaxTokens    int
	ResponseJSON bool
}

// Message is a single chat message.
type Message struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// ChatResponse holds the LLM response.
type ChatResponse struct {
	Content string
	Usage   *Usage
}

// Usage holds token usage information.
type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

// NewBackend creates the appropriate backend from config.
func NewBackend(cfg *config.Config) (Backend, error) {
	switch cfg.LLM.Backend {
	case "local":
		return NewLocalBackend(cfg.LocalLLM), nil
	case "vertex_ai":
		return NewVertexAIBackend(cfg.VertexAI)
	default:
		return nil, fmt.Errorf("unknown LLM backend: %q", cfg.LLM.Backend)
	}
}
