package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// MockSyncStore implements SyncStore for testing.
type MockSyncStore struct {
	syncPosition     int64
	toolStats        map[string]*mockToolStat
	sessions         map[string]*mockSession
	recentEvents     []*Event
	fingerprints     map[string]time.Time
	upsertCalls      int
	insertEventCalls int
}

type mockToolStat struct {
	calls     int64
	errors    int64
	latencyMs int64
}

type mockSession struct {
	id         string
	cwd        string
	startedAt  time.Time
	endedAt    *time.Time
	toolCalls  int64
	errors     int64
}

func NewMockSyncStore() *MockSyncStore {
	return &MockSyncStore{
		toolStats:    make(map[string]*mockToolStat),
		sessions:     make(map[string]*mockSession),
		recentEvents: make([]*Event, 0),
		fingerprints: make(map[string]time.Time),
	}
}

func (m *MockSyncStore) GetSyncPosition(ctx context.Context) (int64, error) {
	return m.syncPosition, nil
}

func (m *MockSyncStore) SetSyncPosition(ctx context.Context, pos int64) error {
	m.syncPosition = pos
	return nil
}

func (m *MockSyncStore) UpsertToolStats(ctx context.Context, date string, toolName string, serverName string, calls int64, errors int64, latencyMs int64) error {
	m.upsertCalls++
	key := date + "|" + toolName
	if m.toolStats[key] == nil {
		m.toolStats[key] = &mockToolStat{}
	}
	m.toolStats[key].calls += calls
	m.toolStats[key].errors += errors
	m.toolStats[key].latencyMs += latencyMs
	return nil
}

func (m *MockSyncStore) UpsertSession(ctx context.Context, id string, cwd string, startedAt time.Time) error {
	m.sessions[id] = &mockSession{
		id:        id,
		cwd:       cwd,
		startedAt: startedAt,
	}
	return nil
}

func (m *MockSyncStore) UpdateSessionEnd(ctx context.Context, id string, endedAt time.Time) error {
	if sess, ok := m.sessions[id]; ok {
		sess.endedAt = &endedAt
	}
	return nil
}

func (m *MockSyncStore) IncrementSessionStats(ctx context.Context, id string, toolCalls int64, errors int64) error {
	if sess, ok := m.sessions[id]; ok {
		sess.toolCalls += toolCalls
		sess.errors += errors
	}
	return nil
}

func (m *MockSyncStore) InsertRecentEvent(ctx context.Context, timestamp time.Time, sessionID string, eventType string, toolName string, serverName string, durationMs int64, success bool) error {
	m.insertEventCalls++
	event := &Event{
		Timestamp:  timestamp,
		SessionID:  sessionID,
		EventType:  eventType,
		ToolName:   toolName,
		DurationMs: durationMs,
		Success:    success,
	}
	m.recentEvents = append(m.recentEvents, event)
	return nil
}

func (m *MockSyncStore) HasEventFingerprint(ctx context.Context, fingerprint string) (bool, error) {
	_, exists := m.fingerprints[fingerprint]
	return exists, nil
}

func (m *MockSyncStore) StoreEventFingerprint(ctx context.Context, fingerprint string, timestamp time.Time) error {
	m.fingerprints[fingerprint] = timestamp
	return nil
}

func TestSyncEngine_Sync_NewEvents(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	content := `{"ts":"2026-01-10T10:00:00Z","sid":"sess-1","type":"SessionStart","cwd":"/home/user"}
{"ts":"2026-01-10T10:01:00Z","sid":"sess-1","type":"PostToolUse","tool":"Read","ok":true,"dur_ms":50}
{"ts":"2026-01-10T10:02:00Z","sid":"sess-1","type":"PostToolUse","tool":"mcp__github__create_issue","ok":true,"dur_ms":200}
{"ts":"2026-01-10T10:03:00Z","sid":"sess-1","type":"SessionEnd"}
`
	if err := os.WriteFile(eventsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	store := NewMockSyncStore()
	config := SyncConfig{
		EventsFile: eventsFile,
		BatchSize:  1000,
	}
	engine := NewSyncEngine(config, store)

	ctx := context.Background()
	result, err := engine.Sync(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.EventsProcessed != 4 {
		t.Errorf("expected 4 events processed, got %d", result.EventsProcessed)
	}

	// Check session was created
	if sess, ok := store.sessions["sess-1"]; !ok {
		t.Error("expected session 'sess-1' to be created")
	} else {
		if sess.cwd != "/home/user" {
			t.Errorf("expected cwd '/home/user', got '%s'", sess.cwd)
		}
		if sess.endedAt == nil {
			t.Error("expected session to have end time")
		}
	}

	// Check tool stats (should have 2 PostToolUse events)
	if store.upsertCalls != 2 {
		t.Errorf("expected 2 upsert calls, got %d", store.upsertCalls)
	}

	// Check recent events inserted
	if store.insertEventCalls != 2 {
		t.Errorf("expected 2 recent event inserts, got %d", store.insertEventCalls)
	}
}

func TestSyncEngine_Sync_IncrementalSync(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	// Write initial events
	content1 := `{"ts":"2026-01-10T10:00:00Z","sid":"sess-1","type":"SessionStart"}
{"ts":"2026-01-10T10:01:00Z","sid":"sess-1","type":"PostToolUse","tool":"Read","ok":true}
`
	if err := os.WriteFile(eventsFile, []byte(content1), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	store := NewMockSyncStore()
	config := SyncConfig{
		EventsFile: eventsFile,
		BatchSize:  1000,
	}
	engine := NewSyncEngine(config, store)

	ctx := context.Background()

	// First sync
	result1, err := engine.Sync(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result1.EventsProcessed != 2 {
		t.Errorf("expected 2 events on first sync, got %d", result1.EventsProcessed)
	}

	// Add more events
	content2 := `{"ts":"2026-01-10T10:02:00Z","sid":"sess-1","type":"PostToolUse","tool":"Write","ok":true}
{"ts":"2026-01-10T10:03:00Z","sid":"sess-1","type":"SessionEnd"}
`
	f, err := os.OpenFile(eventsFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open file for append: %v", err)
	}
	if _, err := f.WriteString(content2); err != nil {
		f.Close()
		t.Fatalf("failed to append to file: %v", err)
	}
	f.Close()

	// Second sync - should only process new events
	result2, err := engine.Sync(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result2.EventsProcessed != 2 {
		t.Errorf("expected 2 events on second sync, got %d", result2.EventsProcessed)
	}

	// Total upserts should be 2 (2 PostToolUse events)
	if store.upsertCalls != 2 {
		t.Errorf("expected 2 total upsert calls, got %d", store.upsertCalls)
	}
}

func TestSyncEngine_Sync_NoNewEvents(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	content := `{"ts":"2026-01-10T10:00:00Z","sid":"sess-1","type":"SessionStart"}
`
	if err := os.WriteFile(eventsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	store := NewMockSyncStore()
	config := SyncConfig{
		EventsFile: eventsFile,
		BatchSize:  1000,
	}
	engine := NewSyncEngine(config, store)

	ctx := context.Background()

	// First sync
	engine.Sync(ctx)

	// Second sync - no new events
	result, err := engine.Sync(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.EventsProcessed != 0 {
		t.Errorf("expected 0 events on second sync, got %d", result.EventsProcessed)
	}
}

func TestSyncEngine_Sync_NonExistentFile(t *testing.T) {
	store := NewMockSyncStore()
	config := SyncConfig{
		EventsFile: "/nonexistent/events.jsonl",
		BatchSize:  1000,
	}
	engine := NewSyncEngine(config, store)

	ctx := context.Background()
	result, err := engine.Sync(ctx)
	if err != nil {
		t.Fatalf("expected no error for non-existent file, got: %v", err)
	}
	if result.EventsProcessed != 0 {
		t.Errorf("expected 0 events, got %d", result.EventsProcessed)
	}
}

func TestSyncEngine_Reset(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	content := `{"ts":"2026-01-10T10:00:00Z","sid":"sess-1","type":"SessionStart"}
`
	if err := os.WriteFile(eventsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	store := NewMockSyncStore()
	config := SyncConfig{
		EventsFile: eventsFile,
		BatchSize:  1000,
	}
	engine := NewSyncEngine(config, store)

	ctx := context.Background()

	// Sync once
	engine.Sync(ctx)
	if store.syncPosition == 0 {
		t.Error("expected sync position to be updated")
	}

	// Reset
	if err := engine.Reset(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pos, _ := engine.GetLastPosition(ctx)
	if pos != 0 {
		t.Errorf("expected position 0 after reset, got %d", pos)
	}
}

func TestSyncEngine_MCPServerExtraction(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	// Events with MCP tools
	content := `{"ts":"2026-01-10T10:00:00Z","sid":"sess-1","type":"PostToolUse","tool":"mcp__github__create_issue","ok":true}
{"ts":"2026-01-10T10:01:00Z","sid":"sess-1","type":"PostToolUse","tool":"mcp__asana__get_tasks","ok":false}
{"ts":"2026-01-10T10:02:00Z","sid":"sess-1","type":"PostToolUse","tool":"Read","ok":true}
`
	if err := os.WriteFile(eventsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	store := NewMockSyncStore()
	config := SyncConfig{
		EventsFile: eventsFile,
		BatchSize:  1000,
	}
	engine := NewSyncEngine(config, store)

	ctx := context.Background()
	result, err := engine.Sync(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.EventsProcessed != 3 {
		t.Errorf("expected 3 events, got %d", result.EventsProcessed)
	}

	// Should have 3 upsert calls (one per PostToolUse)
	if store.upsertCalls != 3 {
		t.Errorf("expected 3 upsert calls, got %d", store.upsertCalls)
	}
}

func TestSyncEngine_BatchProcessing(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	// Write 10 events with unique timestamps (for deduplication)
	var content string
	for i := 0; i < 10; i++ {
		// Each event has a unique timestamp to avoid deduplication
		ts := time.Date(2026, 1, 10, 10, 0, i, 0, time.UTC).Format(time.RFC3339Nano)
		content += `{"ts":"` + ts + `","sid":"sess-1","type":"PostToolUse","tool":"Read","ok":true,"dur_ms":10}` + "\n"
	}
	if err := os.WriteFile(eventsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	store := NewMockSyncStore()
	config := SyncConfig{
		EventsFile: eventsFile,
		BatchSize:  3, // Small batch size to test batching
	}
	engine := NewSyncEngine(config, store)

	ctx := context.Background()
	result, err := engine.Sync(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.EventsProcessed != 10 {
		t.Errorf("expected 10 events, got %d", result.EventsProcessed)
	}

	// All 10 events should be processed (each has unique fingerprint)
	if store.upsertCalls != 10 {
		t.Errorf("expected 10 upsert calls, got %d", store.upsertCalls)
	}

	// No duplicates should be found
	if result.DuplicatesFound != 0 {
		t.Errorf("expected 0 duplicates, got %d", result.DuplicatesFound)
	}
}

func TestDefaultSyncConfig(t *testing.T) {
	config := DefaultSyncConfig()

	if config.EventsFile != "~/.mcp-lens/events.jsonl" {
		t.Errorf("unexpected default events file: %s", config.EventsFile)
	}
	if config.BatchSize != 1000 {
		t.Errorf("expected batch size 1000, got %d", config.BatchSize)
	}
	if config.DataDir != "~/.mcp-lens" {
		t.Errorf("unexpected default data dir: %s", config.DataDir)
	}
}

func TestSyncEngine_Deduplication(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	// Write identical events (should be deduplicated)
	content := `{"ts":"2026-01-10T10:00:00Z","sid":"sess-1","type":"PostToolUse","tool":"Read","ok":true,"dur_ms":10}
{"ts":"2026-01-10T10:00:00Z","sid":"sess-1","type":"PostToolUse","tool":"Read","ok":true,"dur_ms":10}
{"ts":"2026-01-10T10:00:00Z","sid":"sess-1","type":"PostToolUse","tool":"Read","ok":true,"dur_ms":10}
`
	if err := os.WriteFile(eventsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	store := NewMockSyncStore()
	config := SyncConfig{
		EventsFile: eventsFile,
		BatchSize:  1000,
	}
	engine := NewSyncEngine(config, store)

	ctx := context.Background()
	result, err := engine.Sync(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All 3 lines were processed
	if result.EventsProcessed != 3 {
		t.Errorf("expected 3 events processed, got %d", result.EventsProcessed)
	}

	// 2 duplicates should be found
	if result.DuplicatesFound != 2 {
		t.Errorf("expected 2 duplicates, got %d", result.DuplicatesFound)
	}

	// Only 1 event should be stored (the first one)
	if store.upsertCalls != 1 {
		t.Errorf("expected 1 upsert call, got %d", store.upsertCalls)
	}

	// 2 events should be skipped
	if result.EventsSkipped != 2 {
		t.Errorf("expected 2 skipped, got %d", result.EventsSkipped)
	}
}

func TestSyncEngine_Validation(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	// Write events with validation issues
	content := `{"ts":"2026-01-10T10:00:00Z","sid":"","type":"PostToolUse","tool":"Read","ok":true,"dur_ms":10}
{"ts":"2026-01-10T10:00:01Z","sid":"sess-1","type":"","ok":true,"dur_ms":10}
{"ts":"2026-01-10T10:00:02Z","sid":"sess-1","type":"UnknownType","ok":true}
{"ts":"2026-01-10T10:00:03Z","sid":"sess-1","type":"PostToolUse","tool":"","ok":true}
{"ts":"2026-01-10T10:00:04Z","sid":"sess-1","type":"PostToolUse","tool":"Read","ok":true,"dur_ms":10}
`
	if err := os.WriteFile(eventsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	store := NewMockSyncStore()
	config := SyncConfig{
		EventsFile: eventsFile,
		BatchSize:  1000,
	}
	engine := NewSyncEngine(config, store)

	ctx := context.Background()
	result, err := engine.Sync(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All 5 lines were processed
	if result.EventsProcessed != 5 {
		t.Errorf("expected 5 events processed, got %d", result.EventsProcessed)
	}

	// 4 invalid events (missing session_id, missing type, unknown type, missing tool_name)
	if result.InvalidEvents != 4 {
		t.Errorf("expected 4 invalid events, got %d", result.InvalidEvents)
	}

	// Only 1 valid event should be stored
	if store.upsertCalls != 1 {
		t.Errorf("expected 1 upsert call, got %d", store.upsertCalls)
	}

	// 4 events should be skipped
	if result.EventsSkipped != 4 {
		t.Errorf("expected 4 skipped, got %d", result.EventsSkipped)
	}

	// Warnings should be recorded
	if len(result.Warnings) != 4 {
		t.Errorf("expected 4 warnings, got %d", len(result.Warnings))
	}
}
