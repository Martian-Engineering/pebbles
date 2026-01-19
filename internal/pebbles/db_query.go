package pebbles

import (
	"database/sql"
	"fmt"
)

// ListIssues returns all issues ordered by ID.
func ListIssues(root string) ([]Issue, error) {
	if err := EnsureCache(root); err != nil {
		return nil, err
	}
	db, err := openDB(DBPath(root))
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	// Query all issues in a stable order for output.
	rows, err := db.Query(
		"SELECT id, title, description, issue_type, status, priority, created_at, updated_at, closed_at FROM issues ORDER BY id",
	)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var issues []Issue
	// Scan rows into Issue structs.
	for rows.Next() {
		issue, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list issues rows: %w", err)
	}
	return issues, nil
}

// GetIssue returns a single issue and its dependencies.
func GetIssue(root, id string) (Issue, []string, error) {
	if err := EnsureCache(root); err != nil {
		return Issue{}, nil, err
	}
	db, err := openDB(DBPath(root))
	if err != nil {
		return Issue{}, nil, err
	}
	defer func() { _ = db.Close() }()
	resolvedID, err := resolveIssueID(db, id)
	if err != nil {
		return Issue{}, nil, err
	}
	// Fetch the issue row by ID.
	row := db.QueryRow(
		"SELECT id, title, description, issue_type, status, priority, created_at, updated_at, closed_at FROM issues WHERE id = ?",
		resolvedID,
	)
	issue, err := scanIssue(row)
	if err != nil {
		return Issue{}, nil, err
	}
	// Fetch dependencies for the issue.
	deps, err := getDeps(db, resolvedID)
	if err != nil {
		return Issue{}, nil, err
	}
	return issue, deps, nil
}

// ListReadyIssues returns issues that have no open blockers.
func ListReadyIssues(root string) ([]Issue, error) {
	if err := EnsureCache(root); err != nil {
		return nil, err
	}
	db, err := openDB(DBPath(root))
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	// Select issues that are not closed and have no deps on open issues.
	query := `
		SELECT i.id, i.title, i.description, i.issue_type, i.status, i.priority, i.created_at, i.updated_at, i.closed_at
		FROM issues i
		WHERE i.status != ?
		AND NOT EXISTS (
			SELECT 1 FROM deps d
			JOIN issues di ON di.id = d.depends_on_id
			WHERE d.issue_id = i.id AND di.status != ?
		)
		ORDER BY i.id
	`
	rows, err := db.Query(query, StatusClosed, StatusClosed)
	if err != nil {
		return nil, fmt.Errorf("ready issues: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var issues []Issue
	// Scan candidate issues into memory.
	for rows.Next() {
		issue, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ready issues rows: %w", err)
	}
	return issues, nil
}

// IssueExists reports whether an issue exists for the given ID or alias.
func IssueExists(root, id string) (bool, error) {
	if err := EnsureCache(root); err != nil {
		return false, err
	}
	db, err := openDB(DBPath(root))
	if err != nil {
		return false, err
	}
	defer func() { _ = db.Close() }()
	resolvedID, err := resolveIssueID(db, id)
	if err != nil {
		return false, err
	}
	return issueExists(db, resolvedID)
}

// scanIssue scans a single issue row from a row scanner.
func scanIssue(scanner interface{ Scan(...any) error }) (Issue, error) {
	var issue Issue
	// Map columns into the Issue struct in order.
	if err := scanner.Scan(
		&issue.ID,
		&issue.Title,
		&issue.Description,
		&issue.IssueType,
		&issue.Status,
		&issue.Priority,
		&issue.CreatedAt,
		&issue.UpdatedAt,
		&issue.ClosedAt,
	); err != nil {
		return Issue{}, fmt.Errorf("scan issue: %w", err)
	}
	return issue, nil
}

// getIssueByID fetches an issue by ID using the provided DB connection.
func getIssueByID(db *sql.DB, id string) (Issue, error) {
	// Query by ID for dependency tree and status helpers.
	row := db.QueryRow(
		"SELECT id, title, description, issue_type, status, priority, created_at, updated_at, closed_at FROM issues WHERE id = ?",
		id,
	)
	issue, err := scanIssue(row)
	if err != nil {
		return Issue{}, fmt.Errorf("get issue: %w", err)
	}
	return issue, nil
}

// getDeps fetches dependency IDs for an issue.
func getDeps(db *sql.DB, id string) ([]string, error) {
	rows, err := db.Query("SELECT depends_on_id FROM deps WHERE issue_id = ? ORDER BY depends_on_id", id)
	if err != nil {
		return nil, fmt.Errorf("get deps: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var deps []string
	// Collect dependency IDs for the issue.
	for rows.Next() {
		var dep string
		if err := rows.Scan(&dep); err != nil {
			return nil, fmt.Errorf("scan dep: %w", err)
		}
		deps = append(deps, dep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("deps rows: %w", err)
	}
	return deps, nil
}
