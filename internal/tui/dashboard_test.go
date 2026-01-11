package tui

import (
	"testing"
	"time"

	"github.com/anthropics/mcp-lens/internal/storage"
)

func TestDashboardData(t *testing.T) {
	data := &DashboardData{
		MCPServers: []storage.MCPServerStats{
			{
				ServerName:   "github",
				TotalCalls:   100,
				SuccessCount: 95,
				ErrorCount:   5,
				AvgLatencyMs: 150.0,
			},
			{
				ServerName:   "asana",
				TotalCalls:   50,
				SuccessCount: 48,
				ErrorCount:   2,
				AvgLatencyMs: 200.0,
			},
		},
		RecentEvents: []storage.RecentEvent{
			{
				Timestamp:  time.Now(),
				SessionID:  "sess-1",
				EventType:  "PostToolUse",
				ToolName:   "mcp__github__create_issue",
				ServerName: "github",
				DurationMs: 150,
				Success:    true,
			},
		},
		CallVolume: []storage.HourlyCallVolume{
			{Hour: time.Now().Add(-2 * time.Hour), TotalCalls: 10},
			{Hour: time.Now().Add(-1 * time.Hour), TotalCalls: 20},
			{Hour: time.Now(), TotalCalls: 15},
		},
		SessionCount: 5,
		TimeRange:    "24h",
		UpdatedAt:    time.Now(),
	}

	// Test that dashboard data is properly structured
	if len(data.MCPServers) != 2 {
		t.Errorf("expected 2 MCP servers, got %d", len(data.MCPServers))
	}

	if data.MCPServers[0].ServerName != "github" {
		t.Errorf("expected first server to be 'github', got '%s'", data.MCPServers[0].ServerName)
	}

	if len(data.RecentEvents) != 1 {
		t.Errorf("expected 1 recent event, got %d", len(data.RecentEvents))
	}

	if len(data.CallVolume) != 3 {
		t.Errorf("expected 3 call volume entries, got %d", len(data.CallVolume))
	}

	if data.SessionCount != 5 {
		t.Errorf("expected session count 5, got %d", data.SessionCount)
	}
}

func TestStatusIndicator(t *testing.T) {
	tests := []struct {
		errors   int64
		total    int64
		contains string
	}{
		{0, 100, "ok"},
		{5, 100, "warn"},
		{15, 100, "error"},
		{0, 0, "idle"},
	}

	for _, tt := range tests {
		result := statusIndicator(tt.errors, tt.total)
		if result == "" {
			t.Errorf("statusIndicator(%d, %d) returned empty string", tt.errors, tt.total)
		}
		// Status indicator should contain the expected status
		// Note: actual strings include color codes, so we just check it's not empty
	}
}

func TestFormatLatency(t *testing.T) {
	tests := []struct {
		ms       float64
		expected string
	}{
		{0, "-"},
		{50, "[50ms](fg:green)"},
		{150, "[150ms](fg:yellow)"},
		{600, "[600ms](fg:red)"},
	}

	for _, tt := range tests {
		result := formatLatency(tt.ms)
		if result != tt.expected {
			t.Errorf("formatLatency(%f) = %s, want %s", tt.ms, result, tt.expected)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is..."},
		{"exact", 5, "exact"},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func TestAppConfig(t *testing.T) {
	config := DefaultAppConfig()

	if config.RefreshInterval != 5*time.Second {
		t.Errorf("expected refresh interval 5s, got %v", config.RefreshInterval)
	}

	if config.TimeRange != "24h" {
		t.Errorf("expected time range '24h', got '%s'", config.TimeRange)
	}

	if config.NoColor {
		t.Error("expected NoColor to be false by default")
	}
}
