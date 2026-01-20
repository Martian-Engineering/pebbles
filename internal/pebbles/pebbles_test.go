package pebbles

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCreateUpdateClose verifies core issue lifecycle behavior.
func TestCreateUpdateClose(t *testing.T) {
	root := t.TempDir()
	if err := InitProject(root); err != nil {
		t.Fatalf("init project: %v", err)
	}
	// Create a new issue and rebuild the cache.
	issueID := "pb-aaaa"
	if err := AppendEvent(root, NewCreateEvent(issueID, "First", "Desc", "task", "2024-01-01T00:00:00Z", 2)); err != nil {
		t.Fatalf("append create: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	issues, err := ListIssues(root)
	if err != nil {
		t.Fatalf("list issues: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Status != StatusOpen {
		t.Fatalf("expected status open, got %s", issues[0].Status)
	}
	if issues[0].Description != "Desc" {
		t.Fatalf("expected description to be persisted")
	}
	if issues[0].Priority != 2 {
		t.Fatalf("expected priority 2, got %d", issues[0].Priority)
	}
	var issue Issue
	// Update issue fields and verify they persist.
	updatePayload := map[string]string{
		"type":        "bug",
		"priority":    "1",
		"description": "New description",
	}
	if err := AppendEvent(root, NewUpdateEvent(issueID, "2024-01-01T00:30:00Z", updatePayload)); err != nil {
		t.Fatalf("append update: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache after update: %v", err)
	}
	issue, _, err = GetIssue(root, issueID)
	if err != nil {
		t.Fatalf("get issue after update: %v", err)
	}
	if issue.IssueType != "bug" {
		t.Fatalf("expected type bug, got %s", issue.IssueType)
	}
	if issue.Priority != 1 {
		t.Fatalf("expected priority 1, got %d", issue.Priority)
	}
	if issue.Description != "New description" {
		t.Fatalf("expected updated description")
	}
	// Update status and verify.
	if err := AppendEvent(root, NewStatusEvent(issueID, "in_progress", "2024-01-01T01:00:00Z")); err != nil {
		t.Fatalf("append status: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	issue, _, err = GetIssue(root, issueID)
	if err != nil {
		t.Fatalf("get issue: %v", err)
	}
	if issue.Status != "in_progress" {
		t.Fatalf("expected in_progress, got %s", issue.Status)
	}
	// Close the issue and verify closed fields.
	if err := AppendEvent(root, NewCloseEvent(issueID, "2024-01-01T02:00:00Z")); err != nil {
		t.Fatalf("append close: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	issue, _, err = GetIssue(root, issueID)
	if err != nil {
		t.Fatalf("get issue after close: %v", err)
	}
	if issue.Status != StatusClosed {
		t.Fatalf("expected closed, got %s", issue.Status)
	}
	if issue.ClosedAt == "" {
		t.Fatalf("expected closed_at to be set")
	}
}

// TestReopenClearsClosedAt ensures reopening clears the close timestamp.
func TestReopenClearsClosedAt(t *testing.T) {
	root := t.TempDir()
	if err := InitProject(root); err != nil {
		t.Fatalf("init project: %v", err)
	}
	issueID := "pb-reopen"
	// Create and close the issue before reopening.
	if err := AppendEvent(root, NewCreateEvent(issueID, "Reopen", "", "task", "2024-01-01T00:00:00Z", 2)); err != nil {
		t.Fatalf("append create: %v", err)
	}
	if err := AppendEvent(root, NewCloseEvent(issueID, "2024-01-01T01:00:00Z")); err != nil {
		t.Fatalf("append close: %v", err)
	}
	if err := AppendEvent(root, NewStatusEvent(issueID, StatusOpen, "2024-01-01T02:00:00Z")); err != nil {
		t.Fatalf("append reopen status: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	issue, _, err := GetIssue(root, issueID)
	if err != nil {
		t.Fatalf("get issue after reopen: %v", err)
	}
	if issue.Status != StatusOpen {
		t.Fatalf("expected status open, got %s", issue.Status)
	}
	if issue.ClosedAt != "" {
		t.Fatalf("expected closed_at to be cleared")
	}
}

// TestListIssueComments verifies comment events are collected by issue.
func TestListIssueComments(t *testing.T) {
	root := t.TempDir()
	if err := InitProject(root); err != nil {
		t.Fatalf("init project: %v", err)
	}
	issueID := "pb-comment"
	renamedID := "pb-comment-new"
	// Create the issue and append comment events around a rename.
	if err := AppendEvent(root, NewCreateEvent(issueID, "Commented", "", "task", "2024-01-07T00:00:00Z", 2)); err != nil {
		t.Fatalf("append create: %v", err)
	}
	if err := AppendEvent(root, NewCommentEvent(issueID, "First note", "2024-01-07T00:00:01Z")); err != nil {
		t.Fatalf("append comment 1: %v", err)
	}
	if err := AppendEvent(root, NewRenameEvent(issueID, renamedID, "2024-01-07T00:00:02Z")); err != nil {
		t.Fatalf("append rename: %v", err)
	}
	if err := AppendEvent(root, NewCommentEvent(issueID, "Second note", "2024-01-07T00:00:03Z")); err != nil {
		t.Fatalf("append comment 2: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	comments, err := ListIssueComments(root, issueID)
	if err != nil {
		t.Fatalf("list comments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	if comments[0].IssueID != renamedID || comments[1].IssueID != renamedID {
		t.Fatalf("expected comments to resolve to %s", renamedID)
	}
	if comments[0].Body != "First note" {
		t.Fatalf("expected first comment body, got %q", comments[0].Body)
	}
	if comments[1].Body != "Second note" {
		t.Fatalf("expected second comment body, got %q", comments[1].Body)
	}
}

// TestRenameEvent verifies rename events update IDs and resolve aliases.
func TestRenameEvent(t *testing.T) {
	root := t.TempDir()
	if err := InitProject(root); err != nil {
		t.Fatalf("init project: %v", err)
	}
	oldID := "pb-old"
	newID := "pb-new"
	if err := AppendEvent(root, NewCreateEvent(oldID, "First", "", "task", "2024-01-04T00:00:00Z", 2)); err != nil {
		t.Fatalf("append create: %v", err)
	}
	if err := AppendEvent(root, NewRenameEvent(oldID, newID, "2024-01-04T01:00:00Z")); err != nil {
		t.Fatalf("append rename: %v", err)
	}
	if err := AppendEvent(root, NewStatusEvent(oldID, StatusInProgress, "2024-01-04T02:00:00Z")); err != nil {
		t.Fatalf("append status: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	issue, _, err := GetIssue(root, oldID)
	if err != nil {
		t.Fatalf("get issue by old id: %v", err)
	}
	if issue.ID != newID {
		t.Fatalf("expected renamed id %s, got %s", newID, issue.ID)
	}
	if issue.Status != StatusInProgress {
		t.Fatalf("expected status %s, got %s", StatusInProgress, issue.Status)
	}
}

// TestRenameUpdatesDeps ensures dependency rows are updated on rename.
func TestRenameUpdatesDeps(t *testing.T) {
	root := t.TempDir()
	if err := InitProject(root); err != nil {
		t.Fatalf("init project: %v", err)
	}
	issueA := "pb-dep-a"
	issueB := "pb-dep-b"
	renamedB := "pb-dep-new"
	if err := AppendEvent(root, NewCreateEvent(issueA, "Issue A", "", "task", "2024-01-05T00:00:00Z", 2)); err != nil {
		t.Fatalf("append create A: %v", err)
	}
	if err := AppendEvent(root, NewCreateEvent(issueB, "Issue B", "", "task", "2024-01-05T00:00:01Z", 2)); err != nil {
		t.Fatalf("append create B: %v", err)
	}
	if err := AppendEvent(root, NewDepAddEvent(issueA, issueB, DepTypeBlocks, "2024-01-05T00:00:02Z")); err != nil {
		t.Fatalf("append dep: %v", err)
	}
	if err := AppendEvent(root, NewRenameEvent(issueB, renamedB, "2024-01-05T00:00:03Z")); err != nil {
		t.Fatalf("append rename: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	_, deps, err := GetIssue(root, issueA)
	if err != nil {
		t.Fatalf("get issue: %v", err)
	}
	if len(deps) != 1 || deps[0] != renamedB {
		t.Fatalf("expected dependency %s", renamedB)
	}
}

// TestReadyList verifies dependency-based ready filtering.
func TestReadyList(t *testing.T) {
	root := t.TempDir()
	if err := InitProject(root); err != nil {
		t.Fatalf("init project: %v", err)
	}
	// Create issues with blocking and parent-child dependencies.
	issueA := "pb-issue-a"
	issueB := "pb-issue-b"
	issueC := "pb-issue-c"
	if err := AppendEvent(root, NewCreateEvent(issueA, "Issue A", "", "task", "2024-01-02T00:00:00Z", 2)); err != nil {
		t.Fatalf("append create A: %v", err)
	}
	if err := AppendEvent(root, NewCreateEvent(issueB, "Issue B", "", "task", "2024-01-02T00:00:01Z", 2)); err != nil {
		t.Fatalf("append create B: %v", err)
	}
	if err := AppendEvent(root, NewCreateEvent(issueC, "Issue C", "", "task", "2024-01-02T00:00:02Z", 2)); err != nil {
		t.Fatalf("append create C: %v", err)
	}
	if err := AppendEvent(root, NewDepAddEvent(issueA, issueB, DepTypeBlocks, "2024-01-02T00:00:03Z")); err != nil {
		t.Fatalf("append blocking dep: %v", err)
	}
	if err := AppendEvent(root, NewDepAddEvent(issueC, issueB, DepTypeParentChild, "2024-01-02T00:00:04Z")); err != nil {
		t.Fatalf("append parent-child dep: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	ready, err := ListReadyIssues(root)
	if err != nil {
		t.Fatalf("list ready: %v", err)
	}
	if len(ready) != 2 || ready[0].ID != issueB || ready[1].ID != issueC {
		t.Fatalf("expected %s and %s ready", issueB, issueC)
	}
	// Close the blocker and confirm the dependent becomes ready.
	if err := AppendEvent(root, NewCloseEvent(issueB, "2024-01-02T00:00:05Z")); err != nil {
		t.Fatalf("append close: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	ready, err = ListReadyIssues(root)
	if err != nil {
		t.Fatalf("list ready after close: %v", err)
	}
	if len(ready) != 2 || ready[0].ID != issueA || ready[1].ID != issueC {
		t.Fatalf("expected %s and %s ready", issueA, issueC)
	}
}

// TestDependencyTree verifies dependency tree construction.
func TestDependencyTree(t *testing.T) {
	root := t.TempDir()
	if err := InitProject(root); err != nil {
		t.Fatalf("init project: %v", err)
	}
	issueA := "pb-tree-a"
	issueB := "pb-tree-b"
	issueC := "pb-tree-c"
	if err := AppendEvent(root, NewCreateEvent(issueA, "Issue A", "", "task", "2024-01-03T00:00:00Z", 2)); err != nil {
		t.Fatalf("append create A: %v", err)
	}
	if err := AppendEvent(root, NewCreateEvent(issueB, "Issue B", "", "task", "2024-01-03T00:00:01Z", 2)); err != nil {
		t.Fatalf("append create B: %v", err)
	}
	if err := AppendEvent(root, NewCreateEvent(issueC, "Issue C", "", "task", "2024-01-03T00:00:02Z", 2)); err != nil {
		t.Fatalf("append create C: %v", err)
	}
	if err := AppendEvent(root, NewDepAddEvent(issueA, issueB, DepTypeBlocks, "2024-01-03T00:00:03Z")); err != nil {
		t.Fatalf("append dep A->B: %v", err)
	}
	if err := AppendEvent(root, NewDepAddEvent(issueB, issueC, DepTypeBlocks, "2024-01-03T00:00:04Z")); err != nil {
		t.Fatalf("append dep B->C: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	tree, err := DependencyTree(root, issueA)
	if err != nil {
		t.Fatalf("dependency tree: %v", err)
	}
	if tree.Issue.ID != issueA {
		t.Fatalf("expected root %s, got %s", issueA, tree.Issue.ID)
	}
	if len(tree.Dependencies) != 1 || tree.Dependencies[0].Issue.ID != issueB {
		t.Fatalf("expected child %s", issueB)
	}
	if len(tree.Dependencies[0].Dependencies) != 1 || tree.Dependencies[0].Dependencies[0].Issue.ID != issueC {
		t.Fatalf("expected grandchild %s", issueC)
	}
}

// TestListIssueHierarchy verifies parent-child indentation ordering.
func TestListIssueHierarchy(t *testing.T) {
	root := t.TempDir()
	if err := InitProject(root); err != nil {
		t.Fatalf("init project: %v", err)
	}
	issueParent := "pb-parent"
	issueChildA := "pb-child-a"
	issueChildB := "pb-child-b"
	issueGrand := "pb-grandchild"
	issueRoot := "pb-root"
	// Create issues with staggered timestamps for stable ordering.
	if err := AppendEvent(root, NewCreateEvent(issueParent, "Parent", "", "task", "2024-01-06T00:00:00Z", 2)); err != nil {
		t.Fatalf("append parent: %v", err)
	}
	if err := AppendEvent(root, NewCreateEvent(issueChildA, "Child A", "", "task", "2024-01-06T00:00:01Z", 2)); err != nil {
		t.Fatalf("append child A: %v", err)
	}
	if err := AppendEvent(root, NewCreateEvent(issueChildB, "Child B", "", "task", "2024-01-06T00:00:02Z", 2)); err != nil {
		t.Fatalf("append child B: %v", err)
	}
	if err := AppendEvent(root, NewCreateEvent(issueGrand, "Grandchild", "", "task", "2024-01-06T00:00:03Z", 2)); err != nil {
		t.Fatalf("append grandchild: %v", err)
	}
	if err := AppendEvent(root, NewCreateEvent(issueRoot, "Root", "", "task", "2024-01-06T00:00:04Z", 2)); err != nil {
		t.Fatalf("append root: %v", err)
	}
	// Connect the parent-child hierarchy.
	if err := AppendEvent(root, NewDepAddEvent(issueChildA, issueParent, DepTypeParentChild, "2024-01-06T00:00:05Z")); err != nil {
		t.Fatalf("append child A parent: %v", err)
	}
	if err := AppendEvent(root, NewDepAddEvent(issueChildB, issueParent, DepTypeParentChild, "2024-01-06T00:00:06Z")); err != nil {
		t.Fatalf("append child B parent: %v", err)
	}
	if err := AppendEvent(root, NewDepAddEvent(issueGrand, issueChildA, DepTypeParentChild, "2024-01-06T00:00:07Z")); err != nil {
		t.Fatalf("append grandchild parent: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	items, err := ListIssueHierarchy(root)
	if err != nil {
		t.Fatalf("list issue hierarchy: %v", err)
	}
	wantIDs := []string{issueParent, issueChildA, issueGrand, issueChildB, issueRoot}
	wantDepths := []int{0, 1, 2, 1, 0}
	if len(items) != len(wantIDs) {
		t.Fatalf("expected %d items, got %d", len(wantIDs), len(items))
	}
	for i, item := range items {
		if item.Issue.ID != wantIDs[i] {
			t.Fatalf("expected %s at %d, got %s", wantIDs[i], i, item.Issue.ID)
		}
		if item.Depth != wantDepths[i] {
			t.Fatalf("expected depth %d at %d, got %d", wantDepths[i], i, item.Depth)
		}
	}
}

// TestInitCreatesFiles ensures init creates core files.
func TestInitCreatesFiles(t *testing.T) {
	root := t.TempDir()
	if err := InitProject(root); err != nil {
		t.Fatalf("init project: %v", err)
	}
	// Verify config and event log are created.
	paths := []string{
		ConfigPath(root),
		EventsPath(root),
		DBPath(root),
		filepath.Join(PebblesDir(root), ".gitignore"),
	}
	for _, path := range paths {
		if _, err := fileExists(path); err != nil {
			t.Fatalf("expected file %s: %v", filepath.Base(path), err)
		}
	}
}

// fileExists checks that a file exists on disk.
func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
}
