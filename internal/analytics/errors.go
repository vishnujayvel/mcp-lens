package analytics

import (
	"context"
	"sort"
	"time"

	"github.com/anthropics/mcp-lens/internal/storage"
)

// ErrorSummary provides aggregated error information for an MCP server.
type ErrorSummary struct {
	ServerName       string
	TotalCalls       int64
	ErrorCount       int64
	ErrorRate        float64 // Percentage
	RecentErrors     int64   // Errors in last hour
	ErrorTrend       Trend   // Increasing, decreasing, or stable
	MostFailedTools  []ToolErrorInfo
	SeverityLevel    ErrorSeverity
}

// ToolErrorInfo shows error information for a specific tool.
type ToolErrorInfo struct {
	ToolName   string
	ErrorCount int64
	ErrorRate  float64
}

// ErrorSeverity indicates how severe the error situation is.
type ErrorSeverity string

const (
	ErrorSeverityLow      ErrorSeverity = "low"      // <2% error rate
	ErrorSeverityMedium   ErrorSeverity = "medium"   // 2-5% error rate
	ErrorSeverityHigh     ErrorSeverity = "high"     // 5-10% error rate
	ErrorSeverityCritical ErrorSeverity = "critical" // >10% error rate
)

// ErrorAnalyzerConfig configures error analysis.
type ErrorAnalyzerConfig struct {
	LowThreshold      float64 // Below this is "low" severity
	MediumThreshold   float64 // Below this is "medium" severity
	HighThreshold     float64 // Below this is "high" severity
	TopToolsCount     int     // Number of top failing tools to track
}

// DefaultErrorAnalyzerConfig returns default configuration.
func DefaultErrorAnalyzerConfig() ErrorAnalyzerConfig {
	return ErrorAnalyzerConfig{
		LowThreshold:    2.0,
		MediumThreshold: 5.0,
		HighThreshold:   10.0,
		TopToolsCount:   5,
	}
}

// ErrorAnalyzer provides error aggregation and analysis.
type ErrorAnalyzer struct {
	store  ErrorAnalyzerStore
	config ErrorAnalyzerConfig
}

// ErrorAnalyzerStore defines the storage interface needed for error analysis.
type ErrorAnalyzerStore interface {
	GetMCPServerStats(ctx context.Context, filter storage.TimeFilter) ([]storage.MCPServerStats, error)
	GetToolStats(ctx context.Context, filter storage.TimeFilter) ([]storage.ToolStats, error)
	GetRecentEvents(ctx context.Context, limit int) ([]storage.RecentEvent, error)
}

// NewErrorAnalyzer creates a new error analyzer.
func NewErrorAnalyzer(store ErrorAnalyzerStore, config ErrorAnalyzerConfig) *ErrorAnalyzer {
	return &ErrorAnalyzer{
		store:  store,
		config: config,
	}
}

// AnalyzeErrors returns error summaries for all MCP servers.
func (a *ErrorAnalyzer) AnalyzeErrors(ctx context.Context, filter storage.TimeFilter) ([]ErrorSummary, error) {
	serverStats, err := a.store.GetMCPServerStats(ctx, filter)
	if err != nil {
		return nil, err
	}

	toolStats, err := a.store.GetToolStats(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Group tool stats by server
	toolsByServer := make(map[string][]storage.ToolStats)
	for _, t := range toolStats {
		if t.MCPServer != "" {
			toolsByServer[t.MCPServer] = append(toolsByServer[t.MCPServer], t)
		}
	}

	// Get recent events for trend analysis
	recentEvents, _ := a.store.GetRecentEvents(ctx, 100)
	recentErrorsByServer := a.countRecentErrors(recentEvents, time.Hour)

	result := make([]ErrorSummary, 0, len(serverStats))

	for _, s := range serverStats {
		summary := ErrorSummary{
			ServerName: s.ServerName,
			TotalCalls: s.TotalCalls,
			ErrorCount: s.ErrorCount,
		}

		// Calculate error rate
		if s.TotalCalls > 0 {
			summary.ErrorRate = float64(s.ErrorCount) / float64(s.TotalCalls) * 100
		}

		// Get recent errors for this server
		summary.RecentErrors = recentErrorsByServer[s.ServerName]

		// Calculate severity
		summary.SeverityLevel = a.calculateSeverity(summary.ErrorRate)

		// Get most failed tools for this server
		if tools, ok := toolsByServer[s.ServerName]; ok {
			summary.MostFailedTools = a.getTopFailingTools(tools)
		}

		// Default trend to stable (would need historical comparison for accurate trend)
		summary.ErrorTrend = TrendStable

		result = append(result, summary)
	}

	// Sort by error rate (highest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].ErrorRate > result[j].ErrorRate
	})

	return result, nil
}

// GetCriticalServers returns servers with critical error rates.
func (a *ErrorAnalyzer) GetCriticalServers(ctx context.Context, filter storage.TimeFilter) ([]ErrorSummary, error) {
	summaries, err := a.AnalyzeErrors(ctx, filter)
	if err != nil {
		return nil, err
	}

	var critical []ErrorSummary
	for _, s := range summaries {
		if s.SeverityLevel == ErrorSeverityCritical {
			critical = append(critical, s)
		}
	}

	return critical, nil
}

// GetErrorTotals returns overall error statistics.
func (a *ErrorAnalyzer) GetErrorTotals(ctx context.Context, filter storage.TimeFilter) (*ErrorTotals, error) {
	summaries, err := a.AnalyzeErrors(ctx, filter)
	if err != nil {
		return nil, err
	}

	totals := &ErrorTotals{
		ServerCount: len(summaries),
	}

	for _, s := range summaries {
		totals.TotalCalls += s.TotalCalls
		totals.TotalErrors += s.ErrorCount

		switch s.SeverityLevel {
		case ErrorSeverityCritical:
			totals.CriticalServers++
		case ErrorSeverityHigh:
			totals.HighErrorServers++
		case ErrorSeverityMedium:
			totals.MediumErrorServers++
		}
	}

	if totals.TotalCalls > 0 {
		totals.OverallErrorRate = float64(totals.TotalErrors) / float64(totals.TotalCalls) * 100
	}

	return totals, nil
}

// ErrorTotals provides aggregate error statistics.
type ErrorTotals struct {
	ServerCount        int
	TotalCalls         int64
	TotalErrors        int64
	OverallErrorRate   float64
	CriticalServers    int
	HighErrorServers   int
	MediumErrorServers int
}

// countRecentErrors counts errors per server from recent events.
func (a *ErrorAnalyzer) countRecentErrors(events []storage.RecentEvent, window time.Duration) map[string]int64 {
	counts := make(map[string]int64)
	cutoff := time.Now().Add(-window)

	for _, e := range events {
		if e.Timestamp.After(cutoff) && !e.Success && e.ServerName != "" {
			counts[e.ServerName]++
		}
	}

	return counts
}

// getTopFailingTools returns the tools with highest error rates for a server.
func (a *ErrorAnalyzer) getTopFailingTools(tools []storage.ToolStats) []ToolErrorInfo {
	// Filter to tools with errors and calculate error rate
	var withErrors []ToolErrorInfo
	for _, t := range tools {
		if t.ErrorCount > 0 {
			errorRate := float64(0)
			if t.TotalCalls > 0 {
				errorRate = float64(t.ErrorCount) / float64(t.TotalCalls) * 100
			}
			withErrors = append(withErrors, ToolErrorInfo{
				ToolName:   t.ToolName,
				ErrorCount: t.ErrorCount,
				ErrorRate:  errorRate,
			})
		}
	}

	// Sort by error count
	sort.Slice(withErrors, func(i, j int) bool {
		return withErrors[i].ErrorCount > withErrors[j].ErrorCount
	})

	// Return top N
	if len(withErrors) > a.config.TopToolsCount {
		withErrors = withErrors[:a.config.TopToolsCount]
	}

	return withErrors
}

// calculateSeverity determines error severity based on rate.
func (a *ErrorAnalyzer) calculateSeverity(errorRate float64) ErrorSeverity {
	switch {
	case errorRate >= a.config.HighThreshold:
		return ErrorSeverityCritical
	case errorRate >= a.config.MediumThreshold:
		return ErrorSeverityHigh
	case errorRate >= a.config.LowThreshold:
		return ErrorSeverityMedium
	default:
		return ErrorSeverityLow
	}
}

// ErrorEvent represents a single error occurrence for detailed analysis.
type ErrorEvent struct {
	Timestamp  time.Time
	SessionID  string
	ServerName string
	ToolName   string
	DurationMs int64
}

// GetRecentErrors returns recent error events.
func (a *ErrorAnalyzer) GetRecentErrors(ctx context.Context, limit int) ([]ErrorEvent, error) {
	events, err := a.store.GetRecentEvents(ctx, limit*2) // Get more to filter
	if err != nil {
		return nil, err
	}

	var errors []ErrorEvent
	for _, e := range events {
		if !e.Success {
			errors = append(errors, ErrorEvent{
				Timestamp:  e.Timestamp,
				SessionID:  e.SessionID,
				ServerName: e.ServerName,
				ToolName:   e.ToolName,
				DurationMs: e.DurationMs,
			})
			if len(errors) >= limit {
				break
			}
		}
	}

	return errors, nil
}
