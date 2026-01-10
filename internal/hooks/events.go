// Package hooks handles Claude Code hook event reception and processing.
package hooks

import (
	"encoding/json"
	"time"
)

// HookEvent represents the base structure of all Claude Code hook events.
type HookEvent struct {
	SessionID      string    `json:"session_id"`
	TranscriptPath string    `json:"transcript_path"`
	Cwd            string    `json:"cwd"`
	PermissionMode string    `json:"permission_mode"`
	HookEventName  string    `json:"hook_event_name"`
	Timestamp      time.Time `json:"timestamp,omitempty"`
}

// ToolUseEvent extends HookEvent for PreToolUse and PostToolUse events.
type ToolUseEvent struct {
	HookEvent
	ToolName     string                 `json:"tool_name"`
	ToolInput    map[string]interface{} `json:"tool_input"`
	ToolResponse map[string]interface{} `json:"tool_response,omitempty"`
}

// ParsedEvent wraps a hook event with metadata.
type ParsedEvent struct {
	Raw        []byte
	Event      HookEvent
	Tool       *ToolUseEvent
	ReceivedAt time.Time
}

// ParseEvent parses a raw JSON hook event.
func ParseEvent(data []byte) (*ParsedEvent, error) {
	var base HookEvent
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, err
	}

	parsed := &ParsedEvent{
		Raw:        data,
		Event:      base,
		ReceivedAt: time.Now(),
	}

	// Set timestamp if not provided
	if base.Timestamp.IsZero() {
		parsed.Event.Timestamp = parsed.ReceivedAt
	}

	// Parse tool-specific fields if this is a tool event
	if base.HookEventName == "PreToolUse" || base.HookEventName == "PostToolUse" {
		var toolEvent ToolUseEvent
		if err := json.Unmarshal(data, &toolEvent); err != nil {
			return nil, err
		}
		parsed.Tool = &toolEvent
	}

	return parsed, nil
}

// IsToolEvent returns true if this is a PreToolUse or PostToolUse event.
func (p *ParsedEvent) IsToolEvent() bool {
	return p.Tool != nil
}

// IsSessionEvent returns true if this is a session lifecycle event.
func (p *ParsedEvent) IsSessionEvent() bool {
	return p.Event.HookEventName == "SessionStart" || p.Event.HookEventName == "SessionEnd"
}

// GetToolName returns the tool name if this is a tool event, otherwise empty string.
func (p *ParsedEvent) GetToolName() string {
	if p.Tool != nil {
		return p.Tool.ToolName
	}
	return ""
}

// IsSuccess returns whether the tool call was successful (for PostToolUse events).
func (p *ParsedEvent) IsSuccess() bool {
	if p.Tool == nil || p.Event.HookEventName != "PostToolUse" {
		return true // Non-tool events are considered successful
	}

	// Check for success field in tool_response
	if p.Tool.ToolResponse != nil {
		if success, ok := p.Tool.ToolResponse["success"].(bool); ok {
			return success
		}
		// Check for error field
		if _, hasError := p.Tool.ToolResponse["error"]; hasError {
			return false
		}
	}

	return true // Default to success if no indicators
}

// SupportedEventTypes lists all supported Claude Code hook event types.
var SupportedEventTypes = []string{
	"PreToolUse",
	"PostToolUse",
	"UserPromptSubmit",
	"Stop",
	"SubagentStop",
	"Notification",
	"SessionStart",
	"SessionEnd",
	"PreCompact",
	"PermissionRequest",
}

// IsValidEventType checks if an event type is supported.
func IsValidEventType(eventType string) bool {
	for _, t := range SupportedEventTypes {
		if t == eventType {
			return true
		}
	}
	return false
}
