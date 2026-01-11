package collector

import (
	"testing"
	"time"
)

func TestEventFingerprint(t *testing.T) {
	ts := time.Date(2026, 1, 10, 10, 0, 0, 0, time.UTC)

	// Same event should produce same fingerprint
	event1 := &Event{
		Timestamp:  ts,
		SessionID:  "session-1",
		EventType:  "PostToolUse",
		ToolName:   "mcp__github__create_issue",
		DurationMs: 150,
		Success:    true,
	}

	event2 := &Event{
		Timestamp:  ts,
		SessionID:  "session-1",
		EventType:  "PostToolUse",
		ToolName:   "mcp__github__create_issue",
		DurationMs: 150,
		Success:    true,
	}

	fp1 := EventFingerprint(event1)
	fp2 := EventFingerprint(event2)

	if fp1 != fp2 {
		t.Errorf("Same events should have same fingerprint: %s != %s", fp1, fp2)
	}

	// Different events should produce different fingerprints
	event3 := &Event{
		Timestamp:  ts,
		SessionID:  "session-1",
		EventType:  "PostToolUse",
		ToolName:   "mcp__github__create_issue",
		DurationMs: 151, // Different duration
		Success:    true,
	}

	fp3 := EventFingerprint(event3)
	if fp1 == fp3 {
		t.Error("Different events should have different fingerprints")
	}

	// Different session should produce different fingerprint
	event4 := &Event{
		Timestamp:  ts,
		SessionID:  "session-2", // Different session
		EventType:  "PostToolUse",
		ToolName:   "mcp__github__create_issue",
		DurationMs: 150,
		Success:    true,
	}

	fp4 := EventFingerprint(event4)
	if fp1 == fp4 {
		t.Error("Different sessions should have different fingerprints")
	}

	// Fingerprint should be 32 hex chars (16 bytes)
	if len(fp1) != 32 {
		t.Errorf("Fingerprint should be 32 chars, got %d", len(fp1))
	}
}

func TestEventFingerprintTimezoneNormalization(t *testing.T) {
	// Events with same instant but different timezones should have same fingerprint
	utcTime := time.Date(2026, 1, 10, 10, 0, 0, 0, time.UTC)
	pstLocation, _ := time.LoadLocation("America/Los_Angeles")
	pstTime := utcTime.In(pstLocation)

	event1 := &Event{
		Timestamp:  utcTime,
		SessionID:  "session-1",
		EventType:  "PostToolUse",
		ToolName:   "Read",
		DurationMs: 10,
		Success:    true,
	}

	event2 := &Event{
		Timestamp:  pstTime,
		SessionID:  "session-1",
		EventType:  "PostToolUse",
		ToolName:   "Read",
		DurationMs: 10,
		Success:    true,
	}

	fp1 := EventFingerprint(event1)
	fp2 := EventFingerprint(event2)

	if fp1 != fp2 {
		t.Errorf("Same instant in different timezones should have same fingerprint: %s != %s", fp1, fp2)
	}
}

func TestEventValidator_ValidEvents(t *testing.T) {
	v := NewEventValidator()
	ts := time.Now()

	tests := []struct {
		name  string
		event *Event
	}{
		{
			name: "valid SessionStart",
			event: &Event{
				Timestamp: ts,
				SessionID: "session-1",
				EventType: "SessionStart",
				Cwd:       "/home/user/project",
			},
		},
		{
			name: "valid PostToolUse",
			event: &Event{
				Timestamp:  ts,
				SessionID:  "session-1",
				EventType:  "PostToolUse",
				ToolName:   "Read",
				DurationMs: 10,
				Success:    true,
			},
		},
		{
			name: "valid Stop",
			event: &Event{
				Timestamp: ts,
				SessionID: "session-1",
				EventType: "Stop",
			},
		},
		{
			name: "valid PreToolUse",
			event: &Event{
				Timestamp: ts,
				SessionID: "session-1",
				EventType: "PreToolUse",
				ToolName:  "Write",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !v.Validate(tt.event) {
				t.Errorf("Expected event to be valid: %+v", tt.event)
			}
		})
	}

	valid, invalid := v.Stats()
	if valid != int64(len(tests)) {
		t.Errorf("Expected %d valid events, got %d", len(tests), valid)
	}
	if invalid != 0 {
		t.Errorf("Expected 0 invalid events, got %d", invalid)
	}
}

func TestEventValidator_InvalidEvents(t *testing.T) {
	ts := time.Now()

	tests := []struct {
		name  string
		event *Event
	}{
		{
			name: "missing session_id",
			event: &Event{
				Timestamp: ts,
				SessionID: "",
				EventType: "PostToolUse",
				ToolName:  "Read",
			},
		},
		{
			name: "missing event_type",
			event: &Event{
				Timestamp: ts,
				SessionID: "session-1",
				EventType: "",
			},
		},
		{
			name: "unknown event_type",
			event: &Event{
				Timestamp: ts,
				SessionID: "session-1",
				EventType: "UnknownType",
			},
		},
		{
			name: "PostToolUse missing tool_name",
			event: &Event{
				Timestamp:  ts,
				SessionID:  "session-1",
				EventType:  "PostToolUse",
				ToolName:   "",
				DurationMs: 10,
			},
		},
		{
			name: "zero timestamp",
			event: &Event{
				Timestamp: time.Time{},
				SessionID: "session-1",
				EventType: "PostToolUse",
				ToolName:  "Read",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewEventValidator()
			if v.Validate(tt.event) {
				t.Errorf("Expected event to be invalid: %+v", tt.event)
			}
			if len(v.Warnings) == 0 {
				t.Error("Expected warning to be recorded")
			}
		})
	}
}

func TestEventValidator_ValidEventTypes(t *testing.T) {
	validTypes := []string{
		"SessionStart",
		"SessionEnd",
		"PreToolUse",
		"PostToolUse",
		"Stop",
		"SubagentStop",
		"Notification",
		"PreCompact",
	}

	for _, eventType := range validTypes {
		t.Run(eventType, func(t *testing.T) {
			if !isValidEventType(eventType) {
				t.Errorf("Expected %s to be valid", eventType)
			}
		})
	}

	invalidTypes := []string{
		"",
		"Unknown",
		"sessionstart", // Case sensitive
		"POST_TOOL_USE",
	}

	for _, eventType := range invalidTypes {
		t.Run(eventType, func(t *testing.T) {
			if isValidEventType(eventType) {
				t.Errorf("Expected %s to be invalid", eventType)
			}
		})
	}
}

func TestEventValidator_Stats(t *testing.T) {
	v := NewEventValidator()
	ts := time.Now()

	// Add some valid events
	for i := 0; i < 5; i++ {
		v.Validate(&Event{
			Timestamp: ts,
			SessionID: "session-1",
			EventType: "Stop",
		})
	}

	// Add some invalid events
	for i := 0; i < 3; i++ {
		v.Validate(&Event{
			Timestamp: ts,
			SessionID: "",
			EventType: "Stop",
		})
	}

	valid, invalid := v.Stats()
	if valid != 5 {
		t.Errorf("Expected 5 valid, got %d", valid)
	}
	if invalid != 3 {
		t.Errorf("Expected 3 invalid, got %d", invalid)
	}
	if len(v.Warnings) != 3 {
		t.Errorf("Expected 3 warnings, got %d", len(v.Warnings))
	}
}
