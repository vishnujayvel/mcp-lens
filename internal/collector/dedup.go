package collector

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// EventFingerprint generates a unique fingerprint for an event.
// This is used for deduplication during sync.
func EventFingerprint(event *Event) string {
	// Create a deterministic string from key event fields
	data := fmt.Sprintf("%s|%s|%s|%s|%d|%t",
		event.Timestamp.UTC().Format("2006-01-02T15:04:05.000Z"),
		event.SessionID,
		event.EventType,
		event.ToolName,
		event.DurationMs,
		event.Success,
	)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes (32 hex chars)
}

// EventValidator validates event data quality.
type EventValidator struct {
	// Tracks validation statistics
	ValidCount   int64
	InvalidCount int64
	Warnings     []string
}

// NewEventValidator creates a new validator.
func NewEventValidator() *EventValidator {
	return &EventValidator{
		Warnings: make([]string, 0),
	}
}

// Validate checks an event for data quality issues.
// Returns true if the event should be processed, false to skip.
func (v *EventValidator) Validate(event *Event) bool {
	// Required: SessionID
	if event.SessionID == "" {
		v.InvalidCount++
		v.Warnings = append(v.Warnings, "event missing session_id")
		return false
	}

	// Required: EventType
	if event.EventType == "" {
		v.InvalidCount++
		v.Warnings = append(v.Warnings, fmt.Sprintf("event %s missing event_type", event.SessionID))
		return false
	}

	// Validate EventType is known
	if !isValidEventType(event.EventType) {
		v.InvalidCount++
		v.Warnings = append(v.Warnings, fmt.Sprintf("event %s has unknown type: %s", event.SessionID, event.EventType))
		return false
	}

	// Required for PostToolUse: ToolName
	if event.EventType == "PostToolUse" && event.ToolName == "" {
		v.InvalidCount++
		v.Warnings = append(v.Warnings, fmt.Sprintf("PostToolUse event %s missing tool_name", event.SessionID))
		return false
	}

	// Timestamp should be reasonable (not zero, not in future)
	if event.Timestamp.IsZero() {
		v.InvalidCount++
		v.Warnings = append(v.Warnings, fmt.Sprintf("event %s has zero timestamp", event.SessionID))
		return false
	}

	v.ValidCount++
	return true
}

// isValidEventType checks if the event type is recognized.
func isValidEventType(eventType string) bool {
	validTypes := map[string]bool{
		"SessionStart":  true,
		"SessionEnd":    true,
		"PreToolUse":    true,
		"PostToolUse":   true,
		"Stop":          true,
		"SubagentStop":  true,
		"Notification":  true,
		"PreCompact":    true,
	}
	return validTypes[eventType]
}

// Stats returns validation statistics.
func (v *EventValidator) Stats() (valid, invalid int64) {
	return v.ValidCount, v.InvalidCount
}
