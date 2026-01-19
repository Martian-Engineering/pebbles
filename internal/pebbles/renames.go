package pebbles

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// resolveIssueID follows rename mappings to return the current issue ID.
func resolveIssueID(db *sql.DB, id string) (string, error) {
	current := strings.TrimSpace(id)
	if current == "" {
		return "", fmt.Errorf("issue id is required")
	}
	// Walk rename edges until the current ID is stable.
	visited := make(map[string]bool)
	for {
		if visited[current] {
			return "", fmt.Errorf("rename cycle detected for %s", id)
		}
		visited[current] = true
		// Follow any rename mapping for the current ID.
		next, err := lookupRename(db, current)
		if err != nil {
			return "", err
		}
		if next == "" {
			return current, nil
		}
		current = next
	}
}

// lookupRename fetches a rename mapping for an issue ID.
func lookupRename(db *sql.DB, id string) (string, error) {
	row := db.QueryRow("SELECT new_id FROM renames WHERE old_id = ?", id)
	var next string
	if err := row.Scan(&next); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("lookup rename: %w", err)
	}
	return next, nil
}

// issueExists reports whether an issue exists for the given ID.
func issueExists(db *sql.DB, id string) (bool, error) {
	var count int
	row := db.QueryRow("SELECT COUNT(1) FROM issues WHERE id = ?", id)
	if err := row.Scan(&count); err != nil {
		return false, fmt.Errorf("check issue exists: %w", err)
	}
	return count > 0, nil
}

// ensureIssueMissing asserts that no issue exists with the given ID.
func ensureIssueMissing(db *sql.DB, id string) error {
	exists, err := issueExists(db, id)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("issue already exists: %s", id)
	}
	return nil
}
