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
	if err := AppendEvent(root, NewCreateEvent(issueID, "First", "task", "2024-01-01T00:00:00Z")); err != nil {
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
	// Update status and verify.
	if err := AppendEvent(root, NewStatusEvent(issueID, "in_progress", "2024-01-01T01:00:00Z")); err != nil {
		t.Fatalf("append status: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	issue, _, err := GetIssue(root, issueID)
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

// TestReadyList verifies dependency-based ready filtering.
func TestReadyList(t *testing.T) {
	root := t.TempDir()
	if err := InitProject(root); err != nil {
		t.Fatalf("init project: %v", err)
	}
	// Create two issues with a dependency.
	issueA := "pb-issue-a"
	issueB := "pb-issue-b"
	if err := AppendEvent(root, NewCreateEvent(issueA, "Issue A", "task", "2024-01-02T00:00:00Z")); err != nil {
		t.Fatalf("append create A: %v", err)
	}
	if err := AppendEvent(root, NewCreateEvent(issueB, "Issue B", "task", "2024-01-02T00:00:01Z")); err != nil {
		t.Fatalf("append create B: %v", err)
	}
	if err := AppendEvent(root, NewDepAddEvent(issueA, issueB, "2024-01-02T00:00:02Z")); err != nil {
		t.Fatalf("append dep: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	ready, err := ListReadyIssues(root)
	if err != nil {
		t.Fatalf("list ready: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != issueB {
		t.Fatalf("expected only %s ready", issueB)
	}
	// Close the blocker and confirm the dependent becomes ready.
	if err := AppendEvent(root, NewCloseEvent(issueB, "2024-01-02T00:00:03Z")); err != nil {
		t.Fatalf("append close: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	ready, err = ListReadyIssues(root)
	if err != nil {
		t.Fatalf("list ready after close: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != issueA {
		t.Fatalf("expected only %s ready", issueA)
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
