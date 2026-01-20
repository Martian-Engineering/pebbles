package pebbles

import (
	"database/sql"
	"fmt"
	"strings"
)

// resetSchema drops the issue and dependency tables.
func resetSchema(db *sql.DB) error {
	queries := []string{
		"DROP TABLE IF EXISTS deps",
		"DROP TABLE IF EXISTS issues",
		"DROP TABLE IF EXISTS renames",
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
			description TEXT NOT NULL,
			issue_type TEXT NOT NULL,
			status TEXT NOT NULL,
			priority INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			closed_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS deps (
			issue_id TEXT NOT NULL,
			depends_on_id TEXT NOT NULL,
			dep_type TEXT NOT NULL,
			PRIMARY KEY (issue_id, depends_on_id, dep_type)
		)`,
		`CREATE TABLE IF NOT EXISTS renames (
			old_id TEXT PRIMARY KEY,
			new_id TEXT NOT NULL
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
	case EventTypeRename:
		return applyRename(db, event)
	case EventTypeStatus:
		resolved, err := resolveEventIssueID(db, event)
		if err != nil {
			return err
		}
		return applyStatus(db, resolved)
	case EventTypeUpdate:
		resolved, err := resolveEventIssueID(db, event)
		if err != nil {
			return err
		}
		return applyUpdate(db, resolved)
	case EventTypeClose:
		resolved, err := resolveEventIssueID(db, event)
		if err != nil {
			return err
		}
		return applyClose(db, resolved)
	case EventTypeComment:
		resolved, err := resolveEventIssueID(db, event)
		if err != nil {
			return err
		}
		return applyComment(db, resolved)
	case EventTypeDepAdd:
		resolved, err := resolveEventDependencyIDs(db, event)
		if err != nil {
			return err
		}
		return applyDepAdd(db, resolved)
	case EventTypeDepRemove:
		resolved, err := resolveEventDependencyIDs(db, event)
		if err != nil {
			return err
		}
		return applyDepRemove(db, resolved)
	default:
		return fmt.Errorf("unknown event type: %s", event.Type)
	}
}

// resolveEventIssueID returns a copy of the event with a resolved IssueID.
func resolveEventIssueID(db *sql.DB, event Event) (Event, error) {
	resolvedID, err := resolveIssueID(db, event.IssueID)
	if err != nil {
		return Event{}, err
	}
	event.IssueID = resolvedID
	return event, nil
}

// resolveEventDependencyIDs resolves dependency IDs to their current values.
func resolveEventDependencyIDs(db *sql.DB, event Event) (Event, error) {
	resolvedIssueID, err := resolveIssueID(db, event.IssueID)
	if err != nil {
		return Event{}, err
	}
	dependsOn := event.Payload["depends_on"]
	if dependsOn == "" {
		return Event{}, fmt.Errorf("dependency event missing depends_on")
	}
	depType := NormalizeDepType(event.Payload["dep_type"])
	// Resolve the dependency target before rewriting the event payload.
	resolvedDependsOn, err := resolveIssueID(db, dependsOn)
	if err != nil {
		return Event{}, err
	}
	event.IssueID = resolvedIssueID
	event.Payload = map[string]string{
		"depends_on": resolvedDependsOn,
		"dep_type":   depType,
	}
	return event, nil
}

// applyCreate inserts a new issue from a create event.
func applyCreate(db *sql.DB, event Event) error {
	title, ok := event.Payload["title"]
	if !ok || title == "" {
		return fmt.Errorf("create event missing title")
	}
	description := event.Payload["description"]
	// Default issue type to task when omitted.
	issueType := event.Payload["type"]
	if issueType == "" {
		issueType = "task"
	}
	priority := parsePriority(event.Payload["priority"])
	// Insert the new issue using the event timestamp.
	_, err := db.Exec(
		`INSERT INTO issues (id, title, description, issue_type, status, priority, created_at, updated_at, closed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, "")`,
		event.IssueID,
		title,
		description,
		issueType,
		StatusOpen,
		priority,
		event.Timestamp,
		event.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert issue: %w", err)
	}
	return nil
}

// applyRename renames an issue ID and updates dependencies.
func applyRename(db *sql.DB, event Event) error {
	newID := event.Payload["new_id"]
	if newID == "" {
		return fmt.Errorf("rename event missing new_id")
	}
	// Resolve the current issue ID and validate the target ID.
	resolvedOldID, err := resolveIssueID(db, event.IssueID)
	if err != nil {
		return err
	}
	resolvedNewID, err := resolveIssueID(db, newID)
	if err != nil {
		return err
	}
	if resolvedNewID != newID {
		return fmt.Errorf("rename target already mapped to %s", resolvedNewID)
	}
	if resolvedOldID == newID {
		return fmt.Errorf("rename target matches current id")
	}
	if err := ensureIssueExists(db, resolvedOldID); err != nil {
		return err
	}
	if err := ensureIssueMissing(db, newID); err != nil {
		return err
	}
	// Apply the rename across issues and dependencies, then persist the mapping.
	if err := updateIssueID(db, resolvedOldID, newID, event.Timestamp); err != nil {
		return err
	}
	if err := updateDepsForRename(db, resolvedOldID, newID); err != nil {
		return err
	}
	if err := upsertRename(db, resolvedOldID, newID); err != nil {
		return err
	}
	return nil
}

// applyStatus updates an issue status from a status update event.
func applyStatus(db *sql.DB, event Event) error {
	status := event.Payload["status"]
	if status == "" {
		return fmt.Errorf("status event missing status")
	}
	var result sql.Result
	var err error
	if status == StatusClosed {
		// Apply the status update and touch updated_at.
		result, err = db.Exec(
			"UPDATE issues SET status = ?, updated_at = ? WHERE id = ?",
			status,
			event.Timestamp,
			event.IssueID,
		)
	} else {
		// Reopening clears closed_at while updating status/updated_at.
		result, err = db.Exec(
			"UPDATE issues SET status = ?, updated_at = ?, closed_at = ? WHERE id = ?",
			status,
			event.Timestamp,
			"",
			event.IssueID,
		)
	}
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return requireRow(result, "status update for missing issue")
}

// applyUpdate updates issue fields from an update event.
func applyUpdate(db *sql.DB, event Event) error {
	var updates []string
	var args []any
	if issueType, ok := event.Payload["type"]; ok {
		updates = append(updates, "issue_type = ?")
		args = append(args, issueType)
	}
	if description, ok := event.Payload["description"]; ok {
		updates = append(updates, "description = ?")
		args = append(args, description)
	}
	if priority, ok := event.Payload["priority"]; ok {
		updates = append(updates, "priority = ?")
		args = append(args, parsePriority(priority))
	}
	if len(updates) == 0 {
		return fmt.Errorf("update event missing fields")
	}
	updates = append(updates, "updated_at = ?")
	args = append(args, event.Timestamp, event.IssueID)
	query := fmt.Sprintf("UPDATE issues SET %s WHERE id = ?", strings.Join(updates, ", "))
	result, err := db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update issue: %w", err)
	}
	return requireRow(result, "update for missing issue")
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

// applyComment validates comment events without mutating the cache.
func applyComment(db *sql.DB, event Event) error {
	body := strings.TrimSpace(event.Payload["body"])
	if body == "" {
		return fmt.Errorf("comment event missing body")
	}
	// Comments don't mutate issue rows, but they must target an existing issue.
	if err := ensureIssueExists(db, event.IssueID); err != nil {
		return err
	}
	return nil
}

// applyDepAdd inserts a dependency from a dep_add event.
func applyDepAdd(db *sql.DB, event Event) error {
	dependsOn := event.Payload["depends_on"]
	if dependsOn == "" {
		return fmt.Errorf("dep_add event missing depends_on")
	}
	depType := NormalizeDepType(event.Payload["dep_type"])
	// Validate both ends exist before writing the dependency.
	if err := ensureIssueExists(db, event.IssueID); err != nil {
		return err
	}
	if err := ensureIssueExists(db, dependsOn); err != nil {
		return err
	}
	// Insert a dependency edge, ignoring duplicates.
	_, err := db.Exec(
		"INSERT OR IGNORE INTO deps (issue_id, depends_on_id, dep_type) VALUES (?, ?, ?)",
		event.IssueID,
		dependsOn,
		depType,
	)
	if err != nil {
		return fmt.Errorf("insert dependency: %w", err)
	}
	return nil
}

// applyDepRemove removes a dependency from a dep_rm event.
func applyDepRemove(db *sql.DB, event Event) error {
	dependsOn := event.Payload["depends_on"]
	if dependsOn == "" {
		return fmt.Errorf("dep_rm event missing depends_on")
	}
	depType := NormalizeDepType(event.Payload["dep_type"])
	// Validate both issues exist before attempting removal.
	if err := ensureIssueExists(db, event.IssueID); err != nil {
		return err
	}
	if err := ensureIssueExists(db, dependsOn); err != nil {
		return err
	}
	// Delete the dependency edge if present.
	_, err := db.Exec(
		"DELETE FROM deps WHERE issue_id = ? AND depends_on_id = ? AND dep_type = ?",
		event.IssueID,
		dependsOn,
		depType,
	)
	if err != nil {
		return fmt.Errorf("delete dependency: %w", err)
	}
	return nil
}

// ensureIssueExists verifies a referenced issue exists.
func ensureIssueExists(db *sql.DB, issueID string) error {
	exists, err := issueExists(db, issueID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("missing issue: %s", issueID)
	}
	return nil
}

// updateIssueID swaps issue IDs and stamps updated_at.
func updateIssueID(db *sql.DB, oldID, newID, timestamp string) error {
	result, err := db.Exec(
		"UPDATE issues SET id = ?, updated_at = ? WHERE id = ?",
		newID,
		timestamp,
		oldID,
	)
	if err != nil {
		return fmt.Errorf("rename issue: %w", err)
	}
	return requireRow(result, "rename for missing issue")
}

// updateDepsForRename rewrites dependency edges for a renamed issue.
func updateDepsForRename(db *sql.DB, oldID, newID string) error {
	if _, err := db.Exec("UPDATE deps SET issue_id = ? WHERE issue_id = ?", newID, oldID); err != nil {
		return fmt.Errorf("rename dependency issue_id: %w", err)
	}
	if _, err := db.Exec("UPDATE deps SET depends_on_id = ? WHERE depends_on_id = ?", newID, oldID); err != nil {
		return fmt.Errorf("rename dependency depends_on_id: %w", err)
	}
	return nil
}

// upsertRename records an issue ID rename mapping.
func upsertRename(db *sql.DB, oldID, newID string) error {
	if _, err := db.Exec(
		"INSERT OR REPLACE INTO renames (old_id, new_id) VALUES (?, ?)",
		oldID,
		newID,
	); err != nil {
		return fmt.Errorf("insert rename: %w", err)
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
