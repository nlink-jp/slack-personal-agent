package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.LLM.Backend != "local" {
		t.Errorf("expected default backend 'local', got %q", cfg.LLM.Backend)
	}
	if cfg.Polling.IntervalSec != 120 {
		t.Errorf("expected default interval 120, got %d", cfg.Polling.IntervalSec)
	}
	if cfg.Polling.MaxRatePerMin != 45 {
		t.Errorf("expected default max rate 45, got %d", cfg.Polling.MaxRatePerMin)
	}
	if cfg.Memory.HotToWarmMin != 1440 {
		t.Errorf("expected default hot-to-warm 1440, got %d", cfg.Memory.HotToWarmMin)
	}
	if cfg.Response.TimeoutSec != 120 {
		t.Errorf("expected default timeout 120, got %d", cfg.Response.TimeoutSec)
	}
}

func TestPollingConfigDuration(t *testing.T) {
	p := PollingConfig{IntervalSec: 60, PriorityBoostSec: 10}

	if p.Interval() != 60*time.Second {
		t.Errorf("expected 60s, got %v", p.Interval())
	}
	if p.PriorityBoostInterval() != 10*time.Second {
		t.Errorf("expected 10s, got %v", p.PriorityBoostInterval())
	}
}

func TestMemoryConfigDuration(t *testing.T) {
	m := MemoryConfig{HotToWarmMin: 1440, WarmToColdMin: 10080}

	if m.HotToWarmDuration() != 1440*time.Minute {
		t.Errorf("expected 1440m, got %v", m.HotToWarmDuration())
	}
	if m.WarmToColdDuration() != 10080*time.Minute {
		t.Errorf("expected 10080m, got %v", m.WarmToColdDuration())
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	content := `
[[workspace]]
name = "test-ws"

[[workspace]]
name = "other-ws"

[llm]
backend = "vertex_ai"

[polling]
interval_sec = 60
max_rate_per_min = 30
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(cfg.Workspaces))
	}
	if cfg.Workspaces[0].Name != "test-ws" {
		t.Errorf("expected workspace name 'test-ws', got %q", cfg.Workspaces[0].Name)
	}
	if cfg.Workspaces[1].Name != "other-ws" {
		t.Errorf("expected workspace name 'other-ws', got %q", cfg.Workspaces[1].Name)
	}
	if cfg.LLM.Backend != "vertex_ai" {
		t.Errorf("expected backend 'vertex_ai', got %q", cfg.LLM.Backend)
	}
	if cfg.Polling.IntervalSec != 60 {
		t.Errorf("expected interval 60, got %d", cfg.Polling.IntervalSec)
	}
	if cfg.Polling.MaxRatePerMin != 30 {
		t.Errorf("expected max rate 30, got %d", cfg.Polling.MaxRatePerMin)
	}
	// Defaults preserved for unset fields
	if cfg.LocalLLM.Endpoint != "http://localhost:1234/v1" {
		t.Errorf("expected default local endpoint, got %q", cfg.LocalLLM.Endpoint)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/config.toml")
	if err != nil {
		t.Fatal(err)
	}
	// Should return defaults
	if cfg.LLM.Backend != "local" {
		t.Errorf("expected default backend, got %q", cfg.LLM.Backend)
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("SPA_LLM_BACKEND", "vertex_ai")
	t.Setenv("SPA_POLLING_INTERVAL", "30")

	cfg, err := Load("/nonexistent/config.toml")
	if err != nil {
		t.Fatal(err)
	}

	if cfg.LLM.Backend != "vertex_ai" {
		t.Errorf("expected env override 'vertex_ai', got %q", cfg.LLM.Backend)
	}
	if cfg.Polling.IntervalSec != 30 {
		t.Errorf("expected env override 30, got %d", cfg.Polling.IntervalSec)
	}
}

func TestValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Polling.IntervalSec = 1   // too low
	cfg.Polling.MaxRatePerMin = 99 // too high
	cfg.Memory.HotToWarmMin = 5    // too low

	validate(cfg)

	if cfg.Polling.IntervalSec != 120 {
		t.Errorf("expected validated interval 120, got %d", cfg.Polling.IntervalSec)
	}
	if cfg.Polling.MaxRatePerMin != 45 {
		t.Errorf("expected validated max rate 45, got %d", cfg.Polling.MaxRatePerMin)
	}
	if cfg.Memory.HotToWarmMin != 1440 {
		t.Errorf("expected validated hot-to-warm 1440, got %d", cfg.Memory.HotToWarmMin)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.toml")

	cfg := DefaultConfig()
	cfg.Workspaces = []WorkspaceConfig{
		{Name: "saved-ws"},
	}
	cfg.LLM.Backend = "vertex_ai"

	if err := Save(cfg, path); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded.Workspaces) != 1 || loaded.Workspaces[0].Name != "saved-ws" {
		t.Errorf("expected saved workspace, got %+v", loaded.Workspaces)
	}
	if loaded.LLM.Backend != "vertex_ai" {
		t.Errorf("expected saved backend, got %q", loaded.LLM.Backend)
	}
}
