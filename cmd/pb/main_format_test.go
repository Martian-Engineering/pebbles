package main

import (
	"strings"
	"testing"

	"pebbles/internal/pebbles"
)

// TestFormatIssueLineColors verifies that list output includes ANSI color codes.
func TestFormatIssueLineColors(t *testing.T) {
	previous := colorEnabled
	colorEnabled = true
	defer func() {
		colorEnabled = previous
	}()

	issue := pebbles.Issue{
		ID:        "pb-1",
		Title:     "Fix bug",
		IssueType: "bug",
		Status:    pebbles.StatusInProgress,
		Priority:  0,
	}
	widths := issueColumnWidthsForIssues([]pebbles.Issue{issue})
	output := formatIssueLine(issue, 0, widths)
	if !strings.Contains(output, ansiBrightYellow) {
		t.Fatalf("expected status color in output: %q", output)
	}
	if !strings.Contains(output, ansiBrightRed) {
		t.Fatalf("expected priority color in output: %q", output)
	}
	if !strings.Contains(output, ansiRed) {
		t.Fatalf("expected type color in output: %q", output)
	}
}

// TestRenderMarkdownHighlights verifies ANSI highlighting for bold and code spans.
func TestRenderMarkdownHighlights(t *testing.T) {
	previous := colorEnabled
	colorEnabled = true
	defer func() {
		colorEnabled = previous
	}()

	input := "Make **bold** and `code` work"
	output := renderMarkdown(input)
	if strings.Contains(output, "**") {
		t.Fatalf("expected bold markers to be removed: %q", output)
	}
	if !strings.Contains(output, ansiBold+"bold"+ansiReset) {
		t.Fatalf("expected bold styling in output: %q", output)
	}
	if !strings.Contains(output, ansiCyan+"`code`"+ansiReset) {
		t.Fatalf("expected code styling in output: %q", output)
	}
}
