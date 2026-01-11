package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/mcp-lens/internal/storage"
)

// mockErrorAnalyzerStore implements ErrorAnalyzerStore for testing.
type mockErrorAnalyzerStore struct {
	serverStats  []storage.MCPServerStats
	toolStats    []storage.ToolStats
	recentEvents []storage.RecentEvent
	err          error
}

func (m *mockErrorAnalyzerStore) GetMCPServerStats(ctx context.Context, filter storage.TimeFilter) ([]storage.MCPServerStats, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.serverStats, nil
}

func (m *mockErrorAnalyzerStore) GetToolStats(ctx context.Context, filter storage.TimeFilter) ([]storage.ToolStats, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.toolStats, nil
}

func (m *mockErrorAnalyzerStore) GetRecentEvents(ctx context.Context, limit int) ([]storage.RecentEvent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.recentEvents, nil
}

func TestErrorAnalyzer_AnalyzeErrors(t *testing.T) {
	tests := []struct {
		name              string
		serverStats       []storage.MCPServerStats
		toolStats         []storage.ToolStats
		expectedLen       int
		expectedFirst     string        // Expected first server (highest error rate)
		expectedSeverity  ErrorSeverity
		expectedErrorRate float64
	}{
		{
			name:        "no servers",
			serverStats: []storage.MCPServerStats{},
			expectedLen: 0,
		},
		{
			name: "low error rate",
			serverStats: []storage.MCPServerStats{
				{
					ServerName: "healthy",
					TotalCalls: 100,
					ErrorCount: 1, // 1% error rate
				},
			},
			expectedLen:       1,
			expectedFirst:     "healthy",
			expectedSeverity:  ErrorSeverityLow,
			expectedErrorRate: 1.0,
		},
		{
			name: "medium error rate",
			serverStats: []storage.MCPServerStats{
				{
					ServerName: "medium",
					TotalCalls: 100,
					ErrorCount: 3, // 3% error rate
				},
			},
			expectedLen:       1,
			expectedFirst:     "medium",
			expectedSeverity:  ErrorSeverityMedium,
			expectedErrorRate: 3.0,
		},
		{
			name: "high error rate",
			serverStats: []storage.MCPServerStats{
				{
					ServerName: "high",
					TotalCalls: 100,
					ErrorCount: 7, // 7% error rate
				},
			},
			expectedLen:       1,
			expectedFirst:     "high",
			expectedSeverity:  ErrorSeverityHigh,
			expectedErrorRate: 7.0,
		},
		{
			name: "critical error rate",
			serverStats: []storage.MCPServerStats{
				{
					ServerName: "critical",
					TotalCalls: 100,
					ErrorCount: 15, // 15% error rate
				},
			},
			expectedLen:       1,
			expectedFirst:     "critical",
			expectedSeverity:  ErrorSeverityCritical,
			expectedErrorRate: 15.0,
		},
		{
			name: "multiple servers sorted by error rate",
			serverStats: []storage.MCPServerStats{
				{
					ServerName: "low",
					TotalCalls: 100,
					ErrorCount: 1,
				},
				{
					ServerName: "high",
					TotalCalls: 100,
					ErrorCount: 20,
				},
				{
					ServerName: "medium",
					TotalCalls: 100,
					ErrorCount: 5,
				},
			},
			expectedLen:       3,
			expectedFirst:     "high", // 20% highest
			expectedSeverity:  ErrorSeverityCritical,
			expectedErrorRate: 20.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &mockErrorAnalyzerStore{
				serverStats: tt.serverStats,
				toolStats:   tt.toolStats,
			}
			analyzer := NewErrorAnalyzer(store, DefaultErrorAnalyzerConfig())

			result, err := analyzer.AnalyzeErrors(context.Background(), storage.TimeFilter{})
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
				if result[0].SeverityLevel != tt.expectedSeverity {
					t.Errorf("expected severity %s, got %s", tt.expectedSeverity, result[0].SeverityLevel)
				}
				// Use tolerance for floating point comparison
				if diff := result[0].ErrorRate - tt.expectedErrorRate; diff > 0.01 || diff < -0.01 {
					t.Errorf("expected error rate %.2f, got %.2f", tt.expectedErrorRate, result[0].ErrorRate)
				}
			}
		})
	}
}

func TestErrorAnalyzer_MostFailedTools(t *testing.T) {
	store := &mockErrorAnalyzerStore{
		serverStats: []storage.MCPServerStats{
			{
				ServerName: "playwright",
				TotalCalls: 100,
				ErrorCount: 10,
			},
		},
		toolStats: []storage.ToolStats{
			{
				MCPServer:  "playwright",
				ToolName:   "click",
				TotalCalls: 50,
				ErrorCount: 5,
			},
			{
				MCPServer:  "playwright",
				ToolName:   "navigate",
				TotalCalls: 30,
				ErrorCount: 3,
			},
			{
				MCPServer:  "playwright",
				ToolName:   "screenshot",
				TotalCalls: 20,
				ErrorCount: 2,
			},
			{
				MCPServer:  "other-server",
				ToolName:   "other-tool",
				TotalCalls: 100,
				ErrorCount: 50,
			},
		},
	}

	analyzer := NewErrorAnalyzer(store, DefaultErrorAnalyzerConfig())

	result, err := analyzer.AnalyzeErrors(context.Background(), storage.TimeFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}

	if len(result[0].MostFailedTools) != 3 {
		t.Errorf("expected 3 failed tools, got %d", len(result[0].MostFailedTools))
	}

	// Should be sorted by error count (highest first)
	if result[0].MostFailedTools[0].ToolName != "click" {
		t.Errorf("expected first failed tool 'click', got %s", result[0].MostFailedTools[0].ToolName)
	}

	if result[0].MostFailedTools[0].ErrorCount != 5 {
		t.Errorf("expected 5 errors, got %d", result[0].MostFailedTools[0].ErrorCount)
	}

	// Verify error rate calculation
	expectedRate := float64(5) / float64(50) * 100 // 10%
	if result[0].MostFailedTools[0].ErrorRate != expectedRate {
		t.Errorf("expected error rate %.2f%%, got %.2f%%", expectedRate, result[0].MostFailedTools[0].ErrorRate)
	}
}

func TestErrorAnalyzer_GetCriticalServers(t *testing.T) {
	store := &mockErrorAnalyzerStore{
		serverStats: []storage.MCPServerStats{
			{
				ServerName: "healthy",
				TotalCalls: 100,
				ErrorCount: 1, // 1%
			},
			{
				ServerName: "critical-1",
				TotalCalls: 100,
				ErrorCount: 15, // 15%
			},
			{
				ServerName: "warning",
				TotalCalls: 100,
				ErrorCount: 5, // 5%
			},
			{
				ServerName: "critical-2",
				TotalCalls: 100,
				ErrorCount: 20, // 20%
			},
		},
	}

	analyzer := NewErrorAnalyzer(store, DefaultErrorAnalyzerConfig())

	result, err := analyzer.GetCriticalServers(context.Background(), storage.TimeFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 critical servers, got %d", len(result))
	}

	// Should be sorted by error rate (critical-2 first)
	if result[0].ServerName != "critical-2" {
		t.Errorf("expected critical-2 first, got %s", result[0].ServerName)
	}
}

func TestErrorAnalyzer_GetErrorTotals(t *testing.T) {
	store := &mockErrorAnalyzerStore{
		serverStats: []storage.MCPServerStats{
			{
				ServerName: "server-1",
				TotalCalls: 100,
				ErrorCount: 5, // 5% - medium
			},
			{
				ServerName: "server-2",
				TotalCalls: 200,
				ErrorCount: 30, // 15% - critical
			},
			{
				ServerName: "server-3",
				TotalCalls: 100,
				ErrorCount: 8, // 8% - high
			},
		},
	}

	analyzer := NewErrorAnalyzer(store, DefaultErrorAnalyzerConfig())

	totals, err := analyzer.GetErrorTotals(context.Background(), storage.TimeFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if totals.ServerCount != 3 {
		t.Errorf("expected 3 servers, got %d", totals.ServerCount)
	}

	if totals.TotalCalls != 400 {
		t.Errorf("expected 400 total calls, got %d", totals.TotalCalls)
	}

	if totals.TotalErrors != 43 {
		t.Errorf("expected 43 total errors, got %d", totals.TotalErrors)
	}

	expectedOverallRate := float64(43) / float64(400) * 100
	if totals.OverallErrorRate != expectedOverallRate {
		t.Errorf("expected overall error rate %.2f%%, got %.2f%%", expectedOverallRate, totals.OverallErrorRate)
	}

	if totals.CriticalServers != 1 {
		t.Errorf("expected 1 critical server, got %d", totals.CriticalServers)
	}

	// server-1 (5%) and server-3 (8%) are both "high" (>= 5%, < 10%)
	if totals.HighErrorServers != 2 {
		t.Errorf("expected 2 high error servers, got %d", totals.HighErrorServers)
	}

	// No servers in medium range (2-5%)
	if totals.MediumErrorServers != 0 {
		t.Errorf("expected 0 medium error servers, got %d", totals.MediumErrorServers)
	}
}

func TestErrorAnalyzer_RecentErrors(t *testing.T) {
	now := time.Now()

	store := &mockErrorAnalyzerStore{
		serverStats: []storage.MCPServerStats{
			{
				ServerName: "test-server",
				TotalCalls: 100,
				ErrorCount: 10,
			},
		},
		recentEvents: []storage.RecentEvent{
			{
				Timestamp:  now,
				SessionID:  "session-1",
				ServerName: "test-server",
				ToolName:   "tool-a",
				Success:    true, // Not an error
				DurationMs: 100,
			},
			{
				Timestamp:  now.Add(-time.Minute),
				SessionID:  "session-1",
				ServerName: "test-server",
				ToolName:   "tool-b",
				Success:    false, // Error
				DurationMs: 200,
			},
			{
				Timestamp:  now.Add(-2 * time.Minute),
				SessionID:  "session-2",
				ServerName: "test-server",
				ToolName:   "tool-c",
				Success:    false, // Error
				DurationMs: 300,
			},
		},
	}

	analyzer := NewErrorAnalyzer(store, DefaultErrorAnalyzerConfig())

	errors, err := analyzer.GetRecentErrors(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only return the 2 errors
	if len(errors) != 2 {
		t.Errorf("expected 2 recent errors, got %d", len(errors))
	}

	if errors[0].ToolName != "tool-b" {
		t.Errorf("expected first error tool 'tool-b', got %s", errors[0].ToolName)
	}
}

func TestErrorAnalyzer_CountRecentErrors(t *testing.T) {
	now := time.Now()

	store := &mockErrorAnalyzerStore{
		serverStats: []storage.MCPServerStats{
			{
				ServerName: "server-1",
				TotalCalls: 100,
				ErrorCount: 10,
			},
			{
				ServerName: "server-2",
				TotalCalls: 100,
				ErrorCount: 5,
			},
		},
		recentEvents: []storage.RecentEvent{
			// Recent errors within the hour
			{
				Timestamp:  now.Add(-10 * time.Minute),
				ServerName: "server-1",
				Success:    false,
			},
			{
				Timestamp:  now.Add(-20 * time.Minute),
				ServerName: "server-1",
				Success:    false,
			},
			{
				Timestamp:  now.Add(-30 * time.Minute),
				ServerName: "server-2",
				Success:    false,
			},
			// Old error (more than an hour ago) - should not be counted
			{
				Timestamp:  now.Add(-2 * time.Hour),
				ServerName: "server-1",
				Success:    false,
			},
			// Success - should not be counted
			{
				Timestamp:  now.Add(-5 * time.Minute),
				ServerName: "server-1",
				Success:    true,
			},
		},
	}

	analyzer := NewErrorAnalyzer(store, DefaultErrorAnalyzerConfig())

	result, err := analyzer.AnalyzeErrors(context.Background(), storage.TimeFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find server-1 in results
	var server1 *ErrorSummary
	for i := range result {
		if result[i].ServerName == "server-1" {
			server1 = &result[i]
			break
		}
	}

	if server1 == nil {
		t.Fatal("server-1 not found in results")
	}

	// Should have 2 recent errors (within the hour)
	if server1.RecentErrors != 2 {
		t.Errorf("expected 2 recent errors for server-1, got %d", server1.RecentErrors)
	}
}

func TestDefaultErrorAnalyzerConfig(t *testing.T) {
	config := DefaultErrorAnalyzerConfig()

	if config.LowThreshold != 2.0 {
		t.Errorf("expected LowThreshold 2.0, got %.2f", config.LowThreshold)
	}

	if config.MediumThreshold != 5.0 {
		t.Errorf("expected MediumThreshold 5.0, got %.2f", config.MediumThreshold)
	}

	if config.HighThreshold != 10.0 {
		t.Errorf("expected HighThreshold 10.0, got %.2f", config.HighThreshold)
	}

	if config.TopToolsCount != 5 {
		t.Errorf("expected TopToolsCount 5, got %d", config.TopToolsCount)
	}
}
