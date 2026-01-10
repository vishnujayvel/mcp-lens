// Package metrics computes observability metrics from stored events.
package metrics

import (
	"context"
	"time"

	"github.com/anthropics/mcp-lens/internal/storage"
)

// Calculator computes metrics from stored events.
type Calculator struct {
	store storage.Store
}

// NewCalculator creates a new metrics calculator.
func NewCalculator(store storage.Store) *Calculator {
	return &Calculator{store: store}
}

// DashboardSummary contains high-level metrics for the main dashboard.
type DashboardSummary struct {
	TotalSessions    int64
	TotalEvents      int64
	TotalTokens      int64
	TotalCostUSD     float64
	ActiveMCPServers int
	AvgSessionLength time.Duration
	TokensByModel    map[string]int64
	CostByModel      map[string]float64
	RecentEvents     []storage.Event
}

// MCPUtilization tracks how often an MCP server is used.
type MCPUtilization struct {
	ServerName     string
	CallCount      int64
	Percentage     float64
	TrendDirection string // "up", "down", "stable"
	SuccessRate    float64
	AvgLatencyMs   float64
}

// ToolSuccessRate tracks success/failure rates per tool.
type ToolSuccessRate struct {
	ToolName     string
	MCPServer    string
	TotalCalls   int64
	SuccessRate  float64
	ErrorRate    float64
	AvgLatencyMs float64
	CommonErrors []string
}

// UnusedServer represents an MCP server with no recent activity.
type UnusedServer struct {
	ServerName  string
	LastUsedAt  time.Time
	DaysSinceUse int
	Status      string // "never_used" or "inactive"
}

// CostForecast provides cost projections.
type CostForecast struct {
	DailyAverage    float64
	WeeklyEstimate  float64
	MonthlyEstimate float64
	Confidence      string // "low", "medium", "high"
	DataDays        int    // Number of days of data used
}

// GetDashboardSummary returns high-level metrics for the dashboard.
func (c *Calculator) GetDashboardSummary(ctx context.Context, filter storage.TimeFilter) (*DashboardSummary, error) {
	// Get cost summary
	costSummary, err := c.store.GetCostSummary(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Get sessions
	sessions, err := c.store.GetSessions(ctx, storage.SessionFilter{
		TimeFilter: filter,
		Limit:      1000,
	})
	if err != nil {
		return nil, err
	}

	// Get MCP stats
	mcpStats, err := c.store.GetMCPServerStats(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Get recent events
	recentEvents, err := c.store.GetEvents(ctx, storage.EventFilter{
		TimeFilter: filter,
		Limit:      10,
	})
	if err != nil {
		return nil, err
	}

	// Calculate totals
	var totalEvents int64
	for _, s := range sessions {
		totalEvents += int64(s.TotalEvents)
	}

	// Calculate average session length
	var avgSessionLength time.Duration
	if len(sessions) > 0 {
		var totalDuration time.Duration
		var completedSessions int
		for _, s := range sessions {
			if s.EndedAt != nil {
				totalDuration += s.EndedAt.Sub(s.StartedAt)
				completedSessions++
			}
		}
		if completedSessions > 0 {
			avgSessionLength = totalDuration / time.Duration(completedSessions)
		}
	}

	return &DashboardSummary{
		TotalSessions:    int64(len(sessions)),
		TotalEvents:      totalEvents,
		TotalTokens:      costSummary.TotalTokens,
		TotalCostUSD:     costSummary.TotalCostUSD,
		ActiveMCPServers: len(mcpStats),
		AvgSessionLength: avgSessionLength,
		TokensByModel:    make(map[string]int64),
		CostByModel:      make(map[string]float64),
		RecentEvents:     recentEvents,
	}, nil
}

// GetMCPUtilization returns utilization stats for all MCP servers.
func (c *Calculator) GetMCPUtilization(ctx context.Context, filter storage.TimeFilter) ([]MCPUtilization, error) {
	stats, err := c.store.GetMCPServerStats(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Calculate total calls for percentage
	var totalCalls int64
	for _, s := range stats {
		totalCalls += s.TotalCalls
	}

	result := make([]MCPUtilization, len(stats))
	for i, s := range stats {
		percentage := float64(0)
		if totalCalls > 0 {
			percentage = float64(s.TotalCalls) / float64(totalCalls) * 100
		}

		successRate := float64(0)
		if s.TotalCalls > 0 {
			successRate = float64(s.SuccessCount) / float64(s.TotalCalls) * 100
		}

		result[i] = MCPUtilization{
			ServerName:     s.ServerName,
			CallCount:      s.TotalCalls,
			Percentage:     percentage,
			TrendDirection: "stable", // TODO: Calculate trend from historical data
			SuccessRate:    successRate,
			AvgLatencyMs:   s.AvgLatencyMs,
		}
	}

	return result, nil
}

// GetToolSuccessRates returns success rates for all tools.
func (c *Calculator) GetToolSuccessRates(ctx context.Context, filter storage.TimeFilter) ([]ToolSuccessRate, error) {
	stats, err := c.store.GetToolStats(ctx, filter)
	if err != nil {
		return nil, err
	}

	result := make([]ToolSuccessRate, len(stats))
	for i, s := range stats {
		successRate := float64(0)
		errorRate := float64(0)
		if s.TotalCalls > 0 {
			successRate = float64(s.SuccessCount) / float64(s.TotalCalls) * 100
			errorRate = float64(s.ErrorCount) / float64(s.TotalCalls) * 100
		}

		result[i] = ToolSuccessRate{
			ToolName:     s.ToolName,
			MCPServer:    s.MCPServer,
			TotalCalls:   s.TotalCalls,
			SuccessRate:  successRate,
			ErrorRate:    errorRate,
			AvgLatencyMs: s.AvgLatencyMs,
		}
	}

	return result, nil
}

// GetUnusedServers returns MCP servers with no recent activity.
func (c *Calculator) GetUnusedServers(ctx context.Context, unusedSince time.Duration) ([]UnusedServer, error) {
	// Get all MCP server stats
	stats, err := c.store.GetMCPServerStats(ctx, storage.TimeFilter{})
	if err != nil {
		return nil, err
	}

	threshold := time.Now().Add(-unusedSince)
	var unused []UnusedServer

	for _, s := range stats {
		if s.LastUsedAt.Before(threshold) {
			daysSince := int(time.Since(s.LastUsedAt).Hours() / 24)
			status := "inactive"
			if s.TotalCalls == 0 {
				status = "never_used"
			}

			unused = append(unused, UnusedServer{
				ServerName:   s.ServerName,
				LastUsedAt:   s.LastUsedAt,
				DaysSinceUse: daysSince,
				Status:       status,
			})
		}
	}

	return unused, nil
}

// GetCostForecast returns cost projections based on recent usage.
func (c *Calculator) GetCostForecast(ctx context.Context, lookbackDays int) (*CostForecast, error) {
	// Get costs for the lookback period
	from := time.Now().AddDate(0, 0, -lookbackDays)
	filter := storage.TimeFilter{From: from}

	costSummary, err := c.store.GetCostSummary(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Calculate daily average
	dailyAverage := costSummary.TotalCostUSD / float64(lookbackDays)
	if dailyAverage < 0 {
		dailyAverage = 0
	}

	// Determine confidence based on data availability
	confidence := "low"
	if lookbackDays >= 7 && costSummary.TotalCostUSD > 0 {
		confidence = "medium"
	}
	if lookbackDays >= 14 && costSummary.TotalCostUSD > 0 {
		confidence = "high"
	}

	return &CostForecast{
		DailyAverage:    dailyAverage,
		WeeklyEstimate:  dailyAverage * 7,
		MonthlyEstimate: dailyAverage * 30,
		Confidence:      confidence,
		DataDays:        lookbackDays,
	}, nil
}

// TimeFilter is a convenience re-export.
type TimeFilter = storage.TimeFilter
