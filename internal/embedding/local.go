package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nlink-jp/slack-personal-agent/internal/config"
)

// LocalEmbedder implements Embedder using an OpenAI-compatible /v1/embeddings API.
type LocalEmbedder struct {
	endpoint   string
	model      string
	apiKey     string
	dimensions atomic.Int32
	httpClient *http.Client
}

// NewLocalEmbedder creates a new local embedding backend.
func NewLocalEmbedder(cfg config.EmbeddingLocalConfig) *LocalEmbedder {
	return &LocalEmbedder{
		endpoint: strings.TrimRight(cfg.Endpoint, "/"),
		model:    cfg.Model,
		apiKey:   cfg.APIKey,
		httpClient: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}
}

func (e *LocalEmbedder) ModelID() string {
	return fmt.Sprintf("local:%s:%d", e.model, e.dimensions.Load())
}

func (e *LocalEmbedder) Dimensions() int {
	return int(e.dimensions.Load())
}

// Embed generates embeddings via the OpenAI-compatible /v1/embeddings endpoint.
func (e *LocalEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	reqBody := embeddingRequest{
		Model: e.model,
		Input: texts,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.endpoint+"/embeddings", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("embedding request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("embedding API error %d: %s", resp.StatusCode, string(body))
	}

	var embResp embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}

	result := make([][]float32, len(embResp.Data))
	for i, d := range embResp.Data {
		result[i] = d.Embedding
		// Auto-detect dimensions from first response (atomic for concurrent safety)
		if e.dimensions.Load() == 0 && len(d.Embedding) > 0 {
			e.dimensions.CompareAndSwap(0, int32(len(d.Embedding)))
		}
	}

	return result, nil
}

// OpenAI-compatible embedding request/response types

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []embeddingData `json:"data"`
}

type embeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}
