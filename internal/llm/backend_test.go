package llm

import (
	"testing"

	"github.com/nlink-jp/slack-personal-agent/internal/config"
)

func TestNewBackendLocal(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.LLM.Backend = "local"

	b, err := NewBackend(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if b.Name() != "local:google/gemma-4-26b-a4b" {
		t.Errorf("expected local backend name, got %q", b.Name())
	}
}

func TestNewBackendUnknown(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.LLM.Backend = "unknown"

	_, err := NewBackend(cfg)
	if err == nil {
		t.Error("expected error for unknown backend")
	}
}

func TestTokenEstimation(t *testing.T) {
	tests := []struct {
		text string
		min  int
	}{
		{"hello world", 2},
		{"こんにちは世界", 10},          // CJK: ~2 tokens per char
		{`{"key": "value"}`, 5}, // JSON punctuation
		{"", 0},
	}

	for _, tt := range tests {
		got := EstimateTokenCount(tt.text)
		if got < tt.min {
			t.Errorf("EstimateTokenCount(%q) = %d, want >= %d", tt.text, got, tt.min)
		}
	}
}

func TestLocalBackendName(t *testing.T) {
	b := NewLocalBackend(config.LocalLLMConfig{
		Endpoint: "http://localhost:1234/v1",
		Model:    "test-model",
	})
	if b.Name() != "local:test-model" {
		t.Errorf("expected 'local:test-model', got %q", b.Name())
	}
}

func TestIsFormatUnsupportedError(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{&apiError{StatusCode: 400, Body: "response_format not supported"}, true},
		{&apiError{StatusCode: 422, Body: "unknown field response_format"}, true},
		{&apiError{StatusCode: 500, Body: "internal error"}, false},
		{&apiError{StatusCode: 400, Body: "rate limit exceeded"}, false},
	}

	for _, tt := range tests {
		got := isFormatUnsupportedError(tt.err)
		if got != tt.want {
			t.Errorf("isFormatUnsupportedError(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

// Compile-time interface check
var _ Backend = (*LocalBackend)(nil)
