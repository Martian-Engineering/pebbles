package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"pebbles/internal/pebbles"
)

var (
	buildVersion = "dev"
	buildCommit  = "unknown"
	buildDate    = "unknown"
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
		return
	}
	if os.Args[1] == "--version" || os.Args[1] == "-v" {
		printVersion()
		return
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
	case "comment":
		runComment(root, args)
	case "import":
		runImport(root, args)
	case "dep":
		runDep(root, args)
	case "ready":
		runReady(root, args)
	case "prefix":
		runPrefix(root, args)
	case "rename":
		runRename(root, args)
	case "rename-prefix":
		runRenamePrefix(root, args)
	case "log":
		runLog(root, args)
	case "help":
		printUsage()
	case "version":
		printVersion()
	default:
		printUsage()
		os.Exit(1)
	}
}

// printVersion prints version metadata for the build.
func printVersion() {
	message := buildVersion
	if buildCommit != "unknown" || buildDate != "unknown" {
		message = fmt.Sprintf("%s (%s %s)", buildVersion, buildCommit, buildDate)
	}
	fmt.Println(message)
}

// runInit handles pb init.
func runInit(root string, args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	prefix := fs.String("prefix", "", "Prefix for new issue IDs")
	_ = fs.Parse(args)
	prefixSet := false
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == "prefix" {
			prefixSet = true
		}
	})
	trimmed := strings.TrimSpace(*prefix)
	if prefixSet && trimmed == "" {
		exitError(fmt.Errorf("prefix is required"))
	}
	if err := pebbles.InitProjectWithPrefix(root, trimmed); err != nil {
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
	issueID, err := pebbles.GenerateUniqueIssueID(
		cfg.Prefix,
		*title,
		timestamp,
		pebbles.HostLabel(),
		func(candidate string) (bool, error) {
			return pebbles.IssueExists(root, candidate)
		},
	)
	if err != nil {
		exitError(err)
	}
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
	status := fs.String("status", "", "Filter by status (comma-separated)")
	issueType := fs.String("type", "", "Filter by issue type (comma-separated)")
	priority := fs.String("priority", "", "Filter by priority (P0-P4, comma-separated)")
	jsonOut := fs.Bool("json", false, "Output JSON")
	_ = fs.Parse(args)
	// Validate the project and requested filters before listing.
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	filters, err := parseListFilters(*status, *issueType, *priority)
	if err != nil {
		exitError(err)
	}
	issues, err := pebbles.ListIssueHierarchy(root)
	if err != nil {
		exitError(err)
	}
	// JSON output skips column formatting and writes a single payload.
	if *jsonOut {
		entries := make([]issueJSON, 0, len(issues))
		for _, item := range issues {
			if !filters.matches(item.Issue) {
				continue
			}
			entry, err := issueJSONWithDeps(root, item.Issue)
			if err != nil {
				exitError(err)
			}
			entries = append(entries, entry)
		}
		if err := printJSON(entries); err != nil {
			exitError(err)
		}
		return
	}
	widths := issueColumnWidthsForHierarchy(issues)
	for _, item := range issues {
		if !filters.matches(item.Issue) {
			continue
		}
		fmt.Println(formatIssueLine(item.Issue, item.Depth, widths))
	}
}

// runShow handles pb show.
func runShow(root string, args []string) {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output JSON")
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
	comments, err := pebbles.ListIssueComments(root, id)
	if err != nil {
		exitError(err)
	}
	if *jsonOut {
		if err := printJSON(buildIssueDetailJSON(issue, deps, comments)); err != nil {
			exitError(err)
		}
		return
	}
	printIssue(root, issue, deps, comments)
}

// optionalString tracks whether a string flag was explicitly set.
type optionalString struct {
	value string
	set   bool
}

// String returns the current value for flag usage output.
func (opt *optionalString) String() string {
	if opt == nil {
		return ""
	}
	return opt.value
}

// Set records a flag value and marks it as set.
func (opt *optionalString) Set(value string) error {
	opt.value = value
	opt.set = true
	return nil
}

// runUpdate handles pb update.
func runUpdate(root string, args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	status := fs.String("status", "", "New status")
	var issueType optionalString
	var description optionalString
	var priority optionalString
	fs.Var(&issueType, "type", "New issue type")
	fs.Var(&description, "description", "New description")
	fs.Var(&priority, "priority", "New priority (P0-P4)")
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
	if strings.TrimSpace(*status) == "" && !issueType.set && !description.set && !priority.set {
		exitError(fmt.Errorf("at least one field is required"))
	}
	if issueType.set && strings.TrimSpace(issueType.value) == "" {
		exitError(fmt.Errorf("type cannot be empty"))
	}
	if priority.set && strings.TrimSpace(priority.value) == "" {
		exitError(fmt.Errorf("priority cannot be empty"))
	}
	id := fs.Arg(0)
	// Confirm the issue exists in the cache.
	if _, _, err := pebbles.GetIssue(root, id); err != nil {
		exitError(err)
	}
	timestamp := pebbles.NowTimestamp()
	if strings.TrimSpace(*status) != "" {
		event := pebbles.NewStatusEvent(id, *status, timestamp)
		// Append the event and rebuild the cache for consistency.
		if err := pebbles.AppendEvent(root, event); err != nil {
			exitError(err)
		}
	}
	updatePayload := make(map[string]string)
	if issueType.set {
		updatePayload["type"] = issueType.value
	}
	if description.set {
		updatePayload["description"] = description.value
	}
	if priority.set {
		parsed, err := pebbles.ParsePriority(priority.value)
		if err != nil {
			exitError(err)
		}
		updatePayload["priority"] = fmt.Sprintf("%d", parsed)
	}
	if len(updatePayload) > 0 {
		event := pebbles.NewUpdateEvent(id, timestamp, updatePayload)
		if err := pebbles.AppendEvent(root, event); err != nil {
			exitError(err)
		}
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

// runComment handles pb comment.
func runComment(root string, args []string) {
	fs := flag.NewFlagSet("comment", flag.ExitOnError)
	body := fs.String("body", "", "Comment body")
	_ = fs.Parse(reorderFlags(args, map[string]bool{"--body": true}))
	// Validate inputs before appending a comment event.
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	if fs.NArg() != 1 {
		exitError(fmt.Errorf("comment requires issue id"))
	}
	if strings.TrimSpace(*body) == "" {
		exitError(fmt.Errorf("comment body is required"))
	}
	id := fs.Arg(0)
	// Confirm the issue exists in the cache.
	if _, _, err := pebbles.GetIssue(root, id); err != nil {
		exitError(err)
	}
	event := pebbles.NewCommentEvent(id, *body, pebbles.NowTimestamp())
	// Append the event and rebuild the cache.
	if err := pebbles.AppendEvent(root, event); err != nil {
		exitError(err)
	}
	if err := pebbles.RebuildCache(root); err != nil {
		exitError(err)
	}
}

// runImport handles pb import.
func runImport(root string, args []string) {
	if len(args) < 1 {
		exitError(fmt.Errorf("usage: pb import <beads> [flags]"))
	}
	switch args[0] {
	case "beads":
		runImportBeads(root, args[1:])
	default:
		exitError(fmt.Errorf("usage: pb import <beads> [flags]"))
	}
}

// runImportBeads imports Beads issues into Pebbles.
func runImportBeads(root string, args []string) {
	fs := flag.NewFlagSet("import beads", flag.ExitOnError)
	from := fs.String("from", "", "Beads repo root (default: current directory)")
	prefix := fs.String("prefix", "", "Issue prefix override")
	includeTombstones := fs.Bool("include-tombstones", false, "Import tombstone issues")
	dryRun := fs.Bool("dry-run", false, "Preview import without writing")
	backup := fs.Bool("backup", false, "Backup existing .pebbles directory")
	force := fs.Bool("force", false, "Overwrite existing .pebbles directory")
	_ = fs.Parse(reorderFlags(args, map[string]bool{"--from": true, "--prefix": true}))
	// Reject unexpected positional arguments early.
	if fs.NArg() != 0 {
		exitError(fmt.Errorf("usage: pb import beads [flags]"))
	}
	if *backup && *force {
		exitError(fmt.Errorf("choose either --backup or --force"))
	}
	// Resolve the source repo and build an import plan.
	sourceRoot, err := resolveImportRoot(root, *from)
	if err != nil {
		exitError(err)
	}
	plan, err := pebbles.PlanBeadsImport(pebbles.BeadsImportOptions{
		SourceRoot:        sourceRoot,
		Prefix:            *prefix,
		IncludeTombstones: *includeTombstones,
		Now:               time.Now,
	})
	if err != nil {
		exitError(err)
	}
	// Apply the plan when this isn't a dry run.
	if !*dryRun {
		if err := prepareBeadsImportTarget(root, plan.Result.Prefix, *backup, *force); err != nil {
			exitError(err)
		}
		result, err := pebbles.ApplyBeadsImportPlan(root, plan)
		if err != nil {
			exitError(err)
		}
		printBeadsImportSummary(result, false, root)
		return
	}
	printBeadsImportSummary(plan.Result, true, root)
}

// runDep handles pb dep commands.
func runDep(root string, args []string) {
	// Validate CLI arguments for dependency creation.
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	if len(args) < 1 {
		exitError(fmt.Errorf("usage: pb dep <add|rm|tree> [args]"))
	}
	// Route subcommands for dependency operations.
	action := args[0]
	switch action {
	case "add":
		addFlags := flag.NewFlagSet("dep add", flag.ExitOnError)
		depType := addFlags.String("type", pebbles.DepTypeBlocks, "Dependency type (blocks or parent-child)")
		_ = addFlags.Parse(reorderFlags(args[1:], map[string]bool{"--type": true}))
		if addFlags.NArg() != 2 {
			exitError(fmt.Errorf("usage: pb dep add [--type <type>] <issue> <depends-on>"))
		}
		runDepAdd(root, addFlags.Arg(0), addFlags.Arg(1), pebbles.NormalizeDepType(*depType))
	case "rm":
		rmFlags := flag.NewFlagSet("dep rm", flag.ExitOnError)
		depType := rmFlags.String("type", pebbles.DepTypeBlocks, "Dependency type (blocks or parent-child)")
		_ = rmFlags.Parse(reorderFlags(args[1:], map[string]bool{"--type": true}))
		if rmFlags.NArg() != 2 {
			exitError(fmt.Errorf("usage: pb dep rm [--type <type>] <issue> <depends-on>"))
		}
		runDepRemove(root, rmFlags.Arg(0), rmFlags.Arg(1), pebbles.NormalizeDepType(*depType))
	case "tree":
		if len(args) != 2 {
			exitError(fmt.Errorf("usage: pb dep tree <issue>"))
		}
		runDepTree(root, args[1])
	default:
		exitError(fmt.Errorf("usage: pb dep <add|rm|tree> [args]"))
	}
}

// runDepAdd appends a dependency add event.
func runDepAdd(root, issueID, dependsOn, depType string) {
	// Ensure both sides exist before appending the event.
	issue, _, err := pebbles.GetIssue(root, issueID)
	if err != nil {
		exitError(err)
	}
	parent, _, err := pebbles.GetIssue(root, dependsOn)
	if err != nil {
		exitError(err)
	}
	issueID = issue.ID
	dependsOn = parent.ID
	var events []pebbles.Event
	// Parent-child deps should use parent-based child IDs for lineage.
	if depType == pebbles.DepTypeParentChild && !pebbles.HasParentChildSuffix(dependsOn, issueID) {
		childID, err := pebbles.NextChildIssueID(root, dependsOn)
		if err != nil {
			exitError(err)
		}
		rename := pebbles.NewRenameEvent(issueID, childID, pebbles.NowTimestamp())
		events = append(events, rename)
		issueID = childID
	}
	events = append(events, pebbles.NewDepAddEvent(issueID, dependsOn, depType, pebbles.NowTimestamp()))
	// Append the events and rebuild the cache once.
	for _, event := range events {
		if err := pebbles.AppendEvent(root, event); err != nil {
			exitError(err)
		}
	}
	if err := pebbles.RebuildCache(root); err != nil {
		exitError(err)
	}
}

// runDepRemove appends a dependency removal event.
func runDepRemove(root, issueID, dependsOn, depType string) {
	// Ensure both sides exist before appending the event.
	if _, _, err := pebbles.GetIssue(root, issueID); err != nil {
		exitError(err)
	}
	if _, _, err := pebbles.GetIssue(root, dependsOn); err != nil {
		exitError(err)
	}
	event := pebbles.NewDepRemoveEvent(issueID, dependsOn, depType, pebbles.NowTimestamp())
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
	jsonOut := fs.Bool("json", false, "Output JSON")
	_ = fs.Parse(args)
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	issues, err := pebbles.ListReadyIssues(root)
	if err != nil {
		exitError(err)
	}
	if *jsonOut {
		entries := make([]issueJSON, 0, len(issues))
		for _, issue := range issues {
			entry, err := issueJSONWithDeps(root, issue)
			if err != nil {
				exitError(err)
			}
			entries = append(entries, entry)
		}
		if err := printJSON(entries); err != nil {
			exitError(err)
		}
		return
	}
	widths := issueColumnWidthsForIssues(issues)
	for _, issue := range issues {
		fmt.Println(formatIssueLine(issue, 0, widths))
	}
}

// runPrefix handles pb prefix commands.
func runPrefix(root string, args []string) {
	fs := flag.NewFlagSet("prefix", flag.ExitOnError)
	_ = fs.Parse(args)
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	if fs.NArg() < 1 {
		exitError(fmt.Errorf("usage: pb prefix set <prefix>"))
	}
	action := fs.Arg(0)
	switch action {
	case "set":
		if fs.NArg() != 2 {
			exitError(fmt.Errorf("usage: pb prefix set <prefix>"))
		}
		runPrefixSet(root, fs.Arg(1))
	default:
		exitError(fmt.Errorf("usage: pb prefix set <prefix>"))
	}
}

// runPrefixSet updates the prefix stored in the Pebbles config.
func runPrefixSet(root, prefix string) {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		exitError(fmt.Errorf("prefix is required"))
	}
	cfg, err := pebbles.LoadConfig(root)
	if err != nil {
		exitError(err)
	}
	cfg.Prefix = trimmed
	if err := pebbles.WriteConfig(root, cfg); err != nil {
		exitError(err)
	}
	fmt.Printf("Prefix set to %s\n", trimmed)
}

// runRename handles pb rename.
func runRename(root string, args []string) {
	fs := flag.NewFlagSet("rename", flag.ExitOnError)
	_ = fs.Parse(args)
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	if fs.NArg() != 2 {
		exitError(fmt.Errorf("usage: pb rename <old> <new>"))
	}
	oldID := strings.TrimSpace(fs.Arg(0))
	newID := strings.TrimSpace(fs.Arg(1))
	if oldID == "" || newID == "" {
		exitError(fmt.Errorf("rename requires non-empty ids"))
	}
	// Validate the old and new identifiers before appending the event.
	if _, _, err := pebbles.GetIssue(root, oldID); err != nil {
		exitError(err)
	}
	exists, err := pebbles.IssueExists(root, newID)
	if err != nil {
		exitError(err)
	}
	if exists {
		exitError(fmt.Errorf("issue id already exists: %s", newID))
	}
	// Append the rename event and rebuild the cache.
	event := pebbles.NewRenameEvent(oldID, newID, pebbles.NowTimestamp())
	if err := pebbles.AppendEvent(root, event); err != nil {
		exitError(err)
	}
	if err := pebbles.RebuildCache(root); err != nil {
		exitError(err)
	}
	fmt.Printf("Renamed %s -> %s\n", oldID, newID)
}

// runRenamePrefix updates IDs to a new prefix.
func runRenamePrefix(root string, args []string) {
	fs := flag.NewFlagSet("rename-prefix", flag.ExitOnError)
	full := fs.Bool("full", false, "Rename all issues")
	open := fs.Bool("open", false, "Rename only open issues")
	_ = fs.Parse(reorderFlags(args, map[string]bool{}))
	if err := ensureProject(root); err != nil {
		exitError(err)
	}
	if fs.NArg() != 1 {
		exitError(fmt.Errorf("usage: pb rename-prefix [--full|--open] <prefix>"))
	}
	if *full && *open {
		exitError(fmt.Errorf("choose either --full or --open"))
	}
	if !*full && !*open {
		*open = true
	}
	newPrefix := strings.TrimSpace(fs.Arg(0))
	if newPrefix == "" {
		exitError(fmt.Errorf("prefix is required"))
	}
	issues, err := pebbles.ListIssues(root)
	if err != nil {
		exitError(err)
	}
	// Build the rename plan before writing any events.
	events := make([]pebbles.Event, 0)
	seen := make(map[string]bool)
	for _, issue := range issues {
		if *open && issue.Status == pebbles.StatusClosed {
			continue
		}
		prefix, suffix, ok := splitIssueID(issue.ID)
		if !ok {
			exitError(fmt.Errorf("invalid issue id: %s", issue.ID))
		}
		if prefix == newPrefix {
			continue
		}
		targetID := fmt.Sprintf("%s-%s", newPrefix, suffix)
		if seen[targetID] {
			exitError(fmt.Errorf("duplicate target id: %s", targetID))
		}
		seen[targetID] = true
		exists, err := pebbles.IssueExists(root, targetID)
		if err != nil {
			exitError(err)
		}
		if exists {
			exitError(fmt.Errorf("issue id already exists: %s", targetID))
		}
		events = append(events, pebbles.NewRenameEvent(issue.ID, targetID, pebbles.NowTimestamp()))
	}
	// Append all rename events in order, then rebuild the cache once.
	for _, event := range events {
		if err := pebbles.AppendEvent(root, event); err != nil {
			exitError(err)
		}
	}
	if err := pebbles.RebuildCache(root); err != nil {
		exitError(err)
	}
	fmt.Printf("Renamed %d issues to %s\n", len(events), newPrefix)
}

// ensureProject checks that the .pebbles directory exists.
func ensureProject(root string) error {
	if _, err := os.Stat(pebbles.EventsPath(root)); err != nil {
		return fmt.Errorf("pebbles not initialized; run pb init")
	}
	return nil
}

func resolveImportRoot(root, from string) (string, error) {
	trimmed := strings.TrimSpace(from)
	if trimmed == "" {
		return root, nil
	}
	// Resolve relative paths against the current working directory.
	if !filepath.IsAbs(trimmed) {
		trimmed = filepath.Join(root, trimmed)
	}
	resolved, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve source path: %w", err)
	}
	if _, err := os.Stat(resolved); err != nil {
		return "", fmt.Errorf("source path not found: %w", err)
	}
	return resolved, nil
}

func prepareBeadsImportTarget(root, prefix string, backup, force bool) error {
	if strings.TrimSpace(prefix) == "" {
		return fmt.Errorf("prefix is required")
	}
	pebblesDir := pebbles.PebblesDir(root)
	if _, err := os.Stat(pebblesDir); err == nil {
		// Ensure we only proceed when the caller requested backup or overwrite.
		if !backup && !force {
			return fmt.Errorf("pebbles already initialized; use --backup or --force")
		}
		if backup {
			backupName := fmt.Sprintf(".pebbles.backup-%s", time.Now().UTC().Format("20060102T150405Z"))
			backupPath := filepath.Join(root, backupName)
			if _, err := os.Stat(backupPath); err == nil {
				return fmt.Errorf("backup path already exists: %s", backupPath)
			}
			if err := os.Rename(pebblesDir, backupPath); err != nil {
				return fmt.Errorf("backup .pebbles: %w", err)
			}
		}
		if force {
			if err := os.RemoveAll(pebblesDir); err != nil {
				return fmt.Errorf("remove .pebbles: %w", err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check .pebbles: %w", err)
	}
	// Initialize a new Pebbles directory with the chosen prefix.
	if err := pebbles.InitProjectWithPrefix(root, prefix); err != nil {
		return err
	}
	return nil
}

func printBeadsImportSummary(result pebbles.BeadsImportResult, dryRun bool, targetRoot string) {
	fmt.Printf("Source: %s\n", result.SourceRoot)
	fmt.Printf("Target: %s\n", targetRoot)
	fmt.Printf("Prefix: %s\n", result.Prefix)
	fmt.Printf(
		"Issues: %d total, %d imported, %d skipped (%d tombstones)\n",
		result.IssuesTotal,
		result.IssuesImported,
		result.IssuesSkipped,
		result.TombstonesSkipped,
	)
	fmt.Printf("Events planned: %d\n", result.EventsPlanned)
	if dryRun {
		fmt.Println("Dry run: no events written.")
	} else {
		fmt.Printf("Events written: %d\n", result.EventsWritten)
	}
	// Print warnings after the core summary for easy scanning.
	if len(result.Warnings) == 0 {
		return
	}
	fmt.Printf("Warnings: %d\n", len(result.Warnings))
	for _, warning := range result.Warnings {
		fmt.Printf("  - %s\n", warning)
	}
}

// printIssue renders a single issue to stdout.
func printIssue(root string, issue pebbles.Issue, deps []string, comments []pebbles.IssueComment) {
	// Header includes the status icon and priority badge.
	statusIcon := renderStatusIcon(issue.Status)
	priorityLabel := renderPriorityLabel(issue.Priority)
	statusLabel := renderStatusLabel(issue.Status)
	header := fmt.Sprintf(
		"%s %s · %s   [● %s · %s]",
		statusIcon,
		issue.ID,
		issue.Title,
		priorityLabel,
		statusLabel,
	)
	fmt.Println(header)
	// Core metadata block.
	fmt.Printf("Type: %s\n", renderIssueType(issue.IssueType))
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
		fmt.Println(renderMarkdown(issue.Description))
	}
	// Dependency list with status per dependency.
	fmt.Println("\nDEPENDENCIES")
	if len(deps) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, dep := range deps {
			status, err := pebbles.IssueStatus(root, dep)
			if err != nil {
				fmt.Printf("  → %s (unknown)\n", dep)
				continue
			}
			fmt.Printf("  → %s (%s)\n", dep, status)
		}
	}
	// Comments keep issue discussion history close to the details.
	printIssueComments(comments)
}

// printIssueComments prints issue comments with timestamps and indentation.
func printIssueComments(comments []pebbles.IssueComment) {
	fmt.Println("\nCOMMENTS")
	if len(comments) == 0 {
		fmt.Println("  (none)")
		return
	}
	for index, comment := range comments {
		if index > 0 {
			fmt.Println("")
		}
		fmt.Printf("  %s\n", formatCommentTimestamp(comment.Timestamp))
		for _, line := range formatCommentBodyLines(comment.Body) {
			fmt.Printf("    %s\n", line)
		}
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
	fmt.Println("  version     Show pb version")
	fmt.Println("  update      Update an issue")
	fmt.Println("  close       Close an issue")
	fmt.Println("  comment     Add a comment to an issue")
	fmt.Println("  rename      Rename an issue id")
	fmt.Println("  rename-prefix Rename issues to a new prefix (flags before prefix)")
	fmt.Println("  ready       Show issues ready to work (no blockers)")
	fmt.Println("  log         Show the event log (pretty view)")
	fmt.Println("")
	fmt.Println("Import:")
	fmt.Println("  import beads Import issues from a Beads project")
	fmt.Println("")
	fmt.Println("Dependencies:")
	fmt.Println("  dep add     Add a dependency (--type blocks|parent-child)")
	fmt.Println("  dep rm      Remove a dependency (--type blocks|parent-child)")
	fmt.Println("  dep tree    Show dependency tree")
	fmt.Println("")
	fmt.Println("Prefixes:")
	fmt.Println("  prefix set  Update the prefix used for new ids")
	fmt.Println("")
	fmt.Println("Setup:")
	fmt.Println("  init        Initialize a pebbles project")
	fmt.Println("  init --prefix <prefix> Initialize with a custom prefix")
	fmt.Println("  help        Show this help")
	fmt.Println("")
	fmt.Println("Styling:")
	fmt.Println("  list/show output uses ANSI colors when stdout is a TTY.")
	fmt.Println("  Set NO_COLOR=1 or PB_NO_COLOR=1 to disable.")
}

// listFilters holds optional filters for pb list output.
type listFilters struct {
	statuses   map[string]bool
	types      map[string]bool
	priorities map[int]bool
}

// parseListFilters builds the filter set for pb list.
func parseListFilters(statusInput, typeInput, priorityInput string) (listFilters, error) {
	statuses, err := parseListStatusFilter(statusInput)
	if err != nil {
		return listFilters{}, err
	}
	priorities, err := parseListPriorityFilter(priorityInput)
	if err != nil {
		return listFilters{}, err
	}
	return listFilters{
		statuses:   statuses,
		types:      parseListTypeFilter(typeInput),
		priorities: priorities,
	}, nil
}

// parseListStatusFilter validates and normalizes status filters.
func parseListStatusFilter(input string) (map[string]bool, error) {
	values := splitCSV(input)
	if len(values) == 0 {
		return nil, nil
	}
	allowed := map[string]bool{
		pebbles.StatusOpen:       true,
		pebbles.StatusInProgress: true,
		pebbles.StatusClosed:     true,
	}
	statuses := make(map[string]bool, len(values))
	// Normalize and validate each status value.
	for _, value := range values {
		normalized := normalizeStatusFilter(value)
		if normalized == "" {
			continue
		}
		if !allowed[normalized] {
			return nil, fmt.Errorf("unknown status: %s", value)
		}
		statuses[normalized] = true
	}
	if len(statuses) == 0 {
		return nil, nil
	}
	return statuses, nil
}

// parseListTypeFilter normalizes the type filter to lowercase.
func parseListTypeFilter(input string) map[string]bool {
	values := splitCSV(input)
	if len(values) == 0 {
		return nil
	}
	types := make(map[string]bool, len(values))
	// Normalize types to lowercase for case-insensitive matching.
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		types[normalized] = true
	}
	if len(types) == 0 {
		return nil
	}
	return types
}

// parseListPriorityFilter parses priority filters into numeric values.
func parseListPriorityFilter(input string) (map[int]bool, error) {
	values := splitCSV(input)
	if len(values) == 0 {
		return nil, nil
	}
	priorities := make(map[int]bool, len(values))
	// Parse each priority entry into its numeric form.
	for _, value := range values {
		priority, err := pebbles.ParsePriority(value)
		if err != nil {
			return nil, err
		}
		priorities[priority] = true
	}
	if len(priorities) == 0 {
		return nil, nil
	}
	return priorities, nil
}

// matches reports whether an issue passes the configured filters.
func (filters listFilters) matches(issue pebbles.Issue) bool {
	if filters.statuses != nil && !filters.statuses[issue.Status] {
		return false
	}
	if filters.types != nil && !filters.types[strings.ToLower(issue.IssueType)] {
		return false
	}
	if filters.priorities != nil && !filters.priorities[issue.Priority] {
		return false
	}
	return true
}

// splitCSV breaks a comma-separated string into trimmed values.
func splitCSV(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	parts := strings.Split(input, ",")
	values := make([]string, 0, len(parts))
	// Trim each entry and drop empty segments.
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	return values
}

// normalizeStatusFilter allows hyphenated status names in filters.
func normalizeStatusFilter(input string) string {
	trimmed := strings.ToLower(strings.TrimSpace(input))
	return strings.ReplaceAll(trimmed, "-", "_")
}

// exitError prints an error to stderr and exits.
func exitError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(1)
}

// issueColumnWidths stores column widths for list formatting.
type issueColumnWidths struct {
	status    int
	id        int
	priority  int
	issueType int
}

// issueColumnWidthsForHierarchy computes display widths for a hierarchy list.
func issueColumnWidthsForHierarchy(items []pebbles.IssueHierarchyItem) issueColumnWidths {
	var widths issueColumnWidths
	for _, item := range items {
		updateIssueColumnWidths(&widths, item.Issue)
	}
	return widths
}

// issueColumnWidthsForIssues computes display widths for a flat list.
func issueColumnWidthsForIssues(issues []pebbles.Issue) issueColumnWidths {
	var widths issueColumnWidths
	for _, issue := range issues {
		updateIssueColumnWidths(&widths, issue)
	}
	return widths
}

// updateIssueColumnWidths expands column widths to fit the issue fields.
func updateIssueColumnWidths(widths *issueColumnWidths, issue pebbles.Issue) {
	statusIcon := pebbles.StatusIcon(issue.Status)
	priority := priorityDisplay(issue)
	widths.status = maxWidth(widths.status, displayWidth(statusIcon))
	widths.id = maxWidth(widths.id, displayWidth(issue.ID))
	widths.priority = maxWidth(widths.priority, displayWidth(priority))
	widths.issueType = maxWidth(widths.issueType, displayWidth(issue.IssueType))
}

// maxWidth returns the larger of two widths.
func maxWidth(current, candidate int) int {
	if candidate > current {
		return candidate
	}
	return current
}

// displayWidth returns the rune width of a display string.
func displayWidth(value string) int {
	return utf8.RuneCountInString(value)
}

// visibleWidth returns the display width of a string without ANSI escapes.
func visibleWidth(value string) int {
	width := 0
	for i := 0; i < len(value); i++ {
		if value[i] == '\x1b' && i+1 < len(value) && value[i+1] == '[' {
			i += 2
			for i < len(value) && value[i] != 'm' {
				i++
			}
			continue
		}
		_, size := utf8.DecodeRuneInString(value[i:])
		width++
		i += size - 1
	}
	return width
}

// padDisplay right-pads a value based on visible width.
func padDisplay(value string, width int) string {
	if width <= 0 {
		return value
	}
	padding := width - visibleWidth(value)
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}

// priorityDisplay formats the priority string used in list output.
func priorityDisplay(issue pebbles.Issue) string {
	return fmt.Sprintf("● %s", pebbles.PriorityLabel(issue.Priority))
}

// formatIssueLine returns a formatted list output for an issue.
func formatIssueLine(issue pebbles.Issue, depth int, widths issueColumnWidths) string {
	indent := strings.Repeat("  ", depth)
	statusIcon := renderStatusIcon(issue.Status)
	priorityLabel := fmt.Sprintf("● %s", renderPriorityLabel(issue.Priority))
	issueType := renderIssueType(issue.IssueType)
	return fmt.Sprintf(
		"%s%s %s [%s] [%s] - %s",
		indent,
		padDisplay(statusIcon, widths.status),
		padDisplay(issue.ID, widths.id),
		padDisplay(priorityLabel, widths.priority),
		padDisplay(issueType, widths.issueType),
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

// formatCommentTimestamp renders comment timestamps with time-of-day.
func formatCommentTimestamp(timestamp string) string {
	parsed, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return timestamp
	}
	return parsed.UTC().Format("2006-01-02 15:04:05")
}

// formatCommentBodyLines splits a comment body into display lines.
func formatCommentBodyLines(body string) []string {
	if strings.TrimSpace(body) == "" {
		return []string{"(empty)"}
	}
	return strings.Split(body, "\n")
}

// splitIssueID separates an issue ID into prefix and suffix.
func splitIssueID(issueID string) (string, string, bool) {
	parts := strings.SplitN(issueID, "-", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// reorderFlags moves flags (and their values) before positional args.
func reorderFlags(args []string, flagsWithValues map[string]bool) []string {
	var flags []string
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i:]...)
			break
		}
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			if strings.Contains(arg, "=") {
				continue
			}
			if flagsWithValues[arg] && i+1 < len(args) {
				flags = append(flags, args[i+1])
				i++
			}
			continue
		}
		positionals = append(positionals, arg)
	}
	return append(flags, positionals...)
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
