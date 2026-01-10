package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/anthropics/mcp-lens/internal/hooks"
	"github.com/anthropics/mcp-lens/internal/metrics"
	"github.com/anthropics/mcp-lens/internal/storage"
	"github.com/anthropics/mcp-lens/internal/web"
)

// TestEnv holds the integration test environment.
type TestEnv struct {
	Store     storage.Store
	MockStore *storage.MockStore
	Receiver  *hooks.Receiver
	Processor *hooks.Processor
	WebServer *web.Server
	ctx       context.Context
	cancel    context.CancelFunc
	hookPort  int
	webPort   int
}

// SetupTestEnv creates a complete test environment using MockStore.
func SetupTestEnv(t *testing.T, hookPort, webPort int) *TestEnv {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	// Create mock store
	mockStore := storage.NewMockStore()

	// Create receiver
	receiverConfig := hooks.ReceiverConfig{
		Port:        hookPort,
		BindAddress: "127.0.0.1",
	}
	receiver := hooks.NewReceiver(receiverConfig)

	// Start receiver
	if err := receiver.Start(ctx); err != nil {
		cancel()
		t.Fatalf("Failed to start receiver: %v", err)
	}

	// Create and start processor
	processor := hooks.NewProcessor(mockStore, receiver.Events())
	processor.Start(ctx)

	// Create web server
	webConfig := web.ServerConfig{
		Port:        webPort,
		BindAddress: "127.0.0.1",
	}
	webServer, err := web.NewServer(webConfig, mockStore)
	if err != nil {
		receiver.Stop(ctx)
		cancel()
		t.Fatalf("Failed to create web server: %v", err)
	}

	// Start web server
	go webServer.Start(ctx)
	time.Sleep(100 * time.Millisecond) // Give server time to start

	return &TestEnv{
		Store:     mockStore,
		MockStore: mockStore,
		Receiver:  receiver,
		Processor: processor,
		WebServer: webServer,
		ctx:       ctx,
		cancel:    cancel,
		hookPort:  hookPort,
		webPort:   webPort,
	}
}

// Teardown cleans up the test environment.
func (e *TestEnv) Teardown() {
	if e.WebServer != nil {
		e.WebServer.Stop(e.ctx)
	}
	if e.Receiver != nil {
		e.Receiver.Stop(e.ctx)
	}
	if e.Processor != nil {
		e.Processor.Stop()
	}
	if e.Store != nil {
		e.Store.Close()
	}
	e.cancel()
}

// PostHookEvent sends a hook event to the receiver.
func (e *TestEnv) PostHookEvent(t *testing.T, event interface{}) *http.Response {
	t.Helper()

	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	url := fmt.Sprintf("http://%s/hook", e.Receiver.Address())
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to POST event: %v", err)
	}

	return resp
}

// === Integration Tests ===

func TestIT_HOOK_001_ReceiverAcceptsValidEvents(t *testing.T) {
	env := SetupTestEnv(t, 19876, 19877)
	defer env.Teardown()

	// Test PreToolUse event
	preEvent := map[string]interface{}{
		"session_id":      "test-session-001",
		"hook_event_name": "PreToolUse",
		"tool_name":       "mcp__everything__echo",
		"tool_input": map[string]interface{}{
			"message": "Hello, MCP Lens!",
		},
	}

	resp := env.PostHookEvent(t, preEvent)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("PreToolUse: expected 200, got %d", resp.StatusCode)
	}

	// Test PostToolUse event
	postEvent := map[string]interface{}{
		"session_id":      "test-session-001",
		"hook_event_name": "PostToolUse",
		"tool_name":       "mcp__everything__echo",
		"tool_input": map[string]interface{}{
			"message": "Hello, MCP Lens!",
		},
		"tool_response": map[string]interface{}{
			"content": "Echo: Hello, MCP Lens!",
		},
	}

	resp2 := env.PostHookEvent(t, postEvent)
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("PostToolUse: expected 200, got %d", resp2.StatusCode)
	}

	t.Log("IT-HOOK-001: PASSED - Receiver accepts valid events")
}

func TestIT_HOOK_002_ReceiverRejectsInvalidEvents(t *testing.T) {
	env := SetupTestEnv(t, 19878, 19879)
	defer env.Teardown()

	url := fmt.Sprintf("http://%s/hook", env.Receiver.Address())

	// Test invalid JSON
	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte("invalid json")))
	if err != nil {
		t.Fatalf("Failed to POST: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Invalid JSON: expected 400, got %d", resp.StatusCode)
	}

	// Test missing required fields
	emptyEvent := map[string]interface{}{}
	body, _ := json.Marshal(emptyEvent)
	resp2, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to POST: %v", err)
	}
	resp2.Body.Close()

	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("Empty event: expected 400, got %d", resp2.StatusCode)
	}

	// Test unknown event type
	unknownEvent := map[string]interface{}{
		"session_id":      "test-session",
		"hook_event_name": "UnknownEventType",
	}
	body3, _ := json.Marshal(unknownEvent)
	resp3, err := http.Post(url, "application/json", bytes.NewReader(body3))
	if err != nil {
		t.Fatalf("Failed to POST: %v", err)
	}
	resp3.Body.Close()

	if resp3.StatusCode != http.StatusBadRequest {
		t.Errorf("Unknown event: expected 400, got %d", resp3.StatusCode)
	}

	t.Log("IT-HOOK-002: PASSED - Receiver rejects invalid events")
}

func TestIT_PROC_001_ProcessorStoresEventsCorrectly(t *testing.T) {
	env := SetupTestEnv(t, 19880, 19881)
	defer env.Teardown()

	sessionID := "test-session-proc-001"

	// Send PreToolUse event
	preEvent := map[string]interface{}{
		"session_id":      sessionID,
		"hook_event_name": "PreToolUse",
		"tool_name":       "mcp__everything__add",
		"tool_input": map[string]interface{}{
			"a": 5,
			"b": 3,
		},
	}
	env.PostHookEvent(t, preEvent).Body.Close()

	// Small delay to ensure processing
	time.Sleep(100 * time.Millisecond)

	// Send PostToolUse event
	postEvent := map[string]interface{}{
		"session_id":      sessionID,
		"hook_event_name": "PostToolUse",
		"tool_name":       "mcp__everything__add",
		"tool_input": map[string]interface{}{
			"a": 5,
			"b": 3,
		},
		"tool_response": map[string]interface{}{
			"result": 8,
		},
	}
	env.PostHookEvent(t, postEvent).Body.Close()

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Query events from storage
	events, err := env.Store.GetEvents(env.ctx, storage.EventFilter{
		SessionID: sessionID,
	})
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	// Should have at least 1 event (PostToolUse - PreToolUse may not be stored)
	if len(events) < 1 {
		t.Errorf("Expected at least 1 event, got %d", len(events))
	}

	// Verify event fields
	found := false
	for _, e := range events {
		if e.ToolName == "mcp__everything__add" {
			found = true
			if e.MCPServer != "everything" {
				t.Errorf("Expected MCP server 'everything', got '%s'", e.MCPServer)
			}
			break
		}
	}

	if !found {
		t.Error("Expected to find event with tool_name 'mcp__everything__add'")
	}

	t.Log("IT-PROC-001: PASSED - Processor stores events correctly")
}

func TestIT_PROC_002_ProcessorHandlesMCPToolIdentification(t *testing.T) {
	env := SetupTestEnv(t, 19882, 19883)
	defer env.Teardown()

	sessionID := "test-session-proc-002"

	// Send events for different MCP servers
	mcpEvents := []struct {
		toolName       string
		expectedServer string
	}{
		{"mcp__filesystem__read_file", "filesystem"},
		{"mcp__github__create_issue", "github"},
		{"mcp__everything__echo", "everything"},
	}

	for _, tc := range mcpEvents {
		// PreToolUse
		preEvent := map[string]interface{}{
			"session_id":      sessionID,
			"hook_event_name": "PreToolUse",
			"tool_name":       tc.toolName,
			"tool_input":      map[string]interface{}{"test": true},
		}
		env.PostHookEvent(t, preEvent).Body.Close()
		time.Sleep(50 * time.Millisecond)

		// PostToolUse
		postEvent := map[string]interface{}{
			"session_id":      sessionID,
			"hook_event_name": "PostToolUse",
			"tool_name":       tc.toolName,
			"tool_input":      map[string]interface{}{"test": true},
			"tool_response":   map[string]interface{}{"ok": true},
		}
		env.PostHookEvent(t, postEvent).Body.Close()
	}

	// Wait for processing
	time.Sleep(300 * time.Millisecond)

	// Query MCP server stats
	mcpStats, err := env.Store.GetMCPServerStats(env.ctx, storage.TimeFilter{})
	if err != nil {
		t.Fatalf("Failed to get MCP stats: %v", err)
	}

	expectedServers := map[string]bool{
		"filesystem": true,
		"github":     true,
		"everything": true,
	}

	for _, stat := range mcpStats {
		if expectedServers[stat.ServerName] {
			delete(expectedServers, stat.ServerName)
			t.Logf("Found MCP server: %s (calls: %d)", stat.ServerName, stat.TotalCalls)
		}
	}

	if len(expectedServers) > 0 {
		t.Errorf("Missing MCP servers: %v", expectedServers)
	}

	t.Log("IT-PROC-002: PASSED - MCP tool identification works")
}

func TestIT_STOR_001_MockStorePersistsData(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMockStore()
	defer store.Close()

	event := &storage.Event{
		SessionID:  "persist-test-session",
		EventType:  "PostToolUse",
		ToolName:   "mcp__test__tool",
		MCPServer:  "test",
		Success:    true,
		DurationMs: 100,
		CreatedAt:  time.Now(),
	}

	if err := store.StoreEvent(ctx, event); err != nil {
		t.Fatalf("Failed to store event: %v", err)
	}

	// Query for events
	events, err := store.GetEvents(ctx, storage.EventFilter{
		SessionID: "persist-test-session",
	})
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	if events[0].ToolName != "mcp__test__tool" {
		t.Errorf("Expected tool name 'mcp__test__tool', got '%s'", events[0].ToolName)
	}

	t.Log("IT-STOR-001: PASSED - MockStore persists data correctly")
}

func TestIT_STOR_002_StorageTimeFilteringWorks(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMockStore()
	defer store.Close()

	// Create events with different timestamps
	now := time.Now()
	events := []*storage.Event{
		{SessionID: "s1", EventType: "PostToolUse", ToolName: "t1", CreatedAt: now.Add(-2 * time.Hour)},
		{SessionID: "s1", EventType: "PostToolUse", ToolName: "t2", CreatedAt: now.Add(-30 * time.Minute)},
		{SessionID: "s1", EventType: "PostToolUse", ToolName: "t3", CreatedAt: now.Add(-5 * time.Minute)},
	}

	for _, e := range events {
		if err := store.StoreEvent(ctx, e); err != nil {
			t.Fatalf("Failed to store event: %v", err)
		}
	}

	// Query last hour
	oneHourAgo := now.Add(-1 * time.Hour)
	filtered, err := store.GetEvents(ctx, storage.EventFilter{
		TimeFilter: storage.TimeFilter{From: oneHourAgo},
	})
	if err != nil {
		t.Fatalf("Failed to get filtered events: %v", err)
	}

	if len(filtered) != 2 {
		t.Errorf("Expected 2 events within last hour, got %d", len(filtered))
	}

	t.Log("IT-STOR-002: PASSED - Time filtering works correctly")
}

func TestIT_METR_001_MetricsCalculatorAggregatesCorrectly(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMockStore()
	defer store.Close()

	// Create multiple events
	now := time.Now()
	for i := 0; i < 10; i++ {
		event := &storage.Event{
			SessionID:  "metrics-test",
			EventType:  "PostToolUse",
			ToolName:   fmt.Sprintf("tool_%d", i%3),
			MCPServer:  map[int]string{0: "mcp1", 1: "mcp2", 2: ""}[i%3],
			Success:    i%4 != 0, // 75% success rate
			DurationMs: int64(100 + i*10),
			CreatedAt:  now.Add(-time.Duration(i) * time.Minute),
		}
		if err := store.StoreEvent(ctx, event); err != nil {
			t.Fatalf("Failed to store event: %v", err)
		}
	}

	// Create calculator
	calc := metrics.NewCalculator(store)

	// Get dashboard summary
	summary, err := calc.GetDashboardSummary(ctx, storage.TimeFilter{})
	if err != nil {
		t.Fatalf("Failed to get dashboard summary: %v", err)
	}

	if summary.TotalEvents != 10 {
		t.Errorf("Expected 10 total events, got %d", summary.TotalEvents)
	}

	// Check MCP utilization
	mcpUtil, err := calc.GetMCPUtilization(ctx, storage.TimeFilter{})
	if err != nil {
		t.Fatalf("Failed to get MCP utilization: %v", err)
	}

	if len(mcpUtil) < 1 {
		t.Error("Expected at least 1 MCP server in utilization")
	}

	t.Log("IT-METR-001: PASSED - Metrics aggregation works correctly")
}

func TestIT_METR_002_MCPServerStatsCalculated(t *testing.T) {
	ctx := context.Background()
	store := storage.NewMockStore()
	defer store.Close()

	// Create events for multiple MCP servers with varying latencies
	servers := []struct {
		name      string
		calls     int
		latencies []int64
		errors    int
	}{
		{"filesystem", 5, []int64{50, 60, 70, 80, 90}, 0},
		{"github", 3, []int64{100, 200, 300}, 1},
		{"everything", 4, []int64{20, 30, 40, 50}, 0},
	}

	now := time.Now()
	for _, srv := range servers {
		for i := 0; i < srv.calls; i++ {
			event := &storage.Event{
				SessionID:  "mcp-stats-test",
				EventType:  "PostToolUse",
				ToolName:   fmt.Sprintf("mcp__%s__tool%d", srv.name, i),
				MCPServer:  srv.name,
				Success:    i >= srv.errors,
				DurationMs: srv.latencies[i],
				CreatedAt:  now.Add(-time.Duration(i) * time.Minute),
			}
			if err := store.StoreEvent(ctx, event); err != nil {
				t.Fatalf("Failed to store event: %v", err)
			}
		}
	}

	// Get MCP server stats
	stats, err := store.GetMCPServerStats(ctx, storage.TimeFilter{})
	if err != nil {
		t.Fatalf("Failed to get MCP stats: %v", err)
	}

	if len(stats) != 3 {
		t.Errorf("Expected 3 MCP servers, got %d", len(stats))
	}

	// Verify specific stats
	statsMap := make(map[string]storage.MCPServerStats)
	for _, s := range stats {
		statsMap[s.ServerName] = s
	}

	// Check filesystem stats
	if fs, ok := statsMap["filesystem"]; ok {
		if fs.TotalCalls != 5 {
			t.Errorf("Filesystem: expected 5 calls, got %d", fs.TotalCalls)
		}
	} else {
		t.Error("Missing filesystem stats")
	}

	// Check github error count
	if gh, ok := statsMap["github"]; ok {
		if gh.ErrorCount != 1 {
			t.Errorf("Github: expected 1 error, got %d", gh.ErrorCount)
		}
	}

	t.Log("IT-METR-002: PASSED - MCP server stats calculated correctly")
}

func TestIT_WEB_001_DashboardLoadsSuccessfully(t *testing.T) {
	env := SetupTestEnv(t, 19890, 19891)
	defer env.Teardown()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", env.webPort)

	pages := []string{"/", "/mcp", "/tools", "/costs", "/sessions"}

	for _, page := range pages {
		resp, err := http.Get(baseURL + page)
		if err != nil {
			t.Errorf("Failed to GET %s: %v", page, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Page %s: expected 200, got %d", page, resp.StatusCode)
		}

		resp.Body.Close()
	}

	t.Log("IT-WEB-001: PASSED - Dashboard pages load successfully")
}

func TestIT_WEB_002_HTMXPartialsReturnData(t *testing.T) {
	env := SetupTestEnv(t, 19892, 19893)
	defer env.Teardown()

	// First, add some test data
	event := &storage.Event{
		SessionID:  "web-test-session",
		EventType:  "PostToolUse",
		ToolName:   "mcp__everything__echo",
		MCPServer:  "everything",
		Success:    true,
		DurationMs: 150,
		CreatedAt:  time.Now(),
	}
	if err := env.Store.StoreEvent(env.ctx, event); err != nil {
		t.Fatalf("Failed to store test event: %v", err)
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", env.webPort)

	partials := []string{
		"/partials/metrics",
		"/partials/mcp-table",
		"/partials/recent-events",
	}

	for _, partial := range partials {
		req, _ := http.NewRequest("GET", baseURL+partial, nil)
		req.Header.Set("HX-Request", "true") // Simulate HTMX request

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Errorf("Failed to GET %s: %v", partial, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Partial %s: expected 200, got %d", partial, resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if len(body) == 0 {
			t.Errorf("Partial %s: returned empty body", partial)
		}

		t.Logf("Partial %s: returned %d bytes", partial, len(body))
	}

	t.Log("IT-WEB-002: PASSED - HTMX partials return data")
}

func TestIT_IDENT_001_BuiltInToolsNotFlaggedAsMCP(t *testing.T) {
	identifier := hooks.NewRuleBasedIdentifier()

	builtInTools := []string{"Read", "Write", "Bash", "Glob", "Grep", "Edit", "WebFetch", "WebSearch"}

	for _, tool := range builtInTools {
		result := identifier.Identify(tool, nil)
		if result != "" {
			t.Errorf("Built-in tool %s incorrectly identified as MCP server: %s", tool, result)
		}
	}

	t.Log("IT-IDENT-001: PASSED - Built-in tools not flagged as MCP")
}

func TestIT_IDENT_002_MCPToolsCorrectlyIdentified(t *testing.T) {
	identifier := hooks.NewRuleBasedIdentifier()

	testCases := []struct {
		toolName       string
		expectedServer string
	}{
		{"mcp__filesystem__read_file", "filesystem"},
		{"mcp__github__create_pr", "github"},
		{"mcp__everything__echo", "everything"},
		{"mcp__notion__search", "notion"},
		{"mcp_memory_add", "memory"},
	}

	for _, tc := range testCases {
		result := identifier.Identify(tc.toolName, nil)
		if result != tc.expectedServer {
			t.Errorf("Tool %s: expected server '%s', got '%s'", tc.toolName, tc.expectedServer, result)
		}
	}

	t.Log("IT-IDENT-002: PASSED - MCP tools correctly identified")
}

// === End-to-End Test ===

func TestE2E_001_FullFlowWithSimulatedMCPEvents(t *testing.T) {
	env := SetupTestEnv(t, 19894, 19895)
	defer env.Teardown()

	sessionID := "e2e-test-session-001"

	// Simulate a sequence of MCP calls like Claude Code would generate

	// 1. Session start
	sessionStart := map[string]interface{}{
		"session_id":      sessionID,
		"hook_event_name": "SessionStart",
	}
	env.PostHookEvent(t, sessionStart).Body.Close()

	// 2. Multiple tool calls
	tools := []struct {
		name  string
		input map[string]interface{}
	}{
		{"mcp__everything__echo", map[string]interface{}{"message": "Hello"}},
		{"mcp__everything__add", map[string]interface{}{"a": 5, "b": 3}},
		{"mcp__everything__longRunningOperation", map[string]interface{}{"duration": 500}},
	}

	for _, tool := range tools {
		// PreToolUse
		pre := map[string]interface{}{
			"session_id":      sessionID,
			"hook_event_name": "PreToolUse",
			"tool_name":       tool.name,
			"tool_input":      tool.input,
		}
		env.PostHookEvent(t, pre).Body.Close()
		time.Sleep(50 * time.Millisecond)

		// PostToolUse
		post := map[string]interface{}{
			"session_id":      sessionID,
			"hook_event_name": "PostToolUse",
			"tool_name":       tool.name,
			"tool_input":      tool.input,
			"tool_response":   map[string]interface{}{"success": true},
		}
		env.PostHookEvent(t, post).Body.Close()
	}

	// 3. Session end
	sessionEnd := map[string]interface{}{
		"session_id":      sessionID,
		"hook_event_name": "SessionEnd",
	}
	env.PostHookEvent(t, sessionEnd).Body.Close()

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Verify data in storage
	events, err := env.Store.GetEvents(env.ctx, storage.EventFilter{SessionID: sessionID})
	if err != nil {
		t.Fatalf("Failed to get events: %v", err)
	}

	// Should have at least session events + tool events
	if len(events) < 3 {
		t.Errorf("Expected at least 3 events, got %d", len(events))
	}

	// Verify MCP server stats
	mcpStats, err := env.Store.GetMCPServerStats(env.ctx, storage.TimeFilter{})
	if err != nil {
		t.Fatalf("Failed to get MCP stats: %v", err)
	}

	foundEverything := false
	for _, stat := range mcpStats {
		if stat.ServerName == "everything" {
			foundEverything = true
			if stat.TotalCalls < 3 {
				t.Errorf("Expected at least 3 calls to 'everything' server, got %d", stat.TotalCalls)
			}
			break
		}
	}

	if !foundEverything {
		t.Error("Expected 'everything' MCP server in stats")
	}

	// Verify dashboard loads with data
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", env.webPort)
	resp, err := http.Get(baseURL + "/mcp")
	if err != nil {
		t.Fatalf("Failed to get MCP dashboard: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("MCP dashboard: expected 200, got %d", resp.StatusCode)
	}

	t.Log("E2E-001: PASSED - Full flow with simulated MCP events")
}

func TestE2E_002_ErrorTrackingWithDeliberateFailures(t *testing.T) {
	env := SetupTestEnv(t, 19896, 19897)
	defer env.Teardown()

	sessionID := "e2e-error-test"

	// Send successful events
	for i := 0; i < 3; i++ {
		pre := map[string]interface{}{
			"session_id":      sessionID,
			"hook_event_name": "PreToolUse",
			"tool_name":       "mcp__everything__echo",
			"tool_input":      map[string]interface{}{"message": "ok"},
		}
		env.PostHookEvent(t, pre).Body.Close()
		time.Sleep(30 * time.Millisecond)

		post := map[string]interface{}{
			"session_id":      sessionID,
			"hook_event_name": "PostToolUse",
			"tool_name":       "mcp__everything__echo",
			"tool_input":      map[string]interface{}{"message": "ok"},
			"tool_response":   map[string]interface{}{"success": true},
		}
		env.PostHookEvent(t, post).Body.Close()
	}

	// Send error event
	errorPre := map[string]interface{}{
		"session_id":      sessionID,
		"hook_event_name": "PreToolUse",
		"tool_name":       "mcp__everything__add",
		"tool_input":      map[string]interface{}{"a": "invalid", "b": 5},
	}
	env.PostHookEvent(t, errorPre).Body.Close()
	time.Sleep(30 * time.Millisecond)

	errorPost := map[string]interface{}{
		"session_id":      sessionID,
		"hook_event_name": "PostToolUse",
		"tool_name":       "mcp__everything__add",
		"tool_input":      map[string]interface{}{"a": "invalid", "b": 5},
		"tool_response": map[string]interface{}{
			"error": "Invalid input: 'a' must be a number",
		},
	}
	env.PostHookEvent(t, errorPost).Body.Close()

	// Wait for processing
	time.Sleep(300 * time.Millisecond)

	// Verify error is tracked
	mcpStats, err := env.Store.GetMCPServerStats(env.ctx, storage.TimeFilter{})
	if err != nil {
		t.Fatalf("Failed to get MCP stats: %v", err)
	}

	for _, stat := range mcpStats {
		if stat.ServerName == "everything" {
			if stat.ErrorCount < 1 {
				t.Errorf("Expected at least 1 error, got %d", stat.ErrorCount)
			}
			successRate := float64(stat.SuccessCount) / float64(stat.TotalCalls) * 100
			t.Logf("Everything server: %d calls, %d errors, %.1f%% success",
				stat.TotalCalls, stat.ErrorCount, successRate)
			break
		}
	}

	t.Log("E2E-002: PASSED - Error tracking works correctly")
}

// === Benchmark Tests ===

func BenchmarkEventProcessing(b *testing.B) {
	ctx := context.Background()
	store := storage.NewMockStore()
	defer store.Close()

	event := &storage.Event{
		SessionID:  "benchmark-session",
		EventType:  "PostToolUse",
		ToolName:   "mcp__test__tool",
		MCPServer:  "test",
		Success:    true,
		DurationMs: 100,
		CreatedAt:  time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event.ID = 0 // Reset ID
		if err := store.StoreEvent(ctx, event); err != nil {
			b.Fatalf("Failed to store: %v", err)
		}
	}
}

func BenchmarkMCPStatsQuery(b *testing.B) {
	ctx := context.Background()
	store := storage.NewMockStore()
	defer store.Close()

	// Seed with test data
	now := time.Now()
	for i := 0; i < 1000; i++ {
		event := &storage.Event{
			SessionID:  fmt.Sprintf("session-%d", i%10),
			EventType:  "PostToolUse",
			ToolName:   fmt.Sprintf("tool_%d", i%5),
			MCPServer:  fmt.Sprintf("mcp%d", i%3),
			Success:    i%4 != 0,
			DurationMs: int64(50 + i%200),
			CreatedAt:  now.Add(-time.Duration(i) * time.Second),
		}
		store.StoreEvent(ctx, event)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.GetMCPServerStats(ctx, storage.TimeFilter{
			From: now.Add(-1 * time.Hour),
		})
		if err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}
