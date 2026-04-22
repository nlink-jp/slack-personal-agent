package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nlink-jp/nlk/backoff"
	"github.com/nlink-jp/slack-personal-agent/internal/config"
)

// LocalBackend implements Backend for OpenAI-compatible local LLM servers.
type LocalBackend struct {
	endpoint   string
	model      string
	apiKey     string
	httpClient *http.Client
}

// NewLocalBackend creates a new local LLM backend.
func NewLocalBackend(cfg config.LocalLLMConfig) *LocalBackend {
	return &LocalBackend{
		endpoint: strings.TrimRight(cfg.Endpoint, "/"),
		model:    cfg.Model,
		apiKey:   cfg.APIKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (b *LocalBackend) Name() string              { return "local:" + b.model }
func (b *LocalBackend) EstimateTokens(text string) int { return EstimateTokenCount(text) }

// Chat sends a non-streaming request to the LLM.
func (b *LocalBackend) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	resp, err := b.doChat(ctx, req)
	if err != nil && req.ResponseJSON && isFormatUnsupportedError(err) {
		reqCopy := *req
		reqCopy.ResponseJSON = false
		return b.doChat(ctx, &reqCopy)
	}
	return resp, err
}

// ChatStream sends a streaming request to the LLM.
func (b *LocalBackend) ChatStream(ctx context.Context, req *ChatRequest, cb StreamCallback) error {
	err := b.doChatStream(ctx, req, cb)
	if err != nil && req.ResponseJSON && isFormatUnsupportedError(err) {
		reqCopy := *req
		reqCopy.ResponseJSON = false
		return b.doChatStream(ctx, &reqCopy, cb)
	}
	return err
}

var llmBackoff = backoff.New(
	backoff.WithBase(2*time.Second),
	backoff.WithMax(30*time.Second),
	backoff.WithJitter(500*time.Millisecond),
)

func (b *LocalBackend) doChat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	body := b.buildRequestBody(req, false)
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(llmBackoff.Duration(attempt - 1)):
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
			b.endpoint+"/chat/completions", bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		b.setHeaders(httpReq)

		resp, err := b.httpClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("LLM request: %w", err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read LLM response: %w", err)
		}

		// Retry on 429 and 5xx
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = &apiError{StatusCode: resp.StatusCode, Body: string(respBody)}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, &apiError{StatusCode: resp.StatusCode, Body: string(respBody)}
		}

		var chatResp openAIChatResponse
		if err := json.Unmarshal(respBody, &chatResp); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}

		if len(chatResp.Choices) == 0 {
			return nil, fmt.Errorf("LLM returned no choices")
		}

		return &ChatResponse{
			Content: chatResp.Choices[0].Message.Content,
			Usage: &Usage{
				InputTokens:  chatResp.Usage.PromptTokens,
				OutputTokens: chatResp.Usage.CompletionTokens,
				TotalTokens:  chatResp.Usage.TotalTokens,
			},
		}, nil
	}

	return nil, fmt.Errorf("LLM request failed after retries: %w", lastErr)
}

func (b *LocalBackend) doChatStream(ctx context.Context, req *ChatRequest, cb StreamCallback) error {
	body := b.buildRequestBody(req, true)
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		b.endpoint+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return err
	}
	b.setHeaders(httpReq)

	resp, err := b.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("LLM stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &apiError{StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			cb("", true)
			return nil
		}

		var chunk openAIStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta.Content
			if delta != "" {
				cb(delta, false)
			}
		}
	}

	cb("", true)
	return scanner.Err()
}

func (b *LocalBackend) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if b.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+b.apiKey)
	}
}

func (b *LocalBackend) buildRequestBody(req *ChatRequest, stream bool) openAIChatRequest {
	var messages []openAIMessage
	if req.SystemPrompt != "" {
		messages = append(messages, openAIMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, m := range req.Messages {
		messages = append(messages, openAIMessage{Role: m.Role, Content: m.Content})
	}

	r := openAIChatRequest{
		Model:    b.model,
		Messages: messages,
		Stream:   stream,
	}
	if req.Temperature != nil {
		r.Temperature = req.Temperature
	}
	if req.MaxTokens > 0 {
		r.MaxTokens = req.MaxTokens
	}
	if req.ResponseJSON {
		r.ResponseFormat = &openAIResponseFormat{Type: "json_object"}
	}
	return r
}

// apiError represents an HTTP error from the LLM API.
type apiError struct {
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("LLM error %d: %s", e.StatusCode, e.Body)
}

func isFormatUnsupportedError(err error) bool {
	ae, ok := err.(*apiError)
	if !ok {
		return false
	}
	if ae.StatusCode != 400 && ae.StatusCode != 422 {
		return false
	}
	body := strings.ToLower(ae.Body)
	keywords := []string{"response_format", "not supported", "unsupported", "unknown field"}
	for _, kw := range keywords {
		if strings.Contains(body, kw) {
			return true
		}
	}
	return false
}

// OpenAI-compatible request/response types

type openAIChatRequest struct {
	Model          string               `json:"model"`
	Messages       []openAIMessage      `json:"messages"`
	Stream         bool                 `json:"stream"`
	Temperature    *float32             `json:"temperature,omitempty"`
	MaxTokens      int                  `json:"max_tokens,omitempty"`
	ResponseFormat *openAIResponseFormat `json:"response_format,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponseFormat struct {
	Type string `json:"type"`
}

type openAIChatResponse struct {
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

type openAIChoice struct {
	Message openAIMessage `json:"message"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAIStreamChunk struct {
	Choices []openAIStreamChoice `json:"choices"`
}

type openAIStreamChoice struct {
	Delta openAIDelta `json:"delta"`
}

type openAIDelta struct {
	Content string `json:"content"`
}
