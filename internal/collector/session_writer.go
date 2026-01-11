package collector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// SessionWriter writes events to per-session files.
// This eliminates contention between different Claude sessions.
//
// File structure:
//
//	~/.mcp-lens/events/
//	├── sess-abc123.jsonl
//	├── sess-def456.jsonl
//	└── ...
//
// The sync engine reads all session files and merges them.
type SessionWriter struct {
	eventsDir string
}

// NewSessionWriter creates a writer that uses per-session files.
func NewSessionWriter(eventsDir string) (*SessionWriter, error) {
	if err := os.MkdirAll(eventsDir, 0755); err != nil {
		return nil, fmt.Errorf("creating events directory: %w", err)
	}
	return &SessionWriter{eventsDir: eventsDir}, nil
}

// WriteEvent appends an event to the session-specific file.
// Each session writes to its own file, eliminating cross-session contention.
// File locking is still used for safety (same session in multiple processes).
func (w *SessionWriter) WriteEvent(event *Event) error {
	if event.SessionID == "" {
		return fmt.Errorf("event has no session ID")
	}

	filename := filepath.Join(w.eventsDir, sanitizeFilename(event.SessionID)+".jsonl")

	// Marshal event
	data, err := json.Marshal(event.ToJSONL())
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}
	data = append(data, '\n')

	// Open with append mode
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	// Still use flock for same-session safety
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("writing event: %w", err)
	}

	return f.Sync()
}

// ListSessionFiles returns all session files in the events directory.
func (w *SessionWriter) ListSessionFiles() ([]string, error) {
	pattern := filepath.Join(w.eventsDir, "*.jsonl")
	return filepath.Glob(pattern)
}

// CleanupOldSessions removes session files older than the given duration.
func (w *SessionWriter) CleanupOldSessions(olderThan time.Duration) (int, error) {
	files, err := w.ListSessionFiles()
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-olderThan)
	removed := 0

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(file); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}

// sanitizeFilename ensures the session ID is safe for use as a filename.
func sanitizeFilename(sessionID string) string {
	// Replace any characters that might be problematic
	safe := make([]byte, 0, len(sessionID))
	for i := 0; i < len(sessionID); i++ {
		c := sessionID[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' {
			safe = append(safe, c)
		} else {
			safe = append(safe, '_')
		}
	}
	return string(safe)
}

// MultiFileParser reads events from multiple session files.
type MultiFileParser struct {
	eventsDir string
}

// NewMultiFileParser creates a parser for per-session event files.
func NewMultiFileParser(eventsDir string) *MultiFileParser {
	return &MultiFileParser{eventsDir: eventsDir}
}

// ParseAllSessions reads all events from all session files.
// Returns events sorted by timestamp.
func (p *MultiFileParser) ParseAllSessions() ([]*Event, error) {
	files, err := filepath.Glob(filepath.Join(p.eventsDir, "*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("listing session files: %w", err)
	}

	var allEvents []*Event
	for _, file := range files {
		parser := NewParser(file)
		events, _, err := parser.ParseFromPosition(0)
		if err != nil {
			// Log but continue with other files
			fmt.Printf("warning: failed to parse %s: %v\n", file, err)
			continue
		}
		allEvents = append(allEvents, events...)
	}

	// Sort by timestamp
	sortEventsByTimestamp(allEvents)

	return allEvents, nil
}

// sortEventsByTimestamp sorts events in chronological order.
func sortEventsByTimestamp(events []*Event) {
	// Simple bubble sort for now (could use sort.Slice)
	for i := 0; i < len(events); i++ {
		for j := i + 1; j < len(events); j++ {
			if events[j].Timestamp.Before(events[i].Timestamp) {
				events[i], events[j] = events[j], events[i]
			}
		}
	}
}
