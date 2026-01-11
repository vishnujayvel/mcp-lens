package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/anthropics/mcp-lens/internal/storage"
)

func newStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show stats summary (one-shot)",
		Long:  `Display a summary of MCP server and tool usage statistics.`,
		RunE:  runStats,
	}
}

func runStats(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	store, err := openStorage(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()

	// Parse time range
	filter := parseTimeRange(timeRange)

	// Get stats from aggregated tool_stats table
	mcpStats, err := store.GetMCPServerStatsAggregated(ctx, filter)
	if err != nil {
		return fmt.Errorf("getting MCP stats: %w", err)
	}

	sessions, err := store.GetSessions(ctx, storage.SessionFilter{TimeFilter: filter, Limit: 1000})
	if err != nil {
		return fmt.Errorf("getting sessions: %w", err)
	}

	// Calculate totals
	totalCalls := int64(0)
	totalErrors := int64(0)
	for _, s := range mcpStats {
		totalCalls += s.TotalCalls
		totalErrors += s.ErrorCount
	}

	// Print summary
	fmt.Printf("\nMCP Lens Stats (last %s)\n", timeRange)
	fmt.Println("─────────────────────────")
	fmt.Printf("Sessions:     %d\n", len(sessions))
	fmt.Printf("Tool Calls:   %d\n", totalCalls)

	errorRate := 0.0
	if totalCalls > 0 {
		errorRate = float64(totalErrors) / float64(totalCalls) * 100
	}
	fmt.Printf("Errors:       %d (%.1f%%)\n", totalErrors, errorRate)

	if len(mcpStats) > 0 {
		fmt.Println("\nTop MCP Servers:")
		for i, s := range mcpStats {
			if i >= 5 {
				break
			}
			fmt.Printf("  %-12s %5d calls   avg %dms\n", s.ServerName, s.TotalCalls, int(s.AvgLatencyMs))
		}
	} else {
		fmt.Println("\n(No MCP server data)")
	}

	fmt.Println()
	return nil
}

func parseTimeRange(r string) storage.TimeFilter {
	now := time.Now()
	var from time.Time

	switch r {
	case "1h":
		from = now.Add(-1 * time.Hour)
	case "24h":
		from = now.Add(-24 * time.Hour)
	case "7d":
		from = now.Add(-7 * 24 * time.Hour)
	case "30d":
		from = now.Add(-30 * 24 * time.Hour)
	default:
		from = now.Add(-24 * time.Hour)
	}

	return storage.TimeFilter{
		From: from,
		To:   now,
	}
}
