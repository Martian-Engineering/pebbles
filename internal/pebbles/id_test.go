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
