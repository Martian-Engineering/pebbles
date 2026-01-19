package pebbles

import (
	"os"
	"testing"
)

// TestLoadEventLog verifies log entries include line numbers.
func TestLoadEventLog(t *testing.T) {
	root := t.TempDir()
	if err := InitProject(root); err != nil {
		t.Fatalf("init project: %v", err)
	}
	issueID := "pb-log-1"
	if err := AppendEvent(root, NewCreateEvent(issueID, "Log One", "", "task", "2024-02-01T00:00:00Z", 2)); err != nil {
		t.Fatalf("append create: %v", err)
	}
	if err := AppendEvent(root, NewStatusEvent(issueID, "in_progress", "2024-02-01T00:01:00Z")); err != nil {
		t.Fatalf("append status: %v", err)
	}
	entries, err := LoadEventLog(root)
	if err != nil {
		t.Fatalf("load event log: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Line != 1 || entries[1].Line != 2 {
		t.Fatalf("expected line numbers 1 and 2, got %d and %d", entries[0].Line, entries[1].Line)
	}
}

// TestLoadEventLogSkipsBlankLines ensures blank lines are ignored.
func TestLoadEventLogSkipsBlankLines(t *testing.T) {
	root := t.TempDir()
	if err := InitProject(root); err != nil {
		t.Fatalf("init project: %v", err)
	}
	issueID := "pb-log-2"
	if err := AppendEvent(root, NewCreateEvent(issueID, "Log Two", "", "task", "2024-02-02T12:00:00Z", 2)); err != nil {
		t.Fatalf("append create: %v", err)
	}
	path := EventsPath(root)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("open events log: %v", err)
	}
	if _, err := file.WriteString("\n\n"); err != nil {
		_ = file.Close()
		t.Fatalf("append blank lines: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close events log: %v", err)
	}
	if err := AppendEvent(root, NewCloseEvent(issueID, "2024-02-02T13:00:00Z")); err != nil {
		t.Fatalf("append close: %v", err)
	}
	entries, err := LoadEventLog(root)
	if err != nil {
		t.Fatalf("load event log: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}
