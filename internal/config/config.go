// Package config provides configuration management for MCP Lens.
package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Config represents the complete MCP Lens configuration.
type Config struct {
	Server    ServerConfig    `toml:"server"`
	Storage   StorageConfig   `toml:"storage"`
	Dashboard DashboardConfig `toml:"dashboard"`
	TUI       TUIConfig       `toml:"tui"`
	Cost      CostConfig      `toml:"cost"`
	Alerts    AlertsConfig    `toml:"alerts"`
}

// ServerConfig configures the HTTP servers.
type ServerConfig struct {
	HookPort      int    `toml:"hook_port"`
	DashboardPort int    `toml:"dashboard_port"`
	BindAddress   string `toml:"bind_address"`
}

// StorageConfig configures data storage.
type StorageConfig struct {
	DataDir       string `toml:"data_dir"`
	DatabasePath  string `toml:"database_path"`
	EventsFile    string `toml:"events_file"`
	RetentionDays int    `toml:"retention_days"`
}

// DashboardConfig configures the web dashboard.
type DashboardConfig struct {
	RefreshInterval int    `toml:"refresh_interval"`
	Theme           string `toml:"theme"`
}

// TUIConfig configures the terminal user interface.
type TUIConfig struct {
	RefreshInterval time.Duration `toml:"refresh_interval"`
	DefaultRange    string        `toml:"default_range"`
}

// CostConfig configures cost calculation.
type CostConfig struct {
	Models ModelPricing `toml:"models"`
}

// ModelPricing defines per-model pricing.
type ModelPricing struct {
	Opus   TokenPricing `toml:"opus"`
	Sonnet TokenPricing `toml:"sonnet"`
	Haiku  TokenPricing `toml:"haiku"`
}

// TokenPricing defines input/output token costs per 1M tokens.
type TokenPricing struct {
	Input  float64 `toml:"input"`
	Output float64 `toml:"output"`
}

// AlertsConfig configures budget alerts.
type AlertsConfig struct {
	Enabled       bool    `toml:"enabled"`
	BudgetDaily   float64 `toml:"budget_daily"`
	BudgetWeekly  float64 `toml:"budget_weekly"`
	BudgetMonthly float64 `toml:"budget_monthly"`
	WebhookURL    string  `toml:"webhook_url"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".mcp-lens")
	defaultDBPath := filepath.Join(dataDir, "data.db")
	eventsFile := filepath.Join(dataDir, "events.jsonl")

	return &Config{
		Server: ServerConfig{
			HookPort:      9876,
			DashboardPort: 9877,
			BindAddress:   "127.0.0.1",
		},
		Storage: StorageConfig{
			DataDir:       dataDir,
			DatabasePath:  defaultDBPath,
			EventsFile:    eventsFile,
			RetentionDays: 30,
		},
		Dashboard: DashboardConfig{
			RefreshInterval: 30,
			Theme:           "auto",
		},
		TUI: TUIConfig{
			RefreshInterval: 5 * time.Second,
			DefaultRange:    "24h",
		},
		Cost: CostConfig{
			Models: ModelPricing{
				Opus: TokenPricing{
					Input:  15.0,
					Output: 75.0,
				},
				Sonnet: TokenPricing{
					Input:  3.0,
					Output: 15.0,
				},
				Haiku: TokenPricing{
					Input:  0.25,
					Output: 1.25,
				},
			},
		},
		Alerts: AlertsConfig{
			Enabled: false,
		},
	}
}

// LoadFromFile loads configuration from a TOML file.
func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Start with defaults
	cfg := DefaultConfig()

	// Unmarshal over defaults
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Load loads configuration from the default location or environment.
func Load() (*Config, error) {
	// Check for config file path in environment
	configPath := os.Getenv("MCP_LENS_CONFIG")
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return DefaultConfig(), nil
		}
		configPath = filepath.Join(homeDir, ".config", "mcp-lens", "config.toml")
	}

	// Try to load from file
	cfg, err := LoadFromFile(configPath)
	if err != nil {
		// If file doesn't exist, use defaults
		if os.IsNotExist(err) {
			cfg = DefaultConfig()
		} else {
			return nil, err
		}
	}

	// Apply environment overrides
	cfg.ApplyEnvOverrides()

	return cfg, nil
}

// ApplyEnvOverrides applies environment variable overrides to the config.
func (c *Config) ApplyEnvOverrides() {
	// Server overrides
	if v := os.Getenv("MCP_LENS_HOOK_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Server.HookPort = port
		}
	}
	if v := os.Getenv("MCP_LENS_DASHBOARD_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Server.DashboardPort = port
		}
	}
	if v := os.Getenv("MCP_LENS_BIND_ADDRESS"); v != "" {
		c.Server.BindAddress = v
	}

	// Storage overrides
	if v := os.Getenv("MCP_LENS_DATABASE_PATH"); v != "" {
		c.Storage.DatabasePath = v
	}
	if v := os.Getenv("MCP_LENS_RETENTION_DAYS"); v != "" {
		if days, err := strconv.Atoi(v); err == nil {
			c.Storage.RetentionDays = days
		}
	}

	// Dashboard overrides
	if v := os.Getenv("MCP_LENS_REFRESH_INTERVAL"); v != "" {
		if interval, err := strconv.Atoi(v); err == nil {
			c.Dashboard.RefreshInterval = interval
		}
	}
	if v := os.Getenv("MCP_LENS_THEME"); v != "" {
		c.Dashboard.Theme = v
	}
}

// CalculateCost calculates the cost for a given model and token counts.
// Returns the cost in USD.
func (c *Config) CalculateCost(model string, inputTokens, outputTokens int64) float64 {
	var pricing TokenPricing

	switch strings.ToLower(model) {
	case "opus", "claude-opus-4", "claude-opus-4-20250514":
		pricing = c.Cost.Models.Opus
	case "sonnet", "claude-sonnet-4", "claude-sonnet-4-20250514":
		pricing = c.Cost.Models.Sonnet
	case "haiku", "claude-haiku-3-5", "claude-3-5-haiku-20241022":
		pricing = c.Cost.Models.Haiku
	default:
		return 0
	}

	inputCost := float64(inputTokens) / 1_000_000 * pricing.Input
	outputCost := float64(outputTokens) / 1_000_000 * pricing.Output

	return inputCost + outputCost
}

// HookAddress returns the full hook receiver address.
func (c *Config) HookAddress() string {
	return c.Server.BindAddress + ":" + strconv.Itoa(c.Server.HookPort)
}

// DashboardAddress returns the full dashboard address.
func (c *Config) DashboardAddress() string {
	return c.Server.BindAddress + ":" + strconv.Itoa(c.Server.DashboardPort)
}

// EnsureDataDir creates the data directory if it doesn't exist.
func (c *Config) EnsureDataDir() error {
	dir := filepath.Dir(c.Storage.DatabasePath)
	return os.MkdirAll(dir, 0755)
}
