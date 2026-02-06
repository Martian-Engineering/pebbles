package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
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
	payloadKeyOrder        = []string{"title", "description", "body", "type", "priority", "status", "depends_on", "dep_type"}
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

// logDetailSections splits detail lines from description/body text.
type logDetailSections struct {
	Lines       []string
	Description string
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

// enrichEvent overlays issue metadata needed for log output.
func enrichEvent(event pebbles.Event, descriptions map[string]string) pebbles.Event {
	if event.Type != pebbles.EventTypeClose {
		return event
	}
	if event.Payload == nil {
		event.Payload = map[string]string{}
	}
	if event.Payload["description"] == "" {
		if description := descriptionForIssue(descriptions, event.IssueID); description != "" {
			event.Payload["description"] = description
		}
	}
	return event
}

// runLog handles pb log.
func runLog(root string, args []string) {
	fs := flag.NewFlagSet("log", flag.ExitOnError)
	setFlagUsage(fs, logHelp)
	var limit int
	fs.IntVar(&limit, "limit", 0, "Limit number of events")
	fs.IntVar(&limit, "n", 0, "Alias for --limit")
	sinceInput := fs.String("since", "", "Only show events on or after timestamp")
	untilInput := fs.String("until", "", "Only show events on or before timestamp")
	noGit := fs.Bool("no-git", false, "Skip git blame attribution")
	table := fs.Bool("table", false, "Use table output")
	noPager := fs.Bool("no-pager", false, "Disable pager")
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
	descriptions, err := issueDescriptionMap(root)
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
	// JSON output is streamed directly to stdout (no pager).
	if *jsonOut {
		for _, entry := range filtered {
			attribution := attributionForLine(attributions, entry.Entry.Line)
			event := enrichEvent(entry.Entry.Event, descriptions)
			line := logLine{
				Actor:      attribution.Author,
				ActorDate:  attribution.Date,
				EventTime:  formatEventTime(entry),
				EventType:  logEventLabel(event),
				IssueID:    event.IssueID,
				IssueTitle: titleForIssue(titles, event.IssueID),
				Details:    logEventDetails(event),
			}
			if err := printLogJSON(entry, line); err != nil {
				exitError(err)
			}
		}
		return
	}
	// Build formatted output before writing to a pager or stdout.
	var output strings.Builder
	for index, entry := range filtered {
		attribution := attributionForLine(attributions, entry.Entry.Line)
		event := enrichEvent(entry.Entry.Event, descriptions)
		line := logLine{
			Actor:      attribution.Author,
			ActorDate:  attribution.Date,
			EventTime:  formatEventTime(entry),
			EventType:  logEventLabel(event),
			IssueID:    event.IssueID,
			IssueTitle: titleForIssue(titles, event.IssueID),
			Details:    logEventDetails(event),
		}
		// Render the selected view for each entry.
		if *table {
			output.WriteString(formatLogLine(line, defaultLogColumnWidths))
			output.WriteString("\n")
			continue
		}
		output.WriteString(formatPrettyLog(entry, line))
		if index < len(filtered)-1 {
			output.WriteString("\n\n")
		} else {
			output.WriteString("\n")
		}
	}
	// Decide whether to use a pager and write output.
	usePager := shouldUsePager(*noPager, isTTY(os.Stdout))
	if err := writeLogOutput(output.String(), usePager); err != nil {
		exitError(err)
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

// issueDescriptionMap builds a map of issue IDs to descriptions for log output.
func issueDescriptionMap(root string) (map[string]string, error) {
	issues, err := pebbles.ListIssues(root)
	if err != nil {
		return nil, err
	}
	descriptions := make(map[string]string, len(issues))
	for _, issue := range issues {
		descriptions[issue.ID] = issue.Description
	}
	return descriptions, nil
}

// titleForIssue returns the title for an issue ID or "unknown".
func titleForIssue(titles map[string]string, issueID string) string {
	title := titles[issueID]
	if title == "" {
		return "unknown"
	}
	return title
}

// descriptionForIssue returns the description for an issue ID when present.
func descriptionForIssue(descriptions map[string]string, issueID string) string {
	return descriptions[issueID]
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
	case pebbles.EventTypeTitleUpdated:
		return "title"
	case pebbles.EventTypeStatus:
		return "status"
	case pebbles.EventTypeUpdate:
		return "update"
	case pebbles.EventTypeClose:
		return "close"
	case pebbles.EventTypeComment:
		return "comment"
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
		parts := make([]string, 0, 3)
		if issueType := event.Payload["type"]; issueType != "" {
			parts = append(parts, fmt.Sprintf("type=%s", issueType))
		}
		if priority := event.Payload["priority"]; priority != "" {
			parts = append(parts, fmt.Sprintf("priority=%s", formatPriority(priority)))
		}
		if description := event.Payload["description"]; description != "" {
			parts = append(parts, fmt.Sprintf("description=%s", formatPayloadValue("description", description)))
		}
		return strings.Join(parts, " ")
	case pebbles.EventTypeTitleUpdated:
		if title := event.Payload["title"]; title != "" {
			return fmt.Sprintf("title=%s", formatPayloadValue("title", title))
		}
	case pebbles.EventTypeStatus:
		if status := event.Payload["status"]; status != "" {
			return fmt.Sprintf("status=%s", status)
		}
	case pebbles.EventTypeUpdate:
		parts := make([]string, 0, 3)
		if issueType := event.Payload["type"]; issueType != "" {
			parts = append(parts, fmt.Sprintf("type=%s", issueType))
		}
		if priority := event.Payload["priority"]; priority != "" {
			parts = append(parts, fmt.Sprintf("priority=%s", formatPriority(priority)))
		}
		if description := event.Payload["description"]; description != "" {
			parts = append(parts, fmt.Sprintf("description=%s", formatPayloadValue("description", description)))
		}
		return strings.Join(parts, " ")
	case pebbles.EventTypeClose:
		if description := event.Payload["description"]; description != "" {
			return fmt.Sprintf("description=%s", formatPayloadValue("description", description))
		}
	case pebbles.EventTypeComment:
		if body := event.Payload["body"]; body != "" {
			return fmt.Sprintf("body=%s", formatPayloadValue("body", body))
		}
	case pebbles.EventTypeDepAdd, pebbles.EventTypeDepRemove:
		parts := make([]string, 0, 2)
		if dependsOn := event.Payload["depends_on"]; dependsOn != "" {
			parts = append(parts, fmt.Sprintf("depends_on=%s", dependsOn))
		}
		if depType := strings.TrimSpace(event.Payload["dep_type"]); depType != "" {
			normalized := pebbles.NormalizeDepType(depType)
			if normalized != pebbles.DepTypeBlocks {
				parts = append(parts, fmt.Sprintf("dep_type=%s", normalized))
			}
		}
		return strings.Join(parts, " ")
	default:
		return formatPayloadPairs(event.Payload)
	}
	return ""
}

// logEventDetailSections returns payload detail lines and description/body text.
func logEventDetailSections(event pebbles.Event) logDetailSections {
	switch event.Type {
	case pebbles.EventTypeCreate:
		// Limit create output to the fields that matter in logs.
		var lines []string
		if issueType := event.Payload["type"]; issueType != "" {
			lines = append(lines, fmt.Sprintf("type=%s", issueType))
		}
		if priority := event.Payload["priority"]; priority != "" {
			lines = append(lines, fmt.Sprintf("priority=%s", formatPriority(priority)))
		}
		return logDetailSections{
			Lines:       lines,
			Description: logDetailDescription(event.Payload["description"]),
		}
	case pebbles.EventTypeTitleUpdated:
		if title := event.Payload["title"]; title != "" {
			return logDetailSections{Lines: []string{fmt.Sprintf("title=%s", formatPayloadValue("title", title))}}
		}
	case pebbles.EventTypeStatus:
		if status := event.Payload["status"]; status != "" {
			return logDetailSections{Lines: []string{fmt.Sprintf("status=%s", status)}}
		}
	case pebbles.EventTypeUpdate:
		var lines []string
		if issueType := event.Payload["type"]; issueType != "" {
			lines = append(lines, fmt.Sprintf("type=%s", issueType))
		}
		if priority := event.Payload["priority"]; priority != "" {
			lines = append(lines, fmt.Sprintf("priority=%s", formatPriority(priority)))
		}
		return logDetailSections{
			Lines:       lines,
			Description: logDetailDescription(event.Payload["description"]),
		}
	case pebbles.EventTypeClose:
		return logDetailSections{Description: logDetailDescription(event.Payload["description"])}
	case pebbles.EventTypeComment:
		return logDetailSections{Description: logDetailDescription(event.Payload["body"])}
	case pebbles.EventTypeDepAdd, pebbles.EventTypeDepRemove:
		var lines []string
		if dependsOn := event.Payload["depends_on"]; dependsOn != "" {
			lines = append(lines, fmt.Sprintf("depends_on=%s", dependsOn))
		}
		if depType := strings.TrimSpace(event.Payload["dep_type"]); depType != "" {
			normalized := pebbles.NormalizeDepType(depType)
			if normalized != pebbles.DepTypeBlocks {
				lines = append(lines, fmt.Sprintf("dep_type=%s", normalized))
			}
		}
		return logDetailSections{Lines: lines}
	default:
		return logDetailSections{Lines: formatPayloadLines(event.Payload)}
	}
	return logDetailSections{}
}

// logDetailDescription normalizes description/body text for log rendering.
func logDetailDescription(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return value
}

// formatPriority normalizes priority payload values as P0-P4 when possible.
func formatPriority(value string) string {
	parsed, err := pebbles.ParsePriority(value)
	if err != nil {
		return value
	}
	return pebbles.PriorityLabel(parsed)
}

// formatPayloadLines formats payload key/value pairs as individual lines.
func formatPayloadLines(payload map[string]string) []string {
	if len(payload) == 0 {
		return nil
	}
	keys := orderedPayloadKeys(payload)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", key, formatPayloadValue(key, payload[key])))
	}
	return lines
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

// logEventTypeColor returns the ANSI color for a log event label.
func logEventTypeColor(eventType string) string {
	// Keep event colors distinct so each log entry is easy to scan.
	switch strings.ToLower(eventType) {
	case "create":
		return ansiBrightGreen
	case "status":
		return ansiBrightYellow
	case "update":
		return ansiBrightBlue
	case "close":
		return ansiBrightMagenta
	case "comment":
		return ansiBrightCyan
	case "dep_add":
		return ansiBrightWhite
	case "dep_rm":
		return ansiBrightRed
	default:
		return ansiBrightWhite
	}
}

// renderLogEventType returns a colored event label when enabled.
func renderLogEventType(eventType string) string {
	return colorize(eventType, ansiBold+logEventTypeColor(eventType))
}

// renderLogHeaderLabel applies high-contrast styling to the log header label.
func renderLogHeaderLabel(label string) string {
	return colorize(label, ansiBold+ansiBrightYellow)
}

// renderLogLineNumber returns a colored line number for log output.
func renderLogLineNumber(line int) string {
	return colorize(strconv.Itoa(line), ansiBold+ansiBrightWhite)
}

// renderLogLabel applies high-contrast styling to log field labels.
func renderLogLabel(label string) string {
	return colorize(label, ansiBold+ansiBrightBlue)
}

// renderLogValue applies high-contrast styling to log field values.
func renderLogValue(value string) string {
	return colorize(value, ansiBrightWhite)
}

// renderLogIssueID returns a colored issue ID for log output.
func renderLogIssueID(issueID string) string {
	return colorize(issueID, ansiBold+ansiBrightMagenta)
}

// renderLogDetail formats a log detail key/value with ANSI styling.
func renderLogDetail(detail string) string {
	parts := strings.SplitN(detail, "=", 2)
	if len(parts) != 2 {
		return detail
	}
	key := parts[0]
	value := parts[1]
	return renderLogLabel(key) + "=" + renderLogDetailValue(key, value)
}

// renderLogDetailValue returns a colored detail value when enabled.
func renderLogDetailValue(key, value string) string {
	// Use existing list/show color rules for known detail values.
	switch key {
	case "status":
		return renderStatusValue(value)
	case "priority":
		priority, err := pebbles.ParsePriority(value)
		if err != nil {
			return value
		}
		return renderPriorityLabel(priority)
	case "type":
		return renderIssueType(value)
	case "depends_on":
		return renderLogIssueID(value)
	default:
		return renderLogValue(value)
	}
}

// formatPrettyLog renders a multi-line log entry for humans.
func formatPrettyLog(entry logEntry, line logLine) string {
	var output strings.Builder
	// Header line includes the log line number, event type, and issue id.
	output.WriteString(fmt.Sprintf("%s %s %s %s\n", renderLogHeaderLabel("event"), renderLogLineNumber(entry.Entry.Line), renderLogEventType(line.EventType), renderLogIssueID(line.IssueID)))
	// Add core metadata lines with aligned labels.
	output.WriteString(fmt.Sprintf("%s %s\n", renderLogLabel("Title:"), colorize(line.IssueTitle, ansiBold+ansiBrightWhite)))
	output.WriteString(fmt.Sprintf("%s  %s\n", renderLogLabel("When:"), renderLogValue(line.EventTime)))
	output.WriteString(fmt.Sprintf("%s %s (%s)\n", renderLogLabel("Actor:"), renderLogValue(line.Actor), renderLogValue(line.ActorDate)))
	// Render payload details with indentation or an explicit none marker.
	details := logEventDetailSections(entry.Entry.Event)
	if len(details.Lines) == 0 && details.Description == "" {
		output.WriteString(fmt.Sprintf("%s %s", renderLogLabel("Details:"), renderLogDetailValue("details", "(none)")))
		return output.String()
	}
	output.WriteString(fmt.Sprintf("%s\n", renderLogLabel("Details:")))
	for index, detail := range details.Lines {
		output.WriteString("  ")
		output.WriteString(renderLogDetail(detail))
		if index < len(details.Lines)-1 {
			output.WriteString("\n")
		}
	}
	if details.Description != "" {
		if len(details.Lines) > 0 {
			output.WriteString("\n\n")
		} else {
			output.WriteString("\n")
		}
		rendered := renderMarkdown(details.Description)
		descriptionLines := strings.Split(rendered, "\n")
		for index, line := range descriptionLines {
			output.WriteString("  ")
			output.WriteString(line)
			if index < len(descriptionLines)-1 {
				output.WriteString("\n")
			}
		}
	}
	return output.String()
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

// isTTY reports whether a file handle refers to a terminal.
func isTTY(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// shouldUsePager decides whether to route output through a pager.
func shouldUsePager(noPager bool, tty bool) bool {
	if noPager {
		return false
	}
	return tty
}

// resolvePagerCommand returns the pager command to execute.
func resolvePagerCommand() []string {
	if value := strings.TrimSpace(os.Getenv("PB_PAGER")); value != "" {
		return strings.Fields(value)
	}
	if value := strings.TrimSpace(os.Getenv("PAGER")); value != "" {
		return strings.Fields(value)
	}
	return []string{"less", "-FRX"}
}

// writeLogOutput writes output to stdout or through a pager when requested.
func writeLogOutput(output string, usePager bool) error {
	if !usePager {
		_, err := fmt.Fprint(os.Stdout, output)
		return err
	}
	pager := resolvePagerCommand()
	if len(pager) == 0 {
		_, err := fmt.Fprint(os.Stdout, output)
		return err
	}
	// Pipe the full output to the pager's stdin.
	cmd := exec.Command(pager[0], pager[1:]...)
	cmd.Stdin = strings.NewReader(output)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		_, _ = fmt.Fprint(os.Stdout, output)
	}
	return nil
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
