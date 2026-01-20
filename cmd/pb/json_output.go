package main

import (
	"encoding/json"
	"fmt"

	"pebbles/internal/pebbles"
)

// issueJSON describes the JSON payload for list/ready issue output.
type issueJSON struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	IssueType   string   `json:"type"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	ClosedAt    string   `json:"closed_at"`
	Deps        []string `json:"deps"`
}

// issueCommentJSON represents a single comment entry in JSON output.
type issueCommentJSON struct {
	Body      string `json:"body"`
	Timestamp string `json:"timestamp"`
}

// issueDetailJSON describes the JSON payload for pb show output.
type issueDetailJSON struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	IssueType   string             `json:"type"`
	Status      string             `json:"status"`
	Priority    string             `json:"priority"`
	CreatedAt   string             `json:"created_at"`
	UpdatedAt   string             `json:"updated_at"`
	ClosedAt    string             `json:"closed_at"`
	Deps        []string           `json:"deps"`
	Comments    []issueCommentJSON `json:"comments"`
}

// buildIssueJSON converts an issue and its deps into the list/ready JSON shape.
func buildIssueJSON(issue pebbles.Issue, deps []string) issueJSON {
	// Ensure deps always encodes as an array instead of null.
	if deps == nil {
		deps = []string{}
	}
	return issueJSON{
		ID:          issue.ID,
		Title:       issue.Title,
		Description: issue.Description,
		IssueType:   issue.IssueType,
		Status:      issue.Status,
		Priority:    pebbles.PriorityLabel(issue.Priority),
		CreatedAt:   issue.CreatedAt,
		UpdatedAt:   issue.UpdatedAt,
		ClosedAt:    issue.ClosedAt,
		Deps:        deps,
	}
}

// buildIssueDetailJSON converts an issue, deps, and comments into show output.
func buildIssueDetailJSON(issue pebbles.Issue, deps []string, comments []pebbles.IssueComment) issueDetailJSON {
	// Mirror the list/ready fields and attach the full comment history.
	if deps == nil {
		deps = []string{}
	}
	return issueDetailJSON{
		ID:          issue.ID,
		Title:       issue.Title,
		Description: issue.Description,
		IssueType:   issue.IssueType,
		Status:      issue.Status,
		Priority:    pebbles.PriorityLabel(issue.Priority),
		CreatedAt:   issue.CreatedAt,
		UpdatedAt:   issue.UpdatedAt,
		ClosedAt:    issue.ClosedAt,
		Deps:        deps,
		Comments:    buildIssueCommentsJSON(comments),
	}
}

// buildIssueCommentsJSON converts issue comments to JSON-friendly structs.
func buildIssueCommentsJSON(comments []pebbles.IssueComment) []issueCommentJSON {
	if len(comments) == 0 {
		return []issueCommentJSON{}
	}
	converted := make([]issueCommentJSON, 0, len(comments))
	for _, comment := range comments {
		converted = append(converted, issueCommentJSON{
			Body:      comment.Body,
			Timestamp: comment.Timestamp,
		})
	}
	return converted
}

// issueJSONWithDeps loads deps for an issue and returns JSON-ready fields.
func issueJSONWithDeps(root string, issue pebbles.Issue) (issueJSON, error) {
	_, deps, err := pebbles.GetIssue(root, issue.ID)
	if err != nil {
		return issueJSON{}, fmt.Errorf("get deps for %s: %w", issue.ID, err)
	}
	return buildIssueJSON(issue, deps), nil
}

// printJSON marshals the provided payload and writes it to stdout.
func printJSON(payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
