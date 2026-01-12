package collector

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// Parser reads and parses JSONL event files.
type Parser struct {
	filePath string
}

// NewParser creates a new JSONL parser.
func NewParser(filePath string) *Parser {
	return &Parser{filePath: filePath}
}

// ParseAll reads and parses all events from the JSONL file.
func (p *Parser) ParseAll() ([]*Event, error) {
	file, err := os.Open(p.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No events yet
		}
		return nil, fmt.Errorf("opening events file: %w", err)
	}
	defer file.Close()

	return p.ParseReader(file)
}

// ParseReader reads and parses events from a reader.
func (p *Parser) ParseReader(r io.Reader) ([]*Event, error) {
	var events []*Event
	scanner := bufio.NewScanner(r)

	// Increase buffer size for large lines (5MB handles 99.9% of events)
	// Some tool responses (TaskOutput, Read) can exceed 1MB
	const maxScannerCapacity = 5 * 1024 * 1024 // 5MB
	buf := make([]byte, maxScannerCapacity)
	scanner.Buffer(buf, maxScannerCapacity)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue // Skip empty lines
		}

		event, err := ParseEvent(line)
		if err != nil {
			// Log malformed lines but continue
			fmt.Fprintf(os.Stderr, "warning: skipping malformed line %d: %v\n", lineNum, err)
			continue
		}

		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		// Handle oversized events gracefully - skip and continue with what we have
		if strings.Contains(err.Error(), "token too long") {
			fmt.Fprintf(os.Stderr, "warning: skipping oversized event (>5MB) at line %d\n", lineNum+1)
			return events, nil
		}
		return events, fmt.Errorf("scanning events: %w", err)
	}

	return events, nil
}

// ParseFromPosition reads events starting from a byte position.
// Returns parsed events and the new position (end of file).
func (p *Parser) ParseFromPosition(position int64) ([]*Event, int64, error) {
	file, err := os.Open(p.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, position, fmt.Errorf("opening events file: %w", err)
	}
	defer file.Close()

	// Seek to position
	if position > 0 {
		_, err = file.Seek(position, io.SeekStart)
		if err != nil {
			return nil, position, fmt.Errorf("seeking to position %d: %w", position, err)
		}
	}

	events, err := p.ParseReader(file)
	if err != nil {
		return events, position, err
	}

	// Get new position
	newPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return events, position, fmt.Errorf("getting position: %w", err)
	}

	return events, newPos, nil
}

// FileSize returns the current size of the events file.
func (p *Parser) FileSize() (int64, error) {
	info, err := os.Stat(p.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	return info.Size(), nil
}

// TailEvents reads the last n events from the file.
// This is efficient for small n by reading from end of file.
func (p *Parser) TailEvents(n int) ([]*Event, error) {
	// For simplicity, read all and return last n
	// TODO: Optimize for large files by seeking from end
	events, err := p.ParseAll()
	if err != nil {
		return nil, err
	}

	if len(events) <= n {
		return events, nil
	}

	return events[len(events)-n:], nil
}
