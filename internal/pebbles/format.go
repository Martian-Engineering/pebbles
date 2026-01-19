package pebbles

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// DefaultPriority is the fallback priority when none is provided.
	DefaultPriority = 2
)

// ParsePriority converts a priority string (P0-P4 or 0-4) into an int.
func ParsePriority(input string) (int, error) {
	trimmed := strings.TrimSpace(strings.ToUpper(input))
	if trimmed == "" {
		return DefaultPriority, nil
	}
	if strings.HasPrefix(trimmed, "P") {
		trimmed = strings.TrimPrefix(trimmed, "P")
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid priority: %s", input)
	}
	if value < 0 || value > 4 {
		return 0, fmt.Errorf("priority must be P0-P4")
	}
	return value, nil
}

// PriorityLabel formats a priority integer as P0-P4.
func PriorityLabel(priority int) string {
	if priority < 0 || priority > 4 {
		priority = DefaultPriority
	}
	return fmt.Sprintf("P%d", priority)
}

// StatusIcon returns the display icon for a status.
func StatusIcon(status string) string {
	switch status {
	case StatusOpen:
		return "○"
	case StatusInProgress:
		return "◐"
	case StatusClosed:
		return "●"
	default:
		return "○"
	}
}

// StatusLabel formats a status for display in headers.
func StatusLabel(status string) string {
	switch status {
	case StatusOpen:
		return "OPEN"
	case StatusInProgress:
		return "IN_PROGRESS"
	case StatusClosed:
		return "CLOSED"
	default:
		return strings.ToUpper(status)
	}
}

// parsePriority defaults missing or invalid values to DefaultPriority.
func parsePriority(input string) int {
	priority, err := ParsePriority(input)
	if err != nil {
		return DefaultPriority
	}
	return priority
}
