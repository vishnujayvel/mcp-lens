package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/anthropics/mcp-lens/internal/collector"
)

var tailLines int

func newTailCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Stream events in real-time",
		Long:  `Watch new events as they are recorded to the JSONL file.`,
		RunE:  runTail,
	}

	cmd.Flags().IntVarP(&tailLines, "lines", "n", 10, "Number of historical lines to show")

	return cmd
}

func runTail(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	eventsFile := expandPath(cfg.Storage.EventsFile)
	parser := collector.NewParser(eventsFile)

	// Show recent events first
	events, err := parser.TailEvents(tailLines)
	if err != nil {
		return fmt.Errorf("reading events: %w", err)
	}

	fmt.Println("Streaming events (Ctrl+C to stop)...")
	fmt.Println()

	for _, e := range events {
		printEvent(e)
	}

	// Get current position
	lastPos, err := parser.FileSize()
	if err != nil {
		return fmt.Errorf("getting file size: %w", err)
	}

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Poll for new events
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			fmt.Println("\nStopped.")
			return nil
		case <-ticker.C:
			newEvents, newPos, err := parser.ParseFromPosition(lastPos)
			if err != nil {
				// Log but continue
				continue
			}

			for _, e := range newEvents {
				printEvent(e)
			}
			lastPos = newPos
		}
	}
}

func printEvent(e *collector.Event) {
	status := "✓"
	if !e.Success {
		status = "✗"
	}

	latency := ""
	if e.DurationMs > 0 {
		latency = fmt.Sprintf("%dms", e.DurationMs)
	}

	fmt.Printf("%s  %-12s  %-35s  %6s  %s\n",
		e.Timestamp.Format("15:04:05"),
		e.EventType,
		truncate(e.ToolName, 35),
		latency,
		status,
	)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
