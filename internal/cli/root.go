// Package cli provides the command-line interface for MCP Lens.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/anthropics/mcp-lens/internal/collector"
	"github.com/anthropics/mcp-lens/internal/config"
	"github.com/anthropics/mcp-lens/internal/storage"
	"github.com/anthropics/mcp-lens/internal/tui"
)

var (
	// Global flags
	cfgFile   string
	dataDir   string
	timeRange string
	noColor   bool
	refresh   int

	// Version info (set at build time)
	Version   = "dev"
	BuildDate = "unknown"
)

// NewRootCmd creates the root command.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "mcp-lens",
		Short: "MCP server observability for Claude Code",
		Long: `MCP Lens provides observability and analytics for Claude Code MCP server usage.

Launch the TUI dashboard by running without any subcommand.
Use subcommands for specific operations.`,
		RunE: runDashboard,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "Data directory (default: ~/.mcp-lens)")
	rootCmd.PersistentFlags().StringVarP(&timeRange, "range", "r", "24h", "Time range: 1h, 24h, 7d, 30d")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colors")
	rootCmd.PersistentFlags().IntVar(&refresh, "refresh", 5, "TUI refresh interval in seconds")

	// Add subcommands
	rootCmd.AddCommand(newStatsCmd())
	rootCmd.AddCommand(newTailCmd())
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newSyncCmd())
	rootCmd.AddCommand(newPurgeCmd())
	rootCmd.AddCommand(newVersionCmd())

	return rootCmd
}

// runDashboard launches the TUI dashboard.
func runDashboard(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Open storage
	store, err := openStorage(cfg)
	if err != nil {
		return fmt.Errorf("opening storage: %w", err)
	}
	defer store.Close()

	// Create sync engine
	eventsFile := expandPath(cfg.Storage.EventsFile)
	syncConfig := collector.SyncConfig{
		EventsFile: eventsFile,
		BatchSize:  1000,
		DataDir:    expandPath(cfg.Storage.DataDir),
	}

	// Create sync engine with store adapter
	syncStore := &sqliteSyncAdapter{store: store}
	syncEngine := collector.NewSyncEngine(syncConfig, syncStore)

	// Create TUI app
	tuiConfig := tui.AppConfig{
		RefreshInterval: cfg.TUI.RefreshInterval,
		TimeRange:       timeRange,
		NoColor:         noColor,
	}
	if refresh > 0 {
		tuiConfig.RefreshInterval = cfg.TUI.RefreshInterval
	}

	app := tui.NewApp(tuiConfig, store, syncEngine)

	// Handle signals
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		app.Stop()
		cancel()
	}()

	// Run TUI
	return app.Run(ctx)
}

// loadConfig loads the configuration.
func loadConfig() (*config.Config, error) {
	cfg := config.DefaultConfig()

	// Override data dir if specified
	if dataDir != "" {
		cfg.Storage.DataDir = dataDir
		cfg.Storage.DatabasePath = filepath.Join(dataDir, "data.db")
		cfg.Storage.EventsFile = filepath.Join(dataDir, "events.jsonl")
	}

	return cfg, nil
}

// openStorage opens the SQLite storage.
func openStorage(cfg *config.Config) (*storage.SQLiteStore, error) {
	dbPath := expandPath(cfg.Storage.DatabasePath)
	return storage.NewSQLiteStore(dbPath)
}

// expandPath expands ~ to home directory.
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// sqliteSyncAdapter adapts SQLiteStore to collector.SyncStore interface.
type sqliteSyncAdapter struct {
	store *storage.SQLiteStore
}

func (a *sqliteSyncAdapter) GetSyncPosition(ctx context.Context) (int64, error) {
	return a.store.GetSyncPosition(ctx)
}

func (a *sqliteSyncAdapter) SetSyncPosition(ctx context.Context, pos int64) error {
	return a.store.SetSyncPosition(ctx, pos)
}

func (a *sqliteSyncAdapter) UpsertToolStats(ctx context.Context, date string, toolName string, serverName string, calls int64, errors int64, latencyMs int64) error {
	return a.store.UpsertToolStats(ctx, date, toolName, serverName, calls, errors, latencyMs)
}

func (a *sqliteSyncAdapter) UpsertSession(ctx context.Context, id string, cwd string, startedAt time.Time) error {
	return a.store.UpsertSession(ctx, id, cwd, startedAt)
}

func (a *sqliteSyncAdapter) UpdateSessionEnd(ctx context.Context, id string, endedAt time.Time) error {
	return a.store.UpdateSessionEnd(ctx, id, endedAt)
}

func (a *sqliteSyncAdapter) IncrementSessionStats(ctx context.Context, id string, toolCalls int64, errors int64) error {
	return a.store.IncrementSessionStats(ctx, id, toolCalls, errors)
}

func (a *sqliteSyncAdapter) InsertRecentEvent(ctx context.Context, timestamp time.Time, sessionID string, eventType string, toolName string, serverName string, durationMs int64, success bool) error {
	return a.store.InsertRecentEvent(ctx, timestamp, sessionID, eventType, toolName, serverName, durationMs, success)
}

func (a *sqliteSyncAdapter) HasEventFingerprint(ctx context.Context, fingerprint string) (bool, error) {
	return a.store.HasEventFingerprint(ctx, fingerprint)
}

func (a *sqliteSyncAdapter) StoreEventFingerprint(ctx context.Context, fingerprint string, timestamp time.Time) error {
	return a.store.StoreEventFingerprint(ctx, fingerprint, timestamp)
}

// Execute runs the CLI.
func Execute() error {
	return NewRootCmd().Execute()
}
