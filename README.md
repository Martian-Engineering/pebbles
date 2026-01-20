# Pebbles

A minimal, git-friendly issue tracker that actually stays out of your way.

## TL;DR

### The Problem

You're working with AI agents across multiple git branches. Your issue tracker needs to:
- Work offline without syncing daemons
- Merge cleanly without conflicts
- Not require constant maintenance (`bd doctor --fix` anyone?)
- Support parallel work in git worktrees
- Stay simple and predictable

GitHub Issues requires internet. Beads grew complex. Linear costs money. TODO comments scatter everywhere.

### The Solution

Pebbles uses an **append-only event log** as its source of truth. Merging branches? Just merge the event logs. No conflicts, ever.

```bash
pb init
pb create --title="Add authentication" --type=task
pb list
pb update pb-abc --status in_progress
pb close pb-abc
```

### Why Pebbles?

|  | **Pebbles** | **Beads** | **GitHub Issues** | **Linear/Jira** |
|---|:---:|:---:|:---:|:---:|
| **Offline-first** | ✅ | ✅ | ❌ | ❌ |
| **Git-native** | ✅ | ✅ | ❌ | ❌ |
| **No daemon/syncing** | ✅ | ❌ | N/A | N/A |
| **Zero merge conflicts** | ✅ | ❌ | N/A | N/A |
| **No maintenance commands** | ✅ | ❌ | ✅ | ✅ |
| **Cost** | Free | Free | Free | $$$ |
| **Setup complexity** | Low | High | None | Medium |

## Why This Project Exists

I built Pebbles in anger after fighting with Beads one too many times.

I was an early Beads adopter. The core workflow was fantastic:
- Discuss plans with AI agents
- Translate into self-contained issues
- Use issues as context for getting work done
- Capture requirements, bugs, and ideas as they arise

Beads removed the friction of figuring out where to log work by providing a structured, local, simple interface.

**And then Beads kept growing.**

Each release added more JSON files to `.beads/`. Daemon mode appeared. Syncing needed a separate branch to avoid polluting main. The `bd doctor --fix` command ran constantly (rarely successfully). Merge conflicts with `issues.jsonl` became routine. I spent more time fighting my issue tracker than doing real work.

This likely stemmed from Steve Yegge's vision for GasTown—Beads kept adding features I didn't need, bringing complexity I didn't want.

**Pebbles is a return to basics.** It does one thing well: track issues in git without getting in your way. I don't plan to add features beyond making it fast and simple. It primarily needs to disappear.

## Quick Start

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/Martian-Engineering/pebbles/master/scripts/install.sh | sh

# Initialize in your project
pb init

# Create your first issue
pb create --title="Add user authentication" --type=task --priority=P1

# List all issues
pb list

# Start work
pb update pb-abc --status in_progress

# Add findings as you work
pb create --title="Authentication needs rate limiting" --type=bug --priority=P2

# Close when done
pb close pb-abc --description="Implemented JWT auth with refresh tokens"

# Everything is in git - just commit
git add .pebbles/events.jsonl
git commit -m "docs: track authentication work"
```

## Design Philosophy

### 1. Append-Only Event Log Architecture

**This is the killer feature.** The `.pebbles/events.jsonl` file is the source of truth. Everything else is derived.

```bash
# The event log is just JSONL - human-readable, merge-friendly
cat .pebbles/events.jsonl
{"event_type":"create","issue_id":"pb-abc","timestamp":"2026-01-19T10:00:00Z",...}
{"event_type":"status","issue_id":"pb-abc","timestamp":"2026-01-19T11:00:00Z",...}
```

**Why this matters:**
- **Zero merge conflicts**: Merging git branches = appending event logs. No special handling needed.
- **Perfect for AI agents**: Multiple agents in parallel worktrees can work simultaneously
- **Simple sync model**: Pull changes, replay the log, done
- **Audit trail included**: Every change is recorded with full history
- **Disposable cache**: The SQLite database is rebuilt from events. Delete it anytime.

### 2. No Background Processes

No daemon. No sync service. No doctor command. Commands read the event log, do their work, append new events, rebuild the cache, and exit.

Simple tools that compose.

### 3. Git-Native, Not Git-Automated

Pebbles writes to `.pebbles/events.jsonl`. **You** decide when to commit, branch, and push. No automatic git operations. No surprise commits.

Your workflow, your control.

### 4. Minimal Feature Set

Issue tracking needs:
- Create, update, close issues ✅
- Dependencies and blockers ✅
- Priority and type metadata ✅
- Comments ✅
- Filter and query ✅

Everything else is scope creep.

### 5. Optimized for AI Agents

Issue IDs are deterministic and collision-resistant. Commands output clean text for humans, JSON for machines. The event log provides perfect context for LLMs picking up work.

## Installation

### Quick Install

```bash
# Install latest version
curl -fsSL https://raw.githubusercontent.com/Martian-Engineering/pebbles/master/scripts/install.sh | sh

# Install a specific version
PB_VERSION=v0.3.0 curl -fsSL https://raw.githubusercontent.com/Martian-Engineering/pebbles/master/scripts/install.sh | sh

# Install to custom directory
PB_INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/Martian-Engineering/pebbles/master/scripts/install.sh | sh
```

### Build from Source

```bash
git clone https://github.com/Martian-Engineering/pebbles.git
cd pebbles
go build -o pb ./cmd/pb
```

## Comparison Tables

### Pebbles vs Beads

|  | **Pebbles** | **Beads** |
|---|---|---|
| **Merge conflicts** | Never | Frequent |
| **Background processes** | None | Daemon mode |
| **Maintenance commands** | None | `bd doctor --fix` |
| **Event log format** | Append-only | Complex state |
| **SQLite cache** | Git-ignored, disposable | Sometimes tracked |
| **Philosophy** | Minimal, stable | Growing feature set |
| **Best for** | AI agents, simplicity | GasTown integration |

### Pebbles vs GitHub Issues

|  | **Pebbles** | **GitHub Issues** |
|---|---|---|
| **Offline work** | Full functionality | Read-only (cached) |
| **In-repo context** | Native | External |
| **Branch-specific issues** | Natural | Workarounds |
| **Private work** | Always | Repo must be private |
| **Dependencies** | Built-in | Manual labels |
| **AI agent integration** | Optimized | API rate limits |

### Pebbles vs Linear/Jira

|  | **Pebbles** | **Linear/Jira** |
|---|---|---|
| **Cost** | $0 | $8-15/user/month |
| **Overhead** | Seconds to init | Hours to configure |
| **Context switching** | None (in terminal) | Browser tab required |
| **Customization** | Code is config | UI configuration |
| **Data ownership** | Your git repo | Their servers |

## Commands

### Initialize

```bash
# Initialize in current directory
pb init

# Initialize with custom prefix for issue IDs
pb init --prefix myproject
```

### Creating Issues

```bash
# Create a task
pb create --title="Add login" --type=task --priority=P2

# Create with description
pb create --title="Fix bug" --type=bug --description="Users report timeout errors"

# Import from Beads
pb import beads --from /path/to/beads/repo --backup
```

### Listing and Querying

```bash
# List all issues
pb list

# Filter by status
pb list --status open,in_progress

# Filter by type and priority
pb list --type task --priority P0,P1

# Show ready issues (no open blockers)
pb ready

# Show issue details
pb show pb-abc
```

Issues display in parent-child order with indentation:

```
○ pb-abc [● P2] [task] - Add authentication
  ○ pb-abc.1 [● P1] [task] - Research auth libraries
  ○ pb-abc.2 [○ P2] [task] - Implement JWT handling
```

### Updating Issues

```bash
# Update status
pb update pb-abc --status in_progress

# Update multiple fields
pb update pb-abc --status in_progress --type bug --priority P0

# Update description
pb update pb-abc --description "Found root cause in session handling"

# Close an issue
pb close pb-abc

# Add a comment
pb comment pb-abc --body "Investigating the race condition"
```

### Dependencies

```bash
# Add dependency (pb-a depends on pb-b, meaning pb-b blocks pb-a)
pb dep add pb-a pb-b

# Add parent-child dependency (child ID becomes <parent>.<N>)
pb dep add --type parent-child pb-child pb-parent

# Remove dependency
pb dep rm pb-a pb-b

# Visualize dependency tree
pb dep tree pb-abc
```

### Renaming

```bash
# Rename a specific issue
pb rename pb-abc pb-new-id

# Rename all open issues to new prefix
pb rename-prefix --open newprefix

# Rename ALL issues (including closed)
pb rename-prefix --full newprefix

# Set prefix for future issues
pb prefix set newprefix
```

### Event Log

```bash
# View recent events (pretty format)
pb log --limit 20

# Table view
pb log --table --limit 20

# Filter by date
pb log --since 2026-01-01 --until 2026-01-31

# JSON output for parsing
pb log --json

# Skip git blame attribution (faster)
pb log --no-git
```

**Pretty format:**
```
event 12 status pb-abc
Title: Add authentication
When:  2026-01-19 10:45:12
Actor: Josh Lehman (2026-01-19)
Details:
  status=in_progress
```

**Table format:**
```
<actor> <actor_date> <event_time> <type> <issue_id> <title> [details]
```

### Version

```bash
pb version
```

## Architecture

### Event Log as Source of Truth

```
.pebbles/
├── events.jsonl          # Source of truth (commit this)
└── pebbles.db           # SQLite cache (gitignored, disposable)
```

**Event log** (`.pebbles/events.jsonl`):
- Append-only JSONL file
- Every issue operation appends an event
- Human-readable, grep-friendly
- Merges cleanly in git
- Complete audit trail

**SQLite cache** (`.pebbles/pebbles.db`):
- Derived from event log
- Rebuilt automatically by commands
- Enables fast queries
- Safe to delete—will be regenerated
- Never committed to git

### Sync Model

1. Commands append events to `events.jsonl`
2. Commands rebuild `pebbles.db` from the full event log
3. After pulling changes, any `pb` command triggers a rebuild
4. No manual sync needed

### Deterministic Issue IDs

Issue IDs use format: `<prefix>-<hash>`

Example: `pb-7c9ef95f`

Hash is computed from:
- Project prefix
- Issue title
- Timestamp
- Hostname

**Collision handling:** IDs start with 3 characters. On collision, expand to 4, then 5, etc.

**Parent-child IDs:** When a child issue is linked to a parent via parent-child dependency, it gets renamed to `<parent>.<N>` using the first available suffix number.

### Merge Safety

**Scenario:** Two developers working in parallel branches both create and update issues.

**Traditional issue tracker:** Merge conflicts, manual resolution needed.

**Pebbles:** Event logs append independently. Merge the branches, the event logs combine, cache rebuilds automatically. No conflicts.

This is why Pebbles is perfect for AI agents working in parallel git worktrees.

## Filtering and Styling

### Filter Flags

All filter flags accept comma-separated values (case-insensitive):

```bash
# Status: open, in_progress, closed (hyphens accepted: in-progress)
pb list --status open,in_progress

# Type: task, bug, epic, etc.
pb list --type task,epic

# Priority: P0, P1, P2, P3, P4 (or just 0-4)
pb list --priority P0,P1
```

### Color Output

`pb list` and `pb show` use ANSI colors when outputting to a terminal.

Disable with:
```bash
NO_COLOR=1 pb list
# or
PB_NO_COLOR=1 pb list
```

### Paging

`pb log` pipes output through a pager when outputting to a terminal.

Pager selection order:
1. `PB_PAGER` environment variable
2. `PAGER` environment variable
3. `less -FRX` (default)

Disable paging:
```bash
pb log --no-pager
```

## FAQ

### Why not just use GitHub Issues?

GitHub Issues requires internet, lives outside your repo, and doesn't support branch-specific work naturally. Pebbles is offline-first, git-native, and perfect for tracking context that evolves with your branches.

### Why not stick with Beads?

Beads is excellent but grew complex. Background daemons, doctor commands, merge conflicts, and feature bloat. Pebbles freezes the "classic Beads" workflow and commits to staying simple.

### Can I migrate from Beads?

Yes:
```bash
pb import beads --from /path/to/beads/repo --backup
```

### How do I share issues with my team?

Commit `.pebbles/events.jsonl` to git. Push the branch. Teammates pull and their cache rebuilds automatically.

### What if I delete the database?

The database is disposable. Run any `pb` command and it rebuilds from `events.jsonl`.

### Why Go instead of Rust?

Personal preference. Go is simple, fast enough, and cross-compiles easily. Pebbles is ~10K lines and doesn't need the optimization Rust provides.

### What about web UI / integrations / automation?

Out of scope. Pebbles is a CLI tool that stays in its lane. If you need those features, use a different tool.

## Notes

- **The event log is the source of truth.** Everything else is derived.
- **Git merges are safe** because events are append-only.
- **Run `pb init`** in your project root before using other commands.
- **Release notes** live in `CHANGELOG.md`.
- **Release process** documented in `docs/RELEASING.md`.

## License

MIT

## Contributing

Pebbles is intentionally minimal. Feature requests will likely be declined to preserve simplicity. Bug reports and performance improvements are welcome.
