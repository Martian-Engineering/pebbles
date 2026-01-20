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
