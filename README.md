# Pebbles

Pebbles is a minimal, git-friendly issue tracker that uses an append-only event log.

## Architecture

- Append-only event log: `.pebbles/events.jsonl`
- SQLite cache: `.pebbles/pebbles.db` (rebuilt from the log)
- Deterministic IDs: project prefix + hash of title + timestamp + host (3-char suffix by default; expands on collision)
- Parent-child IDs: child issues linked via parent-child deps are renamed to `<parent>.<N>` using the first available suffix

## Install

```bash
# Install latest
curl -fsSL https://raw.githubusercontent.com/Martian-Engineering/pebbles/master/scripts/install.sh | sh
# Install a specific version
PB_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/Martian-Engineering/pebbles/master/scripts/install.sh | sh
# Install to a custom directory
PB_INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/Martian-Engineering/pebbles/master/scripts/install.sh | sh
```

Build from source:

```bash
go build -o pb ./cmd/pb
```


## Usage

```bash
# Initialize a project
pb init

# Initialize with a custom prefix for new issue IDs
pb init --prefix peb

# Import issues from a Beads repo
pb import beads --from /path/to/repo --backup

# Create an issue
pb create --title="Add login" --type=task --priority=P2 --description="Track login work"

# List issues
pb list

# Show issue details
pb show pb-abc

# Update status and fields
pb update pb-abc --status in_progress --type bug --priority P1 --description "Investigate regressions"

# Close an issue
pb close pb-abc

# Add a comment
pb comment pb-abc --body "Investigating the root cause"

# Rename an issue id
pb rename pb-abc pb-new

# Rename open issues to a new prefix (flags before prefix)
pb rename-prefix --open peb

# Rename all issues to a new prefix
pb rename-prefix --full peb

# Set the prefix for new issue IDs
pb prefix set peb

# Add a dependency
pb dep add pb-issue-a pb-issue-b

# Add a parent-child dependency (child IDs become <parent>.<N>)
pb dep add --type parent-child pb-child pb-parent

# Remove a dependency
pb dep rm pb-issue-a pb-issue-b

# Show dependency tree
pb dep tree pb-issue-a

# List ready issues (no open blockers)
pb ready

# Show the event log (pretty view)
pb log --limit 20

# Show the event log in table view
pb log --table --limit 20
```

## Listing Issues

`pb list` prints issues in parent-child order (children are indented two spaces per
depth) with the line format:

```
○ pb-abc [● P2] [task] - Title
```

Filtering flags (comma-separated, case-insensitive):

- `--status`: `open`, `in_progress`, `closed` (hyphens are accepted, e.g. `in-progress`)
- `--type`: issue type values like `task` or `epic`
- `--priority`: `P0`-`P4` (or `0`-`4`)

Examples:

```bash
pb list --status open
pb list --status open,in_progress --type task
pb list --priority P0,P1
```
## Styling

`pb list` and `pb show` use ANSI colors when stdout is a TTY. Set `NO_COLOR=1`
or `PB_NO_COLOR=1` to disable.
## Log Output

`pb log` prints a multi-line, human-friendly block per event (newest-first by
timestamp):

```
event 12 status pb-7c9ef95f
Title: Add pb log command
When:  2026-01-19 10:45:12
Actor: Josh Lehman (2026-01-19)
Details:
  status=in_progress
```

Use `--table` for the compact column view:

```
<actor> <actor_date> <event_time> <type> <issue_id> <title> [details]
```

Details are rendered per event type:

- create: `type=<issue_type> priority=<P0-P4> description="<text>"`
- status_update: `status=<status>`
- close: `description="<text>"`
- comment: `body="<text>"`
- dep_add/dep_rm: `depends_on=<issue_id>`
- unknown types: payload key/value pairs ordered as `title`, `description`,
  `body`, `type`, `priority`, `status`, `depends_on`, then alphabetically

Flags:

- `--limit`/`-n`: limit number of events
- `--since`, `--until`: filter by timestamp (RFC3339/RFC3339Nano or YYYY-MM-DD)
- `--no-git`: skip git blame attribution
- `--json`: emit JSON lines instead of the pretty view
- `--table`: use the compact column view
- `--no-pager`: disable paging

Paging:

- When stdout is a TTY (and not `--json`), output is piped to a pager.
- Pager selection order: `PB_PAGER`, then `PAGER`, then `less -FRX`.

Actor and actor_date come from `git blame` of `.pebbles/events.jsonl`. When git
data is unavailable (or `--no-git` is used), they render as `unknown`.

## Notes

- The event log is the source of truth. The SQLite cache is derived.
- Git merges are safe because events are append-only.
- Run `pb init` in the project root before using other commands.
- Release notes live in `CHANGELOG.md`.
