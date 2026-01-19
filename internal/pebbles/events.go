package pebbles

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// AppendEvent appends a single event to the events log.
func AppendEvent(root string, event Event) error {
	path := EventsPath(root)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open events log: %w", err)
	}
	defer func() { _ = file.Close() }()
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	return nil
}

// LoadEvents reads all events from the events log.
func LoadEvents(root string) ([]Event, error) {
	return readEvents(EventsPath(root))
}

// readEvents reads events from a JSONL file path.
func readEvents(path string) ([]Event, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open events log: %w", err)
	}
	defer func() { _ = file.Close() }()
	// Scan the file line by line to decode JSONL records.
	scanner := bufio.NewScanner(file)
	var events []Event
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		// Decode each event line into the Event struct.
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			return nil, fmt.Errorf("parse event: %w", err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan events log: %w", err)
	}
	return events, nil
}
