package main

import (
	"flag"
	"fmt"
)

const rootHelp = `Pebbles - A minimal issue tracker with append-only event log.

Usage:
  pb <command> [flags]
  pb <command> --help
  pb help
  pb --version

Issue fields:
  Status values: open, in_progress, closed (filters accept in-progress)
  Type values: free-form; common: task, bug, feature, epic
  Priority values: P0-P4 (or 0-4)

Common workflows:
  pb init --prefix pb
  pb create --title "Add API docs" --description "Document auth flow"
  pb list --status open
  pb update <id> --status in_progress
  pb comment <id> --body "Working on the draft"
  pb close <id>
  pb log --since 2024-01-01

Working With Issues:
  create         Create a new issue
  list           List issues with filters
  show           Show issue details
  update         Update status or fields on an issue
  close          Close an issue
  reopen         Reopen a closed issue
  comment        Add a comment to an issue
  rename         Rename an issue id
  rename-prefix  Rename issue ids to a new prefix
  ready          Show issues ready to work (no blockers)
  log            Show the event log

Import:
  import beads   Import issues from a Beads project

Dependencies:
  dep            Manage dependencies (add, rm, tree)

Prefixes:
  prefix set     Update the prefix used for new ids

Git Integration:
  sync           Commit pebbles events to git

Setup:
  init           Initialize a pebbles project
  self-update    Check for updates and install the latest release
  version        Print pb version
  help           Show this help

Styling:
  list/show output uses ANSI colors when stdout is a TTY.
  Set NO_COLOR=1 or PB_NO_COLOR=1 to disable.
`

const initHelp = `Initialize a Pebbles project.

Usage:
  pb init
  pb init --prefix pb

Flags:
  --prefix <prefix>  Optional. Defaults to the repo folder name on first init.

Details:
  - Creates .pebbles/ with config, events log, and cache.
  - If already initialized, leaves existing config unchanged.
  - Use pb prefix set to change the prefix later.

Workflows:
  - Run once per repo: pb init --prefix pb
`

const createHelp = `Create a new issue.

Usage:
  pb create --title "Fix login error"
  pb create --title "Improve onboarding" --description "Clarify step 2"
  pb create --title "Triage crash" --type bug --priority P1

Flags:
  --title <text>         Required. Example: --title "Fix login error"
  --description <text>   Optional. Markdown accepted. Example: --description "Steps to reproduce..."
  --type <type>          Optional. Free-form; common: task, bug, feature, epic. Default: task.
  --priority <P0-P4>     Optional. P0-P4 or 0-4 (default P2). Example: --priority P1

Details:
  - Generates a new issue id using the project prefix and prints it.

Workflows:
  - Capture a quick task: pb create --title "Follow up with client"
  - File a bug with context: pb create --title "Login fails" --type bug --description "..."
`

const listHelp = `List issues with optional filters.

Usage:
  pb list
  pb list --status open
  pb list --type bug,feature --priority P0,P1
  pb list --stale --stale-days 14
  pb list --blocked
  pb list --json

Flags:
  --status <status>[,<status>...]   Filter by status (open, in_progress, closed; hyphens ok). Example: --status open,in-progress
  --type <type>[,<type>...]         Filter by type (case-insensitive). Example: --type bug,task
  --priority <P0-P4>[,<P0-P4>...]   Filter by priority (P0-P4 or 0-4). Example: --priority P0,P1
  --stale                           Show open issues with no activity. Example: --stale --stale-days 30
  --stale-days <days>               Days without activity (default 30, must be > 0). Example: --stale-days 14
  --blocked                         Show issues blocked by open dependencies. Example: --blocked
  --json                            Output JSON array of issues (includes deps). Example: --json

Details:
  - Status filters accept "in-progress" as an alias for "in_progress".

Workflows:
  - Triage open bugs: pb list --status open --type bug
  - Find blocked work: pb list --blocked
  - Export for scripts: pb list --json
`

const showHelp = `Show issue details.

Usage:
  pb show <id>
  pb show <id> --json

Flags:
  --json   Output JSON object (issue, deps, comments). Example: --json

Details:
  - Default output includes description, hierarchy, dependencies, and comments.

Workflows:
  - Inspect an issue: pb show pb-123
  - Scriptable output: pb show pb-123 --json
`

const updateHelp = `Update status or fields on an issue.

Usage:
  pb update <id> --status in_progress
  pb update <id> --type bug --priority P1
  pb update --description "Updated scope" <id>
  pb update <id> --parent pb-epic

Flags:
  --status <status>      New status (open, in_progress, closed). Example: --status in_progress
  --type <type>          Replace issue type (free-form). Example: --type chore
  --description <text>   Replace description (Markdown ok). Example: --description "New details"
  --priority <P0-P4>     Replace priority (P0-P4 or 0-4). Example: --priority P0
  --parent <id|none>     Replace parent issue. Example: --parent pb-epic

Details:
  - You can update multiple fields in one command.
  - Setting status to closed sets closed_at; other statuses clear closed_at.
  - Clear the parent with --parent none (or --parent "").

Workflows:
  - Start work: pb update <id> --status in_progress
  - Raise priority: pb update <id> --priority P1
  - Set parent: pb update <id> --parent pb-epic
`

const closeHelp = `Close an issue.

Usage:
  pb close <id>

Details:
  - Sets status to closed and updates closed_at.

Workflows:
  - Finish work: pb close pb-123
`

const reopenHelp = `Reopen a closed issue.

Usage:
  pb reopen <id>

Details:
  - Sets status back to open and clears closed_at.

Workflows:
  - Reopen for follow-up: pb reopen pb-123
`

const commentHelp = `Add a comment to an issue.

Usage:
  pb comment <id> --body "Investigated logs; suspect token refresh"
  pb comment <id> --body "Next steps: add retry to client"

Flags:
  --body <text>   Required. Example: --body "Meeting notes..."

Details:
  - Wrap the body in quotes if it contains spaces or newlines.

Workflows:
  - Record progress: pb comment <id> --body "Implemented parser"
  - Capture decisions: pb comment <id> --body "Agreed to ship Friday"
`

const importHelp = `Import issues into Pebbles.

Usage:
  pb import beads [flags]

Details:
  - Only the "beads" importer is available today.

Workflows:
  - Preview import: pb import beads --from ../beads --dry-run
  - Migrate with backup: pb import beads --from ../beads --backup
`

const importBeadsHelp = `Import issues from a Beads project.

Usage:
  pb import beads
  pb import beads --from ../beads --dry-run
  pb import beads --from ../beads --backup
  pb import beads --from ../beads --force --prefix pb

Flags:
  --from <path>              Source Beads repo (default: current directory). Example: --from ../beads
  --prefix <prefix>          Override target prefix (defaults to Beads prefix). Example: --prefix pb
  --include-tombstones       Include deleted issues. Example: --include-tombstones
  --dry-run                  Preview changes without writing. Example: --dry-run
  --backup                   Move existing .pebbles to a backup dir (exclusive with --force). Example: --backup
  --force                    Remove existing .pebbles before import (exclusive with --backup). Example: --force

Details:
  - Use --dry-run first to review the import plan.

Workflows:
  - Always run a dry run first: pb import beads --from ../beads --dry-run
  - Preserve existing data: pb import beads --from ../beads --backup
`

const depHelp = `Manage dependencies between issues.

Usage:
  pb dep add <issue> <depends-on> [--type blocks|parent-child]
  pb dep rm <issue> <depends-on> [--type blocks|parent-child]
  pb dep tree <issue>

Flags:
  --type <blocks|parent-child>  Dependency type for add/rm. Example: --type parent-child

Details:
  - "blocks" means <issue> depends on <depends-on>.
  - "parent-child" builds epic/subtask hierarchies.

Workflows:
  - Block a task: pb dep add pb-123 pb-456
  - Create an epic child: pb dep add pb-201 pb-200 --type parent-child
  - Visualize hierarchy or blockers: pb dep tree pb-200
`

const depAddHelp = `Add a dependency between issues.

Usage:
  pb dep add <issue> <depends-on>
  pb dep add <issue> <depends-on> --type parent-child

Flags:
  --type <blocks|parent-child>  Dependency type (default blocks). Example: --type blocks

Details:
  - parent-child renames the child id to <parent>.<N> when needed.

Workflows:
  - Block a task on another: pb dep add pb-123 pb-456
  - Create a child issue under an epic: pb dep add pb-201 pb-200 --type parent-child
`

const depRmHelp = `Remove a dependency between issues.

Usage:
  pb dep rm <issue> <depends-on>
  pb dep rm <issue> <depends-on> --type parent-child

Flags:
  --type <blocks|parent-child>  Dependency type (default blocks). Example: --type blocks

Details:
  - Removes the dependency edge but keeps issue ids unchanged.

Workflows:
  - Unblock a task: pb dep rm pb-123 pb-456
  - Detach a child issue: pb dep rm pb-201 pb-200 --type parent-child
`

const depTreeHelp = `Show the dependency tree for an issue.

Usage:
  pb dep tree <issue>

Details:
  - Prints parent-child hierarchy when present; otherwise blockers.

Workflows:
  - Inspect hierarchy/blockers: pb dep tree pb-123
`

const readyHelp = `List issues that are ready to work (open and unblocked).

Usage:
  pb ready
  pb ready --json

Flags:
  --json   Output JSON array of issues (includes deps). Example: --json

Details:
  - Ready issues are open and have no blocking dependencies.

Workflows:
  - Daily queue: pb ready
  - Scriptable output: pb ready --json
`

const prefixHelp = `Update the prefix used for new issue ids.

Usage:
  pb prefix set <prefix>

Details:
  - Updates the prefix for future issue ids only.
  - Use pb rename-prefix to change existing issue ids.

Workflows:
  - Set a short prefix: pb prefix set pb
`

const renameHelp = `Rename an issue id.

Usage:
  pb rename <old> <new>

Details:
  - Updates the issue id and dependency references.
  - Fails if the new id already exists.

Workflows:
  - Fix a typo: pb rename pb-abc pb-abd
`

const renamePrefixHelp = `Rename existing issue ids to a new prefix.

Usage:
  pb rename-prefix <prefix>
  pb rename-prefix --open <prefix>
  pb rename-prefix --full <prefix>

Flags:
  --open   Rename only open issues (default when no flag provided). Example: --open pb
  --full   Rename all issues (open and closed). Example: --full pb

Details:
  - Keeps the existing suffix after the "-" in each id.
  - All ids must be in <prefix>-<suffix> format.

Workflows:
  - Change prefix for open work: pb rename-prefix --open pb
  - Migrate everything: pb rename-prefix --full pb
`

const logHelp = `Show the event log.

Usage:
  pb log
  pb log --limit 50
  pb log --since 2024-01-01
  pb log --until 2024-01-31
  pb log --table
  pb log --json
  pb log --no-git

Flags:
  --limit, -n <count>   Limit events (0 = no limit). Example: --limit 100
  --since <timestamp>   Only events on/after time (RFC3339 or YYYY-MM-DD). Example: --since 2024-01-01
  --until <timestamp>   Only events on/before time. Example: --until 2024-01-31
  --no-git              Skip git blame attribution. Example: --no-git
  --table               Render table output. Example: --table
  --no-pager            Disable pager output. Example: --no-pager
  --json                Output JSON lines. Example: --json

Details:
  - --json outputs one JSON object per line (no pager).
  - --table prints a single line per event instead of blocks.

Workflows:
  - Recent activity: pb log --limit 50
  - Script export: pb log --json
  - Faster on large repos: pb log --no-git --table
`

const selfUpdateHelp = `Check for updates and install the latest release.

Usage:
  pb self-update
  pb self-update --check

Flags:
  --check   Only check for updates. Example: --check

Details:
  - Downloads and replaces the current binary when updates are available.
  - Install requires a release build; use --check with dev builds.

Workflows:
  - Verify before updating: pb self-update --check
`

const syncHelp = `Commit pebbles events to make them visible across worktrees.

Usage:
  pb sync
  pb sync --push

Flags:
  --push   Push to remote after committing. Example: --push

Details:
  - Commits .pebbles/events.jsonl with message "pebbles: sync".
  - Idempotent: does nothing if there are no uncommitted changes.
  - Does NOT push by default

Workflows:
  - Sync after creating issues: pb sync
  - Sync and push: pb sync --push
`

func setFlagUsage(fs *flag.FlagSet, help string) {
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), help)
	}
}

func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "--help"
}
