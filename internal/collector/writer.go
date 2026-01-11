package collector

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// EventWriter provides safe concurrent writes to the JSONL file.
// It uses file locking (flock) to prevent corruption from multiple processes.
type EventWriter struct {
	path   string
	mu     sync.Mutex // In-process mutex
	maxAge time.Duration
}

// WriterConfig configures the event writer.
type WriterConfig struct {
	EventsFile     string
	MaxFileAgeDays int // Rotate after this many days (0 = no rotation)
}

// NewEventWriter creates a new event writer.
func NewEventWriter(config WriterConfig) (*EventWriter, error) {
	path := config.EventsFile

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating directory: %w", err)
	}

	maxAge := time.Duration(0)
	if config.MaxFileAgeDays > 0 {
		maxAge = time.Duration(config.MaxFileAgeDays) * 24 * time.Hour
	}

	return &EventWriter{
		path:   path,
		maxAge: maxAge,
	}, nil
}

// WriteEvent appends an event to the JSONL file with proper locking.
// This is safe to call from multiple goroutines and multiple processes.
func (w *EventWriter) WriteEvent(event *Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Marshal event to JSON
	data, err := json.Marshal(event.ToJSONL())
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}
	data = append(data, '\n')

	// Open file with append mode
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	// Acquire exclusive lock (blocks if another process holds it)
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	// Write with a single syscall (atomic for small writes)
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("writing event: %w", err)
	}

	// Ensure data is flushed to disk
	if err := f.Sync(); err != nil {
		return fmt.Errorf("syncing file: %w", err)
	}

	return nil
}

// WriteEvents writes multiple events atomically.
func (w *EventWriter) WriteEvents(events []*Event) error {
	if len(events) == 0 {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Marshal all events
	var data []byte
	for _, event := range events {
		line, err := json.Marshal(event.ToJSONL())
		if err != nil {
			return fmt.Errorf("marshaling event: %w", err)
		}
		data = append(data, line...)
		data = append(data, '\n')
	}

	// Open file with append mode
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	// Acquire exclusive lock
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	// Write all events
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("writing events: %w", err)
	}

	if err := f.Sync(); err != nil {
		return fmt.Errorf("syncing file: %w", err)
	}

	return nil
}

// RotateIfNeeded rotates the log file if it's older than maxAge.
// Returns the path to the rotated file, or empty string if no rotation occurred.
func (w *EventWriter) RotateIfNeeded() (string, error) {
	if w.maxAge == 0 {
		return "", nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	info, err := os.Stat(w.path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("stating file: %w", err)
	}

	// Check if file is old enough to rotate
	if time.Since(info.ModTime()) < w.maxAge {
		return "", nil
	}

	// Generate rotated filename with timestamp
	rotatedPath := w.path + "." + time.Now().Format("2006-01-02-150405")

	// Acquire lock before rotating
	f, err := os.OpenFile(w.path, os.O_RDONLY, 0644)
	if err != nil {
		return "", fmt.Errorf("opening for rotation: %w", err)
	}
	defer f.Close()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return "", fmt.Errorf("acquiring lock for rotation: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	// Rename (atomic on same filesystem)
	if err := os.Rename(w.path, rotatedPath); err != nil {
		return "", fmt.Errorf("rotating file: %w", err)
	}

	return rotatedPath, nil
}

// Path returns the path to the events file.
func (w *EventWriter) Path() string {
	return w.path
}

// jsonlEvent is the JSONL format for events.
type jsonlEvent struct {
	Timestamp  string `json:"ts"`
	SessionID  string `json:"sid"`
	EventType  string `json:"type"`
	ToolName   string `json:"tool,omitempty"`
	Success    bool   `json:"ok,omitempty"`
	DurationMs int64  `json:"dur_ms,omitempty"`
	Cwd        string `json:"cwd,omitempty"`
}

// ToJSONL converts an Event to the JSONL format.
func (e *Event) ToJSONL() jsonlEvent {
	return jsonlEvent{
		Timestamp:  e.Timestamp.UTC().Format(time.RFC3339),
		SessionID:  e.SessionID,
		EventType:  e.EventType,
		ToolName:   e.ToolName,
		Success:    e.Success,
		DurationMs: e.DurationMs,
		Cwd:        e.Cwd,
	}
}
