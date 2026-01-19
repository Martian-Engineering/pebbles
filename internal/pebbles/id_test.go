package pebbles

import (
	"strings"
	"testing"
)

func TestGenerateIssueIDDefaultLength(t *testing.T) {
	id := GenerateIssueID("pb", "Title", "2024-01-01T00:00:00Z", "host")
	parts := strings.SplitN(id, "-", 2)
	if len(parts) != 2 {
		t.Fatalf("expected prefix and suffix, got %q", id)
	}
	if parts[0] != "pb" {
		t.Fatalf("expected prefix pb, got %s", parts[0])
	}
	if len(parts[1]) != defaultIssueIDSuffixLength {
		t.Fatalf("expected suffix length %d, got %d", defaultIssueIDSuffixLength, len(parts[1]))
	}
}

func TestGenerateUniqueIssueIDExpandsOnCollision(t *testing.T) {
	prefix := "pb"
	title := "Title"
	timestamp := "2024-01-01T00:00:00Z"
	host := "host"
	hash := issueIDHash(prefix, title, timestamp, host)
	first := issueIDFromHash(prefix, hash, defaultIssueIDSuffixLength)
	second := issueIDFromHash(prefix, hash, defaultIssueIDSuffixLength+1)
	seen := map[string]bool{first: true}
	// Force one collision to ensure the suffix expands.
	id, err := GenerateUniqueIssueID(prefix, title, timestamp, host, func(candidate string) (bool, error) {
		return seen[candidate], nil
	})
	if err != nil {
		t.Fatalf("generate unique issue id: %v", err)
	}
	if id != second {
		t.Fatalf("expected %s, got %s", second, id)
	}
}

func TestHasParentChildSuffix(t *testing.T) {
	parent := "pb-abc"
	if !HasParentChildSuffix(parent, "pb-abc.2") {
		t.Fatalf("expected parent suffix match")
	}
	if HasParentChildSuffix(parent, "pb-abc.2.3") {
		t.Fatalf("expected non-numeric suffix to be rejected")
	}
	if HasParentChildSuffix(parent, "pb-abc-2") {
		t.Fatalf("expected dash suffix to be rejected")
	}
}

func TestNextChildIssueIDSkipsUsedSuffixes(t *testing.T) {
	root := t.TempDir()
	if err := InitProject(root); err != nil {
		t.Fatalf("init project: %v", err)
	}
	parentID := "pb-parent"
	childOne := parentID + ".1"
	childTwo := parentID + ".2"
	childThree := parentID + ".3"
	if err := AppendEvent(root, NewCreateEvent(parentID, "Parent", "", "task", "2024-01-06T00:00:00Z", 2)); err != nil {
		t.Fatalf("append parent: %v", err)
	}
	if err := AppendEvent(root, NewCreateEvent(childOne, "Child 1", "", "task", "2024-01-06T00:00:01Z", 2)); err != nil {
		t.Fatalf("append child 1: %v", err)
	}
	if err := AppendEvent(root, NewCreateEvent(childTwo, "Child 2", "", "task", "2024-01-06T00:00:02Z", 2)); err != nil {
		t.Fatalf("append child 2: %v", err)
	}
	if err := AppendEvent(root, NewCreateEvent(childThree, "Child 3", "", "task", "2024-01-06T00:00:03Z", 2)); err != nil {
		t.Fatalf("append child 3: %v", err)
	}
	if err := AppendEvent(root, NewDepAddEvent(childOne, parentID, DepTypeParentChild, "2024-01-06T00:00:04Z")); err != nil {
		t.Fatalf("append child 1 dep: %v", err)
	}
	if err := AppendEvent(root, NewDepAddEvent(childThree, parentID, DepTypeParentChild, "2024-01-06T00:00:05Z")); err != nil {
		t.Fatalf("append child 3 dep: %v", err)
	}
	if err := RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}
	next, err := NextChildIssueID(root, parentID)
	if err != nil {
		t.Fatalf("next child issue id: %v", err)
	}
	want := parentID + ".4"
	if next != want {
		t.Fatalf("expected %s, got %s", want, next)
	}
}
