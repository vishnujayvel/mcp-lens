package collector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParser_ParseAll(t *testing.T) {
	// Create temp file with test events
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	content := `{"ts":"2026-01-10T10:00:00Z","sid":"sess-1","type":"SessionStart","cwd":"/home/user"}
{"ts":"2026-01-10T10:01:00Z","sid":"sess-1","type":"PostToolUse","tool":"Read","ok":true}
{"ts":"2026-01-10T10:02:00Z","sid":"sess-1","type":"PostToolUse","tool":"mcp__github__create_issue","ok":true}
{"ts":"2026-01-10T10:03:00Z","sid":"sess-1","type":"SessionEnd"}
`
	if err := os.WriteFile(eventsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	parser := NewParser(eventsFile)
	events, err := parser.ParseAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	if events[0].EventType != "SessionStart" {
		t.Errorf("expected first event type 'SessionStart', got '%s'", events[0].EventType)
	}
	if events[2].ToolName != "mcp__github__create_issue" {
		t.Errorf("expected tool name for third event, got '%s'", events[2].ToolName)
	}
}

func TestParser_ParseAll_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	if err := os.WriteFile(eventsFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	parser := NewParser(eventsFile)
	events, err := parser.ParseAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestParser_ParseAll_NonExistentFile(t *testing.T) {
	parser := NewParser("/nonexistent/path/events.jsonl")
	events, err := parser.ParseAll()
	if err != nil {
		t.Fatalf("expected nil error for non-existent file, got: %v", err)
	}
	if events != nil {
		t.Errorf("expected nil events for non-existent file")
	}
}

func TestParser_ParseAll_MalformedLines(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	// Include some malformed lines
	content := `{"ts":"2026-01-10T10:00:00Z","sid":"sess-1","type":"SessionStart"}
{invalid json}
{"ts":"2026-01-10T10:01:00Z","sid":"sess-1","type":"PostToolUse","tool":"Read","ok":true}
not json at all
{"ts":"2026-01-10T10:02:00Z","sid":"sess-1","type":"SessionEnd"}
`
	if err := os.WriteFile(eventsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	parser := NewParser(eventsFile)
	events, err := parser.ParseAll()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have parsed 3 valid events, skipped 2 malformed
	if len(events) != 3 {
		t.Errorf("expected 3 events (skipping malformed), got %d", len(events))
	}
}

func TestParser_ParseFromPosition(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	line1 := `{"ts":"2026-01-10T10:00:00Z","sid":"sess-1","type":"SessionStart"}`
	line2 := `{"ts":"2026-01-10T10:01:00Z","sid":"sess-1","type":"PostToolUse","tool":"Read","ok":true}`
	line3 := `{"ts":"2026-01-10T10:02:00Z","sid":"sess-1","type":"SessionEnd"}`

	content := line1 + "\n" + line2 + "\n" + line3 + "\n"
	if err := os.WriteFile(eventsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	parser := NewParser(eventsFile)

	// First read all events to get positions
	events, newPos, err := parser.ParseFromPosition(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Position should be at end of file
	fileInfo, _ := os.Stat(eventsFile)
	if newPos != fileInfo.Size() {
		t.Errorf("expected position %d, got %d", fileInfo.Size(), newPos)
	}

	// Read from position after first line - should get 2 events
	firstLinePos := int64(len(line1) + 1) // +1 for newline
	events, _, err = parser.ParseFromPosition(firstLinePos)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events from position, got %d", len(events))
	}
}

func TestParser_ParseFromPosition_NoNewEvents(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	content := `{"ts":"2026-01-10T10:00:00Z","sid":"sess-1","type":"SessionStart"}
`
	if err := os.WriteFile(eventsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	parser := NewParser(eventsFile)

	// Read all
	_, pos, _ := parser.ParseFromPosition(0)

	// Read from end - should get no new events
	events, newPos, err := parser.ParseFromPosition(pos)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 new events, got %d", len(events))
	}
	if newPos != pos {
		t.Errorf("expected position unchanged, was %d now %d", pos, newPos)
	}
}

func TestParser_FileSize(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	content := `{"ts":"2026-01-10T10:00:00Z","sid":"sess-1","type":"SessionStart"}`
	if err := os.WriteFile(eventsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	parser := NewParser(eventsFile)
	size, err := parser.FileSize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), size)
	}
}

func TestParser_FileSize_NonExistent(t *testing.T) {
	parser := NewParser("/nonexistent/file")
	size, err := parser.FileSize()
	if err != nil {
		t.Fatalf("expected nil error for non-existent file, got: %v", err)
	}
	if size != 0 {
		t.Errorf("expected size 0 for non-existent file, got %d", size)
	}
}

func TestParser_TailEvents(t *testing.T) {
	tmpDir := t.TempDir()
	eventsFile := filepath.Join(tmpDir, "events.jsonl")

	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, `{"ts":"2026-01-10T10:00:00Z","sid":"sess-1","type":"PostToolUse"}`)
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(eventsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	parser := NewParser(eventsFile)

	// Get last 3 events
	events, err := parser.TailEvents(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	// Get more events than exist
	events, err = parser.TailEvents(20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 10 {
		t.Errorf("expected 10 events (all), got %d", len(events))
	}
}

func TestParser_ParseReader(t *testing.T) {
	content := `{"ts":"2026-01-10T10:00:00Z","sid":"sess-1","type":"SessionStart"}
{"ts":"2026-01-10T10:01:00Z","sid":"sess-1","type":"PostToolUse","tool":"Bash","ok":true}
`
	reader := strings.NewReader(content)

	parser := NewParser("")
	events, err := parser.ParseReader(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}
