package collector

import (
	"bufio"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestEventWriter_WriteEvent(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	writer, err := NewEventWriter(WriterConfig{
		EventsFile: eventsFile,
	})
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	event := &Event{
		Timestamp:  time.Now(),
		SessionID:  "test-session",
		EventType:  "PostToolUse",
		ToolName:   "Read",
		Success:    true,
		DurationMs: 100,
	}

	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}

	// Verify file contents
	data, err := os.ReadFile(eventsFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if len(data) == 0 {
		t.Error("file is empty")
	}
}

func TestEventWriter_ConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	writer, err := NewEventWriter(WriterConfig{
		EventsFile: eventsFile,
	})
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	const numGoroutines = 10
	const eventsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch concurrent writers
	for i := 0; i < numGoroutines; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				event := &Event{
					Timestamp:  time.Now(),
					SessionID:  "session-" + string(rune('A'+workerID)),
					EventType:  "PostToolUse",
					ToolName:   "Read",
					Success:    true,
					DurationMs: int64(j),
				}
				if err := writer.WriteEvent(event); err != nil {
					t.Errorf("worker %d failed to write: %v", workerID, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Count lines in file
	f, err := os.Open(eventsFile)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		lineCount++
		// Verify each line is valid JSON (not corrupted)
		if line[0] != '{' || line[len(line)-1] != '}' {
			t.Errorf("corrupted line %d: %s", lineCount, line)
		}
	}

	expectedLines := numGoroutines * eventsPerGoroutine
	if lineCount != expectedLines {
		t.Errorf("expected %d lines, got %d", expectedLines, lineCount)
	}
}

func TestEventWriter_WriteEvents_Batch(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	writer, err := NewEventWriter(WriterConfig{
		EventsFile: eventsFile,
	})
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	events := make([]*Event, 5)
	for i := 0; i < 5; i++ {
		events[i] = &Event{
			Timestamp:  time.Now().Add(time.Duration(i) * time.Second),
			SessionID:  "test-session",
			EventType:  "PostToolUse",
			ToolName:   "Read",
			Success:    true,
			DurationMs: int64(i * 10),
		}
	}

	if err := writer.WriteEvents(events); err != nil {
		t.Fatalf("failed to write events: %v", err)
	}

	// Count lines
	f, err := os.Open(eventsFile)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	if lineCount != 5 {
		t.Errorf("expected 5 lines, got %d", lineCount)
	}
}

func TestEventWriter_DirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "subdir", "deep", "events.jsonl")

	writer, err := NewEventWriter(WriterConfig{
		EventsFile: eventsFile,
	})
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	event := &Event{
		Timestamp: time.Now(),
		SessionID: "test",
		EventType: "Stop",
	}

	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(filepath.Dir(eventsFile)); os.IsNotExist(err) {
		t.Error("directory was not created")
	}
}

func TestEventWriter_RotateIfNeeded(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	// Create writer with very short rotation period for testing
	writer := &EventWriter{
		path:   eventsFile,
		maxAge: 1 * time.Millisecond, // Very short for testing
	}

	// Write initial event
	event := &Event{
		Timestamp: time.Now(),
		SessionID: "test",
		EventType: "Stop",
	}
	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}

	// Wait for file to be "old enough"
	time.Sleep(10 * time.Millisecond)

	// Rotate
	rotatedPath, err := writer.RotateIfNeeded()
	if err != nil {
		t.Fatalf("failed to rotate: %v", err)
	}

	if rotatedPath == "" {
		t.Error("expected rotation to occur")
	}

	// Verify rotated file exists
	if _, err := os.Stat(rotatedPath); os.IsNotExist(err) {
		t.Errorf("rotated file does not exist: %s", rotatedPath)
	}

	// Verify original path no longer exists
	if _, err := os.Stat(eventsFile); !os.IsNotExist(err) {
		t.Error("original file should have been renamed")
	}
}

func TestEvent_ToJSONL(t *testing.T) {
	ts := time.Date(2026, 1, 10, 10, 0, 0, 0, time.UTC)
	event := &Event{
		Timestamp:  ts,
		SessionID:  "sess-1",
		EventType:  "PostToolUse",
		ToolName:   "mcp__github__create_issue",
		Success:    true,
		DurationMs: 150,
		Cwd:        "/home/user",
	}

	jsonl := event.ToJSONL()

	if jsonl.Timestamp != "2026-01-10T10:00:00Z" {
		t.Errorf("unexpected timestamp: %s", jsonl.Timestamp)
	}
	if jsonl.SessionID != "sess-1" {
		t.Errorf("unexpected session_id: %s", jsonl.SessionID)
	}
	if jsonl.EventType != "PostToolUse" {
		t.Errorf("unexpected event_type: %s", jsonl.EventType)
	}
	if jsonl.ToolName != "mcp__github__create_issue" {
		t.Errorf("unexpected tool_name: %s", jsonl.ToolName)
	}
	if !jsonl.Success {
		t.Error("expected success to be true")
	}
	if jsonl.DurationMs != 150 {
		t.Errorf("unexpected duration_ms: %d", jsonl.DurationMs)
	}
	if jsonl.Cwd != "/home/user" {
		t.Errorf("unexpected cwd: %s", jsonl.Cwd)
	}
}
