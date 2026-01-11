package collector

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSessionWriter_WriteEvent(t *testing.T) {
	tmpDir := t.TempDir()
	eventsDir := filepath.Join(tmpDir, "events")

	writer, err := NewSessionWriter(eventsDir)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	event := &Event{
		Timestamp:  time.Now(),
		SessionID:  "test-session-123",
		EventType:  "PostToolUse",
		ToolName:   "Read",
		Success:    true,
		DurationMs: 100,
	}

	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}

	// Verify file was created with correct name
	expectedFile := filepath.Join(eventsDir, "test-session-123.jsonl")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", expectedFile)
	}

	// Verify content
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if len(data) == 0 {
		t.Error("file is empty")
	}
}

func TestSessionWriter_DifferentSessions_NoContention(t *testing.T) {
	tmpDir := t.TempDir()
	eventsDir := filepath.Join(tmpDir, "events")

	writer, err := NewSessionWriter(eventsDir)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	const numSessions = 5
	const eventsPerSession = 100

	var wg sync.WaitGroup
	wg.Add(numSessions)

	start := time.Now()

	// Launch concurrent writers for DIFFERENT sessions
	for i := 0; i < numSessions; i++ {
		go func(sessionNum int) {
			defer wg.Done()
			sessionID := string(rune('A'+sessionNum)) + "-session"

			for j := 0; j < eventsPerSession; j++ {
				event := &Event{
					Timestamp:  time.Now(),
					SessionID:  sessionID,
					EventType:  "PostToolUse",
					ToolName:   "Read",
					Success:    true,
					DurationMs: int64(j),
				}
				if err := writer.WriteEvent(event); err != nil {
					t.Errorf("session %s failed to write: %v", sessionID, err)
				}
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Should be fast since no contention
	t.Logf("Wrote %d events across %d sessions in %v", numSessions*eventsPerSession, numSessions, elapsed)

	// Verify each session file has correct number of events
	files, err := writer.ListSessionFiles()
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if len(files) != numSessions {
		t.Errorf("expected %d files, got %d", numSessions, len(files))
	}

	totalEvents := 0
	for _, file := range files {
		count := countLines(t, file)
		if count != eventsPerSession {
			t.Errorf("file %s has %d events, expected %d", file, count, eventsPerSession)
		}
		totalEvents += count
	}

	if totalEvents != numSessions*eventsPerSession {
		t.Errorf("total events %d != expected %d", totalEvents, numSessions*eventsPerSession)
	}
}

// TestSessionWriter_SameSession_AgentAndSubagent simulates the scenario where
// a main agent and subagents write to the same session file concurrently.
// This is a realistic scenario in Claude Code where Task agents run in parallel.
func TestSessionWriter_SameSession_AgentAndSubagent(t *testing.T) {
	tmpDir := t.TempDir()
	eventsDir := filepath.Join(tmpDir, "events")

	writer, err := NewSessionWriter(eventsDir)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	const sessionID = "main-session-with-subagents"
	const numAgents = 5       // 1 main + 4 subagents
	const eventsPerAgent = 50 // Each agent writes 50 events

	var wg sync.WaitGroup
	wg.Add(numAgents)

	// Simulate main agent and subagents writing concurrently to SAME session
	for i := 0; i < numAgents; i++ {
		go func(agentNum int) {
			defer wg.Done()
			agentName := "main"
			if agentNum > 0 {
				agentName = string(rune('A'+agentNum-1)) + "-subagent"
			}

			for j := 0; j < eventsPerAgent; j++ {
				event := &Event{
					Timestamp:  time.Now(),
					SessionID:  sessionID, // SAME session for all agents!
					EventType:  "PostToolUse",
					ToolName:   agentName + "-tool",
					Success:    true,
					DurationMs: int64(j),
				}
				if err := writer.WriteEvent(event); err != nil {
					t.Errorf("agent %s failed to write: %v", agentName, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify single file was created
	files, err := writer.ListSessionFiles()
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 file (same session), got %d", len(files))
	}

	// Count lines and verify no corruption
	expectedEvents := numAgents * eventsPerAgent
	sessionFile := filepath.Join(eventsDir, sessionID+".jsonl")

	f, err := os.Open(sessionFile)
	if err != nil {
		t.Fatalf("failed to open session file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineCount := 0
	corruptedLines := 0

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		lineCount++

		// Verify each line is valid JSON
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			corruptedLines++
			t.Logf("Corrupted line %d: %s", lineCount, line)
		}
	}

	if corruptedLines > 0 {
		t.Errorf("Found %d corrupted lines out of %d", corruptedLines, lineCount)
	}

	if lineCount != expectedEvents {
		t.Errorf("expected %d events, got %d", expectedEvents, lineCount)
	}

	t.Logf("Successfully wrote %d events from %d concurrent agents to same session file", lineCount, numAgents)
}

// TestSessionWriter_HighContention_SameSession tests extreme contention
// with many goroutines writing to the same session file simultaneously.
func TestSessionWriter_HighContention_SameSession(t *testing.T) {
	tmpDir := t.TempDir()
	eventsDir := filepath.Join(tmpDir, "events")

	writer, err := NewSessionWriter(eventsDir)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	const sessionID = "high-contention-session"
	const numGoroutines = 20
	const eventsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	start := time.Now()
	errors := make(chan error, numGoroutines*eventsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				event := &Event{
					Timestamp:  time.Now(),
					SessionID:  sessionID,
					EventType:  "PostToolUse",
					ToolName:   "Worker-" + string(rune('0'+workerID)),
					Success:    true,
					DurationMs: int64(workerID*100 + j),
				}
				if err := writer.WriteEvent(event); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)
	elapsed := time.Since(start)

	// Check for errors
	errorCount := 0
	for err := range errors {
		errorCount++
		t.Logf("Write error: %v", err)
	}

	if errorCount > 0 {
		t.Errorf("Got %d write errors", errorCount)
	}

	// Verify data integrity
	sessionFile := filepath.Join(eventsDir, sessionID+".jsonl")
	lineCount := countLines(t, sessionFile)
	expectedEvents := numGoroutines * eventsPerGoroutine

	if lineCount != expectedEvents {
		t.Errorf("expected %d events, got %d (lost %d)", expectedEvents, lineCount, expectedEvents-lineCount)
	}

	// Verify no corruption
	verifyJSONLIntegrity(t, sessionFile)

	t.Logf("High contention test: %d goroutines × %d events = %d total in %v",
		numGoroutines, eventsPerGoroutine, lineCount, elapsed)
}

func TestSessionWriter_ListSessionFiles(t *testing.T) {
	tmpDir := t.TempDir()
	eventsDir := filepath.Join(tmpDir, "events")

	writer, err := NewSessionWriter(eventsDir)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	// Write to multiple sessions
	sessions := []string{"sess-a", "sess-b", "sess-c"}
	for _, sid := range sessions {
		event := &Event{
			Timestamp: time.Now(),
			SessionID: sid,
			EventType: "Stop",
		}
		if err := writer.WriteEvent(event); err != nil {
			t.Fatalf("failed to write: %v", err)
		}
	}

	files, err := writer.ListSessionFiles()
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if len(files) != len(sessions) {
		t.Errorf("expected %d files, got %d", len(sessions), len(files))
	}
}

func TestSessionWriter_CleanupOldSessions(t *testing.T) {
	tmpDir := t.TempDir()
	eventsDir := filepath.Join(tmpDir, "events")

	writer, err := NewSessionWriter(eventsDir)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	// Write to a session
	event := &Event{
		Timestamp: time.Now(),
		SessionID: "old-session",
		EventType: "Stop",
	}
	if err := writer.WriteEvent(event); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// Wait a bit and cleanup with short duration
	time.Sleep(10 * time.Millisecond)
	removed, err := writer.CleanupOldSessions(1 * time.Millisecond)
	if err != nil {
		t.Fatalf("failed to cleanup: %v", err)
	}

	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	// Verify file is gone
	files, _ := writer.ListSessionFiles()
	if len(files) != 0 {
		t.Errorf("expected 0 files after cleanup, got %d", len(files))
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dash", "with-dash"},
		{"with_underscore", "with_underscore"},
		{"with spaces", "with_spaces"},
		{"with/slashes", "with_slashes"},
		{"with:colons", "with_colons"},
		{"MixedCase123", "MixedCase123"},
		{"special!@#$%", "special_____"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMultiFileParser_ParseAllSessions(t *testing.T) {
	tmpDir := t.TempDir()
	eventsDir := filepath.Join(tmpDir, "events")

	writer, err := NewSessionWriter(eventsDir)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	// Write events to multiple sessions with different timestamps
	baseTime := time.Date(2026, 1, 10, 10, 0, 0, 0, time.UTC)

	sessions := []struct {
		id     string
		offset time.Duration
	}{
		{"sess-c", 2 * time.Minute}, // Latest
		{"sess-a", 0},               // Earliest
		{"sess-b", 1 * time.Minute}, // Middle
	}

	for _, s := range sessions {
		event := &Event{
			Timestamp: baseTime.Add(s.offset),
			SessionID: s.id,
			EventType: "PostToolUse",
			ToolName:  "Read",
			Success:   true,
		}
		if err := writer.WriteEvent(event); err != nil {
			t.Fatalf("failed to write: %v", err)
		}
	}

	// Parse all sessions
	parser := NewMultiFileParser(eventsDir)
	events, err := parser.ParseAllSessions()
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	// Verify sorted by timestamp (earliest first)
	if events[0].SessionID != "sess-a" {
		t.Errorf("expected first event from sess-a, got %s", events[0].SessionID)
	}
	if events[1].SessionID != "sess-b" {
		t.Errorf("expected second event from sess-b, got %s", events[1].SessionID)
	}
	if events[2].SessionID != "sess-c" {
		t.Errorf("expected third event from sess-c, got %s", events[2].SessionID)
	}
}

// Helper functions

func countLines(t *testing.T, filename string) int {
	t.Helper()
	f, err := os.Open(filename)
	if err != nil {
		t.Fatalf("failed to open %s: %v", filename, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		if scanner.Text() != "" {
			count++
		}
	}
	return count
}

func verifyJSONLIntegrity(t *testing.T, filename string) {
	t.Helper()
	f, err := os.Open(filename)
	if err != nil {
		t.Fatalf("failed to open %s: %v", filename, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Errorf("line %d is not valid JSON: %v\nContent: %s", lineNum, err, line)
		}
	}
}
