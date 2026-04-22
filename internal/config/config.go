// Package config manages TOML-based application configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
)

// Config holds all application configuration.
type Config struct {
	Workspaces []WorkspaceConfig `toml:"workspace" json:"workspaces"`
	LLM        LLMConfig         `toml:"llm" json:"llm"`
	VertexAI   VertexAIConfig    `toml:"vertex_ai" json:"vertex_ai"`
	LocalLLM   LocalLLMConfig    `toml:"local_llm" json:"local_llm"`
	Embedding  EmbeddingConfig   `toml:"embedding" json:"embedding"`
	Polling    PollingConfig     `toml:"polling" json:"polling"`
	Memory     MemoryConfig      `toml:"memory" json:"memory"`
	Response   ResponseConfig    `toml:"response" json:"response"`
	Window     WindowConfig      `toml:"window" json:"window"`
	Theme      string            `toml:"theme" json:"theme"`
}

// WorkspaceConfig defines a Slack workspace.
// Tokens are stored in macOS Keychain, not here.
type WorkspaceConfig struct {
	Name string `toml:"name" json:"name"`
}

// LLMConfig selects the active LLM backend.
type LLMConfig struct {
	Backend string `toml:"backend" json:"backend"` // "local" or "vertex_ai"
}

// VertexAIConfig holds Vertex AI (Gemini) settings.
type VertexAIConfig struct {
	Project string `toml:"project" json:"project"`
	Region  string `toml:"region" json:"region"`
	Model   string `toml:"model" json:"model"`
}

// LocalLLMConfig holds OpenAI-compatible local LLM settings.
type LocalLLMConfig struct {
	Endpoint string `toml:"endpoint" json:"endpoint"`
	Model    string `toml:"model" json:"model"`
	APIKey   string `toml:"api_key" json:"api_key"`
}

// EmbeddingConfig selects the embedding backend.
// Embedding is independent of the LLM backend to avoid re-indexing on LLM switch.
type EmbeddingConfig struct {
	Backend  string              `toml:"backend" json:"backend"` // "builtin", "local", or "vertex_ai"
	Local    EmbeddingLocalConfig    `toml:"local" json:"local"`
	VertexAI EmbeddingVertexAIConfig `toml:"vertex_ai" json:"vertex_ai"`
}

// EmbeddingLocalConfig holds settings for OpenAI-compatible embedding endpoint.
type EmbeddingLocalConfig struct {
	Endpoint string `toml:"endpoint" json:"endpoint"`
	Model    string `toml:"model" json:"model"`
	APIKey   string `toml:"api_key" json:"api_key"`
}

// EmbeddingVertexAIConfig holds settings for Vertex AI embedding.
type EmbeddingVertexAIConfig struct {
	Model string `toml:"model" json:"model"`
}

// PollingConfig controls Slack API polling behavior.
type PollingConfig struct {
	IntervalSec      int `toml:"interval_sec" json:"interval_sec"`
	PriorityBoostSec int `toml:"priority_boost_sec" json:"priority_boost_sec"`
	MaxRatePerMin    int `toml:"max_rate_per_min" json:"max_rate_per_min"`
}

// Interval returns the polling interval as a time.Duration.
func (p PollingConfig) Interval() time.Duration {
	return time.Duration(p.IntervalSec) * time.Second
}

// PriorityBoostInterval returns the boosted polling interval as a time.Duration.
func (p PollingConfig) PriorityBoostInterval() time.Duration {
	return time.Duration(p.PriorityBoostSec) * time.Second
}

// MemoryConfig controls the 3-tier lifecycle thresholds.
type MemoryConfig struct {
	HotToWarmMin  int `toml:"hot_to_warm_min" json:"hot_to_warm_min"`
	WarmToColdMin int `toml:"warm_to_cold_min" json:"warm_to_cold_min"`
}

// HotToWarmDuration returns the Hot→Warm threshold as a time.Duration.
func (m MemoryConfig) HotToWarmDuration() time.Duration {
	return time.Duration(m.HotToWarmMin) * time.Minute
}

// WarmToColdDuration returns the Warm→Cold threshold as a time.Duration.
func (m MemoryConfig) WarmToColdDuration() time.Duration {
	return time.Duration(m.WarmToColdMin) * time.Minute
}

// ResponseConfig controls the MITL proxy response behavior.
type ResponseConfig struct {
	TimeoutSec int `toml:"timeout_sec" json:"timeout_sec"`
}

// Timeout returns the response approval timeout as a time.Duration.
func (r ResponseConfig) Timeout() time.Duration {
	return time.Duration(r.TimeoutSec) * time.Second
}

// WindowConfig holds window position and size.
type WindowConfig struct {
	X      int `toml:"x" json:"x"`
	Y      int `toml:"y" json:"y"`
	Width  int `toml:"width" json:"width"`
	Height int `toml:"height" json:"height"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Backend: "local",
		},
		VertexAI: VertexAIConfig{
			Region: "us-central1",
			Model:  "gemini-2.5-flash",
		},
		LocalLLM: LocalLLMConfig{
			Endpoint: "http://localhost:1234/v1",
			Model:    "google/gemma-4-26b-a4b",
		},
		Embedding: EmbeddingConfig{
			Backend: "builtin",
			Local: EmbeddingLocalConfig{
				Endpoint: "http://localhost:1234/v1",
				Model:    "text-embedding-nomic-embed-text-v1.5",
			},
			VertexAI: EmbeddingVertexAIConfig{
				Model: "text-embedding-005",
			},
		},
		Polling: PollingConfig{
			IntervalSec:      120,
			PriorityBoostSec: 15,
			MaxRatePerMin:    45,
		},
		Memory: MemoryConfig{
			HotToWarmMin:  1440, // 24 hours
			WarmToColdMin: 10080, // 7 days
		},
		Response: ResponseConfig{
			TimeoutSec: 120,
		},
		Window: WindowConfig{
			Width:  1280,
			Height: 800,
		},
		Theme: "dark",
	}
}

// Load reads config from the given path, falling back to defaults.
// Environment variables (SPA_*) override file values.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if _, err := os.Stat(path); err == nil {
		if _, err := toml.DecodeFile(path, cfg); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
	}

	applyEnvOverrides(cfg)
	validate(cfg)
	return cfg, nil
}

// Save writes the config to the given path, creating directories as needed.
func Save(cfg *Config, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

// DefaultConfigPath returns the platform-specific config file path.
func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "slack-personal-agent", "config.toml")
}

// DefaultDataDir returns the platform-specific data directory.
func DefaultDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "slack-personal-agent")
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SPA_LLM_BACKEND"); v != "" {
		cfg.LLM.Backend = v
	}
	if v := os.Getenv("SPA_VERTEX_PROJECT"); v != "" {
		cfg.VertexAI.Project = v
	}
	if v := os.Getenv("SPA_VERTEX_REGION"); v != "" {
		cfg.VertexAI.Region = v
	}
	if v := os.Getenv("SPA_VERTEX_MODEL"); v != "" {
		cfg.VertexAI.Model = v
	}
	if v := os.Getenv("SPA_LOCAL_ENDPOINT"); v != "" {
		cfg.LocalLLM.Endpoint = v
	}
	if v := os.Getenv("SPA_LOCAL_MODEL"); v != "" {
		cfg.LocalLLM.Model = v
	}
	if v := os.Getenv("SPA_LOCAL_API_KEY"); v != "" {
		cfg.LocalLLM.APIKey = v
	}
	if v := os.Getenv("SPA_EMBEDDING_BACKEND"); v != "" {
		cfg.Embedding.Backend = v
	}
	if v := os.Getenv("SPA_EMBEDDING_LOCAL_ENDPOINT"); v != "" {
		cfg.Embedding.Local.Endpoint = v
	}
	if v := os.Getenv("SPA_EMBEDDING_LOCAL_MODEL"); v != "" {
		cfg.Embedding.Local.Model = v
	}
	if v := os.Getenv("SPA_POLLING_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Polling.IntervalSec = n
		}
	}
	if v := os.Getenv("SPA_MAX_RATE_PER_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Polling.MaxRatePerMin = n
		}
	}
}

func validate(cfg *Config) {
	if cfg.Polling.IntervalSec < 10 {
		cfg.Polling.IntervalSec = 120
	}
	if cfg.Polling.PriorityBoostSec < 5 {
		cfg.Polling.PriorityBoostSec = 15
	}
	if cfg.Polling.MaxRatePerMin < 1 || cfg.Polling.MaxRatePerMin > 50 {
		cfg.Polling.MaxRatePerMin = 45
	}
	if cfg.Memory.HotToWarmMin < 60 {
		cfg.Memory.HotToWarmMin = 1440
	}
	if cfg.Memory.WarmToColdMin < 60 {
		cfg.Memory.WarmToColdMin = 10080
	}
	if cfg.Response.TimeoutSec < 10 {
		cfg.Response.TimeoutSec = 120
	}
}
