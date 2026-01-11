package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var purgeAll bool
var purgeDays int

func newPurgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Delete all collected data",
		Long:  `Remove collected data. Use --all to remove everything, or --days to remove old data.`,
		RunE:  runPurge,
	}

	cmd.Flags().BoolVar(&purgeAll, "all", false, "Delete all data (requires confirmation)")
	cmd.Flags().IntVar(&purgeDays, "days", 0, "Delete data older than N days")

	return cmd
}

func runPurge(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if purgeAll {
		// Confirm deletion
		fmt.Print("This will delete ALL data. Are you sure? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			fmt.Println("Aborted.")
			return nil
		}

		// Delete files
		eventsFile := expandPath(cfg.Storage.EventsFile)
		dbPath := expandPath(cfg.Storage.DatabasePath)

		if err := os.Remove(eventsFile); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Warning: could not remove events file: %v\n", err)
		} else if err == nil {
			fmt.Printf("Removed: %s\n", eventsFile)
		}

		if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Warning: could not remove database: %v\n", err)
		} else if err == nil {
			fmt.Printf("Removed: %s\n", dbPath)
		}

		// Remove WAL files
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")

		fmt.Println("All data purged.")
		return nil
	}

	if purgeDays > 0 {
		store, err := openStorage(cfg)
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()
		olderThan := time.Now().Add(-time.Duration(purgeDays) * 24 * time.Hour)

		deleted, err := store.Cleanup(ctx, olderThan)
		if err != nil {
			return fmt.Errorf("cleaning up: %w", err)
		}

		fmt.Printf("Deleted %d events older than %d days\n", deleted, purgeDays)
		return nil
	}

	fmt.Println("Use --all to delete everything or --days N to delete old data")
	return nil
}
