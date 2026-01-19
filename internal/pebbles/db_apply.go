package pebbles

import (
	"database/sql"
	"fmt"
)

// resetSchema drops the issue and dependency tables.
func resetSchema(db *sql.DB) error {
	queries := []string{
		"DROP TABLE IF EXISTS deps",
		"DROP TABLE IF EXISTS issues",
	}
	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("reset schema: %w", err)
		}
	}
	return nil
}

// ensureSchema creates the issue and dependency tables.
func ensureSchema(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS issues (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			issue_type TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			closed_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS deps (
			issue_id TEXT NOT NULL,
			depends_on_id TEXT NOT NULL,
			PRIMARY KEY (issue_id, depends_on_id)
		)`,
	}
	// Execute each schema statement in order.
	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
	}
	return nil
}

// applyEvents replays events into the SQLite cache.
func applyEvents(db *sql.DB, events []Event) error {
	for _, event := range events {
		if err := applyEvent(db, event); err != nil {
			return err
		}
	}
	return nil
}

// applyEvent applies a single event into the SQLite cache.
func applyEvent(db *sql.DB, event Event) error {
	switch event.Type {
	case EventTypeCreate:
		return applyCreate(db, event)
	case EventTypeStatus:
		return applyStatus(db, event)
	case EventTypeClose:
		return applyClose(db, event)
	case EventTypeDepAdd:
		return applyDepAdd(db, event)
	default:
		return fmt.Errorf("unknown event type: %s", event.Type)
	}
}

// applyCreate inserts a new issue from a create event.
func applyCreate(db *sql.DB, event Event) error {
	title, ok := event.Payload["title"]
	if !ok || title == "" {
		return fmt.Errorf("create event missing title")
	}
	// Default issue type to task when omitted.
	issueType := event.Payload["type"]
	if issueType == "" {
		issueType = "task"
	}
	// Insert the new issue using the event timestamp.
	_, err := db.Exec(
		`INSERT INTO issues (id, title, issue_type, status, created_at, updated_at, closed_at)
		 VALUES (?, ?, ?, ?, ?, ?, "")`,
		event.IssueID,
		title,
		issueType,
		StatusOpen,
		event.Timestamp,
		event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert issue: %w", err)
	}
	return nil
}

// applyStatus updates an issue status from a status update event.
func applyStatus(db *sql.DB, event Event) error {
	status := event.Payload["status"]
	if status == "" {
		return fmt.Errorf("status event missing status")
	}
	// Apply the status update and touch updated_at.
	result, err := db.Exec(
		"UPDATE issues SET status = ?, updated_at = ? WHERE id = ?",
		status,
		event.Timestamp,
		event.IssueID,
	)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return requireRow(result, "status update for missing issue")
}

// applyClose closes an issue from a close event.
func applyClose(db *sql.DB, event Event) error {
	// Close the issue and stamp updated_at/closed_at.
	result, err := db.Exec(
		"UPDATE issues SET status = ?, updated_at = ?, closed_at = ? WHERE id = ?",
		StatusClosed,
		event.Timestamp,
		event.Timestamp,
		event.IssueID,
	)
	if err != nil {
		return fmt.Errorf("close issue: %w", err)
	}
	return requireRow(result, "close for missing issue")
}

// applyDepAdd inserts a dependency from a dep_add event.
func applyDepAdd(db *sql.DB, event Event) error {
	dependsOn := event.Payload["depends_on"]
	if dependsOn == "" {
		return fmt.Errorf("dep_add event missing depends_on")
	}
	// Validate both ends exist before writing the dependency.
	if err := ensureIssueExists(db, event.IssueID); err != nil {
		return err
	}
	if err := ensureIssueExists(db, dependsOn); err != nil {
		return err
	}
	// Insert a dependency edge, ignoring duplicates.
	_, err := db.Exec(
		"INSERT OR IGNORE INTO deps (issue_id, depends_on_id) VALUES (?, ?)",
		event.IssueID,
		dependsOn,
	)
	if err != nil {
		return fmt.Errorf("insert dependency: %w", err)
	}
	return nil
}

// ensureIssueExists verifies a referenced issue exists.
func ensureIssueExists(db *sql.DB, issueID string) error {
	var count int
	row := db.QueryRow("SELECT COUNT(1) FROM issues WHERE id = ?", issueID)
	if err := row.Scan(&count); err != nil {
		return fmt.Errorf("check issue exists: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("missing issue: %s", issueID)
	}
	return nil
}

// requireRow ensures a SQL update affected at least one row.
func requireRow(result sql.Result, msg string) error {
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("%s", msg)
	}
	return nil
}
