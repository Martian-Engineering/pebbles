package main

import (
	"os"
	"strings"

	"pebbles/internal/pebbles"
)

const (
	ansiReset         = "\x1b[0m"
	ansiBold          = "\x1b[1m"
	ansiDim           = "\x1b[2m"
	ansiRed           = "\x1b[31m"
	ansiYellow        = "\x1b[33m"
	ansiBlue          = "\x1b[34m"
	ansiMagenta       = "\x1b[35m"
	ansiCyan          = "\x1b[36m"
	ansiGray          = "\x1b[90m"
	ansiBrightRed     = "\x1b[91m"
	ansiBrightGreen   = "\x1b[92m"
	ansiBrightYellow  = "\x1b[93m"
	ansiBrightBlue    = "\x1b[94m"
	ansiBrightMagenta = "\x1b[95m"
	ansiBrightCyan    = "\x1b[96m"
	ansiBrightWhite   = "\x1b[97m"
)

var colorEnabled = shouldColor()

// shouldColor reports whether ANSI color output should be enabled.
func shouldColor() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("PB_NO_COLOR") != "" {
		return false
	}
	if strings.TrimSpace(os.Getenv("CLICOLOR")) == "0" {
		return false
	}
	return isTTY(os.Stdout)
}

// colorize wraps text with ANSI color codes when enabled.
func colorize(text, code string) string {
	if !colorEnabled || code == "" {
		return text
	}
	return code + text + ansiReset
}

// statusColor returns the ANSI color for a status value.
func statusColor(status string) string {
	switch strings.ToLower(status) {
	case pebbles.StatusInProgress:
		return ansiBrightYellow
	case pebbles.StatusClosed:
		return ansiBrightGreen
	default:
		return ansiBrightWhite
	}
}

// priorityColor returns the ANSI color for a priority value.
func priorityColor(priority int) string {
	// Map priorities to attention-grabbing colors.
	switch priority {
	case 0:
		return ansiBold + ansiBrightRed
	case 1:
		return ansiBrightMagenta
	case 2:
		return ansiBrightYellow
	case 3:
		return ansiBrightBlue
	case 4:
		return ansiBrightCyan
	default:
		return ansiBrightWhite
	}
}

// typeColor returns the ANSI color for an issue type.
func typeColor(issueType string) string {
	// Highlight issue types that carry extra urgency or scope.
	switch strings.ToLower(issueType) {
	case "bug":
		return ansiBrightRed
	case "epic":
		return ansiBrightMagenta
	case "feature":
		return ansiBrightBlue
	case "chore":
		return ansiBrightCyan
	default:
		return ansiBrightWhite
	}
}

// renderStatusIcon returns a colored status icon when enabled.
func renderStatusIcon(status string) string {
	icon := pebbles.StatusIcon(status)
	return colorize(icon, statusColor(status))
}

// renderStatusLabel returns a colored status label when enabled.
func renderStatusLabel(status string) string {
	label := pebbles.StatusLabel(status)
	return colorize(label, statusColor(status))
}

// renderStatusValue returns a colored status string when enabled.
func renderStatusValue(status string) string {
	return colorize(status, statusColor(status))
}

// renderPriorityLabel returns a colored priority label when enabled.
func renderPriorityLabel(priority int) string {
	label := pebbles.PriorityLabel(priority)
	return colorize(label, priorityColor(priority))
}

// renderIssueType returns a colored issue type when enabled.
func renderIssueType(issueType string) string {
	return colorize(issueType, typeColor(issueType))
}
