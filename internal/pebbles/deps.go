package pebbles

import (
	"database/sql"
	"fmt"
)

// DepNode represents an issue with its dependency subtree.
type DepNode struct {
	Issue        Issue
	Dependencies []DepNode
}

// DependencyTree returns a dependency tree rooted at the provided issue ID.
func DependencyTree(root, id string) (DepNode, error) {
	if err := EnsureCache(root); err != nil {
		return DepNode{}, err
	}
	db, err := openDB(DBPath(root))
	if err != nil {
		return DepNode{}, err
	}
	defer func() { _ = db.Close() }()
	// Track visited nodes to avoid infinite loops on cycles.
	visited := make(map[string]bool)
	resolvedID, err := resolveIssueID(db, id)
	if err != nil {
		return DepNode{}, err
	}
	return buildDepTree(db, resolvedID, visited)
}

// IssueStatus returns the status for the given issue ID.
func IssueStatus(root, id string) (string, error) {
	if err := EnsureCache(root); err != nil {
		return "", err
	}
	db, err := openDB(DBPath(root))
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close() }()
	resolvedID, err := resolveIssueID(db, id)
	if err != nil {
		return "", err
	}
	var status string
	row := db.QueryRow("SELECT status FROM issues WHERE id = ?", resolvedID)
	if err := row.Scan(&status); err != nil {
		return "", fmt.Errorf("get issue status: %w", err)
	}
	return status, nil
}

// buildDepTree recursively builds dependency nodes while avoiding cycles.
func buildDepTree(db *sql.DB, id string, visited map[string]bool) (DepNode, error) {
	// Load the issue first so the node always has data.
	issue, err := getIssueByID(db, id)
	if err != nil {
		return DepNode{}, err
	}
	node := DepNode{Issue: issue}
	// Stop recursion when the node was already visited.
	if visited[id] {
		return node, nil
	}
	visited[id] = true
	// Recursively append child dependencies.
	deps, err := getDeps(db, id, DepTypeBlocks)
	if err != nil {
		return DepNode{}, err
	}
	for _, dep := range deps {
		child, err := buildDepTree(db, dep, visited)
		if err != nil {
			return DepNode{}, err
		}
		node.Dependencies = append(node.Dependencies, child)
	}
	return node, nil
}
