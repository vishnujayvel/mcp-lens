// Package collector handles JSONL event collection and parsing.
package collector

import (
	"encoding/json"
	"strings"
	"time"
)

// Event represents a Claude Code hook event in minimal JSONL format.
type Event struct {
	Timestamp  time.Time `json:"ts"`
	SessionID  string    `json:"sid"`
	EventType  string    `json:"type"`
	ToolName   string    `json:"tool,omitempty"`
	DurationMs int64     `json:"dur_ms,omitempty"`
	Success    bool      `json:"ok,omitempty"`
	Cwd        string    `json:"cwd,omitempty"`
}

// FullEvent represents a complete Claude Code hook event payload.
// This is the format received from hooks when storing full payloads.
type FullEvent struct {
	SessionID      string                 `json:"session_id"`
	TranscriptPath string                 `json:"transcript_path,omitempty"`
	Cwd            string                 `json:"cwd,omitempty"`
	HookEventName  string                 `json:"hook_event_name"`
	ToolName       string                 `json:"tool_name,omitempty"`
	ToolInput      map[string]interface{} `json:"tool_input,omitempty"`
	ToolResponse   interface{}            `json:"tool_response,omitempty"` // Can be map or string
}

// ParseEvent parses a JSONL line into an Event.
// It handles both minimal format and full format payloads.
func ParseEvent(line []byte) (*Event, error) {
	// Try minimal format first
	var event Event
	if err := json.Unmarshal(line, &event); err == nil && event.SessionID != "" && event.EventType != "" {
		return &event, nil
	}

	// Try full format
	var full FullEvent
	if err := json.Unmarshal(line, &full); err != nil {
		return nil, err
	}

	// Convert full to minimal
	event = Event{
		Timestamp: time.Now(), // Full format doesn't have timestamp, use current
		SessionID: full.SessionID,
		EventType: full.HookEventName,
		ToolName:  full.ToolName,
		Cwd:       full.Cwd,
		Success:   true,
	}

	// Check for errors in tool response (can be map or string)
	if respMap, ok := full.ToolResponse.(map[string]interface{}); ok {
		if isError, ok := respMap["is_error"].(bool); ok && isError {
			event.Success = false
		}
	}
	// If tool_response is a string, assume success (MCP tools return JSON string)

	return &event, nil
}

// ExtractMCPServer extracts the MCP server name from a tool name.
// Tool names follow the pattern: mcp__SERVER__tool_name
// Returns empty string for built-in tools (Read, Write, Bash, etc.)
func ExtractMCPServer(toolName string) string {
	if !strings.HasPrefix(toolName, "mcp__") {
		return "" // Not an MCP tool
	}

	parts := strings.SplitN(toolName, "__", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[1] // Return SERVER part
}

// IsMCPTool returns true if the tool name is an MCP tool.
func IsMCPTool(toolName string) bool {
	return strings.HasPrefix(toolName, "mcp__")
}

// EventTypes defines valid hook event types.
var EventTypes = []string{
	"SessionStart",
	"SessionEnd",
	"PreToolUse",
	"PostToolUse",
	"Stop",
	"SubagentStop",
	"Notification",
	"PreCompact",
}
