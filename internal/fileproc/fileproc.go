// Package fileproc handles downloading and extracting text content from
// Slack file attachments for knowledge base integration.
package fileproc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MaxFileSize is the maximum file size to download (10 MB).
const MaxFileSize = 10 * 1024 * 1024

// SupportedMIME lists MIME types that can be processed for text extraction.
var SupportedMIME = map[string]bool{
	"text/plain":               true,
	"text/markdown":            true,
	"text/csv":                 true,
	"text/html":                true,
	"text/xml":                 true,
	"application/json":         true,
	"application/xml":          true,
	"application/x-yaml":       true,
	"application/javascript":   true,
	"application/x-python":     true,
	"application/x-sh":         true,
}

// File represents a downloaded Slack file with extracted content.
type File struct {
	ID       string
	Name     string
	MimeType string
	Size     int
	Content  string // Extracted text content
}

// Downloader handles authenticated file downloads from Slack.
type Downloader struct {
	httpClient   *http.Client
	allowedHosts map[string]bool // Hosts that may receive the Bearer token
}

// NewDownloader creates a new file downloader.
// Only sends tokens to files.slack.com by default.
func NewDownloader() *Downloader {
	return &Downloader{
		httpClient: &http.Client{
			Timeout: 2 * time.Minute,
		},
		allowedHosts: map[string]bool{"files.slack.com": true},
	}
}

// NewDownloaderForTest creates a downloader that allows any host (testing only).
func NewDownloaderForTest() *Downloader {
	return &Downloader{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Download fetches a file from Slack using the User Token for authentication.
// Only sends the token to allowed hosts to prevent token leakage.
func (d *Downloader) Download(ctx context.Context, rawURL, token string) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if d.allowedHosts != nil && !d.allowedHosts[parsed.Host] {
		return nil, fmt.Errorf("refusing to send token to non-Slack host: %s", parsed.Host)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	if len(data) > MaxFileSize {
		return nil, fmt.Errorf("file too large (max %d bytes)", MaxFileSize)
	}

	return data, nil
}

// Extractor extracts text content from file data based on MIME type.
type Extractor struct{}

// NewExtractor creates a new text extractor.
func NewExtractor() *Extractor {
	return &Extractor{}
}

// IsSupported returns true if the given MIME type can be processed.
func (e *Extractor) IsSupported(mimeType string) bool {
	// Normalize MIME type (strip parameters like charset)
	base := mimeType
	if idx := strings.IndexByte(mimeType, ';'); idx >= 0 {
		base = strings.TrimSpace(mimeType[:idx])
	}

	if SupportedMIME[base] {
		return true
	}

	// Treat text/* and common code types as supported
	if strings.HasPrefix(base, "text/") {
		return true
	}

	return false
}

// Extract extracts text content from file data.
// Returns the text content and any error.
func (e *Extractor) Extract(data []byte, mimeType string) (string, error) {
	base := mimeType
	if idx := strings.IndexByte(mimeType, ';'); idx >= 0 {
		base = strings.TrimSpace(mimeType[:idx])
	}

	switch {
	case strings.HasPrefix(base, "text/"), SupportedMIME[base]:
		return extractText(data), nil
	default:
		return "", fmt.Errorf("unsupported MIME type: %s", base)
	}
}

// extractText converts raw bytes to text, handling basic cleanup.
func extractText(data []byte) string {
	text := string(data)
	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	// Trim trailing whitespace
	text = strings.TrimSpace(text)
	return text
}

// ProcessFile downloads, validates, and extracts text from a Slack file.
// Returns nil if the file type is not supported (not an error).
func ProcessFile(ctx context.Context, downloader *Downloader, extractor *Extractor, fileID, fileName, mimeType, url, token string, size int) (*File, error) {
	if !extractor.IsSupported(mimeType) {
		return nil, nil // Unsupported type, skip silently
	}

	if size > MaxFileSize {
		return nil, nil // Too large, skip silently
	}

	data, err := downloader.Download(ctx, url, token)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", fileName, err)
	}

	content, err := extractor.Extract(data, mimeType)
	if err != nil {
		return nil, fmt.Errorf("extract %s: %w", fileName, err)
	}

	if content == "" {
		return nil, nil // Empty content, skip
	}

	return &File{
		ID:       fileID,
		Name:     fileName,
		MimeType: mimeType,
		Size:     len(data),
		Content:  content,
	}, nil
}
