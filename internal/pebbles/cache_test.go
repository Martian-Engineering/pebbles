package pebbles

import "testing"

func TestSortEventsOrdersRenameBeforeDeps(t *testing.T) {
	timestamp := "2026-01-19T00:00:00Z"
	events := []Event{
		{Type: EventTypeStatus, Timestamp: timestamp},
		{Type: EventTypeDepAdd, Timestamp: timestamp},
		{Type: EventTypeRename, Timestamp: timestamp},
		{Type: EventTypeCreate, Timestamp: timestamp},
		{Type: EventTypeUpdate, Timestamp: timestamp},
		{Type: EventTypeClose, Timestamp: timestamp},
	}
	sortEvents(events)
	expected := []string{
		EventTypeCreate,
		EventTypeRename,
		EventTypeDepAdd,
		EventTypeStatus,
		EventTypeUpdate,
		EventTypeClose,
	}
	for i, eventType := range expected {
		if events[i].Type != eventType {
			t.Fatalf("expected %s at index %d, got %s", eventType, i, events[i].Type)
		}
	}
}

func TestRebuildCacheIgnoresDuplicateCreateEvents(t *testing.T) {
	root := t.TempDir()
	if err := InitProject(root); err != nil {
		t.Fatalf("init project: %v", err)
	}
	issueID := "pb-dupe"
	if err := AppendEvent(root, NewCreateEvent(issueID, "First", "", "task", "2024-01-01T00:00:00Z", 2)); err != nil {
		t.Fatalf("append create: %v", err)
	}
	if err := AppendEvent(root, NewCreateEvent(issueID, "First", "", "task", "2024-01-01T00:00:01Z", 2)); err != nil {
		t.Fatalf("append duplicate create: %v", err)
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
}
