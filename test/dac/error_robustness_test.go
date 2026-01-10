// Package dac contains Director-Actor-Critic tests for MCP Lens.
package dac

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/mcp-lens/internal/hooks"
	"github.com/anthropics/mcp-lens/internal/storage"
	"github.com/anthropics/mcp-lens/internal/web"
)

// TestEnv holds the DAC test environment.
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
	hookURL   string
}

// SetupDACTestEnv creates a complete test environment for DAC tests.
func SetupDACTestEnv(t *testing.T, hookPort, webPort int) *TestEnv {
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

	hookURL := fmt.Sprintf("http://%s/hook", receiver.Address())

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
		hookURL:   hookURL,
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

// PostRaw sends a raw byte slice to the hook endpoint.
func (e *TestEnv) PostRaw(body []byte, contentType string) (*http.Response, error) {
	req, err := http.NewRequest("POST", e.hookURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return http.DefaultClient.Do(req)
}

// PostJSON sends JSON to the hook endpoint.
func (e *TestEnv) PostJSON(data interface{}) (*http.Response, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return e.PostRaw(body, "application/json")
}

// CheckServerHealthy verifies the server can still process valid requests.
func (e *TestEnv) CheckServerHealthy(t *testing.T) bool {
	t.Helper()

	validEvent := map[string]interface{}{
		"session_id":      "health-check-session",
		"hook_event_name": "PreToolUse",
		"tool_name":       "mcp__test__echo",
		"tool_input": map[string]interface{}{
			"message": "health check",
		},
	}

	resp, err := e.PostJSON(validEvent)
	if err != nil {
		t.Logf("Health check failed: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("Health check returned unexpected status: %d", resp.StatusCode)
		return false
	}

	return true
}

// TestResult stores the result of an error robustness test case.
type TestResult struct {
	InputType  string
	HTTPCode   int
	Crashed    bool
	Recovered  bool
	Status     string
	ErrMessage string
}

// === Error Robustness Tests ===

func TestDAC_ErrorRobustness_EmptyBody(t *testing.T) {
	env := SetupDACTestEnv(t, 29100, 29101)
	defer env.Teardown()

	t.Log("Testing: Empty request body")

	resp, err := env.PostRaw([]byte{}, "application/json")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("Response: %d - %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", resp.StatusCode)
	}

	// Check server still healthy
	if !env.CheckServerHealthy(t) {
		t.Error("Server did not recover after empty body test")
	}

	t.Log("PASS: Empty body handled correctly")
}

func TestDAC_ErrorRobustness_InvalidJSON(t *testing.T) {
	env := SetupDACTestEnv(t, 29102, 29103)
	defer env.Teardown()

	t.Log("Testing: Invalid JSON format")

	invalidJSONStrings := []string{
		"{invalid}",
		"not json at all",
		"{\"unclosed",
		"[[[[[",
		"{\"key\": undefined}",
		"{'single': 'quotes'}",
	}

	for _, invalidJSON := range invalidJSONStrings {
		resp, err := env.PostRaw([]byte(invalidJSON), "application/json")
		if err != nil {
			t.Fatalf("Request failed for '%s': %v", invalidJSON[:min(20, len(invalidJSON))], err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		t.Logf("Input: %s -> %d - %s", invalidJSON[:min(20, len(invalidJSON))], resp.StatusCode, strings.TrimSpace(string(body)))

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Invalid JSON '%s': expected 400, got %d", invalidJSON[:min(20, len(invalidJSON))], resp.StatusCode)
		}
	}

	// Check server still healthy
	if !env.CheckServerHealthy(t) {
		t.Error("Server did not recover after invalid JSON tests")
	}

	t.Log("PASS: Invalid JSON handled correctly")
}

func TestDAC_ErrorRobustness_MissingSessionID(t *testing.T) {
	env := SetupDACTestEnv(t, 29104, 29105)
	defer env.Teardown()

	t.Log("Testing: Missing session_id")

	// Event without session_id
	event := map[string]interface{}{
		"hook_event_name": "PreToolUse",
		"tool_name":       "mcp__test__echo",
		"tool_input":      map[string]interface{}{"message": "test"},
	}

	resp, err := env.PostJSON(event)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("Response: %d - %s", resp.StatusCode, strings.TrimSpace(string(body)))

	// Note: The current implementation may accept events without session_id
	// We're documenting the actual behavior here
	t.Logf("Missing session_id returned: %d", resp.StatusCode)

	// Check server still healthy regardless
	if !env.CheckServerHealthy(t) {
		t.Error("Server did not recover after missing session_id test")
	}

	t.Log("PASS: Missing session_id handled without crash")
}

func TestDAC_ErrorRobustness_MissingHookEventName(t *testing.T) {
	env := SetupDACTestEnv(t, 29106, 29107)
	defer env.Teardown()

	t.Log("Testing: Missing hook_event_name")

	// Event without hook_event_name
	event := map[string]interface{}{
		"session_id": "test-session",
		"tool_name":  "mcp__test__echo",
		"tool_input": map[string]interface{}{"message": "test"},
	}

	resp, err := env.PostJSON(event)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("Response: %d - %s", resp.StatusCode, strings.TrimSpace(string(body)))

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing hook_event_name, got %d", resp.StatusCode)
	}

	// Check server still healthy
	if !env.CheckServerHealthy(t) {
		t.Error("Server did not recover after missing hook_event_name test")
	}

	t.Log("PASS: Missing hook_event_name handled correctly")
}

func TestDAC_ErrorRobustness_NullValues(t *testing.T) {
	env := SetupDACTestEnv(t, 29108, 29109)
	defer env.Teardown()

	t.Log("Testing: Null values for required fields")

	testCases := []struct {
		name  string
		event map[string]interface{}
	}{
		{
			"null session_id",
			map[string]interface{}{
				"session_id":      nil,
				"hook_event_name": "PreToolUse",
				"tool_name":       "mcp__test__echo",
			},
		},
		{
			"null hook_event_name",
			map[string]interface{}{
				"session_id":      "test-session",
				"hook_event_name": nil,
				"tool_name":       "mcp__test__echo",
			},
		},
		{
			"null tool_name",
			map[string]interface{}{
				"session_id":      "test-session",
				"hook_event_name": "PreToolUse",
				"tool_name":       nil,
			},
		},
		{
			"null tool_input",
			map[string]interface{}{
				"session_id":      "test-session",
				"hook_event_name": "PreToolUse",
				"tool_name":       "mcp__test__echo",
				"tool_input":      nil,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := env.PostJSON(tc.event)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			t.Logf("%s -> %d - %s", tc.name, resp.StatusCode, strings.TrimSpace(string(body)))

			// Server should not crash
			if !env.CheckServerHealthy(t) {
				t.Errorf("Server crashed after %s test", tc.name)
			}
		})
	}

	t.Log("PASS: Null values handled without crash")
}

func TestDAC_ErrorRobustness_ExtremelyLongStrings(t *testing.T) {
	env := SetupDACTestEnv(t, 29110, 29111)
	defer env.Teardown()

	t.Log("Testing: Extremely long strings (10KB+ tool_name)")

	// Generate a 10KB string
	longString := strings.Repeat("a", 10*1024) // 10KB

	event := map[string]interface{}{
		"session_id":      "test-session",
		"hook_event_name": "PreToolUse",
		"tool_name":       longString,
		"tool_input":      map[string]interface{}{"message": "test"},
	}

	resp, err := env.PostJSON(event)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("10KB tool_name -> %d - %s", resp.StatusCode, strings.TrimSpace(string(body)))

	// The server should either accept it or return an error, but NOT crash
	t.Logf("Long string returned status: %d", resp.StatusCode)

	// Check server still healthy
	if !env.CheckServerHealthy(t) {
		t.Error("Server did not recover after long string test")
	}

	// Test with even longer strings (100KB)
	veryLongString := strings.Repeat("b", 100*1024) // 100KB
	event2 := map[string]interface{}{
		"session_id":      veryLongString, // 100KB session_id
		"hook_event_name": "PreToolUse",
		"tool_name":       "mcp__test__echo",
		"tool_input":      map[string]interface{}{"data": veryLongString},
	}

	resp2, err := env.PostJSON(event2)
	if err != nil {
		t.Logf("100KB request error (expected): %v", err)
	} else {
		body2, _ := io.ReadAll(resp2.Body)
		resp2.Body.Close()
		t.Logf("100KB data -> %d - %s", resp2.StatusCode, strings.TrimSpace(string(body2)))
	}

	// Check server still healthy
	if !env.CheckServerHealthy(t) {
		t.Error("Server did not recover after very long string test")
	}

	t.Log("PASS: Extremely long strings handled without crash")
}

func TestDAC_ErrorRobustness_NestedJSONObjects(t *testing.T) {
	env := SetupDACTestEnv(t, 29112, 29113)
	defer env.Teardown()

	t.Log("Testing: Nested JSON objects as unexpected field types")

	testCases := []struct {
		name  string
		event map[string]interface{}
	}{
		{
			"object as session_id",
			map[string]interface{}{
				"session_id":      map[string]interface{}{"nested": "object"},
				"hook_event_name": "PreToolUse",
				"tool_name":       "mcp__test__echo",
			},
		},
		{
			"object as hook_event_name",
			map[string]interface{}{
				"session_id":      "test-session",
				"hook_event_name": map[string]interface{}{"type": "PreToolUse"},
				"tool_name":       "mcp__test__echo",
			},
		},
		{
			"array as tool_name",
			map[string]interface{}{
				"session_id":      "test-session",
				"hook_event_name": "PreToolUse",
				"tool_name":       []string{"mcp__test__echo", "other_tool"},
			},
		},
		{
			"deeply nested structure",
			map[string]interface{}{
				"session_id":      "test-session",
				"hook_event_name": "PreToolUse",
				"tool_name":       "mcp__test__echo",
				"tool_input": map[string]interface{}{
					"level1": map[string]interface{}{
						"level2": map[string]interface{}{
							"level3": map[string]interface{}{
								"level4": map[string]interface{}{
									"level5": "deep value",
								},
							},
						},
					},
				},
			},
		},
		{
			"mixed types in tool_input",
			map[string]interface{}{
				"session_id":      "test-session",
				"hook_event_name": "PreToolUse",
				"tool_name":       "mcp__test__echo",
				"tool_input": map[string]interface{}{
					"string":  "value",
					"number":  42,
					"float":   3.14,
					"bool":    true,
					"null":    nil,
					"array":   []interface{}{1, "two", 3.0},
					"nested":  map[string]interface{}{"key": "value"},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := env.PostJSON(tc.event)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			t.Logf("%s -> %d - %s", tc.name, resp.StatusCode, strings.TrimSpace(string(body)))

			// Server should not crash
			if !env.CheckServerHealthy(t) {
				t.Errorf("Server crashed after %s test", tc.name)
			}
		})
	}

	t.Log("PASS: Nested JSON objects handled without crash")
}

func TestDAC_ErrorRobustness_WrongContentType(t *testing.T) {
	env := SetupDACTestEnv(t, 29114, 29115)
	defer env.Teardown()

	t.Log("Testing: Wrong Content-Type headers")

	validJSON := `{"session_id":"test","hook_event_name":"PreToolUse","tool_name":"mcp__test__echo"}`

	contentTypes := []string{
		"text/plain",
		"text/html",
		"application/xml",
		"application/x-www-form-urlencoded",
		"multipart/form-data",
		"", // No content type
		"application/octet-stream",
		"invalid-content-type",
	}

	for _, ct := range contentTypes {
		ctDesc := ct
		if ct == "" {
			ctDesc = "(empty)"
		}

		resp, err := env.PostRaw([]byte(validJSON), ct)
		if err != nil {
			t.Fatalf("Request failed for Content-Type '%s': %v", ctDesc, err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		t.Logf("Content-Type '%s' -> %d - %s", ctDesc, resp.StatusCode, strings.TrimSpace(string(body)))

		// Server should not crash
		if !env.CheckServerHealthy(t) {
			t.Errorf("Server crashed after Content-Type '%s' test", ctDesc)
		}
	}

	t.Log("PASS: Wrong Content-Type headers handled without crash")
}

func TestDAC_ErrorRobustness_EdgeCaseMethods(t *testing.T) {
	env := SetupDACTestEnv(t, 29116, 29117)
	defer env.Teardown()

	t.Log("Testing: Non-POST HTTP methods")

	methods := []string{"GET", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"}

	for _, method := range methods {
		req, err := http.NewRequest(method, env.hookURL, nil)
		if err != nil {
			t.Fatalf("Failed to create %s request: %v", method, err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s request failed: %v", method, err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		t.Logf("Method %s -> %d - %s", method, resp.StatusCode, strings.TrimSpace(string(body)))

		if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusOK {
			// Some methods might be handled differently
			t.Logf("Method %s returned unexpected status: %d", method, resp.StatusCode)
		}
	}

	// Check server still healthy
	if !env.CheckServerHealthy(t) {
		t.Error("Server did not recover after HTTP method tests")
	}

	t.Log("PASS: Non-POST HTTP methods handled without crash")
}

// TestDAC_ErrorRobustness_Combined runs all error robustness tests in sequence
// and produces a summary report.
func TestDAC_ErrorRobustness_Combined(t *testing.T) {
	env := SetupDACTestEnv(t, 29120, 29121)
	defer env.Teardown()

	results := make([]TestResult, 0)

	// Helper to test and record results
	testCase := func(name string, body []byte, contentType string) TestResult {
		result := TestResult{
			InputType: name,
			Crashed:   false,
			Recovered: true,
		}

		resp, err := env.PostRaw(body, contentType)
		if err != nil {
			result.Crashed = true
			result.ErrMessage = err.Error()
			result.Status = "FAIL"
			return result
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		result.HTTPCode = resp.StatusCode
		result.ErrMessage = strings.TrimSpace(string(respBody))

		// Check if server recovered
		result.Recovered = env.CheckServerHealthy(t)
		if !result.Recovered {
			result.Crashed = true
		}

		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusOK {
			result.Status = "PASS"
		} else if resp.StatusCode == http.StatusMethodNotAllowed {
			result.Status = "PASS"
		} else {
			result.Status = "WARN"
		}

		if result.Crashed {
			result.Status = "FAIL"
		}

		return result
	}

	// Test cases
	results = append(results, testCase("Empty body", []byte{}, "application/json"))
	results = append(results, testCase("Invalid JSON", []byte("{invalid}"), "application/json"))

	missingSession, _ := json.Marshal(map[string]interface{}{
		"hook_event_name": "PreToolUse",
		"tool_name":       "test",
	})
	results = append(results, testCase("Missing session_id", missingSession, "application/json"))

	missingEvent, _ := json.Marshal(map[string]interface{}{
		"session_id": "test",
		"tool_name":  "test",
	})
	results = append(results, testCase("Missing hook_event_name", missingEvent, "application/json"))

	nullValues, _ := json.Marshal(map[string]interface{}{
		"session_id":      nil,
		"hook_event_name": nil,
	})
	results = append(results, testCase("Null values", nullValues, "application/json"))

	longTool, _ := json.Marshal(map[string]interface{}{
		"session_id":      "test",
		"hook_event_name": "PreToolUse",
		"tool_name":       strings.Repeat("x", 10*1024),
	})
	results = append(results, testCase("10KB tool_name", longTool, "application/json"))

	validJSON := `{"session_id":"test","hook_event_name":"PreToolUse","tool_name":"echo"}`
	results = append(results, testCase("Wrong Content-Type", []byte(validJSON), "text/plain"))

	// Generate report
	t.Log("")
	t.Log("ACTOR REPORT: Error Handling Robustness")
	t.Log("=======================================")
	t.Logf("%-24s | %-9s | %-8s | %-10s | %s", "Input Type", "HTTP Code", "Crashed?", "Recovered?", "Status")
	t.Log("------------------------|-----------|----------|------------|--------")

	passCount := 0
	for _, r := range results {
		crashed := "No"
		if r.Crashed {
			crashed = "Yes"
		}
		recovered := "Yes"
		if !r.Recovered {
			recovered = "No"
		}

		t.Logf("%-24s | %-9d | %-8s | %-10s | %s", r.InputType, r.HTTPCode, crashed, recovered, r.Status)

		if r.Status == "PASS" {
			passCount++
		}
	}

	t.Log("")
	t.Logf("SUMMARY: %d/%d edge cases handled correctly", passCount, len(results))

	// Check for critical issues
	criticalIssues := []string{}
	for _, r := range results {
		if r.Crashed {
			criticalIssues = append(criticalIssues, fmt.Sprintf("%s caused crash", r.InputType))
		}
	}

	if len(criticalIssues) > 0 {
		t.Log("CRITICAL ISSUES:")
		for _, issue := range criticalIssues {
			t.Logf("  - %s", issue)
		}
		t.Fail()
	} else {
		t.Log("CRITICAL ISSUES: None")
	}
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
