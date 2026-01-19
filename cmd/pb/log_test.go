package main

import (
	"strings"
	"testing"
	"time"

	"pebbles/internal/pebbles"
)

// TestSortLogEntries verifies newest-first ordering with line tie breaks.
func TestSortLogEntries(t *testing.T) {
	entries := []pebbles.EventLogEntry{
		{Line: 1, Event: pebbles.Event{Timestamp: "2024-01-01T00:00:00Z"}},
		{Line: 2, Event: pebbles.Event{Timestamp: "2024-01-02T00:00:00Z"}},
		{Line: 3, Event: pebbles.Event{Timestamp: "2024-01-01T00:00:00Z"}},
	}
	logEntries := buildLogEntries(entries)
	sortLogEntries(logEntries)
	if logEntries[0].Entry.Line != 2 {
		t.Fatalf("expected newest line 2 first, got line %d", logEntries[0].Entry.Line)
	}
	if logEntries[1].Entry.Line != 1 || logEntries[2].Entry.Line != 3 {
		t.Fatalf("expected tie-break by line order, got %d then %d", logEntries[1].Entry.Line, logEntries[2].Entry.Line)
	}
}

// TestLogEventDetails verifies event detail formatting.
func TestLogEventDetails(t *testing.T) {
	create := pebbles.Event{
		Type: pebbles.EventTypeCreate,
		Payload: map[string]string{
			"type":        "task",
			"priority":    "1",
			"description": "Seed data",
		},
	}
	if got := logEventDetails(create); got != `type=task priority=P1 description="Seed data"` {
		t.Fatalf("create details mismatch: %q", got)
	}
	status := pebbles.Event{
		Type:    pebbles.EventTypeStatus,
		Payload: map[string]string{"status": "in_progress"},
	}
	if got := logEventDetails(status); got != "status=in_progress" {
		t.Fatalf("status details mismatch: %q", got)
	}
	depAdd := pebbles.Event{
		Type:    pebbles.EventTypeDepAdd,
		Payload: map[string]string{"depends_on": "pb-1"},
	}
	if got := logEventDetails(depAdd); got != "depends_on=pb-1" {
		t.Fatalf("dep_add details mismatch: %q", got)
	}
	depRm := pebbles.Event{
		Type:    pebbles.EventTypeDepRemove,
		Payload: map[string]string{"depends_on": "pb-2"},
	}
	if got := logEventDetails(depRm); got != "depends_on=pb-2" {
		t.Fatalf("dep_rm details mismatch: %q", got)
	}
	depParent := pebbles.Event{
		Type: pebbles.EventTypeDepAdd,
		Payload: map[string]string{
			"depends_on": "pb-3",
			"dep_type":   pebbles.DepTypeParentChild,
		},
	}
	if got := logEventDetails(depParent); got != "depends_on=pb-3 dep_type=parent-child" {
		t.Fatalf("dep_add parent details mismatch: %q", got)
	}
	closeEvent := pebbles.Event{Type: pebbles.EventTypeClose, Payload: map[string]string{"description": "Done"}}
	if got := logEventDetails(closeEvent); got != "description=Done" {
		t.Fatalf("close details mismatch: %q", got)
	}
	comment := pebbles.Event{Type: pebbles.EventTypeComment, Payload: map[string]string{"body": "Needs attention"}}
	if got := logEventDetails(comment); got != `body="Needs attention"` {
		t.Fatalf("comment details mismatch: %q", got)
	}
	unknown := pebbles.Event{
		Type: "unknown_type",
		Payload: map[string]string{
			"note":     "needs review",
			"priority": "2",
		},
	}
	if got := logEventDetails(unknown); got != `priority=P2 note="needs review"` {
		t.Fatalf("unknown details mismatch: %q", got)
	}
}

// TestPadOrTrim verifies column padding and truncation.
func TestPadOrTrim(t *testing.T) {
	if got := padOrTrim("abc", 5); got != "abc  " {
		t.Fatalf("expected padding, got %q", got)
	}
	if got := padOrTrim("abcdefghijk", 8); got != "abcde..." {
		t.Fatalf("expected truncation, got %q", got)
	}
	if got := padOrTrim("data", 0); got != "data" {
		t.Fatalf("expected passthrough, got %q", got)
	}
}

// TestFormatLogLineIncludesDetails ensures details are appended when present.
func TestFormatLogLineIncludesDetails(t *testing.T) {
	line := logLine{
		Actor:      "unknown",
		ActorDate:  "unknown",
		EventTime:  "2024-01-01 00:00:00",
		EventType:  "status",
		IssueID:    "pb-1",
		IssueTitle: "Title",
		Details:    "status=open",
	}
	widths := logColumnWidths{Actor: 7, ActorDate: 7, EventTime: 19, EventType: 6, IssueID: 4, IssueTitle: 5}
	got := formatLogLine(line, widths)
	if !strings.Contains(got, "status=open") {
		t.Fatalf("expected details in output: %q", got)
	}
}

// TestParseLogTimestamp verifies supported timestamp formats.
func TestParseLogTimestamp(t *testing.T) {
	if _, err := parseLogTimestamp("2024-01-02T03:04:05Z"); err != nil {
		t.Fatalf("expected RFC3339 to parse: %v", err)
	}
	if _, err := parseLogTimestamp("2024-01-02"); err != nil {
		t.Fatalf("expected date to parse: %v", err)
	}
	if _, err := parseLogTimestamp("nope"); err == nil {
		t.Fatalf("expected invalid timestamp error")
	}
}

// TestParseGitTZ verifies git timezone parsing.
func TestParseGitTZ(t *testing.T) {
	if offset, ok := parseGitTZ("-0700"); !ok || offset != -7*60*60 {
		t.Fatalf("expected -0700 offset, got %d (ok=%v)", offset, ok)
	}
	if offset, ok := parseGitTZ("+0530"); !ok || offset != (5*60*60+30*60) {
		t.Fatalf("expected +0530 offset, got %d (ok=%v)", offset, ok)
	}
}

// TestParseGitBlame verifies parsing blame output into attribution lines.
func TestParseGitBlame(t *testing.T) {
	authorTime := int64(1700000000)
	expectedDate := time.Unix(authorTime, 0).UTC().Format("2006-01-02")
	output := []byte(strings.Join([]string{
		"abcd1234 1 1 1",
		"author Alice",
		"author-mail <alice@example.com>",
		"author-time 1700000000",
		"author-tz +0000",
		"summary test",
		"filename .pebbles/events.jsonl",
		"\t{\"type\":\"create\"}",
		"ef567890 2 2 1",
		"author Bob",
		"author-mail <bob@example.com>",
		"author-time 1700003600",
		"author-tz +0000",
		"summary test",
		"filename .pebbles/events.jsonl",
		"\t{\"type\":\"close\"}",
	}, "\n"))
	attributions, err := parseGitBlame(output)
	if err != nil {
		t.Fatalf("parse git blame: %v", err)
	}
	if len(attributions) != 2 {
		t.Fatalf("expected 2 attributions, got %d", len(attributions))
	}
	if attributions[0].Author != "Alice" || attributions[0].Date != expectedDate {
		t.Fatalf("unexpected attribution: %+v", attributions[0])
	}
	if attributions[1].Author != "Bob" {
		t.Fatalf("unexpected attribution: %+v", attributions[1])
	}
}

// TestFormatPrettyLogWithDetails ensures pretty output includes details lines.
func TestFormatPrettyLogWithDetails(t *testing.T) {
	previous := colorEnabled
	colorEnabled = false
	defer func() {
		colorEnabled = previous
	}()

	entry := logEntry{
		Entry: pebbles.EventLogEntry{
			Line:  5,
			Event: pebbles.Event{Type: pebbles.EventTypeCreate, IssueID: "pb-1", Payload: map[string]string{"type": "task", "priority": "2"}},
		},
		ParsedTime: time.Date(2026, 1, 19, 10, 0, 0, 0, time.UTC),
		ParsedOK:   true,
	}
	line := logLine{
		Actor:      "Josh",
		ActorDate:  "2026-01-19",
		EventTime:  "2026-01-19 10:00:00",
		EventType:  "create",
		IssueID:    "pb-1",
		IssueTitle: "Pretty Log",
	}
	output := formatPrettyLog(entry, line)
	if !strings.Contains(output, "event 5 create pb-1") {
		t.Fatalf("missing header: %q", output)
	}
	if !strings.Contains(output, "Details:\n  type=task\n  priority=P2") {
		t.Fatalf("missing detail lines: %q", output)
	}
}

// TestFormatPrettyLogNoDetails ensures empty payloads show (none).
func TestFormatPrettyLogNoDetails(t *testing.T) {
	previous := colorEnabled
	colorEnabled = false
	defer func() {
		colorEnabled = previous
	}()

	entry := logEntry{
		Entry: pebbles.EventLogEntry{
			Line:  2,
			Event: pebbles.Event{Type: pebbles.EventTypeClose, IssueID: "pb-2"},
		},
	}
	line := logLine{
		Actor:      "unknown",
		ActorDate:  "unknown",
		EventTime:  "2026-01-19 10:00:00",
		EventType:  "close",
		IssueID:    "pb-2",
		IssueTitle: "Closed",
	}
	output := formatPrettyLog(entry, line)
	if !strings.Contains(output, "Details: (none)") {
		t.Fatalf("expected no-details marker: %q", output)
	}
}

// TestFormatPrettyLogColors verifies ANSI styling is applied when enabled.
func TestFormatPrettyLogColors(t *testing.T) {
	previous := colorEnabled
	colorEnabled = true
	defer func() {
		colorEnabled = previous
	}()

	entry := logEntry{
		Entry: pebbles.EventLogEntry{
			Line:  7,
			Event: pebbles.Event{Type: pebbles.EventTypeCreate, IssueID: "pb-9", Payload: map[string]string{"type": "task", "priority": "1"}},
		},
		ParsedTime: time.Date(2026, 1, 19, 10, 0, 0, 0, time.UTC),
		ParsedOK:   true,
	}
	line := logLine{
		Actor:      "Josh",
		ActorDate:  "2026-01-19",
		EventTime:  "2026-01-19 10:00:00",
		EventType:  "create",
		IssueID:    "pb-9",
		IssueTitle: "Pretty Log",
	}
	output := formatPrettyLog(entry, line)
	if !strings.Contains(output, ansiBrightBlue) {
		t.Fatalf("expected event type color in output: %q", output)
	}
	if !strings.Contains(output, ansiCyan) {
		t.Fatalf("expected issue id color in output: %q", output)
	}
	if !strings.Contains(output, ansiBold) {
		t.Fatalf("expected title color in output: %q", output)
	}
}

// TestShouldUsePager verifies pager selection logic.
func TestShouldUsePager(t *testing.T) {
	if shouldUsePager(true, true) {
		t.Fatalf("expected no pager when disabled")
	}
	if shouldUsePager(false, false) {
		t.Fatalf("expected no pager when not a tty")
	}
	if !shouldUsePager(false, true) {
		t.Fatalf("expected pager when enabled on tty")
	}
}

// TestResolvePagerCommand verifies env override order.
func TestResolvePagerCommand(t *testing.T) {
	t.Setenv("PB_PAGER", "more")
	t.Setenv("PAGER", "less -FRX")
	command := resolvePagerCommand()
	if len(command) != 1 || command[0] != "more" {
		t.Fatalf("expected PB_PAGER to win, got %v", command)
	}
	t.Setenv("PB_PAGER", "")
	command = resolvePagerCommand()
	if len(command) == 0 || command[0] != "less" {
		t.Fatalf("expected PAGER fallback, got %v", command)
	}
}

// TestEnrichEventAddsDescription verifies close events get descriptions.
func TestEnrichEventAddsDescription(t *testing.T) {
	descriptions := map[string]string{"pb-1": "Done work"}
	event := pebbles.Event{Type: pebbles.EventTypeClose, IssueID: "pb-1"}
	enriched := enrichEvent(event, descriptions)
	if enriched.Payload == nil || enriched.Payload["description"] != "Done work" {
		t.Fatalf("expected description to be enriched, got %+v", enriched.Payload)
	}
}
