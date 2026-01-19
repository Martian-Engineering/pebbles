package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"pebbles/internal/pebbles"
)

// main dispatches pb subcommands.
func main() {
	root, err := os.Getwd()
	if err != nil {
		exitError(fmt.Errorf("get working directory: %w", err))
	}
	// Validate the CLI entrypoint arguments before dispatching.
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	// Route to the subcommand handler.
	switch cmd {
	case "init":
		runInit(root, args)
	case "create":
		runCreate(root, args)
	case "list":
		runList(root, args)
	case "show":
		runShow(root, args)
	case "update":
		runUpdate(root, args)
	case "close":
		runClose(root, args)
	case "dep":
		runDep(root, args)
	case "ready":
		runReady(root, args)
	default:
		printUsage()
		os.Exit(1)
	}
}

// runInit handles pb init.
func runInit(root string, args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	_ = fs.Parse(args)
	if err := pebbles.InitProject(root); err != nil {
		exitError(err)
	}
	fmt.Println("Initialized .pebbles")
}

// runCreate handles pb create.
func runCreate(root string, args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	title := fs.String("title", "", "Issue title")
	issueType := fs.String("type", "task", "Issue type")
	_ = fs.Parse(args)
	// Ensure the project is initialized and inputs are present.
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	if strings.TrimSpace(*title) == "" {
		exitError(fmt.Errorf("title is required"))
	}
	// Load configuration and derive a deterministic issue ID.
	cfg, err := pebbles.LoadConfig(root)
	if err != nil {
		exitError(err)
	}
	timestamp := pebbles.NowTimestamp()
	issueID := pebbles.GenerateIssueID(cfg.Prefix, *title, timestamp, pebbles.HostLabel())
	event := pebbles.NewCreateEvent(issueID, *title, *issueType, timestamp)
	// Append to the event log, then rebuild the cache for reads.
	if err := pebbles.AppendEvent(root, event); err != nil {
		exitError(err)
	}
	if err := pebbles.RebuildCache(root); err != nil {
		exitError(err)
	}
	fmt.Println(issueID)
}

// runList handles pb list.
func runList(root string, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	_ = fs.Parse(args)
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	issues, err := pebbles.ListIssues(root)
	if err != nil {
		exitError(err)
	}
	for _, issue := range issues {
		fmt.Printf("%s\t%s\t%s\n", issue.ID, issue.Status, issue.Title)
	}
}

// runShow handles pb show.
func runShow(root string, args []string) {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	_ = fs.Parse(args)
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	// Validate CLI arguments and load the issue state.
	if fs.NArg() != 1 {
		exitError(fmt.Errorf("show requires issue id"))
	}
	id := fs.Arg(0)
	issue, deps, err := pebbles.GetIssue(root, id)
	if err != nil {
		exitError(err)
	}
	printIssue(issue, deps)
}

// runUpdate handles pb update.
func runUpdate(root string, args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	status := fs.String("status", "", "New status")
	// Support `pb update <id> --status ...` by moving the id to the end.
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append(args[1:], args[0])
	}
	_ = fs.Parse(args)
	// Validate inputs before writing an update event.
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	if fs.NArg() != 1 {
		exitError(fmt.Errorf("update requires issue id"))
	}
	if strings.TrimSpace(*status) == "" {
		exitError(fmt.Errorf("status is required"))
	}
	id := fs.Arg(0)
	// Confirm the issue exists in the cache.
	if _, _, err := pebbles.GetIssue(root, id); err != nil {
		exitError(err)
	}
	event := pebbles.NewStatusEvent(id, *status, pebbles.NowTimestamp())
	// Append the event and rebuild the cache for consistency.
	if err := pebbles.AppendEvent(root, event); err != nil {
		exitError(err)
	}
	if err := pebbles.RebuildCache(root); err != nil {
		exitError(err)
	}
}

// runClose handles pb close.
func runClose(root string, args []string) {
	fs := flag.NewFlagSet("close", flag.ExitOnError)
	_ = fs.Parse(args)
	// Validate inputs before closing the issue.
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	if fs.NArg() != 1 {
		exitError(fmt.Errorf("close requires issue id"))
	}
	id := fs.Arg(0)
	// Confirm the issue exists in the cache.
	if _, _, err := pebbles.GetIssue(root, id); err != nil {
		exitError(err)
	}
	event := pebbles.NewCloseEvent(id, pebbles.NowTimestamp())
	// Append the close event and rebuild the cache.
	if err := pebbles.AppendEvent(root, event); err != nil {
		exitError(err)
	}
	if err := pebbles.RebuildCache(root); err != nil {
		exitError(err)
	}
}

// runDep handles pb dep add.
func runDep(root string, args []string) {
	fs := flag.NewFlagSet("dep", flag.ExitOnError)
	_ = fs.Parse(args)
	// Validate CLI arguments for dependency creation.
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	if fs.NArg() != 3 || fs.Arg(0) != "add" {
		exitError(fmt.Errorf("usage: pb dep add <issue> <depends-on>"))
	}
	issueID := fs.Arg(1)
	dependsOn := fs.Arg(2)
	// Ensure both sides exist before appending the event.
	if _, _, err := pebbles.GetIssue(root, issueID); err != nil {
		exitError(err)
	}
	if _, _, err := pebbles.GetIssue(root, dependsOn); err != nil {
		exitError(err)
	}
	event := pebbles.NewDepAddEvent(issueID, dependsOn, pebbles.NowTimestamp())
	// Append the event and rebuild the cache.
	if err := pebbles.AppendEvent(root, event); err != nil {
		exitError(err)
	}
	if err := pebbles.RebuildCache(root); err != nil {
		exitError(err)
	}
}

// runReady handles pb ready.
func runReady(root string, args []string) {
	fs := flag.NewFlagSet("ready", flag.ExitOnError)
	_ = fs.Parse(args)
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	issues, err := pebbles.ListReadyIssues(root)
	if err != nil {
		exitError(err)
	}
	for _, issue := range issues {
		fmt.Printf("%s\t%s\t%s\n", issue.ID, issue.Status, issue.Title)
	}
}

// ensureProject checks that the .pebbles directory exists.
func ensureProject(root string) error {
	if _, err := os.Stat(pebbles.EventsPath(root)); err != nil {
		return fmt.Errorf("pebbles not initialized; run pb init")
	}
	return nil
}

// printIssue renders a single issue to stdout.
func printIssue(issue pebbles.Issue, deps []string) {
	// Print core issue fields first.
	fmt.Printf("ID: %s\n", issue.ID)
	fmt.Printf("Title: %s\n", issue.Title)
	fmt.Printf("Type: %s\n", issue.IssueType)
	fmt.Printf("Status: %s\n", issue.Status)
	fmt.Printf("Created: %s\n", issue.CreatedAt)
	fmt.Printf("Updated: %s\n", issue.UpdatedAt)
	if issue.ClosedAt != "" {
		fmt.Printf("Closed: %s\n", issue.ClosedAt)
	}
	// Print dependencies after the main issue fields.
	if len(deps) == 0 {
		fmt.Println("Dependencies: none")
		return
	}
	fmt.Println("Dependencies:")
	for _, dep := range deps {
		fmt.Printf("- %s\n", dep)
	}
}

// printUsage prints a brief usage message.
func printUsage() {
	fmt.Println("pb <command> [args]")
	fmt.Println("Commands: init, create, list, show, update, close, dep, ready")
}

// exitError prints an error to stderr and exits.
func exitError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}
