//go:build sqlite

package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSQLiteStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestStoreEvent(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()
	event := &Event{
		SessionID:    "session-123",
		EventType:    "PostToolUse",
		ToolName:     "Read",
		MCPServer:    "filesystem",
		Success:      true,
		DurationMs:   150,
		InputTokens:  100,
		OutputTokens: 200,
		CostUSD:      0.001,
		RawPayload:   []byte(`{"test": "data"}`),
	}

	err := store.StoreEvent(ctx, event)
	if err != nil {
		t.Fatalf("failed to store event: %v", err)
	}

	// Verify event was assigned an ID
	if event.ID == 0 {
		t.Error("event ID should be assigned after storage")
	}

	// Retrieve and verify
	events, err := store.GetEvents(ctx, EventFilter{
		SessionID: "session-123",
	})
	if err != nil {
		t.Fatalf("failed to get events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	got := events[0]
	if got.SessionID != event.SessionID {
		t.Errorf("expected session ID %s, got %s", event.SessionID, got.SessionID)
	}
	if got.ToolName != event.ToolName {
		t.Errorf("expected tool name %s, got %s", event.ToolName, got.ToolName)
	}
	if got.MCPServer != event.MCPServer {
		t.Errorf("expected MCP server %s, got %s", event.MCPServer, got.MCPServer)
	}
	if got.Success != event.Success {
		t.Errorf("expected success %v, got %v", event.Success, got.Success)
	}
	if got.DurationMs != event.DurationMs {
		t.Errorf("expected duration %d, got %d", event.DurationMs, got.DurationMs)
	}
}

func TestGetEventsWithFilter(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Store multiple events
	events := []*Event{
		{SessionID: "session-1", EventType: "PostToolUse", ToolName: "Read", MCPServer: "filesystem"},
		{SessionID: "session-1", EventType: "PostToolUse", ToolName: "Write", MCPServer: "filesystem"},
		{SessionID: "session-2", EventType: "PostToolUse", ToolName: "Bash", MCPServer: ""},
		{SessionID: "session-2", EventType: "SessionEnd", ToolName: "", MCPServer: ""},
	}

	for _, e := range events {
		if err := store.StoreEvent(ctx, e); err != nil {
			t.Fatalf("failed to store event: %v", err)
		}
	}

	// Test filter by session
	filtered, err := store.GetEvents(ctx, EventFilter{SessionID: "session-1"})
	if err != nil {
		t.Fatalf("failed to get events: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 events for session-1, got %d", len(filtered))
	}

	// Test filter by event type
	filtered, err = store.GetEvents(ctx, EventFilter{EventTypes: []string{"SessionEnd"}})
	if err != nil {
		t.Fatalf("failed to get events: %v", err)
	}
	if len(filtered) != 1 {
		t.Errorf("expected 1 SessionEnd event, got %d", len(filtered))
	}

	// Test filter by MCP server
	filtered, err = store.GetEvents(ctx, EventFilter{MCPServers: []string{"filesystem"}})
	if err != nil {
		t.Fatalf("failed to get events: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 filesystem events, got %d", len(filtered))
	}

	// Test limit
	filtered, err = store.GetEvents(ctx, EventFilter{Limit: 2})
	if err != nil {
		t.Fatalf("failed to get events: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 events with limit, got %d", len(filtered))
	}
}

func TestGetSession(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Store a session event
	event := &Event{
		SessionID: "session-123",
		EventType: "SessionStart",
	}
	if err := store.StoreEvent(ctx, event); err != nil {
		t.Fatalf("failed to store event: %v", err)
	}

	session, err := store.GetSession(ctx, "session-123")
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if session.ID != "session-123" {
		t.Errorf("expected session ID session-123, got %s", session.ID)
	}
}

func TestGetMCPServerStats(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Store events for different MCP servers
	events := []*Event{
		{SessionID: "s1", EventType: "PostToolUse", ToolName: "Read", MCPServer: "filesystem", Success: true, DurationMs: 100},
		{SessionID: "s1", EventType: "PostToolUse", ToolName: "Read", MCPServer: "filesystem", Success: true, DurationMs: 200},
		{SessionID: "s1", EventType: "PostToolUse", ToolName: "Read", MCPServer: "filesystem", Success: false, DurationMs: 50},
		{SessionID: "s1", EventType: "PostToolUse", ToolName: "create_issue", MCPServer: "github", Success: true, DurationMs: 500},
	}

	for _, e := range events {
		if err := store.StoreEvent(ctx, e); err != nil {
			t.Fatalf("failed to store event: %v", err)
		}
	}

	stats, err := store.GetMCPServerStats(ctx, TimeFilter{})
	if err != nil {
		t.Fatalf("failed to get MCP stats: %v", err)
	}

	if len(stats) < 2 {
		t.Fatalf("expected at least 2 MCP servers, got %d", len(stats))
	}

	// Find filesystem stats
	var fsStats *MCPServerStats
	for i := range stats {
		if stats[i].ServerName == "filesystem" {
			fsStats = &stats[i]
			break
		}
	}

	if fsStats == nil {
		t.Fatal("filesystem stats not found")
	}

	if fsStats.TotalCalls != 3 {
		t.Errorf("expected 3 calls for filesystem, got %d", fsStats.TotalCalls)
	}
	if fsStats.SuccessCount != 2 {
		t.Errorf("expected 2 successes for filesystem, got %d", fsStats.SuccessCount)
	}
	if fsStats.ErrorCount != 1 {
		t.Errorf("expected 1 error for filesystem, got %d", fsStats.ErrorCount)
	}
}

func TestGetToolStats(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	events := []*Event{
		{SessionID: "s1", EventType: "PostToolUse", ToolName: "Read", MCPServer: "filesystem", Success: true},
		{SessionID: "s1", EventType: "PostToolUse", ToolName: "Read", MCPServer: "filesystem", Success: true},
		{SessionID: "s1", EventType: "PostToolUse", ToolName: "Write", MCPServer: "filesystem", Success: false},
	}

	for _, e := range events {
		if err := store.StoreEvent(ctx, e); err != nil {
			t.Fatalf("failed to store event: %v", err)
		}
	}

	stats, err := store.GetToolStats(ctx, TimeFilter{})
	if err != nil {
		t.Fatalf("failed to get tool stats: %v", err)
	}

	if len(stats) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(stats))
	}
}

func TestCleanup(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	// Store some events
	for i := 0; i < 5; i++ {
		event := &Event{
			SessionID: "session-old",
			EventType: "PostToolUse",
			ToolName:  "Read",
		}
		if err := store.StoreEvent(ctx, event); err != nil {
			t.Fatalf("failed to store event: %v", err)
		}
	}

	// Cleanup events older than now (should remove all)
	deleted, err := store.Cleanup(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("failed to cleanup: %v", err)
	}

	if deleted != 5 {
		t.Errorf("expected 5 deleted, got %d", deleted)
	}

	// Verify no events remain
	events, err := store.GetEvents(ctx, EventFilter{})
	if err != nil {
		t.Fatalf("failed to get events: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events after cleanup, got %d", len(events))
	}
}

func TestGetCostSummary(t *testing.T) {
	store := createTestStore(t)
	defer store.Close()

	ctx := context.Background()

	events := []*Event{
		{SessionID: "s1", EventType: "PostToolUse", InputTokens: 100, OutputTokens: 200, CostUSD: 0.01},
		{SessionID: "s1", EventType: "PostToolUse", InputTokens: 200, OutputTokens: 300, CostUSD: 0.02},
	}

	for _, e := range events {
		if err := store.StoreEvent(ctx, e); err != nil {
			t.Fatalf("failed to store event: %v", err)
		}
	}

	summary, err := store.GetCostSummary(ctx, TimeFilter{})
	if err != nil {
		t.Fatalf("failed to get cost summary: %v", err)
	}

	if summary.TotalTokens != 800 {
		t.Errorf("expected 800 total tokens, got %d", summary.TotalTokens)
	}
	if summary.TotalCostUSD != 0.03 {
		t.Errorf("expected 0.03 total cost, got %f", summary.TotalCostUSD)
	}
}

// Helper to create a test store
func createTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	return store
}
