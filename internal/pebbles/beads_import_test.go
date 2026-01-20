package pebbles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPlanBeadsImportSkipsTombstonesByDefault(t *testing.T) {
	sourceRoot := t.TempDir()
	// Seed one open issue and one tombstone.
	issues := []beadsIssue{
		{
			ID:        "zz-1a",
			Title:     "Open issue",
			Status:    "open",
			Priority:  intPtr(2),
			CreatedAt: "2024-01-01T00:00:00Z",
		},
		{
			ID:        "zz-2b",
			Title:     "Deleted issue",
			Status:    "tombstone",
			Priority:  intPtr(2),
			DeletedAt: "2024-01-02T00:00:00Z",
		},
	}
	writeBeadsIssues(t, sourceRoot, issues)
	// Build the plan without including tombstones.
	now := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	plan, err := PlanBeadsImport(BeadsImportOptions{SourceRoot: sourceRoot, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("plan beads import: %v", err)
	}
	// Verify the tombstone is excluded from the plan.
	if plan.Result.TombstonesSkipped != 1 {
		t.Fatalf("expected 1 tombstone skipped, got %d", plan.Result.TombstonesSkipped)
	}
	if plan.Result.EventsPlanned != 1 {
		t.Fatalf("expected 1 event planned, got %d", plan.Result.EventsPlanned)
	}
	// Apply the plan and ensure only the open issue is imported.
	targetRoot := t.TempDir()
	if err := InitProjectWithPrefix(targetRoot, plan.Result.Prefix); err != nil {
		t.Fatalf("init project: %v", err)
	}
	result, err := ApplyBeadsImportPlan(targetRoot, plan)
	if err != nil {
		t.Fatalf("apply plan: %v", err)
	}
	events, err := LoadEvents(targetRoot)
	if err != nil {
		t.Fatalf("load events: %v", err)
	}
	if len(events) != 1 || result.EventsWritten != 1 {
		t.Fatalf("expected 1 event written, got %d", result.EventsWritten)
	}
}

func TestPlanBeadsImportIncludesTombstones(t *testing.T) {
	sourceRoot := t.TempDir()
	// Seed issues including a tombstone to include in the import.
	issues := []beadsIssue{
		{
			ID:        "zz-1a",
			Title:     "Open issue",
			Status:    "open",
			Priority:  intPtr(2),
			CreatedAt: "2024-01-01T00:00:00Z",
		},
		{
			ID:        "zz-2b",
			Title:     "Deleted issue",
			Status:    "tombstone",
			Priority:  intPtr(2),
			DeletedAt: "2024-01-02T00:00:00Z",
		},
	}
	writeBeadsIssues(t, sourceRoot, issues)
	// Include tombstones to ensure a close event is emitted.
	now := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	plan, err := PlanBeadsImport(BeadsImportOptions{
		SourceRoot:        sourceRoot,
		IncludeTombstones: true,
		Now:               func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("plan beads import: %v", err)
	}
	// Ensure the tombstone is included in the plan and closed.
	if plan.Result.IssuesImported != 2 {
		t.Fatalf("expected 2 issues imported, got %d", plan.Result.IssuesImported)
	}
	closeEvent, ok := findEvent(plan.Events, EventTypeClose, "zz-2b")
	if !ok {
		t.Fatalf("expected close event for tombstone")
	}
	if closeEvent.Timestamp != "2024-01-02T00:00:00Z" {
		t.Fatalf("expected close timestamp to match deleted_at")
	}
}

func TestPlanBeadsImportParentChildDependency(t *testing.T) {
	sourceRoot := t.TempDir()
	// Add a parent issue and a child dependency edge.
	issues := []beadsIssue{
		{
			ID:        "zz-parent",
			Title:     "Parent",
			Status:    "open",
			Priority:  intPtr(2),
			CreatedAt: "2024-01-01T00:00:00Z",
		},
		{
			ID:        "zz-child",
			Title:     "Child",
			Status:    "open",
			Priority:  intPtr(2),
			CreatedAt: "2024-01-01T00:00:01Z",
			Dependencies: []beadsDependency{
				{
					IssueID:     "zz-child",
					DependsOnID: "zz-parent",
					DepType:     DepTypeParentChild,
					CreatedAt:   "2024-01-01T00:00:05Z",
				},
			},
		},
	}
	writeBeadsIssues(t, sourceRoot, issues)
	// Confirm parent-child dependencies are preserved without renames.
	now := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	plan, err := PlanBeadsImport(BeadsImportOptions{SourceRoot: sourceRoot, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("plan beads import: %v", err)
	}
	// Confirm the dependency is preserved with the correct payload.
	depEvent, ok := findEvent(plan.Events, EventTypeDepAdd, "zz-child")
	if !ok {
		t.Fatalf("expected dep_add event")
	}
	if depEvent.Payload["depends_on"] != "zz-parent" || depEvent.Payload["dep_type"] != DepTypeParentChild {
		t.Fatalf("expected parent-child dependency payload")
	}
}

func TestPlanBeadsImportSkipsUnknownDependencyTypes(t *testing.T) {
	sourceRoot := t.TempDir()
	// Use an unsupported dependency type to trigger a warning.
	issues := []beadsIssue{
		{
			ID:        "zz-1a",
			Title:     "Issue",
			Status:    "open",
			Priority:  intPtr(2),
			CreatedAt: "2024-01-01T00:00:00Z",
			Dependencies: []beadsDependency{
				{
					IssueID:     "zz-1a",
					DependsOnID: "zz-2b",
					DepType:     "relates-to",
					CreatedAt:   "2024-01-01T00:00:02Z",
				},
			},
		},
		{
			ID:        "zz-2b",
			Title:     "Other",
			Status:    "open",
			Priority:  intPtr(2),
			CreatedAt: "2024-01-01T00:00:01Z",
		},
	}
	writeBeadsIssues(t, sourceRoot, issues)
	// Unknown dependency types should be omitted with a warning.
	now := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	plan, err := PlanBeadsImport(BeadsImportOptions{SourceRoot: sourceRoot, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("plan beads import: %v", err)
	}
	// Confirm the warning is recorded for unsupported dependencies.
	if hasWarning(plan.Result.Warnings, "unknown dependency type") == false {
		t.Fatalf("expected warning for unknown dependency type")
	}
}

func TestPlanBeadsImportFallsBackToNowForMissingTimestamps(t *testing.T) {
	sourceRoot := t.TempDir()
	// Omit timestamps to force the fallback logic.
	issues := []beadsIssue{
		{
			ID:       "zz-1a",
			Title:    "Issue",
			Status:   "open",
			Priority: intPtr(2),
		},
	}
	writeBeadsIssues(t, sourceRoot, issues)
	// Missing timestamps should default to the provided Now value.
	now := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	plan, err := PlanBeadsImport(BeadsImportOptions{SourceRoot: sourceRoot, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("plan beads import: %v", err)
	}
	// Verify the create event uses the fallback timestamp.
	createEvent, ok := findEvent(plan.Events, EventTypeCreate, "zz-1a")
	if !ok {
		t.Fatalf("expected create event")
	}
	if createEvent.Timestamp != now.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("expected create timestamp to match Now")
	}
}

func writeBeadsIssues(t *testing.T, root string, issues []beadsIssue) {
	t.Helper()
	beadsDir := filepath.Join(root, ".beads")
	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		t.Fatalf("create beads dir: %v", err)
	}
	path := filepath.Join(beadsDir, "issues.jsonl")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create beads issues file: %v", err)
	}
	defer func() { _ = file.Close() }()
	// Encode each issue as a JSON line.
	encoder := json.NewEncoder(file)
	for _, issue := range issues {
		if err := encoder.Encode(issue); err != nil {
			t.Fatalf("write beads issue: %v", err)
		}
	}
}

func findEvent(events []Event, eventType, issueID string) (Event, bool) {
	for _, event := range events {
		if event.Type == eventType && event.IssueID == issueID {
			return event, true
		}
	}
	return Event{}, false
}

func hasWarning(warnings []string, needle string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return true
		}
	}
	return false
}

func intPtr(value int) *int {
	return &value
}
