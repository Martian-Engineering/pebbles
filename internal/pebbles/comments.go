package pebbles

import (
	"database/sql"
	"fmt"
)

// ListIssueComments returns comment events for an issue in append order.
func ListIssueComments(root, id string) ([]IssueComment, error) {
	if err := EnsureCache(root); err != nil {
		return nil, err
	}
	db, err := openDB(DBPath(root))
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	// Resolve the requested ID before comparing against log entries.
	resolvedID, err := resolveIssueID(db, id)
	if err != nil {
		return nil, err
	}
	events, err := LoadEvents(root)
	if err != nil {
		return nil, err
	}
	comments := make([]IssueComment, 0)
	for _, event := range events {
		if event.Type != EventTypeComment {
			continue
		}
		comment, ok, err := commentFromEvent(db, resolvedID, event)
		if err != nil {
			return nil, err
		}
		if ok {
			comments = append(comments, comment)
		}
	}
	return comments, nil
}

// commentFromEvent builds a comment from an event if it matches the issue.
func commentFromEvent(db *sql.DB, resolvedID string, event Event) (IssueComment, bool, error) {
	resolvedEventID, err := resolveIssueID(db, event.IssueID)
	if err != nil {
		return IssueComment{}, false, err
	}
	if resolvedEventID != resolvedID {
		return IssueComment{}, false, nil
	}
	body := ""
	if event.Payload != nil {
		body = event.Payload["body"]
	}
	if body == "" {
		return IssueComment{}, false, fmt.Errorf("comment event missing body for %s", resolvedEventID)
	}
	return IssueComment{
		IssueID:   resolvedEventID,
		Body:      body,
		Timestamp: event.Timestamp,
	}, true, nil
}
