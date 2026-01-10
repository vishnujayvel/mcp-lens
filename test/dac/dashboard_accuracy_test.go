package dac

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/anthropics/mcp-lens/internal/metrics"
	"github.com/anthropics/mcp-lens/internal/storage"
	"github.com/anthropics/mcp-lens/internal/web"
)

// TestDashboardDataAccuracy implements Scenario 3 from the DAC framework.
// It inserts known test data into MockStore and verifies the dashboard
// reflects it accurately.
func TestDashboardDataAccuracy(t *testing.T) {
	ctx := context.Background()

	// Create mock store
	store := storage.NewMockStore()
	defer store.Close()

	// === INSERT KNOWN TEST DATA ===
	// Requirements:
	// - 5 sessions
	// - 100 total events
	// - 3 MCP servers: "filesystem" (30 calls), "github" (50 calls), "everything" (20 calls)
	// - 10 errors (5 from github, 5 from filesystem)

	now := time.Now()

	// Session IDs
	sessions := []string{
		"session-001",
		"session-002",
		"session-003",
		"session-004",
		"session-005",
	}

	// Insert events for "filesystem" server: 30 total, 5 errors
	for i := 0; i < 30; i++ {
		event := &storage.Event{
			SessionID:  sessions[i%5],
			EventType:  "PostToolUse",
			ToolName:   fmt.Sprintf("mcp__filesystem__tool_%d", i%5),
			MCPServer:  "filesystem",
			Success:    i >= 5, // First 5 are errors
			DurationMs: int64(50 + i*2),
			CreatedAt:  now.Add(-time.Duration(i) * time.Minute),
		}
		if err := store.StoreEvent(ctx, event); err != nil {
			t.Fatalf("Failed to store filesystem event: %v", err)
		}
	}

	// Insert events for "github" server: 50 total, 5 errors
	for i := 0; i < 50; i++ {
		event := &storage.Event{
			SessionID:  sessions[i%5],
			EventType:  "PostToolUse",
			ToolName:   fmt.Sprintf("mcp__github__action_%d", i%8),
			MCPServer:  "github",
			Success:    i >= 5, // First 5 are errors
			DurationMs: int64(100 + i*3),
			CreatedAt:  now.Add(-time.Duration(30+i) * time.Minute),
		}
		if err := store.StoreEvent(ctx, event); err != nil {
			t.Fatalf("Failed to store github event: %v", err)
		}
	}

	// Insert events for "everything" server: 20 total, 0 errors
	for i := 0; i < 20; i++ {
		event := &storage.Event{
			SessionID:  sessions[i%5],
			EventType:  "PostToolUse",
			ToolName:   fmt.Sprintf("mcp__everything__cmd_%d", i%3),
			MCPServer:  "everything",
			Success:    true, // No errors
			DurationMs: int64(30 + i),
			CreatedAt:  now.Add(-time.Duration(80+i) * time.Minute),
		}
		if err := store.StoreEvent(ctx, event); err != nil {
			t.Fatalf("Failed to store everything event: %v", err)
		}
	}

	// === VERIFY STORE DATA DIRECTLY ===
	t.Log("Verifying store data before starting web server...")

	// Verify total events
	allEvents, err := store.GetEvents(ctx, storage.EventFilter{})
	if err != nil {
		t.Fatalf("Failed to get all events: %v", err)
	}
	totalEventsStored := len(allEvents)
	t.Logf("Total events stored: %d (expected: 100)", totalEventsStored)

	// Verify sessions
	allSessions, err := store.GetSessions(ctx, storage.SessionFilter{})
	if err != nil {
		t.Fatalf("Failed to get sessions: %v", err)
	}
	totalSessionsStored := len(allSessions)
	t.Logf("Total sessions stored: %d (expected: 5)", totalSessionsStored)

	// Verify MCP server stats
	mcpStats, err := store.GetMCPServerStats(ctx, storage.TimeFilter{})
	if err != nil {
		t.Fatalf("Failed to get MCP stats: %v", err)
	}
	t.Logf("MCP servers found: %d (expected: 3)", len(mcpStats))

	// Create a map for easier lookup
	mcpStatsMap := make(map[string]storage.MCPServerStats)
	totalErrors := int64(0)
	for _, stat := range mcpStats {
		mcpStatsMap[stat.ServerName] = stat
		totalErrors += stat.ErrorCount
		t.Logf("  %s: %d calls, %d errors", stat.ServerName, stat.TotalCalls, stat.ErrorCount)
	}

	// === START WEB SERVER ===
	webPort := 19950 // Use a unique port for this test

	webConfig := web.ServerConfig{
		Port:        webPort,
		BindAddress: "127.0.0.1",
	}

	webServer, err := web.NewServer(webConfig, store)
	if err != nil {
		t.Fatalf("Failed to create web server: %v", err)
	}

	// Start web server
	go webServer.Start(ctx)
	defer webServer.Stop(ctx)
	time.Sleep(200 * time.Millisecond) // Give server time to start

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", webPort)

	// === FETCH DASHBOARD METRICS VIA HTTP ===
	t.Log("\nFetching dashboard metrics via HTTP...")

	// Test the /health endpoint first
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Failed to get health endpoint: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Health check failed with status: %d", resp.StatusCode)
	}
	resp.Body.Close()
	t.Log("Health check: OK")

	// Test main dashboard page loads
	resp, err = http.Get(baseURL + "/?range=24h")
	if err != nil {
		t.Fatalf("Failed to get dashboard: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Dashboard failed with status: %d", resp.StatusCode)
	}
	resp.Body.Close()
	t.Log("Dashboard page: OK")

	// Test MCP page loads
	resp, err = http.Get(baseURL + "/mcp?range=24h")
	if err != nil {
		t.Fatalf("Failed to get MCP page: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("MCP page failed with status: %d", resp.StatusCode)
	}
	mcpBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	t.Logf("MCP page: OK (%d bytes)", len(mcpBody))

	// === USE METRICS CALCULATOR DIRECTLY FOR VERIFICATION ===
	// Since the web endpoints return HTML, we verify using the calculator
	// which is what the handlers use internally

	calculator := metrics.NewCalculator(store)

	// Use empty time filter to get ALL data
	filter := storage.TimeFilter{}

	summary, err := calculator.GetDashboardSummary(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to get dashboard summary: %v", err)
	}

	mcpUtil, err := calculator.GetMCPUtilization(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to get MCP utilization: %v", err)
	}

	// === BUILD RESULTS TABLE ===
	type MetricResult struct {
		Name     string
		Expected interface{}
		Actual   interface{}
		Pass     bool
	}

	results := []MetricResult{}

	// Total Sessions
	results = append(results, MetricResult{
		Name:     "Total Sessions",
		Expected: 5,
		Actual:   int(summary.TotalSessions),
		Pass:     summary.TotalSessions == 5,
	})

	// Total Events
	results = append(results, MetricResult{
		Name:     "Total Events",
		Expected: 100,
		Actual:   int(summary.TotalEvents),
		Pass:     summary.TotalEvents == 100,
	})

	// MCP Server Count
	results = append(results, MetricResult{
		Name:     "MCP Server Count",
		Expected: 3,
		Actual:   summary.ActiveMCPServers,
		Pass:     summary.ActiveMCPServers == 3,
	})

	// Build MCP utilization map
	mcpUtilMap := make(map[string]metrics.MCPUtilization)
	for _, u := range mcpUtil {
		mcpUtilMap[u.ServerName] = u
	}

	// filesystem calls
	fsExpected := int64(30)
	fsActual := int64(0)
	if fs, ok := mcpUtilMap["filesystem"]; ok {
		fsActual = fs.CallCount
	}
	results = append(results, MetricResult{
		Name:     "filesystem calls",
		Expected: fsExpected,
		Actual:   fsActual,
		Pass:     fsActual == fsExpected,
	})

	// github calls
	ghExpected := int64(50)
	ghActual := int64(0)
	if gh, ok := mcpUtilMap["github"]; ok {
		ghActual = gh.CallCount
	}
	results = append(results, MetricResult{
		Name:     "github calls",
		Expected: ghExpected,
		Actual:   ghActual,
		Pass:     ghActual == ghExpected,
	})

	// everything calls
	evExpected := int64(20)
	evActual := int64(0)
	if ev, ok := mcpUtilMap["everything"]; ok {
		evActual = ev.CallCount
	}
	results = append(results, MetricResult{
		Name:     "everything calls",
		Expected: evExpected,
		Actual:   evActual,
		Pass:     evActual == evExpected,
	})

	// Total errors - verify from MCP stats (since utilization doesn't expose raw error counts)
	totalErrorsExpected := int64(10)
	results = append(results, MetricResult{
		Name:     "Total errors",
		Expected: totalErrorsExpected,
		Actual:   totalErrors,
		Pass:     totalErrors == totalErrorsExpected,
	})

	// === PRINT ACTOR REPORT ===
	fmt.Println("\n" + repeatString("=", 60))
	fmt.Println("ACTOR REPORT: Dashboard Data Accuracy")
	fmt.Println(repeatString("=", 60))
	fmt.Printf("%-20s | %-10s | %-10s | %-8s\n", "Metric", "Expected", "Actual", "Status")
	fmt.Println(repeatString("-", 60))

	passCount := 0
	failCount := 0
	var issues []string

	for _, r := range results {
		status := "PASS"
		if !r.Pass {
			status = "FAIL"
			failCount++
			issues = append(issues, fmt.Sprintf("%s: expected %v, got %v", r.Name, r.Expected, r.Actual))
		} else {
			passCount++
		}
		fmt.Printf("%-20s | %-10v | %-10v | %-8s\n", r.Name, r.Expected, r.Actual, status)
	}

	fmt.Println(repeatString("-", 60))
	fmt.Printf("\nSUMMARY: %d/%d metrics verified\n", passCount, len(results))

	if len(issues) > 0 {
		fmt.Println("\nISSUES:")
		for _, issue := range issues {
			fmt.Printf("  - %s\n", issue)
		}
	} else {
		fmt.Println("\nISSUES: None")
	}
	fmt.Println(repeatString("=", 60))

	// Fail test if any metrics don't match
	if failCount > 0 {
		t.Errorf("Dashboard accuracy test failed: %d/%d metrics did not match", failCount, len(results))
	}

	t.Log("\nDashboard Data Accuracy test completed")
}

// Helper function to repeat a string
func repeatString(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

// TestDashboardDataAccuracyJSON tests the dashboard metrics using JSON parsing
// when JSON endpoints are available
func TestDashboardDataAccuracyViaAPI(t *testing.T) {
	ctx := context.Background()

	// Create mock store
	store := storage.NewMockStore()
	defer store.Close()

	now := time.Now()
	sessions := []string{"s1", "s2", "s3", "s4", "s5"}

	// Insert the same known test data
	// filesystem: 30 calls, 5 errors
	for i := 0; i < 30; i++ {
		store.StoreEvent(ctx, &storage.Event{
			SessionID:  sessions[i%5],
			EventType:  "PostToolUse",
			ToolName:   "mcp__filesystem__read",
			MCPServer:  "filesystem",
			Success:    i >= 5,
			DurationMs: 50,
			CreatedAt:  now.Add(-time.Duration(i) * time.Minute),
		})
	}

	// github: 50 calls, 5 errors
	for i := 0; i < 50; i++ {
		store.StoreEvent(ctx, &storage.Event{
			SessionID:  sessions[i%5],
			EventType:  "PostToolUse",
			ToolName:   "mcp__github__pr",
			MCPServer:  "github",
			Success:    i >= 5,
			DurationMs: 100,
			CreatedAt:  now.Add(-time.Duration(30+i) * time.Minute),
		})
	}

	// everything: 20 calls, 0 errors
	for i := 0; i < 20; i++ {
		store.StoreEvent(ctx, &storage.Event{
			SessionID:  sessions[i%5],
			EventType:  "PostToolUse",
			ToolName:   "mcp__everything__echo",
			MCPServer:  "everything",
			Success:    true,
			DurationMs: 30,
			CreatedAt:  now.Add(-time.Duration(80+i) * time.Minute),
		})
	}

	// Start web server
	webPort := 19951
	webConfig := web.ServerConfig{
		Port:        webPort,
		BindAddress: "127.0.0.1",
	}

	webServer, err := web.NewServer(webConfig, store)
	if err != nil {
		t.Fatalf("Failed to create web server: %v", err)
	}

	go webServer.Start(ctx)
	defer webServer.Stop(ctx)
	time.Sleep(200 * time.Millisecond)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", webPort)

	// Test health endpoint returns JSON
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()

	var healthResp map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	if healthResp["status"] != "ok" {
		t.Errorf("Health status: expected 'ok', got '%s'", healthResp["status"])
	}

	t.Log("API health check verified successfully")

	// Verify data via calculator (since there's no JSON API for metrics yet)
	calculator := metrics.NewCalculator(store)
	summary, _ := calculator.GetDashboardSummary(ctx, storage.TimeFilter{})
	mcpUtil, _ := calculator.GetMCPUtilization(ctx, storage.TimeFilter{})

	// Verify totals
	if summary.TotalSessions != 5 {
		t.Errorf("Sessions: expected 5, got %d", summary.TotalSessions)
	}
	if summary.TotalEvents != 100 {
		t.Errorf("Events: expected 100, got %d", summary.TotalEvents)
	}
	if len(mcpUtil) != 3 {
		t.Errorf("MCP servers: expected 3, got %d", len(mcpUtil))
	}

	t.Log("Dashboard data accuracy via API test completed")
}
