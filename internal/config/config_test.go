package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test server defaults
	if cfg.Server.HookPort != 9876 {
		t.Errorf("expected HookPort 9876, got %d", cfg.Server.HookPort)
	}
	if cfg.Server.DashboardPort != 9877 {
		t.Errorf("expected DashboardPort 9877, got %d", cfg.Server.DashboardPort)
	}
	if cfg.Server.BindAddress != "127.0.0.1" {
		t.Errorf("expected BindAddress 127.0.0.1, got %s", cfg.Server.BindAddress)
	}

	// Test storage defaults
	if cfg.Storage.RetentionDays != 30 {
		t.Errorf("expected RetentionDays 30, got %d", cfg.Storage.RetentionDays)
	}

	// Test dashboard defaults
	if cfg.Dashboard.RefreshInterval != 30 {
		t.Errorf("expected RefreshInterval 30, got %d", cfg.Dashboard.RefreshInterval)
	}
	if cfg.Dashboard.Theme != "auto" {
		t.Errorf("expected Theme 'auto', got %s", cfg.Dashboard.Theme)
	}

	// Test cost model defaults
	if cfg.Cost.Models.Opus.Input != 15.0 {
		t.Errorf("expected Opus input cost 15.0, got %f", cfg.Cost.Models.Opus.Input)
	}
	if cfg.Cost.Models.Sonnet.Output != 15.0 {
		t.Errorf("expected Sonnet output cost 15.0, got %f", cfg.Cost.Models.Sonnet.Output)
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[server]
hook_port = 8888
dashboard_port = 8889
bind_address = "0.0.0.0"

[storage]
database_path = "/custom/path/data.db"
retention_days = 60

[dashboard]
refresh_interval = 15
theme = "dark"

[cost.models]
[cost.models.opus]
input = 20.0
output = 80.0
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify loaded values
	if cfg.Server.HookPort != 8888 {
		t.Errorf("expected HookPort 8888, got %d", cfg.Server.HookPort)
	}
	if cfg.Server.DashboardPort != 8889 {
		t.Errorf("expected DashboardPort 8889, got %d", cfg.Server.DashboardPort)
	}
	if cfg.Server.BindAddress != "0.0.0.0" {
		t.Errorf("expected BindAddress 0.0.0.0, got %s", cfg.Server.BindAddress)
	}
	if cfg.Storage.DatabasePath != "/custom/path/data.db" {
		t.Errorf("expected custom database path, got %s", cfg.Storage.DatabasePath)
	}
	if cfg.Storage.RetentionDays != 60 {
		t.Errorf("expected RetentionDays 60, got %d", cfg.Storage.RetentionDays)
	}
	if cfg.Dashboard.RefreshInterval != 15 {
		t.Errorf("expected RefreshInterval 15, got %d", cfg.Dashboard.RefreshInterval)
	}
	if cfg.Dashboard.Theme != "dark" {
		t.Errorf("expected Theme 'dark', got %s", cfg.Dashboard.Theme)
	}
	if cfg.Cost.Models.Opus.Input != 20.0 {
		t.Errorf("expected Opus input 20.0, got %f", cfg.Cost.Models.Opus.Input)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("MCP_LENS_HOOK_PORT", "7777")
	os.Setenv("MCP_LENS_DASHBOARD_PORT", "7778")
	os.Setenv("MCP_LENS_BIND_ADDRESS", "192.168.1.1")
	defer func() {
		os.Unsetenv("MCP_LENS_HOOK_PORT")
		os.Unsetenv("MCP_LENS_DASHBOARD_PORT")
		os.Unsetenv("MCP_LENS_BIND_ADDRESS")
	}()

	cfg := DefaultConfig()
	cfg.ApplyEnvOverrides()

	if cfg.Server.HookPort != 7777 {
		t.Errorf("expected HookPort 7777 from env, got %d", cfg.Server.HookPort)
	}
	if cfg.Server.DashboardPort != 7778 {
		t.Errorf("expected DashboardPort 7778 from env, got %d", cfg.Server.DashboardPort)
	}
	if cfg.Server.BindAddress != "192.168.1.1" {
		t.Errorf("expected BindAddress 192.168.1.1 from env, got %s", cfg.Server.BindAddress)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadFromFile("/nonexistent/path/config.toml")
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestDefaultDatabasePath(t *testing.T) {
	cfg := DefaultConfig()

	// Database path should expand to user home directory
	if cfg.Storage.DatabasePath == "" {
		t.Error("expected non-empty default database path")
	}
}

func TestCalculateCost(t *testing.T) {
	cfg := DefaultConfig()

	// Test Opus cost calculation (per 1M tokens)
	// 1000 input tokens at $15/1M = $0.015
	// 2000 output tokens at $75/1M = $0.15
	inputTokens := int64(1000)
	outputTokens := int64(2000)

	cost := cfg.CalculateCost("opus", inputTokens, outputTokens)
	expectedCost := (float64(inputTokens) / 1_000_000 * 15.0) + (float64(outputTokens) / 1_000_000 * 75.0)

	// Use tolerance for floating point comparison
	tolerance := 0.000001
	if diff := cost - expectedCost; diff > tolerance || diff < -tolerance {
		t.Errorf("expected cost %f, got %f", expectedCost, cost)
	}
}

func TestCalculateCostUnknownModel(t *testing.T) {
	cfg := DefaultConfig()

	// Unknown model should return 0
	cost := cfg.CalculateCost("unknown-model", 1000, 1000)
	if cost != 0 {
		t.Errorf("expected 0 for unknown model, got %f", cost)
	}
}
