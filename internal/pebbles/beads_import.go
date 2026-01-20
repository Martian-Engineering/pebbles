package pebbles

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	beadsStatusTombstone  = "tombstone"
	importPriorityDefault = 2
	importTimeLayout      = time.RFC3339Nano
)

type beadsIssue struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	Status       string            `json:"status"`
	Priority     *int              `json:"priority"`
	IssueType    string            `json:"issue_type"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
	ClosedAt     string            `json:"closed_at"`
	CloseReason  string            `json:"close_reason"`
	DeletedAt    string            `json:"deleted_at"`
	DeletedBy    string            `json:"deleted_by"`
	DeleteReason string            `json:"delete_reason"`
	Dependencies []beadsDependency `json:"dependencies"`
	Comments     []beadsComment    `json:"comments"`
}

type beadsDependency struct {
	IssueID     string `json:"issue_id"`
	DependsOnID string `json:"depends_on_id"`
	DepType     string `json:"type"`
	CreatedAt   string `json:"created_at"`
}

type beadsComment struct {
	Author    string `json:"author"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

type importEvent struct {
	Event    Event
	SortTime time.Time
	Order    int
}

// BeadsImportOptions controls how Beads issues are translated into Pebbles events.
type BeadsImportOptions struct {
	SourceRoot        string
	Prefix            string
	IncludeTombstones bool
	Now               func() time.Time
}

// BeadsImportResult summarizes a Beads import plan or execution.
type BeadsImportResult struct {
	SourceRoot        string
	Prefix            string
	IssuesTotal       int
	IssuesImported    int
	IssuesSkipped     int
	TombstonesSkipped int
	EventsPlanned     int
	EventsWritten     int
	Warnings          []string
}

// BeadsImportPlan holds the events required to recreate Beads issues in Pebbles.
type BeadsImportPlan struct {
	Events []Event
	Result BeadsImportResult
}

// PlanBeadsImport builds a Pebbles event plan from a Beads issues.jsonl export.
func PlanBeadsImport(options BeadsImportOptions) (BeadsImportPlan, error) {
	if strings.TrimSpace(options.SourceRoot) == "" {
		return BeadsImportPlan{}, fmt.Errorf("source root is required")
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	// Load issues from the Beads JSONL export first.
	issues, warnings, err := loadBeadsIssues(options.SourceRoot)
	if err != nil {
		return BeadsImportPlan{}, err
	}
	// Resolve a prefix to seed the Pebbles config.
	prefix, prefixWarnings, err := resolveBeadsPrefix(issues, options.Prefix)
	if err != nil {
		return BeadsImportPlan{}, err
	}
	warnings = append(warnings, prefixWarnings...)
	// Build the import plan and aggregate all warnings.
	plan, err := buildBeadsImportPlan(issues, options.IncludeTombstones, options.Now(), &warnings)
	if err != nil {
		return BeadsImportPlan{}, err
	}
	// Populate metadata for callers and return the full plan.
	plan.Result.SourceRoot = options.SourceRoot
	plan.Result.Prefix = prefix
	plan.Result.Warnings = warnings
	return plan, nil
}

// ApplyBeadsImportPlan appends the planned events to the Pebbles log.
func ApplyBeadsImportPlan(root string, plan BeadsImportPlan) (BeadsImportResult, error) {
	for _, event := range plan.Events {
		if err := AppendEvent(root, event); err != nil {
			return BeadsImportResult{}, err
		}
	}
	if err := RebuildCache(root); err != nil {
		return BeadsImportResult{}, err
	}
	plan.Result.EventsWritten = len(plan.Events)
	return plan.Result, nil
}

func loadBeadsIssues(sourceRoot string) ([]beadsIssue, []string, error) {
	path := filepath.Join(sourceRoot, ".beads", "issues.jsonl")
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open beads issues: %w", err)
	}
	defer func() { _ = file.Close() }()
	// Increase the scanner buffer to handle large issue descriptions.
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 5*1024*1024)
	var issues []beadsIssue
	var warnings []string
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		// Decode each JSON line into a Beads issue record.
		var issue beadsIssue
		if err := json.Unmarshal([]byte(line), &issue); err != nil {
			return nil, nil, fmt.Errorf("parse beads issue line %d: %w", lineNumber, err)
		}
		// Warn and skip when required identifiers are missing.
		if strings.TrimSpace(issue.ID) == "" {
			warnings = append(warnings, fmt.Sprintf("line %d missing issue id", lineNumber))
			continue
		}
		issues = append(issues, issue)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan beads issues: %w", err)
	}
	if len(issues) == 0 {
		return nil, warnings, fmt.Errorf("no beads issues found")
	}
	return issues, warnings, nil
}

func resolveBeadsPrefix(issues []beadsIssue, override string) (string, []string, error) {
	override = strings.TrimSpace(override)
	if override != "" {
		return override, nil, nil
	}
	// Scan issue IDs and count the detected prefixes.
	prefixes := make(map[string]int)
	var warnings []string
	for _, issue := range issues {
		prefix := prefixFromIssueID(issue.ID)
		if prefix == "" {
			warnings = append(warnings, fmt.Sprintf("issue %s missing prefix separator", issue.ID))
			continue
		}
		prefixes[prefix]++
	}
	if len(prefixes) == 0 {
		return "", warnings, fmt.Errorf("unable to detect prefix; provide --prefix")
	}
	if len(prefixes) > 1 {
		var keys []string
		for prefix := range prefixes {
			keys = append(keys, prefix)
		}
		sort.Strings(keys)
		return "", warnings, fmt.Errorf("multiple prefixes detected: %s", strings.Join(keys, ", "))
	}
	for prefix := range prefixes {
		return prefix, warnings, nil
	}
	return "", warnings, fmt.Errorf("unable to detect prefix")
}

func buildBeadsImportPlan(issues []beadsIssue, includeTombstones bool, now time.Time, warnings *[]string) (BeadsImportPlan, error) {
	result := BeadsImportResult{IssuesTotal: len(issues)}
	importedIDs := make(map[string]bool)
	var imported []beadsIssue
	// Filter issues, skipping tombstones and invalid entries.
	for _, issue := range issues {
		issueID := strings.TrimSpace(issue.ID)
		if issueID == "" {
			result.IssuesSkipped++
			continue
		}
		if importedIDs[issueID] {
			*warnings = append(*warnings, fmt.Sprintf("duplicate issue id %s", issueID))
			result.IssuesSkipped++
			continue
		}
		status := normalizeBeadsStatus(issue.Status, issueID, warnings)
		if status == beadsStatusTombstone && !includeTombstones {
			result.TombstonesSkipped++
			result.IssuesSkipped++
			continue
		}
		// Require a non-empty title for Pebbles create events.
		if strings.TrimSpace(issue.Title) == "" {
			*warnings = append(*warnings, fmt.Sprintf("issue %s missing title", issueID))
			result.IssuesSkipped++
			continue
		}
		issue.ID = issueID
		issue.Status = status
		importedIDs[issueID] = true
		imported = append(imported, issue)
	}
	result.IssuesImported = len(imported)
	if result.IssuesImported == 0 {
		return BeadsImportPlan{}, fmt.Errorf("no issues to import")
	}
	// Build event groups with explicit ordering buckets.
	var createEvents []importEvent
	var depAndCommentEvents []importEvent
	var statusEvents []importEvent
	for _, issue := range imported {
		created := buildBeadsCreateEvent(issue, now, warnings)
		createEvents = append(createEvents, created)
		for _, dep := range buildBeadsDependencyEvents(issue, importedIDs, now, warnings) {
			depAndCommentEvents = append(depAndCommentEvents, dep)
		}
		for _, comment := range buildBeadsCommentEvents(issue, now, warnings) {
			depAndCommentEvents = append(depAndCommentEvents, comment)
		}
		for _, status := range buildBeadsStatusEvents(issue, now, warnings) {
			statusEvents = append(statusEvents, status)
		}
	}
	// Sort each bucket and concatenate in the required order.
	sortImportEvents(createEvents)
	sortImportEvents(depAndCommentEvents)
	sortImportEvents(statusEvents)
	var events []Event
	for _, event := range createEvents {
		events = append(events, event.Event)
	}
	for _, event := range depAndCommentEvents {
		events = append(events, event.Event)
	}
	for _, event := range statusEvents {
		events = append(events, event.Event)
	}
	result.EventsPlanned = len(events)
	plan := BeadsImportPlan{Events: events, Result: result}
	return plan, nil
}

func buildBeadsCreateEvent(issue beadsIssue, now time.Time, warnings *[]string) importEvent {
	createdTime, createdStamp := resolveTimestamp(
		[]string{issue.CreatedAt, issue.UpdatedAt},
		now,
		fmt.Sprintf("issue %s create", issue.ID),
		warnings,
	)
	priority := normalizeBeadsPriority(issue.Priority, issue.ID, warnings)
	issueType := normalizeBeadsIssueType(issue.IssueType)
	event := NewCreateEvent(issue.ID, issue.Title, issue.Description, issueType, createdStamp, priority)
	return importEvent{Event: event, SortTime: createdTime, Order: 0}
}

func buildBeadsDependencyEvents(issue beadsIssue, importedIDs map[string]bool, now time.Time, warnings *[]string) []importEvent {
	var events []importEvent
	for _, dep := range issue.Dependencies {
		// Prefer the dependency issue id but fall back to the parent issue id.
		issueID := strings.TrimSpace(dep.IssueID)
		if issueID == "" {
			issueID = issue.ID
		}
		if issueID != issue.ID {
			*warnings = append(*warnings, fmt.Sprintf("dependency issue id mismatch: %s vs %s", issue.ID, issueID))
		}
		dependsOn := strings.TrimSpace(dep.DependsOnID)
		if dependsOn == "" {
			*warnings = append(*warnings, fmt.Sprintf("dependency on issue %s missing depends_on", issueID))
			continue
		}
		// Only the supported dependency types should be imported.
		depType := strings.TrimSpace(dep.DepType)
		if depType != DepTypeBlocks && depType != DepTypeParentChild {
			*warnings = append(*warnings, fmt.Sprintf("issue %s unknown dependency type %s", issueID, depType))
			continue
		}
		// Skip edges referencing issues that were filtered out.
		if !importedIDs[issueID] || !importedIDs[dependsOn] {
			*warnings = append(*warnings, fmt.Sprintf("dependency %s -> %s skipped (missing issue)", issueID, dependsOn))
			continue
		}
		depTime, depStamp := resolveTimestamp(
			[]string{dep.CreatedAt, issue.UpdatedAt, issue.CreatedAt},
			now,
			fmt.Sprintf("dependency %s -> %s", issueID, dependsOn),
			warnings,
		)
		event := NewDepAddEvent(issueID, dependsOn, depType, depStamp)
		events = append(events, importEvent{Event: event, SortTime: depTime, Order: 1})
	}
	return events
}

func buildBeadsCommentEvents(issue beadsIssue, now time.Time, warnings *[]string) []importEvent {
	var events []importEvent
	for _, comment := range issue.Comments {
		// Preserve the author in the comment body since Pebbles lacks author metadata.
		body := strings.TrimSpace(comment.Text)
		if body == "" {
			*warnings = append(*warnings, fmt.Sprintf("issue %s has empty comment", issue.ID))
			continue
		}
		body = formatBeadsCommentBody(comment.Author, body)
		commentTime, commentStamp := resolveTimestamp(
			[]string{comment.CreatedAt, issue.UpdatedAt, issue.CreatedAt},
			now,
			fmt.Sprintf("comment on %s", issue.ID),
			warnings,
		)
		event := NewCommentEvent(issue.ID, body, commentStamp)
		events = append(events, importEvent{Event: event, SortTime: commentTime, Order: 2})
	}
	// Capture close/delete reasons as a final comment entry.
	if reason := buildBeadsReasonComment(issue); reason != "" {
		reasonTime, reasonStamp := resolveTimestamp(
			[]string{issue.ClosedAt, issue.DeletedAt, issue.UpdatedAt, issue.CreatedAt},
			now,
			fmt.Sprintf("close reason on %s", issue.ID),
			warnings,
		)
		event := NewCommentEvent(issue.ID, reason, reasonStamp)
		events = append(events, importEvent{Event: event, SortTime: reasonTime, Order: 2})
	}
	return events
}

func buildBeadsStatusEvents(issue beadsIssue, now time.Time, warnings *[]string) []importEvent {
	var events []importEvent
	switch issue.Status {
	case StatusInProgress:
		// Emit a status update for in-progress issues only.
		statusTime, statusStamp := resolveTimestamp(
			[]string{issue.UpdatedAt, issue.CreatedAt},
			now,
			fmt.Sprintf("status update on %s", issue.ID),
			warnings,
		)
		event := NewStatusEvent(issue.ID, StatusInProgress, statusStamp)
		events = append(events, importEvent{Event: event, SortTime: statusTime, Order: 3})
	case StatusClosed, beadsStatusTombstone:
		// Close events mark closed and tombstone issues in Pebbles.
		closeTime, closeStamp := resolveTimestamp(
			[]string{issue.ClosedAt, issue.DeletedAt, issue.UpdatedAt, issue.CreatedAt},
			now,
			fmt.Sprintf("close issue %s", issue.ID),
			warnings,
		)
		event := NewCloseEvent(issue.ID, closeStamp)
		events = append(events, importEvent{Event: event, SortTime: closeTime, Order: 4})
	}
	return events
}

func normalizeBeadsStatus(status, issueID string, warnings *[]string) string {
	trimmed := strings.TrimSpace(strings.ToLower(status))
	// Normalize hyphenated status values to Pebbles equivalents.
	normalized := strings.ReplaceAll(trimmed, "-", "_")
	switch normalized {
	case StatusOpen:
		return StatusOpen
	case StatusInProgress:
		return StatusInProgress
	case StatusClosed:
		return StatusClosed
	case beadsStatusTombstone:
		return beadsStatusTombstone
	default:
		*warnings = append(*warnings, fmt.Sprintf("issue %s unknown status %q; defaulting to open", issueID, status))
		return StatusOpen
	}
}

func normalizeBeadsPriority(priority *int, issueID string, warnings *[]string) int {
	if priority == nil {
		// Default to P2 when Beads doesn't set a priority value.
		*warnings = append(*warnings, fmt.Sprintf("issue %s missing priority; using P2", issueID))
		return importPriorityDefault
	}
	value := *priority
	// Clamp priority values outside the Pebbles range.
	if value < 0 {
		*warnings = append(*warnings, fmt.Sprintf("issue %s priority %d below P0", issueID, value))
		return 0
	}
	if value > 4 {
		*warnings = append(*warnings, fmt.Sprintf("issue %s priority %d above P4", issueID, value))
		return 4
	}
	return value
}

func normalizeBeadsIssueType(issueType string) string {
	trimmed := strings.TrimSpace(issueType)
	if trimmed == "" {
		return "task"
	}
	return trimmed
}

func formatBeadsCommentBody(author, text string) string {
	trimmed := strings.TrimSpace(author)
	if trimmed == "" {
		return text
	}
	return fmt.Sprintf("Author: %s\n%s", trimmed, text)
}

func buildBeadsReasonComment(issue beadsIssue) string {
	var lines []string
	// Capture any close or delete metadata in a comment body.
	if strings.TrimSpace(issue.CloseReason) != "" {
		lines = append(lines, fmt.Sprintf("Close reason: %s", strings.TrimSpace(issue.CloseReason)))
	}
	if strings.TrimSpace(issue.DeleteReason) != "" {
		lines = append(lines, fmt.Sprintf("Delete reason: %s", strings.TrimSpace(issue.DeleteReason)))
	}
	if strings.TrimSpace(issue.DeletedBy) != "" {
		lines = append(lines, fmt.Sprintf("Deleted by: %s", strings.TrimSpace(issue.DeletedBy)))
	}
	if strings.TrimSpace(issue.DeletedAt) != "" {
		lines = append(lines, fmt.Sprintf("Deleted at: %s", strings.TrimSpace(issue.DeletedAt)))
	}
	return strings.Join(lines, "\n")
}

func resolveTimestamp(values []string, fallback time.Time, context string, warnings *[]string) (time.Time, string) {
	// Walk the candidate timestamps and use the first valid one.
	for _, value := range values {
		parsed, ok := parseTimestamp(value)
		if ok {
			return parsed, formatTimestamp(parsed)
		}
	}
	// Fall back to the provided time when all candidates are missing.
	*warnings = append(*warnings, fmt.Sprintf("%s missing timestamp; using now", context))
	return fallback, formatTimestamp(fallback)
}

func parseTimestamp(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}
	// Try RFC3339Nano first, then fallback to RFC3339.
	parsed, err := time.Parse(importTimeLayout, trimmed)
	if err == nil {
		return parsed, true
	}
	parsed, err = time.Parse(time.RFC3339, trimmed)
	if err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func formatTimestamp(value time.Time) string {
	return value.UTC().Format(importTimeLayout)
}

func sortImportEvents(events []importEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		left := events[i]
		right := events[j]
		// Prefer timestamps, then event ordering, then issue id for stability.
		if left.SortTime.Equal(right.SortTime) {
			if left.Order == right.Order {
				return left.Event.IssueID < right.Event.IssueID
			}
			return left.Order < right.Order
		}
		return left.SortTime.Before(right.SortTime)
	})
}

func prefixFromIssueID(issueID string) string {
	parts := strings.SplitN(issueID, "-", 2)
	if len(parts) != 2 || parts[0] == "" {
		return ""
	}
	return parts[0]
}
