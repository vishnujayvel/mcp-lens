package hooks

import (
	"testing"
)

func TestParseEvent_ToolUse(t *testing.T) {
	data := []byte(`{
		"session_id": "abc123",
		"transcript_path": "/path/to/transcript.jsonl",
		"cwd": "/home/user/project",
		"permission_mode": "default",
		"hook_event_name": "PostToolUse",
		"tool_name": "Read",
		"tool_input": {"file_path": "/test.txt"},
		"tool_response": {"success": true, "content": "file content"}
	}`)

	parsed, err := ParseEvent(data)
	if err != nil {
		t.Fatalf("failed to parse event: %v", err)
	}

	if parsed.Event.SessionID != "abc123" {
		t.Errorf("expected session ID abc123, got %s", parsed.Event.SessionID)
	}

	if parsed.Event.HookEventName != "PostToolUse" {
		t.Errorf("expected event name PostToolUse, got %s", parsed.Event.HookEventName)
	}

	if !parsed.IsToolEvent() {
		t.Error("expected IsToolEvent to return true")
	}

	if parsed.Tool.ToolName != "Read" {
		t.Errorf("expected tool name Read, got %s", parsed.Tool.ToolName)
	}

	if !parsed.IsSuccess() {
		t.Error("expected IsSuccess to return true")
	}
}

func TestParseEvent_SessionStart(t *testing.T) {
	data := []byte(`{
		"session_id": "session-456",
		"transcript_path": "/path/to/transcript.jsonl",
		"cwd": "/home/user/project",
		"permission_mode": "default",
		"hook_event_name": "SessionStart"
	}`)

	parsed, err := ParseEvent(data)
	if err != nil {
		t.Fatalf("failed to parse event: %v", err)
	}

	if !parsed.IsSessionEvent() {
		t.Error("expected IsSessionEvent to return true")
	}

	if parsed.IsToolEvent() {
		t.Error("expected IsToolEvent to return false for session event")
	}
}

func TestParseEvent_FailedTool(t *testing.T) {
	data := []byte(`{
		"session_id": "abc123",
		"hook_event_name": "PostToolUse",
		"tool_name": "Write",
		"tool_input": {"file_path": "/readonly/file.txt"},
		"tool_response": {"success": false, "error": "permission denied"}
	}`)

	parsed, err := ParseEvent(data)
	if err != nil {
		t.Fatalf("failed to parse event: %v", err)
	}

	if parsed.IsSuccess() {
		t.Error("expected IsSuccess to return false for failed tool")
	}
}

func TestParseEvent_ToolWithError(t *testing.T) {
	data := []byte(`{
		"session_id": "abc123",
		"hook_event_name": "PostToolUse",
		"tool_name": "Bash",
		"tool_input": {"command": "invalid-command"},
		"tool_response": {"error": "command not found"}
	}`)

	parsed, err := ParseEvent(data)
	if err != nil {
		t.Fatalf("failed to parse event: %v", err)
	}

	if parsed.IsSuccess() {
		t.Error("expected IsSuccess to return false when error field present")
	}
}

func TestGetToolName(t *testing.T) {
	// Tool event
	toolData := []byte(`{
		"session_id": "abc123",
		"hook_event_name": "PostToolUse",
		"tool_name": "Grep"
	}`)

	parsed, _ := ParseEvent(toolData)
	if parsed.GetToolName() != "Grep" {
		t.Errorf("expected tool name Grep, got %s", parsed.GetToolName())
	}

	// Non-tool event
	sessionData := []byte(`{
		"session_id": "abc123",
		"hook_event_name": "SessionStart"
	}`)

	parsed, _ = ParseEvent(sessionData)
	if parsed.GetToolName() != "" {
		t.Errorf("expected empty tool name for session event, got %s", parsed.GetToolName())
	}
}

func TestIsValidEventType(t *testing.T) {
	validTypes := []string{
		"PreToolUse", "PostToolUse", "SessionStart", "SessionEnd",
		"Stop", "SubagentStop", "UserPromptSubmit", "Notification",
	}

	for _, et := range validTypes {
		if !IsValidEventType(et) {
			t.Errorf("expected %s to be valid", et)
		}
	}

	if IsValidEventType("InvalidEvent") {
		t.Error("expected InvalidEvent to be invalid")
	}
}

func TestParseEvent_InvalidJSON(t *testing.T) {
	data := []byte(`{invalid json}`)

	_, err := ParseEvent(data)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
