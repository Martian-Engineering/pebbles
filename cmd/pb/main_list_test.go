package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"pebbles/internal/pebbles"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}

	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	outCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outCh <- buf.String()
	}()

	fn()

	_ = w.Close()
	out := <-outCh
	_ = r.Close()
	return out
}

func setupListProject(t *testing.T) (root, openID, inProgressID, closedID string) {
	t.Helper()

	root = t.TempDir()
	if err := pebbles.InitProject(root); err != nil {
		t.Fatalf("init project: %v", err)
	}

	openID = "pb-open"
	inProgressID = "pb-progress"
	closedID = "pb-closed"

	if err := pebbles.AppendEvent(root, pebbles.NewCreateEvent(openID, "Open", "", "task", "2024-01-01T00:00:00Z", 2)); err != nil {
		t.Fatalf("append open create: %v", err)
	}
	if err := pebbles.AppendEvent(root, pebbles.NewCreateEvent(inProgressID, "In Progress", "", "task", "2024-01-01T00:10:00Z", 2)); err != nil {
		t.Fatalf("append in_progress create: %v", err)
	}
	if err := pebbles.AppendEvent(root, pebbles.NewStatusEvent(inProgressID, "in_progress", "2024-01-01T00:20:00Z")); err != nil {
		t.Fatalf("append in_progress status: %v", err)
	}
	if err := pebbles.AppendEvent(root, pebbles.NewCreateEvent(closedID, "Closed", "", "task", "2024-01-01T00:30:00Z", 2)); err != nil {
		t.Fatalf("append closed create: %v", err)
	}
	if err := pebbles.AppendEvent(root, pebbles.NewCloseEvent(closedID, "2024-01-01T00:40:00Z")); err != nil {
		t.Fatalf("append closed event: %v", err)
	}
	if err := pebbles.RebuildCache(root); err != nil {
		t.Fatalf("rebuild cache: %v", err)
	}

	return root, openID, inProgressID, closedID
}

func TestListDefaultsToNonClosed(t *testing.T) {
	root, openID, inProgressID, closedID := setupListProject(t)

	out := captureStdout(t, func() {
		runList(root, nil)
	})

	if strings.Contains(out, closedID) {
		t.Fatalf("expected default list output to hide closed issues; output=%q", out)
	}
	if !strings.Contains(out, openID) {
		t.Fatalf("expected default list output to include open issue %s; output=%q", openID, out)
	}
	if !strings.Contains(out, inProgressID) {
		t.Fatalf("expected default list output to include in_progress issue %s; output=%q", inProgressID, out)
	}
}

func TestListAllShowsClosed(t *testing.T) {
	root, _, _, closedID := setupListProject(t)

	out := captureStdout(t, func() {
		runList(root, []string{"--all"})
	})

	if !strings.Contains(out, closedID) {
		t.Fatalf("expected --all list output to include closed issue %s; output=%q", closedID, out)
	}
}

func TestListStatusFilterStillWorks(t *testing.T) {
	root, openID, inProgressID, closedID := setupListProject(t)

	out := captureStdout(t, func() {
		runList(root, []string{"--status", "closed"})
	})

	if !strings.Contains(out, closedID) {
		t.Fatalf("expected --status closed output to include %s; output=%q", closedID, out)
	}
	if strings.Contains(out, openID) || strings.Contains(out, inProgressID) {
		t.Fatalf("expected --status closed output to exclude non-closed issues; output=%q", out)
	}
}

func TestListJSONRespectsDefaultAndAll(t *testing.T) {
	root, openID, inProgressID, closedID := setupListProject(t)

	type listItem struct {
		ID string `json:"id"`
	}

	out := captureStdout(t, func() {
		runList(root, []string{"--json"})
	})

	var got []listItem
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal json: %v; output=%q", err, out)
	}
	ids := make(map[string]bool, len(got))
	for _, item := range got {
		ids[item.ID] = true
	}
	if !ids[openID] || !ids[inProgressID] {
		t.Fatalf("expected default --json output to include open + in_progress; ids=%v output=%q", ids, out)
	}
	if ids[closedID] {
		t.Fatalf("expected default --json output to hide closed; ids=%v output=%q", ids, out)
	}

	outAll := captureStdout(t, func() {
		runList(root, []string{"--json", "--all"})
	})
	got = nil
	if err := json.Unmarshal([]byte(outAll), &got); err != nil {
		t.Fatalf("unmarshal json (--all): %v; output=%q", err, outAll)
	}
	ids = make(map[string]bool, len(got))
	for _, item := range got {
		ids[item.ID] = true
	}
	if !ids[closedID] {
		t.Fatalf("expected --json --all output to include closed; ids=%v output=%q", ids, outAll)
	}
}

