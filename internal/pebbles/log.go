package pebbles

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// EventLogEntry pairs an event with its line number in the log.
type EventLogEntry struct {
	Line  int
	Event Event
}

// LoadEventLog reads the event log and returns entries with line numbers.
func LoadEventLog(root string) ([]EventLogEntry, error) {
	return readEventLog(EventsPath(root))
}

// readEventLog reads a JSONL log file and records line numbers for each event.
func readEventLog(path string) ([]EventLogEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open events log: %w", err)
	}
	defer func() { _ = file.Close() }()
	// Scan the log line by line to capture line numbers.
	scanner := bufio.NewScanner(file)
	var entries []EventLogEntry
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Decode JSON into an Event, reporting the line on errors.
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("parse event line %d: %w", lineNumber, err)
		}
		entries = append(entries, EventLogEntry{Line: lineNumber, Event: event})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan events log: %w", err)
	}
	return entries, nil
}
