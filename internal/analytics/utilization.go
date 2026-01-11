// Package analytics provides MCP server intelligence and analysis.
package analytics

import (
	"context"
	"sort"
	"time"

	"github.com/anthropics/mcp-lens/internal/storage"
)

// ServerUtilization represents utilization metrics for an MCP server.
type ServerUtilization struct {
	ServerName      string
	TotalCalls      int64
	UtilizationPct  float64 // Percentage of total MCP calls
	ErrorRate       float64 // Percentage of calls that failed
	AvgLatencyMs    float64
	LastUsedAt      time.Time
	DaysSinceUse    int
	Trend           Trend // Up, Down, or Stable based on recent activity
	HealthStatus    HealthStatus
}

// Trend indicates the direction of activity change.
type Trend string

const (
	TrendUp     Trend = "up"
	TrendDown   Trend = "down"
	TrendStable Trend = "stable"
)

// HealthStatus indicates the overall health of an MCP server.
type HealthStatus string

const (
	HealthGood     HealthStatus = "good"     // <5% error rate, <500ms latency
	HealthWarning  HealthStatus = "warning"  // 5-10% error rate or 500-1000ms latency
	HealthCritical HealthStatus = "critical" // >10% error rate or >1000ms latency
	HealthUnused   HealthStatus = "unused"   // No calls in threshold period
)

// UtilizationConfig configures utilization analysis.
type UtilizationConfig struct {
	UnusedThresholdDays   int     // Days without activity to consider "unused" (default: 7)
	HighErrorRateThreshold float64 // Error rate % to flag as critical (default: 10.0)
	HighLatencyThresholdMs float64 // Latency ms to flag as slow (default: 1000)
}

// DefaultUtilizationConfig returns default configuration.
func DefaultUtilizationConfig() UtilizationConfig {
	return UtilizationConfig{
		UnusedThresholdDays:    7,
		HighErrorRateThreshold: 10.0,
		HighLatencyThresholdMs: 1000.0,
	}
}

// UtilizationAnalyzer analyzes MCP server utilization.
type UtilizationAnalyzer struct {
	store  UtilizationStore
	config UtilizationConfig
}

// UtilizationStore defines the storage interface needed for utilization analysis.
type UtilizationStore interface {
	GetMCPServerStats(ctx context.Context, filter storage.TimeFilter) ([]storage.MCPServerStats, error)
}

// NewUtilizationAnalyzer creates a new utilization analyzer.
func NewUtilizationAnalyzer(store UtilizationStore, config UtilizationConfig) *UtilizationAnalyzer {
	return &UtilizationAnalyzer{
		store:  store,
		config: config,
	}
}

// AnalyzeUtilization calculates utilization metrics for all MCP servers.
func (a *UtilizationAnalyzer) AnalyzeUtilization(ctx context.Context, filter storage.TimeFilter) ([]ServerUtilization, error) {
	stats, err := a.store.GetMCPServerStats(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(stats) == 0 {
		return []ServerUtilization{}, nil
	}

	// Calculate total MCP calls
	var totalCalls int64
	for _, s := range stats {
		totalCalls += s.TotalCalls
	}

	now := time.Now()
	result := make([]ServerUtilization, len(stats))

	for i, s := range stats {
		util := ServerUtilization{
			ServerName:   s.ServerName,
			TotalCalls:   s.TotalCalls,
			AvgLatencyMs: s.AvgLatencyMs,
			LastUsedAt:   s.LastUsedAt,
		}

		// Calculate utilization percentage
		if totalCalls > 0 {
			util.UtilizationPct = float64(s.TotalCalls) / float64(totalCalls) * 100
		}

		// Calculate error rate
		if s.TotalCalls > 0 {
			util.ErrorRate = float64(s.ErrorCount) / float64(s.TotalCalls) * 100
		}

		// Calculate days since last use
		if !s.LastUsedAt.IsZero() {
			util.DaysSinceUse = int(now.Sub(s.LastUsedAt).Hours() / 24)
		}

		// Determine health status
		util.HealthStatus = a.calculateHealthStatus(util)

		// Trend would require historical data comparison - default to stable
		util.Trend = TrendStable

		result[i] = util
	}

	// Sort by utilization (highest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].UtilizationPct > result[j].UtilizationPct
	})

	return result, nil
}

// GetUnusedServers returns servers that haven't been used within the threshold.
func (a *UtilizationAnalyzer) GetUnusedServers(ctx context.Context) ([]ServerUtilization, error) {
	// Query all time to find servers
	allTimeFilter := storage.TimeFilter{} // No time restriction

	stats, err := a.store.GetMCPServerStats(ctx, allTimeFilter)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	threshold := now.AddDate(0, 0, -a.config.UnusedThresholdDays)

	var unused []ServerUtilization
	for _, s := range stats {
		// Server is unused if last used before threshold
		if s.LastUsedAt.Before(threshold) || s.LastUsedAt.IsZero() {
			daysSince := 0
			if !s.LastUsedAt.IsZero() {
				daysSince = int(now.Sub(s.LastUsedAt).Hours() / 24)
			}

			unused = append(unused, ServerUtilization{
				ServerName:   s.ServerName,
				TotalCalls:   s.TotalCalls,
				LastUsedAt:   s.LastUsedAt,
				DaysSinceUse: daysSince,
				HealthStatus: HealthUnused,
			})
		}
	}

	// Sort by days since use (most stale first)
	sort.Slice(unused, func(i, j int) bool {
		return unused[i].DaysSinceUse > unused[j].DaysSinceUse
	})

	return unused, nil
}

// GetHighErrorServers returns servers with error rate above threshold.
func (a *UtilizationAnalyzer) GetHighErrorServers(ctx context.Context, filter storage.TimeFilter) ([]ServerUtilization, error) {
	utils, err := a.AnalyzeUtilization(ctx, filter)
	if err != nil {
		return nil, err
	}

	var highError []ServerUtilization
	for _, u := range utils {
		if u.ErrorRate >= a.config.HighErrorRateThreshold {
			highError = append(highError, u)
		}
	}

	// Sort by error rate (highest first)
	sort.Slice(highError, func(i, j int) bool {
		return highError[i].ErrorRate > highError[j].ErrorRate
	})

	return highError, nil
}

// GetSlowServers returns servers with average latency above threshold.
func (a *UtilizationAnalyzer) GetSlowServers(ctx context.Context, filter storage.TimeFilter) ([]ServerUtilization, error) {
	utils, err := a.AnalyzeUtilization(ctx, filter)
	if err != nil {
		return nil, err
	}

	var slow []ServerUtilization
	for _, u := range utils {
		if u.AvgLatencyMs >= a.config.HighLatencyThresholdMs {
			slow = append(slow, u)
		}
	}

	// Sort by latency (highest first)
	sort.Slice(slow, func(i, j int) bool {
		return slow[i].AvgLatencyMs > slow[j].AvgLatencyMs
	})

	return slow, nil
}

// calculateHealthStatus determines the health status based on metrics.
func (a *UtilizationAnalyzer) calculateHealthStatus(u ServerUtilization) HealthStatus {
	// Check for unused
	if u.DaysSinceUse >= a.config.UnusedThresholdDays {
		return HealthUnused
	}

	// Check for critical conditions
	if u.ErrorRate >= a.config.HighErrorRateThreshold ||
		u.AvgLatencyMs >= a.config.HighLatencyThresholdMs {
		return HealthCritical
	}

	// Check for warning conditions (half the critical thresholds)
	if u.ErrorRate >= a.config.HighErrorRateThreshold/2 ||
		u.AvgLatencyMs >= a.config.HighLatencyThresholdMs/2 {
		return HealthWarning
	}

	return HealthGood
}

// UtilizationSummary provides an overview of MCP server health.
type UtilizationSummary struct {
	TotalServers    int
	HealthyServers  int
	WarningServers  int
	CriticalServers int
	UnusedServers   int
	TotalCalls      int64
	TotalErrors     int64
	OverallErrorRate float64
}

// GetSummary returns a summary of MCP server utilization.
func (a *UtilizationAnalyzer) GetSummary(ctx context.Context, filter storage.TimeFilter) (*UtilizationSummary, error) {
	utils, err := a.AnalyzeUtilization(ctx, filter)
	if err != nil {
		return nil, err
	}

	summary := &UtilizationSummary{
		TotalServers: len(utils),
	}

	for _, u := range utils {
		summary.TotalCalls += u.TotalCalls

		switch u.HealthStatus {
		case HealthGood:
			summary.HealthyServers++
		case HealthWarning:
			summary.WarningServers++
		case HealthCritical:
			summary.CriticalServers++
		case HealthUnused:
			summary.UnusedServers++
		}

		// Calculate total errors from error rate
		if u.TotalCalls > 0 {
			errors := int64(float64(u.TotalCalls) * u.ErrorRate / 100)
			summary.TotalErrors += errors
		}
	}

	if summary.TotalCalls > 0 {
		summary.OverallErrorRate = float64(summary.TotalErrors) / float64(summary.TotalCalls) * 100
	}

	return summary, nil
}
