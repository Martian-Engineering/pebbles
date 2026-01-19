package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"pebbles/internal/pebbles"
)

const (
	logEventTimeLayout = "2006-01-02 15:04:05"
)

var (
	payloadKeyOrder        = []string{"title", "description", "type", "priority", "status", "depends_on"}
	defaultLogColumnWidths = logColumnWidths{
		Actor:      16,
		ActorDate:  10,
		EventTime:  19,
		EventType:  10,
		IssueID:    18,
		IssueTitle: 40,
	}
)

type logEntry struct {
	Entry      pebbles.EventLogEntry
	ParsedTime time.Time
	ParsedOK   bool
}

type logLine struct {
	Actor      string
	ActorDate  string
	EventTime  string
	EventType  string
	IssueID    string
	IssueTitle string
	Details    string
}

type logColumnWidths struct {
	Actor      int
	ActorDate  int
	EventTime  int
	EventType  int
	IssueID    int
	IssueTitle int
}

type logJSON struct {
	Line       int               `json:"line"`
	Timestamp  string            `json:"timestamp"`
	Type       string            `json:"type"`
	Label      string            `json:"label"`
	IssueID    string            `json:"issue_id"`
	IssueTitle string            `json:"issue_title"`
	Actor      string            `json:"actor"`
	ActorDate  string            `json:"actor_date"`
	Details    string            `json:"details,omitempty"`
	Payload    map[string]string `json:"payload,omitempty"`
}

type gitAttribution struct {
	Author string
	Date   string
}

// runLog handles pb log.
func runLog(root string, args []string) {
	fs := flag.NewFlagSet("log", flag.ExitOnError)
	var limit int
	fs.IntVar(&limit, "limit", 0, "Limit number of events")
	fs.IntVar(&limit, "n", 0, "Alias for --limit")
	sinceInput := fs.String("since", "", "Only show events on or after timestamp")
	untilInput := fs.String("until", "", "Only show events on or before timestamp")
	noGit := fs.Bool("no-git", false, "Skip git blame attribution")
	jsonOut := fs.Bool("json", false, "Output JSON lines")
	_ = fs.Parse(args)
	// Ensure the event log is available before reading.
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	if limit < 0 {
		exitError(fmt.Errorf("limit must be >= 0"))
	}
	// Parse optional time filters.
	since, useSince, err := parseOptionalTimestamp(*sinceInput)
	if err != nil {
		exitError(err)
	}
	until, useUntil, err := parseOptionalTimestamp(*untilInput)
	if err != nil {
		exitError(err)
	}
	entries, err := pebbles.LoadEventLog(root)
	if err != nil {
		exitError(err)
	}
	titles, err := issueTitleMap(root)
	if err != nil {
		exitError(err)
	}
	logEntries := buildLogEntries(entries)
	filtered, err := filterLogEntries(logEntries, since, until, useSince, useUntil)
	if err != nil {
		exitError(err)
	}
	sortLogEntries(filtered)
	// Apply limits after sorting and filtering.
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	var attributions []gitAttribution
	if !*noGit {
		attributions, err = gitBlameAttributions(root, pebbles.EventsPath(root))
		if err != nil {
			attributions = nil
		}
	}
	for _, entry := range filtered {
		attribution := attributionForLine(attributions, entry.Entry.Line)
		line := logLine{
			Actor:      attribution.Author,
			ActorDate:  attribution.Date,
			EventTime:  formatEventTime(entry),
			EventType:  logEventLabel(entry.Entry.Event),
			IssueID:    entry.Entry.Event.IssueID,
			IssueTitle: titleForIssue(titles, entry.Entry.Event.IssueID),
			Details:    logEventDetails(entry.Entry.Event),
		}
		if *jsonOut {
			if err := printLogJSON(entry, line); err != nil {
				exitError(err)
			}
			continue
		}
		fmt.Println(formatLogLine(line, defaultLogColumnWidths))
	}
}

// issueTitleMap builds a map of issue IDs to titles for log output.
func issueTitleMap(root string) (map[string]string, error) {
	issues, err := pebbles.ListIssues(root)
	if err != nil {
		return nil, err
	}
	titles := make(map[string]string, len(issues))
	for _, issue := range issues {
		titles[issue.ID] = issue.Title
	}
	return titles, nil
}

// titleForIssue returns the title for an issue ID or "unknown".
func titleForIssue(titles map[string]string, issueID string) string {
	title := titles[issueID]
	if title == "" {
		return "unknown"
	}
	return title
}

// buildLogEntries parses timestamps once for sorting/filtering.
func buildLogEntries(entries []pebbles.EventLogEntry) []logEntry {
	logEntries := make([]logEntry, 0, len(entries))
	for _, entry := range entries {
		parsed, err := time.Parse(time.RFC3339Nano, entry.Event.Timestamp)
		logEntries = append(logEntries, logEntry{
			Entry:      entry,
			ParsedTime: parsed,
			ParsedOK:   err == nil,
		})
	}
	return logEntries
}

// filterLogEntries applies optional time filters to log entries.
func filterLogEntries(entries []logEntry, since, until time.Time, useSince, useUntil bool) ([]logEntry, error) {
	filtered := make([]logEntry, 0, len(entries))
	for _, entry := range entries {
		// Reject invalid timestamps when range filters are active.
		if (useSince || useUntil) && !entry.ParsedOK {
			return nil, fmt.Errorf("invalid event timestamp at line %d", entry.Entry.Line)
		}
		// Apply time window filters.
		if useSince && entry.ParsedTime.Before(since) {
			continue
		}
		if useUntil && entry.ParsedTime.After(until) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered, nil
}

// sortLogEntries sorts entries newest-first with a file-order tie break.
func sortLogEntries(entries []logEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]
		if left.ParsedOK && right.ParsedOK {
			if !left.ParsedTime.Equal(right.ParsedTime) {
				return left.ParsedTime.After(right.ParsedTime)
			}
		} else if left.ParsedOK != right.ParsedOK {
			return left.ParsedOK
		}
		return left.Entry.Line < right.Entry.Line
	})
}

// formatEventTime renders a log timestamp in a stable layout.
func formatEventTime(entry logEntry) string {
	if entry.ParsedOK {
		return entry.ParsedTime.UTC().Format(logEventTimeLayout)
	}
	return entry.Entry.Event.Timestamp
}

// logEventLabel returns a display label for an event type.
func logEventLabel(event pebbles.Event) string {
	switch event.Type {
	case pebbles.EventTypeCreate:
		return "create"
	case pebbles.EventTypeStatus:
		return "status"
	case pebbles.EventTypeClose:
		return "close"
	case pebbles.EventTypeDepAdd:
		return "dep_add"
	case pebbles.EventTypeDepRemove:
		return "dep_rm"
	default:
		if event.Type == "" {
			return "unknown"
		}
		return event.Type
	}
}

// logEventDetails formats the payload details for known event types.
func logEventDetails(event pebbles.Event) string {
	switch event.Type {
	case pebbles.EventTypeCreate:
		parts := make([]string, 0, 2)
		if issueType := event.Payload["type"]; issueType != "" {
			parts = append(parts, fmt.Sprintf("type=%s", issueType))
		}
		if priority := event.Payload["priority"]; priority != "" {
			parts = append(parts, fmt.Sprintf("priority=%s", formatPriority(priority)))
		}
		return strings.Join(parts, " ")
	case pebbles.EventTypeStatus:
		if status := event.Payload["status"]; status != "" {
			return fmt.Sprintf("status=%s", status)
		}
	case pebbles.EventTypeDepAdd, pebbles.EventTypeDepRemove:
		if dependsOn := event.Payload["depends_on"]; dependsOn != "" {
			return fmt.Sprintf("depends_on=%s", dependsOn)
		}
	default:
		return formatPayloadPairs(event.Payload)
	}
	return ""
}

// formatPriority normalizes priority payload values as P0-P4 when possible.
func formatPriority(value string) string {
	parsed, err := pebbles.ParsePriority(value)
	if err != nil {
		return value
	}
	return pebbles.PriorityLabel(parsed)
}

// formatPayloadPairs formats payload pairs for unknown event types.
func formatPayloadPairs(payload map[string]string) string {
	if len(payload) == 0 {
		return ""
	}
	keys := orderedPayloadKeys(payload)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, formatPayloadValue(key, payload[key])))
	}
	return strings.Join(parts, " ")
}

// orderedPayloadKeys returns payload keys in a consistent, readable order.
func orderedPayloadKeys(payload map[string]string) []string {
	keys := make([]string, 0, len(payload))
	seen := make(map[string]struct{}, len(payload))
	for _, key := range payloadKeyOrder {
		if _, ok := payload[key]; ok {
			keys = append(keys, key)
			seen[key] = struct{}{}
		}
	}
	var extras []string
	for key := range payload {
		if _, ok := seen[key]; ok {
			continue
		}
		extras = append(extras, key)
	}
	sort.Strings(extras)
	return append(keys, extras...)
}

// formatPayloadValue normalizes payload values for output.
func formatPayloadValue(key, value string) string {
	if key == "priority" {
		return formatPriority(value)
	}
	if value == "" {
		return `""`
	}
	if strings.ContainsAny(value, " \t\n\"") {
		return strconv.Quote(value)
	}
	return value
}

// formatLogLine renders columns with padding and optional details.
func formatLogLine(line logLine, widths logColumnWidths) string {
	columns := []string{
		padOrTrim(line.Actor, widths.Actor),
		padOrTrim(line.ActorDate, widths.ActorDate),
		padOrTrim(line.EventTime, widths.EventTime),
		padOrTrim(line.EventType, widths.EventType),
		padOrTrim(line.IssueID, widths.IssueID),
		padOrTrim(line.IssueTitle, widths.IssueTitle),
	}
	result := strings.Join(columns, " ")
	if strings.TrimSpace(line.Details) != "" {
		result = result + " " + line.Details
	}
	return result
}

// padOrTrim truncates long values and pads short ones to width.
func padOrTrim(value string, width int) string {
	if width <= 0 {
		return value
	}
	if len(value) > width {
		if width <= 3 {
			return value[:width]
		}
		return value[:width-3] + "..."
	}
	return fmt.Sprintf("%-*s", width, value)
}

// parseOptionalTimestamp parses a timestamp string if provided.
func parseOptionalTimestamp(input string) (time.Time, bool, error) {
	if strings.TrimSpace(input) == "" {
		return time.Time{}, false, nil
	}
	parsed, err := parseLogTimestamp(input)
	if err != nil {
		return time.Time{}, false, err
	}
	return parsed, true, nil
}

// parseLogTimestamp accepts RFC3339Nano, RFC3339, or YYYY-MM-DD inputs.
func parseLogTimestamp(input string) (time.Time, error) {
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, input)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp: %s", input)
}

// printLogJSON emits a JSON line for a log entry.
func printLogJSON(entry logEntry, line logLine) error {
	payload := entry.Entry.Event.Payload
	if payload == nil {
		payload = map[string]string{}
	}
	record := logJSON{
		Line:       entry.Entry.Line,
		Timestamp:  entry.Entry.Event.Timestamp,
		Type:       entry.Entry.Event.Type,
		Label:      line.EventType,
		IssueID:    entry.Entry.Event.IssueID,
		IssueTitle: line.IssueTitle,
		Actor:      line.Actor,
		ActorDate:  line.ActorDate,
		Details:    line.Details,
		Payload:    payload,
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal log json: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// gitBlameAttributions returns blame metadata for each line in a file.
func gitBlameAttributions(root, path string) ([]gitAttribution, error) {
	relPath, err := filepath.Rel(root, path)
	if err != nil {
		relPath = path
	}
	cmd := exec.Command("git", "-C", root, "blame", "--line-porcelain", "--", relPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git blame %s: %w", relPath, err)
	}
	return parseGitBlame(output)
}

// parseGitBlame converts git blame porcelain output into line metadata.
func parseGitBlame(output []byte) ([]gitAttribution, error) {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	var attributions []gitAttribution
	var current gitAttribution
	var authorTime int64
	var authorTZ string
	for scanner.Scan() {
		line := scanner.Text()
		// Each blame record ends with the source line prefixed by a tab.
		if strings.HasPrefix(line, "\t") {
			attributions = append(attributions, finalizeAttribution(current, authorTime, authorTZ))
			current = gitAttribution{}
			authorTime = 0
			authorTZ = ""
			continue
		}
		// Capture attribution fields from the porcelain header.
		if strings.HasPrefix(line, "author ") {
			current.Author = strings.TrimPrefix(line, "author ")
			continue
		}
		if strings.HasPrefix(line, "author-time ") {
			value := strings.TrimPrefix(line, "author-time ")
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err == nil {
				authorTime = parsed
			}
			continue
		}
		if strings.HasPrefix(line, "author-tz ") {
			authorTZ = strings.TrimPrefix(line, "author-tz ")
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan git blame: %w", err)
	}
	return attributions, nil
}

// finalizeAttribution normalizes blame metadata for output.
func finalizeAttribution(base gitAttribution, authorTime int64, authorTZ string) gitAttribution {
	if base.Author == "" {
		base.Author = "unknown"
	}
	if authorTime == 0 {
		base.Date = "unknown"
		return base
	}
	zoneOffset, ok := parseGitTZ(authorTZ)
	if !ok {
		base.Date = time.Unix(authorTime, 0).UTC().Format("2006-01-02")
		return base
	}
	location := time.FixedZone("git", zoneOffset)
	base.Date = time.Unix(authorTime, 0).In(location).Format("2006-01-02")
	return base
}

// parseGitTZ parses a git timezone offset like -0700 into seconds.
func parseGitTZ(value string) (int, bool) {
	if len(value) != 5 {
		return 0, false
	}
	sign := value[0]
	if sign != '+' && sign != '-' {
		return 0, false
	}
	// Parse hours and minutes from the offset.
	hours, err := strconv.Atoi(value[1:3])
	if err != nil {
		return 0, false
	}
	minutes, err := strconv.Atoi(value[3:5])
	if err != nil {
		return 0, false
	}
	// Convert to seconds, honoring the sign.
	offset := (hours * 60 * 60) + (minutes * 60)
	if sign == '-' {
		offset = -offset
	}
	return offset, true
}

// attributionForLine returns blame metadata for a line or defaults.
func attributionForLine(attributions []gitAttribution, line int) gitAttribution {
	if line <= 0 || line > len(attributions) {
		return gitAttribution{Author: "unknown", Date: "unknown"}
	}
	attribution := attributions[line-1]
	if attribution.Author == "" {
		attribution.Author = "unknown"
	}
	if attribution.Date == "" {
		attribution.Date = "unknown"
	}
	return attribution
}
