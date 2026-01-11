package collector

import (
	"testing"
	"time"
)

func TestParseEvent_MinimalFormat(t *testing.T) {
	input := []byte(`{"ts":"2026-01-10T10:00:00Z","sid":"sess-123","type":"PostToolUse","tool":"Read","ok":true}`)

	event, err := ParseEvent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.SessionID != "sess-123" {
		t.Errorf("expected session ID 'sess-123', got '%s'", event.SessionID)
	}
	if event.EventType != "PostToolUse" {
		t.Errorf("expected event type 'PostToolUse', got '%s'", event.EventType)
	}
	if event.ToolName != "Read" {
		t.Errorf("expected tool name 'Read', got '%s'", event.ToolName)
	}
	if !event.Success {
		t.Error("expected success to be true")
	}
}

func TestParseEvent_FullFormat(t *testing.T) {
	input := []byte(`{
		"session_id": "sess-456",
		"hook_event_name": "PostToolUse",
		"tool_name": "mcp__github__create_issue",
		"tool_response": {"is_error": false}
	}`)

	event, err := ParseEvent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.SessionID != "sess-456" {
		t.Errorf("expected session ID 'sess-456', got '%s'", event.SessionID)
	}
	if event.EventType != "PostToolUse" {
		t.Errorf("expected event type 'PostToolUse', got '%s'", event.EventType)
	}
	if event.ToolName != "mcp__github__create_issue" {
		t.Errorf("expected tool name, got '%s'", event.ToolName)
	}
	if !event.Success {
		t.Error("expected success to be true")
	}
}

func TestParseEvent_FullFormatWithError(t *testing.T) {
	input := []byte(`{
		"session_id": "sess-789",
		"hook_event_name": "PostToolUse",
		"tool_name": "mcp__asana__get_tasks",
		"tool_response": {"is_error": true}
	}`)

	event, err := ParseEvent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.Success {
		t.Error("expected success to be false for error response")
	}
}

func TestParseEvent_InvalidJSON(t *testing.T) {
	input := []byte(`{invalid json}`)

	_, err := ParseEvent(input)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseEvent_SessionStart(t *testing.T) {
	input := []byte(`{"ts":"2026-01-10T10:00:00Z","sid":"sess-start","type":"SessionStart","cwd":"/home/user/project"}`)

	event, err := ParseEvent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.EventType != "SessionStart" {
		t.Errorf("expected event type 'SessionStart', got '%s'", event.EventType)
	}
	if event.Cwd != "/home/user/project" {
		t.Errorf("expected cwd '/home/user/project', got '%s'", event.Cwd)
	}
}

func TestParseEvent_WithDuration(t *testing.T) {
	input := []byte(`{"ts":"2026-01-10T10:00:00Z","sid":"sess-dur","type":"PostToolUse","tool":"Bash","dur_ms":150,"ok":true}`)

	event, err := ParseEvent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.DurationMs != 150 {
		t.Errorf("expected duration 150ms, got %d", event.DurationMs)
	}
}

func TestExtractMCPServer(t *testing.T) {
	tests := []struct {
		toolName string
		expected string
	}{
		{"mcp__github__create_issue", "github"},
		{"mcp__asana__get_tasks", "asana"},
		{"mcp__lazy-mcp-proxy__execute_tool", "lazy-mcp-proxy"},
		{"mcp__task-graph__create_task", "task-graph"},
		{"Read", ""},
		{"Write", ""},
		{"Bash", ""},
		{"Task", ""},
		{"mcp__", ""},
		{"mcp__incomplete", "incomplete"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			result := ExtractMCPServer(tt.toolName)
			if result != tt.expected {
				t.Errorf("ExtractMCPServer(%q) = %q, want %q", tt.toolName, result, tt.expected)
			}
		})
	}
}

func TestIsMCPTool(t *testing.T) {
	tests := []struct {
		toolName string
		expected bool
	}{
		{"mcp__github__create_issue", true},
		{"mcp__asana__get_tasks", true},
		{"Read", false},
		{"Write", false},
		{"Bash", false},
		{"Task", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			result := IsMCPTool(tt.toolName)
			if result != tt.expected {
				t.Errorf("IsMCPTool(%q) = %v, want %v", tt.toolName, result, tt.expected)
			}
		})
	}
}

func TestEventTimestamp(t *testing.T) {
	input := []byte(`{"ts":"2026-01-10T15:30:45Z","sid":"sess-ts","type":"PostToolUse"}`)

	event, err := ParseEvent(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := time.Date(2026, 1, 10, 15, 30, 45, 0, time.UTC)
	if !event.Timestamp.Equal(expected) {
		t.Errorf("expected timestamp %v, got %v", expected, event.Timestamp)
	}
}
