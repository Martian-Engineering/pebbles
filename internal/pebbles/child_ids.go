package pebbles

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// HasParentChildSuffix reports whether childID uses the parentID.<N> suffix format.
func HasParentChildSuffix(parentID, childID string) bool {
	prefix := parentID + "."
	if !strings.HasPrefix(childID, prefix) {
		return false
	}
	suffix := strings.TrimPrefix(childID, prefix)
	if suffix == "" {
		return false
	}
	_, err := strconv.Atoi(suffix)
	return err == nil
}

// NextChildIssueID returns the next available child ID for a parent issue.
func NextChildIssueID(root, parentID string) (string, error) {
	if err := EnsureCache(root); err != nil {
		return "", err
	}
	db, err := openDB(DBPath(root))
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close() }()
	// Resolve and validate the parent issue ID before scanning children.
	resolvedParent, err := resolveIssueID(db, parentID)
	if err != nil {
		return "", err
	}
	if err := ensureIssueExists(db, resolvedParent); err != nil {
		return "", err
	}
	usedSuffixes, err := loadChildSuffixes(db, resolvedParent)
	if err != nil {
		return "", err
	}
	// Pick the smallest available suffix that isn't already in use.
	for suffix := 1; ; suffix++ {
		if usedSuffixes[suffix] {
			continue
		}
		candidate := fmt.Sprintf("%s.%d", resolvedParent, suffix)
		available, err := issueIDAvailable(db, candidate)
		if err != nil {
			return "", err
		}
		if available {
			return candidate, nil
		}
		usedSuffixes[suffix] = true
	}
}

// loadChildSuffixes collects numeric suffixes already used by direct children.
func loadChildSuffixes(db *sql.DB, parentID string) (map[int]bool, error) {
	rows, err := db.Query(
		"SELECT issue_id FROM deps WHERE dep_type = ? AND depends_on_id = ? ORDER BY issue_id",
		DepTypeParentChild,
		parentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list child deps: %w", err)
	}
	defer func() { _ = rows.Close() }()
	used := make(map[int]bool)
	prefix := parentID + "."
	// Track suffixes already assigned to this parent's direct children.
	for rows.Next() {
		var childID string
		if err := rows.Scan(&childID); err != nil {
			return nil, fmt.Errorf("scan child dep: %w", err)
		}
		if !strings.HasPrefix(childID, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(childID, prefix)
		value, err := strconv.Atoi(suffix)
		if err != nil {
			continue
		}
		if value > 0 {
			used[value] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("child deps rows: %w", err)
	}
	return used, nil
}

// issueIDAvailable reports whether an ID is unused and not aliased by a rename.
func issueIDAvailable(db *sql.DB, id string) (bool, error) {
	resolved, err := resolveIssueID(db, id)
	if err != nil {
		return false, err
	}
	if resolved != id {
		return false, nil
	}
	exists, err := issueExists(db, id)
	if err != nil {
		return false, err
	}
	return !exists, nil
}
