package pebbles

import "fmt"

// NewCreateEvent builds a create event for an issue.
func NewCreateEvent(issueID, title, description, issueType, timestamp string, priority int) Event {
	payload := map[string]string{
		"title":       title,
		"description": description,
		"type":        issueType,
		"priority":    fmt.Sprintf("%d", priority),
	}
	return Event{Type: EventTypeCreate, Timestamp: timestamp, IssueID: issueID, Payload: payload}
}

// NewStatusEvent builds a status update event.
func NewStatusEvent(issueID, status, timestamp string) Event {
	payload := map[string]string{"status": status}
	return Event{Type: EventTypeStatus, Timestamp: timestamp, IssueID: issueID, Payload: payload}
}

// NewUpdateEvent builds an issue field update event.
func NewUpdateEvent(issueID, timestamp string, payload map[string]string) Event {
	return Event{Type: EventTypeUpdate, Timestamp: timestamp, IssueID: issueID, Payload: payload}
}

// NewCloseEvent builds a close event.
func NewCloseEvent(issueID, timestamp string) Event {
	return Event{Type: EventTypeClose, Timestamp: timestamp, IssueID: issueID, Payload: map[string]string{}}
}

// NewCommentEvent builds a comment event.
func NewCommentEvent(issueID, body, timestamp string) Event {
	payload := map[string]string{"body": body}
	return Event{Type: EventTypeComment, Timestamp: timestamp, IssueID: issueID, Payload: payload}
}

// NewRenameEvent builds a rename event for an issue ID change.
func NewRenameEvent(issueID, newIssueID, timestamp string) Event {
	payload := map[string]string{"new_id": newIssueID}
	return Event{Type: EventTypeRename, Timestamp: timestamp, IssueID: issueID, Payload: payload}
}

// NewDepAddEvent builds a dependency add event.
func NewDepAddEvent(issueID, dependsOn, depType, timestamp string) Event {
	payload := map[string]string{
		"depends_on": dependsOn,
		"dep_type":   NormalizeDepType(depType),
	}
	return Event{Type: EventTypeDepAdd, Timestamp: timestamp, IssueID: issueID, Payload: payload}
}

// NewDepRemoveEvent builds a dependency removal event.
func NewDepRemoveEvent(issueID, dependsOn, depType, timestamp string) Event {
	payload := map[string]string{
		"depends_on": dependsOn,
		"dep_type":   NormalizeDepType(depType),
	}
	return Event{Type: EventTypeDepRemove, Timestamp: timestamp, IssueID: issueID, Payload: payload}
}
