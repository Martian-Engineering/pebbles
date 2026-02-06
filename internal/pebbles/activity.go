package pebbles

import (
	"fmt"
	"time"
)

// ListIssueActivity returns the most recent activity timestamp for each issue.
func ListIssueActivity(root string) (map[string]time.Time, error) {
	// Ensure the cache is current so rename lookups are accurate.
	if err := EnsureCache(root); err != nil {
		return nil, err
	}
	db, err := openDB(DBPath(root))
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	// Load the event log and track the latest activity per issue.
	events, err := LoadEvents(root)
	if err != nil {
		return nil, err
	}
	activity := make(map[string]time.Time, len(events))
	for _, event := range events {
		if !isActivityEvent(event.Type) {
			continue
		}
		resolvedID, err := resolveIssueID(db, event.IssueID)
		if err != nil {
			return nil, err
		}
		timestamp, err := time.Parse(time.RFC3339Nano, event.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("parse activity timestamp for %s: %w", resolvedID, err)
		}
		if last, ok := activity[resolvedID]; !ok || timestamp.After(last) {
			activity[resolvedID] = timestamp
		}
	}
	return activity, nil
}

// isActivityEvent reports whether an event should count toward issue activity.
func isActivityEvent(eventType string) bool {
	switch eventType {
	case EventTypeCreate, EventTypeTitleUpdated, EventTypeUpdate, EventTypeComment, EventTypeStatus, EventTypeClose:
		return true
	default:
		return false
	}
}
