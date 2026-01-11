// Package tui provides the terminal user interface.
package tui

import (
	"context"
	"fmt"
	"time"

	ui "github.com/gizak/termui/v3"

	"github.com/anthropics/mcp-lens/internal/analytics"
	"github.com/anthropics/mcp-lens/internal/collector"
	"github.com/anthropics/mcp-lens/internal/storage"
)

// App represents the TUI application.
type App struct {
	config            AppConfig
	store             storage.Store
	sync              *collector.SyncEngine
	dashboard         *Dashboard
	stopChan          chan struct{}
	utilizationAnalyzer *analytics.UtilizationAnalyzer
	errorAnalyzer       *analytics.ErrorAnalyzer
}

// AppConfig configures the TUI application.
type AppConfig struct {
	RefreshInterval time.Duration
	TimeRange       string // 1h, 24h, 7d, 30d
	NoColor         bool
}

// DefaultAppConfig returns default TUI configuration.
func DefaultAppConfig() AppConfig {
	return AppConfig{
		RefreshInterval: 5 * time.Second,
		TimeRange:       "24h",
		NoColor:         false,
	}
}

// NewApp creates a new TUI application.
func NewApp(config AppConfig, store storage.Store, sync *collector.SyncEngine) *App {
	app := &App{
		config:   config,
		store:    store,
		sync:     sync,
		stopChan: make(chan struct{}),
	}

	// Initialize analytics analyzers
	app.utilizationAnalyzer = analytics.NewUtilizationAnalyzer(store, analytics.DefaultUtilizationConfig())
	app.errorAnalyzer = analytics.NewErrorAnalyzer(store, analytics.DefaultErrorAnalyzerConfig())

	return app
}

// Run starts the TUI and blocks until exit.
func (a *App) Run(ctx context.Context) error {
	// Initialize termui
	if err := ui.Init(); err != nil {
		return fmt.Errorf("failed to initialize termui: %w", err)
	}
	defer ui.Close()

	// Initial sync
	if a.sync != nil {
		if _, err := a.sync.Sync(ctx); err != nil {
			// Log but continue - we can still show existing data
			fmt.Printf("Warning: initial sync failed: %v\n", err)
		}
	}

	// Create dashboard
	a.dashboard = NewDashboard(a.config.TimeRange)

	// Initial render
	if err := a.refresh(ctx); err != nil {
		return err
	}

	// Start event loop
	uiEvents := ui.PollEvents()
	ticker := time.NewTicker(a.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-a.stopChan:
			return nil
		case e := <-uiEvents:
			switch e.Type {
			case ui.KeyboardEvent:
				if a.handleKey(e.ID) {
					return nil // Exit requested
				}
			case ui.ResizeEvent:
				ui.Clear()
				a.dashboard.Render()
			}
		case <-ticker.C:
			if err := a.refresh(ctx); err != nil {
				// Log but continue
				fmt.Printf("Warning: refresh failed: %v\n", err)
			}
		}
	}
}

// handleKey processes keyboard input. Returns true if exit requested.
func (a *App) handleKey(key string) bool {
	switch key {
	case "q", "<C-c>":
		return true
	case "r":
		ctx := context.Background()
		a.refresh(ctx)
	case "1":
		a.setTimeRange("1h")
	case "2":
		a.setTimeRange("24h")
	case "3":
		a.setTimeRange("7d")
	case "4":
		a.setTimeRange("30d")
	case "j", "<Down>":
		a.dashboard.ScrollDown()
	case "k", "<Up>":
		a.dashboard.ScrollUp()
	}
	return false
}

// setTimeRange changes the time filter and refreshes.
func (a *App) setTimeRange(r string) {
	a.config.TimeRange = r
	a.dashboard.SetTimeRange(r)
	ctx := context.Background()
	a.refresh(ctx)
}

// refresh syncs data and updates the display.
func (a *App) refresh(ctx context.Context) error {
	// Sync new events
	if a.sync != nil {
		a.sync.Sync(ctx)
	}

	// Fetch data
	data, err := a.fetchDashboardData(ctx)
	if err != nil {
		return err
	}

	// Update dashboard
	a.dashboard.Update(data)
	a.dashboard.Render()

	return nil
}

// fetchDashboardData retrieves all data needed for the dashboard.
func (a *App) fetchDashboardData(ctx context.Context) (*DashboardData, error) {
	data := &DashboardData{
		TimeRange: a.config.TimeRange,
		UpdatedAt: time.Now(),
	}

	// Parse time range
	filter := a.parseTimeFilter()

	// Get MCP server stats
	if stats, err := a.store.GetMCPServerStats(ctx, filter); err == nil {
		data.MCPServers = stats
	}

	// Get recent events
	if events, err := a.store.GetRecentEvents(ctx, 10); err == nil {
		data.RecentEvents = events
	}

	// Get call volume for sparkline
	if volume, err := a.store.GetCallVolumeByHour(ctx, filter); err == nil {
		data.CallVolume = volume
	}

	// Get session count
	if sessions, err := a.store.GetSessions(ctx, storage.SessionFilter{TimeFilter: filter, Limit: 1000}); err == nil {
		data.SessionCount = len(sessions)
	}

	// Get analytics data - utilization
	if a.utilizationAnalyzer != nil {
		if util, err := a.utilizationAnalyzer.AnalyzeUtilization(ctx, filter); err == nil {
			data.Utilization = util
		}
	}

	// Get analytics data - error totals
	if a.errorAnalyzer != nil {
		if totals, err := a.errorAnalyzer.GetErrorTotals(ctx, filter); err == nil {
			data.ErrorTotals = totals
		}
	}

	return data, nil
}

// parseTimeFilter converts time range string to TimeFilter.
func (a *App) parseTimeFilter() storage.TimeFilter {
	now := time.Now()
	var from time.Time

	switch a.config.TimeRange {
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

// Stop signals the TUI to exit.
func (a *App) Stop() {
	close(a.stopChan)
}
