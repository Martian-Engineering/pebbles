package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

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
	case "help":
		printUsage()
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
	description := fs.String("description", "", "Issue description")
	issueType := fs.String("type", "task", "Issue type")
	priority := fs.String("priority", "P2", "Issue priority (P0-P4)")
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
	parsedPriority, err := pebbles.ParsePriority(*priority)
	if err != nil {
		exitError(err)
	}
	timestamp := pebbles.NowTimestamp()
	issueID := pebbles.GenerateIssueID(cfg.Prefix, *title, timestamp, pebbles.HostLabel())
	event := pebbles.NewCreateEvent(issueID, *title, *description, *issueType, timestamp, parsedPriority)
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
		fmt.Println(formatIssueLine(issue))
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
	printIssue(root, issue, deps)
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

// runDep handles pb dep commands.
func runDep(root string, args []string) {
	fs := flag.NewFlagSet("dep", flag.ExitOnError)
	_ = fs.Parse(args)
	// Validate CLI arguments for dependency creation.
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	if fs.NArg() < 1 {
		exitError(fmt.Errorf("usage: pb dep <add|rm|tree> [args]"))
	}
	// Route subcommands for dependency operations.
	action := fs.Arg(0)
	switch action {
	case "add":
		if fs.NArg() != 3 {
			exitError(fmt.Errorf("usage: pb dep add <issue> <depends-on>"))
		}
		runDepAdd(root, fs.Arg(1), fs.Arg(2))
	case "rm":
		if fs.NArg() != 3 {
			exitError(fmt.Errorf("usage: pb dep rm <issue> <depends-on>"))
		}
		runDepRemove(root, fs.Arg(1), fs.Arg(2))
	case "tree":
		if fs.NArg() != 2 {
			exitError(fmt.Errorf("usage: pb dep tree <issue>"))
		}
		runDepTree(root, fs.Arg(1))
	default:
		exitError(fmt.Errorf("usage: pb dep <add|rm|tree> [args]"))
	}
}

// runDepAdd appends a dependency add event.
func runDepAdd(root, issueID, dependsOn string) {
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

// runDepRemove appends a dependency removal event.
func runDepRemove(root, issueID, dependsOn string) {
	// Ensure both sides exist before appending the event.
	if _, _, err := pebbles.GetIssue(root, issueID); err != nil {
		exitError(err)
	}
	if _, _, err := pebbles.GetIssue(root, dependsOn); err != nil {
		exitError(err)
	}
	event := pebbles.NewDepRemoveEvent(issueID, dependsOn, pebbles.NowTimestamp())
	// Append the event and rebuild the cache.
	if err := pebbles.AppendEvent(root, event); err != nil {
		exitError(err)
	}
	if err := pebbles.RebuildCache(root); err != nil {
		exitError(err)
	}
}

// runDepTree prints a dependency tree for an issue.
func runDepTree(root, issueID string) {
	node, err := pebbles.DependencyTree(root, issueID)
	if err != nil {
		exitError(err)
	}
	printDepTree(node, 0)
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
		fmt.Println(formatIssueLine(issue))
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
func printIssue(root string, issue pebbles.Issue, deps []string) {
	// Header includes the status icon and priority badge.
	header := fmt.Sprintf(
		"%s %s · %s   [● %s · %s]",
		pebbles.StatusIcon(issue.Status),
		issue.ID,
		issue.Title,
		pebbles.PriorityLabel(issue.Priority),
		pebbles.StatusLabel(issue.Status),
	)
	fmt.Println(header)
	// Core metadata block.
	fmt.Printf("Type: %s\n", issue.IssueType)
	fmt.Printf(
		"Created: %s · Updated: %s\n\n",
		formatDate(issue.CreatedAt),
		formatDate(issue.UpdatedAt),
	)
	// Description section.
	fmt.Println("DESCRIPTION")
	if strings.TrimSpace(issue.Description) == "" {
		fmt.Println("(none)")
	} else {
		fmt.Println(issue.Description)
	}
	// Dependency list with status per dependency.
	fmt.Println("\nDEPENDENCIES")
	if len(deps) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, dep := range deps {
		status, err := pebbles.IssueStatus(root, dep)
		if err != nil {
			fmt.Printf("  → %s (unknown)\n", dep)
			continue
		}
		fmt.Printf("  → %s (%s)\n", dep, status)
	}
}

// printUsage prints a brief usage message.
func printUsage() {
	fmt.Println("Pebbles - A minimal issue tracker with append-only event log.")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  pb [command]")
	fmt.Println("")
	fmt.Println("Working With Issues:")
	fmt.Println("  create      Create a new issue")
	fmt.Println("  list        List issues")
	fmt.Println("  show        Show issue details")
	fmt.Println("  update      Update an issue")
	fmt.Println("  close       Close an issue")
	fmt.Println("  ready       Show issues ready to work (no blockers)")
	fmt.Println("")
	fmt.Println("Dependencies:")
	fmt.Println("  dep add     Add a dependency")
	fmt.Println("  dep rm      Remove a dependency")
	fmt.Println("  dep tree    Show dependency tree")
	fmt.Println("")
	fmt.Println("Setup:")
	fmt.Println("  init        Initialize a pebbles project")
	fmt.Println("  help        Show this help")
}

// exitError prints an error to stderr and exits.
func exitError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

// formatIssueLine returns a formatted list output for an issue.
func formatIssueLine(issue pebbles.Issue) string {
	return fmt.Sprintf(
		"%s %s [● %s] [%s] - %s",
		pebbles.StatusIcon(issue.Status),
		issue.ID,
		pebbles.PriorityLabel(issue.Priority),
		issue.IssueType,
		issue.Title,
	)
}

// formatDate renders a timestamp as YYYY-MM-DD when possible.
func formatDate(timestamp string) string {
	parsed, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return timestamp
	}
	return parsed.Format("2006-01-02")
}

// printDepTree renders dependency nodes with indentation.
func printDepTree(node pebbles.DepNode, depth int) {
	indent := strings.Repeat("  ", depth)
	line := fmt.Sprintf(
		"%s%s %s (%s)",
		indent,
		pebbles.StatusIcon(node.Issue.Status),
		node.Issue.ID,
		node.Issue.Status,
	)
	fmt.Println(line)
	for _, child := range node.Dependencies {
		printDepTree(child, depth+1)
	}
}
