package fileproc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractorIsSupported(t *testing.T) {
	e := NewExtractor()

	supported := []string{
		"text/plain",
		"text/markdown",
		"text/csv",
		"text/plain; charset=utf-8",
		"application/json",
		"application/x-yaml",
		"text/x-go",
	}
	for _, mime := range supported {
		if !e.IsSupported(mime) {
			t.Errorf("expected %q to be supported", mime)
		}
	}

	unsupported := []string{
		"image/png",
		"application/pdf",
		"application/zip",
		"video/mp4",
	}
	for _, mime := range unsupported {
		if e.IsSupported(mime) {
			t.Errorf("expected %q to be unsupported", mime)
		}
	}
}

func TestExtractText(t *testing.T) {
	e := NewExtractor()

	tests := []struct {
		data     string
		mime     string
		expected string
	}{
		{"hello world", "text/plain", "hello world"},
		{"line1\r\nline2\r\n", "text/plain", "line1\nline2"},
		{`{"key": "value"}`, "application/json", `{"key": "value"}`},
		{"  trimmed  \n\n", "text/plain", "trimmed"},
	}

	for _, tt := range tests {
		got, err := e.Extract([]byte(tt.data), tt.mime)
		if err != nil {
			t.Errorf("Extract(%q, %q) error: %v", tt.data, tt.mime, err)
			continue
		}
		if got != tt.expected {
			t.Errorf("Extract(%q, %q) = %q, want %q", tt.data, tt.mime, got, tt.expected)
		}
	}
}

func TestExtractUnsupported(t *testing.T) {
	e := NewExtractor()

	_, err := e.Extract([]byte{0x89, 0x50, 0x4E, 0x47}, "image/png")
	if err == nil {
		t.Error("expected error for unsupported MIME type")
	}
}

func TestDownloader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte("file content here"))
	}))
	defer server.Close()

	d := NewDownloader()
	ctx := context.Background()

	data, err := d.Download(ctx, server.URL, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "file content here" {
		t.Errorf("expected file content, got %q", string(data))
	}
}

func TestDownloaderUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	d := NewDownloader()
	_, err := d.Download(context.Background(), server.URL, "bad-token")
	if err == nil {
		t.Error("expected error for unauthorized response")
	}
}

func TestProcessFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("important document content"))
	}))
	defer server.Close()

	d := NewDownloader()
	e := NewExtractor()
	ctx := context.Background()

	// Supported file
	f, err := ProcessFile(ctx, d, e, "F001", "notes.txt", "text/plain", server.URL, "token", 100)
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Fatal("expected non-nil file")
	}
	if f.Content != "important document content" {
		t.Errorf("expected content, got %q", f.Content)
	}

	// Unsupported file — should return nil without error
	f, err = ProcessFile(ctx, d, e, "F002", "photo.png", "image/png", server.URL, "token", 100)
	if err != nil {
		t.Fatal(err)
	}
	if f != nil {
		t.Error("expected nil for unsupported file type")
	}

	// Too large — should return nil without error
	f, err = ProcessFile(ctx, d, e, "F003", "huge.txt", "text/plain", server.URL, "token", MaxFileSize+1)
	if err != nil {
		t.Fatal(err)
	}
	if f != nil {
		t.Error("expected nil for oversized file")
	}
}
