package pebbles

// NewCreateEvent builds a create event for an issue.
func NewCreateEvent(issueID, title, issueType, timestamp string) Event {
	payload := map[string]string{
		"title": title,
		"type":  issueType,
	}
	return Event{Type: EventTypeCreate, Timestamp: timestamp, IssueID: issueID, Payload: payload}
}

// NewStatusEvent builds a status update event.
func NewStatusEvent(issueID, status, timestamp string) Event {
	payload := map[string]string{"status": status}
	return Event{Type: EventTypeStatus, Timestamp: timestamp, IssueID: issueID, Payload: payload}
}

// NewCloseEvent builds a close event.
func NewCloseEvent(issueID, timestamp string) Event {
	return Event{Type: EventTypeClose, Timestamp: timestamp, IssueID: issueID, Payload: map[string]string{}}
}

// NewDepAddEvent builds a dependency add event.
func NewDepAddEvent(issueID, dependsOn, timestamp string) Event {
	payload := map[string]string{"depends_on": dependsOn}
	return Event{Type: EventTypeDepAdd, Timestamp: timestamp, IssueID: issueID, Payload: payload}
}
