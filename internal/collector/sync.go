package collector

import (
	"context"
	"fmt"
	"time"
)

// SyncEngine processes JSONL events into SQLite aggregations.
type SyncEngine struct {
	parser    *Parser
	store     SyncStore
	config    SyncConfig
	validator *EventValidator
}

// SyncConfig configures the sync engine.
type SyncConfig struct {
	EventsFile   string
	BatchSize    int
	DataDir      string
}

// DefaultSyncConfig returns default sync configuration.
func DefaultSyncConfig() SyncConfig {
	return SyncConfig{
		EventsFile: "~/.mcp-lens/events.jsonl",
		BatchSize:  1000,
		DataDir:    "~/.mcp-lens",
	}
}

// SyncStore defines storage operations needed by the sync engine.
type SyncStore interface {
	// Sync state
	GetSyncPosition(ctx context.Context) (int64, error)
	SetSyncPosition(ctx context.Context, pos int64) error

	// Aggregation
	UpsertToolStats(ctx context.Context, date string, toolName string, serverName string, calls int64, errors int64, latencyMs int64) error
	UpsertSession(ctx context.Context, id string, cwd string, startedAt time.Time) error
	UpdateSessionEnd(ctx context.Context, id string, endedAt time.Time) error
	IncrementSessionStats(ctx context.Context, id string, toolCalls int64, errors int64) error

	// Recent events
	InsertRecentEvent(ctx context.Context, timestamp time.Time, sessionID string, eventType string, toolName string, serverName string, durationMs int64, success bool) error

	// Deduplication
	HasEventFingerprint(ctx context.Context, fingerprint string) (bool, error)
	StoreEventFingerprint(ctx context.Context, fingerprint string, timestamp time.Time) error
}

// NewSyncEngine creates a new sync engine.
func NewSyncEngine(config SyncConfig, store SyncStore) *SyncEngine {
	return &SyncEngine{
		parser:    NewParser(config.EventsFile),
		store:     store,
		config:    config,
		validator: NewEventValidator(),
	}
}

// SyncResult contains the results of a sync operation.
type SyncResult struct {
	EventsProcessed int64
	EventsSkipped   int64 // Invalid or duplicate events
	DuplicatesFound int64
	InvalidEvents   int64
	NewPosition     int64
	Duration        time.Duration
	Errors          []error
	Warnings        []string
}

// Sync processes new events since last sync position.
func (s *SyncEngine) Sync(ctx context.Context) (*SyncResult, error) {
	start := time.Now()
	result := &SyncResult{}

	// Get last position
	lastPos, err := s.store.GetSyncPosition(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting sync position: %w", err)
	}

	// Parse new events
	events, newPos, err := s.parser.ParseFromPosition(lastPos)
	if err != nil {
		return nil, fmt.Errorf("parsing events: %w", err)
	}

	if len(events) == 0 {
		result.NewPosition = newPos
		result.Duration = time.Since(start)
		return result, nil
	}

	// Process events in batches
	for i := 0; i < len(events); i += s.config.BatchSize {
		end := i + s.config.BatchSize
		if end > len(events) {
			end = len(events)
		}

		batch := events[i:end]
		if err := s.processBatch(ctx, batch, result); err != nil {
			result.Errors = append(result.Errors, err)
		}
		result.EventsProcessed += int64(len(batch))
	}

	// Collect validation warnings
	result.Warnings = append(result.Warnings, s.validator.Warnings...)

	// Update sync position
	if err := s.store.SetSyncPosition(ctx, newPos); err != nil {
		return result, fmt.Errorf("updating sync position: %w", err)
	}

	result.NewPosition = newPos
	result.Duration = time.Since(start)
	return result, nil
}

// processBatch processes a batch of events with validation and deduplication.
func (s *SyncEngine) processBatch(ctx context.Context, events []*Event, result *SyncResult) error {
	for _, event := range events {
		// Validate event
		if !s.validator.Validate(event) {
			result.InvalidEvents++
			result.EventsSkipped++
			continue
		}

		// Check for duplicates
		fingerprint := EventFingerprint(event)
		isDup, err := s.store.HasEventFingerprint(ctx, fingerprint)
		if err != nil {
			// Log but continue - treat as not duplicate
			fmt.Printf("warning: error checking fingerprint: %v\n", err)
		}
		if isDup {
			result.DuplicatesFound++
			result.EventsSkipped++
			continue
		}

		// Process the event
		if err := s.processEvent(ctx, event); err != nil {
			// Log error but continue processing
			fmt.Printf("warning: error processing event: %v\n", err)
			continue
		}

		// Store fingerprint after successful processing
		if err := s.store.StoreEventFingerprint(ctx, fingerprint, event.Timestamp); err != nil {
			fmt.Printf("warning: error storing fingerprint: %v\n", err)
		}
	}
	return nil
}

// processEvent processes a single event.
func (s *SyncEngine) processEvent(ctx context.Context, event *Event) error {
	switch event.EventType {
	case "SessionStart":
		return s.store.UpsertSession(ctx, event.SessionID, event.Cwd, event.Timestamp)

	case "SessionEnd", "Stop":
		return s.store.UpdateSessionEnd(ctx, event.SessionID, event.Timestamp)

	case "PostToolUse":
		// Extract MCP server
		serverName := ExtractMCPServer(event.ToolName)

		// Update tool stats
		date := event.Timestamp.Format("2006-01-02")
		var errors int64
		if !event.Success {
			errors = 1
		}
		if err := s.store.UpsertToolStats(ctx, date, event.ToolName, serverName, 1, errors, event.DurationMs); err != nil {
			return err
		}

		// Update session stats
		if err := s.store.IncrementSessionStats(ctx, event.SessionID, 1, errors); err != nil {
			return err
		}

		// Insert into recent events
		if err := s.store.InsertRecentEvent(ctx, event.Timestamp, event.SessionID, event.EventType, event.ToolName, serverName, event.DurationMs, event.Success); err != nil {
			return err
		}
	}

	return nil
}

// Reset clears the sync position to re-process all events.
func (s *SyncEngine) Reset(ctx context.Context) error {
	return s.store.SetSyncPosition(ctx, 0)
}

// GetLastPosition returns the last synced position.
func (s *SyncEngine) GetLastPosition(ctx context.Context) (int64, error) {
	return s.store.GetSyncPosition(ctx)
}
