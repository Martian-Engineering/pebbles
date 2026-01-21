package pebbles

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
)

// IssueHierarchy describes parent-child relationships for an issue.
type IssueHierarchy struct {
	Parents  []Issue
	Children []Issue
	Siblings []Issue
}

// GetIssueHierarchy returns parent, child, and sibling issues for the provided ID.
func GetIssueHierarchy(root, id string) (IssueHierarchy, error) {
	if err := EnsureCache(root); err != nil {
		return IssueHierarchy{}, err
	}
	db, err := openDB(DBPath(root))
	if err != nil {
		return IssueHierarchy{}, err
	}
	defer func() { _ = db.Close() }()
	resolvedID, err := resolveIssueID(db, id)
	if err != nil {
		return IssueHierarchy{}, err
	}
	// Load parent and child relationships from the dependency table.
	parentIDs, err := getDeps(db, resolvedID, DepTypeParentChild)
	if err != nil {
		return IssueHierarchy{}, err
	}
	childIDs, err := getDependents(db, resolvedID, DepTypeParentChild)
	if err != nil {
		return IssueHierarchy{}, err
	}
	siblingIDs, err := collectSiblingIDs(db, resolvedID, parentIDs)
	if err != nil {
		return IssueHierarchy{}, err
	}
	// Hydrate IDs into issues for display-ready output.
	parents, err := loadIssuesByID(db, parentIDs)
	if err != nil {
		return IssueHierarchy{}, err
	}
	children, err := loadIssuesByID(db, childIDs)
	if err != nil {
		return IssueHierarchy{}, err
	}
	siblings, err := loadIssuesByID(db, siblingIDs)
	if err != nil {
		return IssueHierarchy{}, err
	}
	return IssueHierarchy{
		Parents:  parents,
		Children: children,
		Siblings: siblings,
	}, nil
}

// HasParentChildRelations reports whether an issue participates in any parent-child links.
func HasParentChildRelations(root, id string) (bool, error) {
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
	// Check both child and parent positions for a parent-child edge.
	row := db.QueryRow(
		"SELECT 1 FROM deps WHERE dep_type = ? AND (issue_id = ? OR depends_on_id = ?) LIMIT 1",
		DepTypeParentChild,
		resolvedID,
		resolvedID,
	)
	var found int
	if err := row.Scan(&found); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check parent-child relations: %w", err)
	}
	return true, nil
}

// ParentChildTree returns a dependency tree rooted at the top parent of the issue.
func ParentChildTree(root, id string) (DepNode, error) {
	if err := EnsureCache(root); err != nil {
		return DepNode{}, err
	}
	db, err := openDB(DBPath(root))
	if err != nil {
		return DepNode{}, err
	}
	defer func() { _ = db.Close() }()
	resolvedID, err := resolveIssueID(db, id)
	if err != nil {
		return DepNode{}, err
	}
	rootID, err := resolveParentRoot(db, resolvedID)
	if err != nil {
		return DepNode{}, err
	}
	// Build the full parent-child tree while guarding against cycles.
	visited := make(map[string]bool)
	return buildParentChildTree(db, rootID, visited)
}

// collectSiblingIDs gathers sibling IDs for a child issue across all parents.
func collectSiblingIDs(db *sql.DB, issueID string, parentIDs []string) ([]string, error) {
	if len(parentIDs) == 0 {
		return []string{}, nil
	}
	siblingSet := make(map[string]bool)
	// For each parent, collect child issues except the current issue.
	for _, parentID := range parentIDs {
		childIDs, err := getDependents(db, parentID, DepTypeParentChild)
		if err != nil {
			return nil, err
		}
		for _, childID := range childIDs {
			if childID == issueID {
				continue
			}
			siblingSet[childID] = true
		}
	}
	siblings := make([]string, 0, len(siblingSet))
	for id := range siblingSet {
		siblings = append(siblings, id)
	}
	sort.Strings(siblings)
	return siblings, nil
}

// loadIssuesByID returns issues in the same order as the provided IDs.
func loadIssuesByID(db *sql.DB, ids []string) ([]Issue, error) {
	if len(ids) == 0 {
		return []Issue{}, nil
	}
	issues := make([]Issue, 0, len(ids))
	// Fetch each issue individually to preserve input ordering.
	for _, id := range ids {
		issue, err := getIssueByID(db, id)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

// resolveParentRoot walks up parent-child links to find the topmost ancestor.
func resolveParentRoot(db *sql.DB, issueID string) (string, error) {
	current := issueID
	visited := make(map[string]bool)
	// Follow the first parent link until there are no more parents.
	for {
		if visited[current] {
			return current, nil
		}
		visited[current] = true
		parentIDs, err := getDeps(db, current, DepTypeParentChild)
		if err != nil {
			return "", err
		}
		if len(parentIDs) == 0 {
			return current, nil
		}
		current = parentIDs[0]
	}
}

// buildParentChildTree constructs the parent-child dependency tree recursively.
func buildParentChildTree(db *sql.DB, issueID string, visited map[string]bool) (DepNode, error) {
	issue, err := getIssueByID(db, issueID)
	if err != nil {
		return DepNode{}, err
	}
	node := DepNode{Issue: issue}
	// Stop recursion when the node was already visited.
	if visited[issueID] {
		return node, nil
	}
	visited[issueID] = true
	childIDs, err := getDependents(db, issueID, DepTypeParentChild)
	if err != nil {
		return DepNode{}, err
	}
	for _, childID := range childIDs {
		child, err := buildParentChildTree(db, childID, visited)
		if err != nil {
			return DepNode{}, err
		}
		node.Dependencies = append(node.Dependencies, child)
	}
	return node, nil
}
