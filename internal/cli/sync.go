package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/anthropics/mcp-lens/internal/collector"
)

var resetSync bool

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Manually sync JSONL to SQLite",
		Long:  `Process any unsynced events from the JSONL file into SQLite.`,
		RunE:  runSync,
	}

	cmd.Flags().BoolVar(&resetSync, "reset", false, "Reset sync position and re-process all events")

	return cmd
}

func runSync(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	store, err := openStorage(cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	eventsFile := expandPath(cfg.Storage.EventsFile)
	syncConfig := collector.SyncConfig{
		EventsFile: eventsFile,
		BatchSize:  1000,
		DataDir:    expandPath(cfg.Storage.DataDir),
	}

	syncStore := &sqliteSyncAdapter{store: store}
	syncEngine := collector.NewSyncEngine(syncConfig, syncStore)

	ctx := context.Background()

	if resetSync {
		fmt.Println("Resetting sync position...")
		if err := syncEngine.Reset(ctx); err != nil {
			return fmt.Errorf("resetting sync: %w", err)
		}
	}

	fmt.Println("Syncing events...")
	result, err := syncEngine.Sync(ctx)
	if err != nil {
		return fmt.Errorf("syncing: %w", err)
	}

	fmt.Printf("Processed %d events in %s\n", result.EventsProcessed, result.Duration)

	// Show validation and deduplication stats
	if result.EventsSkipped > 0 {
		fmt.Printf("  Skipped:     %d events\n", result.EventsSkipped)
	}
	if result.DuplicatesFound > 0 {
		fmt.Printf("  Duplicates:  %d (ignored)\n", result.DuplicatesFound)
	}
	if result.InvalidEvents > 0 {
		fmt.Printf("  Invalid:     %d (validation failed)\n", result.InvalidEvents)
	}

	if len(result.Errors) > 0 {
		fmt.Printf("  Errors:      %d\n", len(result.Errors))
	}

	// Show warnings if verbose (but limit to first 5)
	if len(result.Warnings) > 0 {
		fmt.Printf("\nValidation warnings:\n")
		for i, w := range result.Warnings {
			if i >= 5 {
				fmt.Printf("  ... and %d more\n", len(result.Warnings)-5)
				break
			}
			fmt.Printf("  - %s\n", w)
		}
	}

	return nil
}
