package pebbles

// Event represents an append-only change in the Pebbles log.
type Event struct {
	Type      string            `json:"type"`
	Timestamp string            `json:"timestamp"`
	IssueID   string            `json:"issue_id"`
	Payload   map[string]string `json:"payload"`
}

// Issue represents the current state of a Pebbles issue.
type Issue struct {
	ID        string
	Title     string
	IssueType string
	Status    string
	CreatedAt string
	UpdatedAt string
	ClosedAt  string
}

// Config stores per-project Pebbles settings.
type Config struct {
	Prefix string `json:"prefix"`
}

const (
	// EventTypeCreate indicates a create event.
	EventTypeCreate = "create"
	// EventTypeStatus indicates a status update event.
	EventTypeStatus = "status_update"
	// EventTypeClose indicates a close event.
	EventTypeClose = "close"
	// EventTypeDepAdd indicates a dependency add event.
	EventTypeDepAdd = "dep_add"
)

const (
	// StatusOpen indicates an open issue.
	StatusOpen = "open"
	// StatusClosed indicates a closed issue.
	StatusClosed = "closed"
)
