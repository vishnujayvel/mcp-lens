package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/mcp-lens/internal/storage"
)

// mockUtilizationStore implements UtilizationStore for testing.
type mockUtilizationStore struct {
	stats []storage.MCPServerStats
	err   error
}

func (m *mockUtilizationStore) GetMCPServerStats(ctx context.Context, filter storage.TimeFilter) ([]storage.MCPServerStats, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.stats, nil
}

func TestUtilizationAnalyzer_AnalyzeUtilization(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		stats          []storage.MCPServerStats
		expectedLen    int
		expectedFirst  string // Expected first server (highest utilization)
		expectedHealth HealthStatus
	}{
		{
			name:        "empty stats",
			stats:       []storage.MCPServerStats{},
			expectedLen: 0,
		},
		{
			name: "single server healthy",
			stats: []storage.MCPServerStats{
				{
					ServerName:   "playwright",
					TotalCalls:   100,
					ErrorCount:   1,
					AvgLatencyMs: 200,
					LastUsedAt:   now,
				},
			},
			expectedLen:    1,
			expectedFirst:  "playwright",
			expectedHealth: HealthGood,
		},
		{
			name: "multiple servers sorted by utilization",
			stats: []storage.MCPServerStats{
				{
					ServerName:   "low-use",
					TotalCalls:   10,
					ErrorCount:   0,
					AvgLatencyMs: 100,
					LastUsedAt:   now,
				},
				{
					ServerName:   "high-use",
					TotalCalls:   90,
					ErrorCount:   0,
					AvgLatencyMs: 100,
					LastUsedAt:   now,
				},
			},
			expectedLen:    2,
			expectedFirst:  "high-use", // 90% utilization
			expectedHealth: HealthGood,
		},
		{
			name: "high error rate triggers critical",
			stats: []storage.MCPServerStats{
				{
					ServerName:   "buggy-server",
					TotalCalls:   100,
					ErrorCount:   15, // 15% error rate
					AvgLatencyMs: 200,
					LastUsedAt:   now,
				},
			},
			expectedLen:    1,
			expectedFirst:  "buggy-server",
			expectedHealth: HealthCritical,
		},
		{
			name: "high latency triggers critical",
			stats: []storage.MCPServerStats{
				{
					ServerName:   "slow-server",
					TotalCalls:   100,
					ErrorCount:   0,
					AvgLatencyMs: 1500, // 1.5 seconds
					LastUsedAt:   now,
				},
			},
			expectedLen:    1,
			expectedFirst:  "slow-server",
			expectedHealth: HealthCritical,
		},
		{
			name: "warning threshold",
			stats: []storage.MCPServerStats{
				{
					ServerName:   "warning-server",
					TotalCalls:   100,
					ErrorCount:   6, // 6% error rate (above 5%, below 10%)
					AvgLatencyMs: 200,
					LastUsedAt:   now,
				},
			},
			expectedLen:    1,
			expectedFirst:  "warning-server",
			expectedHealth: HealthWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockUtilizationStore{stats: tt.stats}
			analyzer := NewUtilizationAnalyzer(store, DefaultUtilizationConfig())

			result, err := analyzer.AnalyzeUtilization(context.Background(), storage.TimeFilter{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != tt.expectedLen {
				t.Errorf("expected %d results, got %d", tt.expectedLen, len(result))
			}

			if tt.expectedLen > 0 {
				if result[0].ServerName != tt.expectedFirst {
					t.Errorf("expected first server %s, got %s", tt.expectedFirst, result[0].ServerName)
				}
				if result[0].HealthStatus != tt.expectedHealth {
					t.Errorf("expected health %s, got %s", tt.expectedHealth, result[0].HealthStatus)
				}
			}
		})
	}
}

func TestUtilizationAnalyzer_GetUnusedServers(t *testing.T) {
	now := time.Now()
	oldTime := now.AddDate(0, 0, -10) // 10 days ago

	tests := []struct {
		name        string
		stats       []storage.MCPServerStats
		expectedLen int
	}{
		{
			name:        "no servers",
			stats:       []storage.MCPServerStats{},
			expectedLen: 0,
		},
		{
			name: "no unused servers",
			stats: []storage.MCPServerStats{
				{
					ServerName: "active",
					TotalCalls: 100,
					LastUsedAt: now,
				},
			},
			expectedLen: 0,
		},
		{
			name: "one unused server",
			stats: []storage.MCPServerStats{
				{
					ServerName: "active",
					TotalCalls: 100,
					LastUsedAt: now,
				},
				{
					ServerName: "stale",
					TotalCalls: 10,
					LastUsedAt: oldTime,
				},
			},
			expectedLen: 1,
		},
		{
			name: "server with zero time is unused",
			stats: []storage.MCPServerStats{
				{
					ServerName: "never-used",
					TotalCalls: 0,
					LastUsedAt: time.Time{}, // Zero time
				},
			},
			expectedLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockUtilizationStore{stats: tt.stats}
			analyzer := NewUtilizationAnalyzer(store, DefaultUtilizationConfig())

			result, err := analyzer.GetUnusedServers(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != tt.expectedLen {
				t.Errorf("expected %d unused servers, got %d", tt.expectedLen, len(result))
			}
		})
	}
}

func TestUtilizationAnalyzer_GetHighErrorServers(t *testing.T) {
	now := time.Now()

	store := &mockUtilizationStore{
		stats: []storage.MCPServerStats{
			{
				ServerName:   "healthy",
				TotalCalls:   100,
				ErrorCount:   2, // 2% error rate
				AvgLatencyMs: 100,
				LastUsedAt:   now,
			},
			{
				ServerName:   "problematic",
				TotalCalls:   100,
				ErrorCount:   15, // 15% error rate
				AvgLatencyMs: 100,
				LastUsedAt:   now,
			},
		},
	}

	analyzer := NewUtilizationAnalyzer(store, DefaultUtilizationConfig())

	result, err := analyzer.GetHighErrorServers(context.Background(), storage.TimeFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 high error server, got %d", len(result))
	}

	if len(result) > 0 && result[0].ServerName != "problematic" {
		t.Errorf("expected 'problematic' server, got %s", result[0].ServerName)
	}
}

func TestUtilizationAnalyzer_GetSlowServers(t *testing.T) {
	now := time.Now()

	store := &mockUtilizationStore{
		stats: []storage.MCPServerStats{
			{
				ServerName:   "fast",
				TotalCalls:   100,
				ErrorCount:   0,
				AvgLatencyMs: 200,
				LastUsedAt:   now,
			},
			{
				ServerName:   "slow",
				TotalCalls:   100,
				ErrorCount:   0,
				AvgLatencyMs: 1500, // 1.5 seconds
				LastUsedAt:   now,
			},
		},
	}

	analyzer := NewUtilizationAnalyzer(store, DefaultUtilizationConfig())

	result, err := analyzer.GetSlowServers(context.Background(), storage.TimeFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 slow server, got %d", len(result))
	}

	if len(result) > 0 && result[0].ServerName != "slow" {
		t.Errorf("expected 'slow' server, got %s", result[0].ServerName)
	}
}

func TestUtilizationAnalyzer_GetSummary(t *testing.T) {
	now := time.Now()
	oldTime := now.AddDate(0, 0, -10)

	store := &mockUtilizationStore{
		stats: []storage.MCPServerStats{
			{
				ServerName:   "healthy",
				TotalCalls:   100,
				ErrorCount:   1,
				AvgLatencyMs: 100,
				LastUsedAt:   now,
			},
			{
				ServerName:   "warning",
				TotalCalls:   100,
				ErrorCount:   6,
				AvgLatencyMs: 100,
				LastUsedAt:   now,
			},
			{
				ServerName:   "critical",
				TotalCalls:   100,
				ErrorCount:   15,
				AvgLatencyMs: 100,
				LastUsedAt:   now,
			},
			{
				ServerName:   "unused",
				TotalCalls:   10,
				ErrorCount:   0,
				AvgLatencyMs: 100,
				LastUsedAt:   oldTime,
			},
		},
	}

	analyzer := NewUtilizationAnalyzer(store, DefaultUtilizationConfig())

	summary, err := analyzer.GetSummary(context.Background(), storage.TimeFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary.TotalServers != 4 {
		t.Errorf("expected 4 total servers, got %d", summary.TotalServers)
	}

	if summary.HealthyServers != 1 {
		t.Errorf("expected 1 healthy server, got %d", summary.HealthyServers)
	}

	if summary.WarningServers != 1 {
		t.Errorf("expected 1 warning server, got %d", summary.WarningServers)
	}

	if summary.CriticalServers != 1 {
		t.Errorf("expected 1 critical server, got %d", summary.CriticalServers)
	}

	if summary.UnusedServers != 1 {
		t.Errorf("expected 1 unused server, got %d", summary.UnusedServers)
	}

	if summary.TotalCalls != 310 {
		t.Errorf("expected 310 total calls, got %d", summary.TotalCalls)
	}
}

func TestUtilizationPercentageCalculation(t *testing.T) {
	now := time.Now()

	store := &mockUtilizationStore{
		stats: []storage.MCPServerStats{
			{
				ServerName:   "server-a",
				TotalCalls:   75,
				ErrorCount:   0,
				AvgLatencyMs: 100,
				LastUsedAt:   now,
			},
			{
				ServerName:   "server-b",
				TotalCalls:   25,
				ErrorCount:   0,
				AvgLatencyMs: 100,
				LastUsedAt:   now,
			},
		},
	}

	analyzer := NewUtilizationAnalyzer(store, DefaultUtilizationConfig())

	result, err := analyzer.AnalyzeUtilization(context.Background(), storage.TimeFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Server-a should have 75% utilization
	if result[0].ServerName != "server-a" {
		t.Errorf("expected server-a first, got %s", result[0].ServerName)
	}

	if result[0].UtilizationPct != 75.0 {
		t.Errorf("expected 75%% utilization, got %.2f%%", result[0].UtilizationPct)
	}

	// Server-b should have 25% utilization
	if result[1].UtilizationPct != 25.0 {
		t.Errorf("expected 25%% utilization, got %.2f%%", result[1].UtilizationPct)
	}
}
