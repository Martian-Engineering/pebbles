# Pebbles

Pebbles is a minimal, git-friendly issue tracker that uses an append-only event log.

## Architecture

- Append-only event log: `.pebbles/events.jsonl`
- SQLite cache: `.pebbles/pebbles.db` (rebuilt from the log)
- Deterministic IDs: project prefix + hash of title + timestamp + host

## Install

```bash
go build -o pb ./cmd/pb
```

## Usage

```bash
# Initialize a project
pb init

# Create an issue
pb create --title="Add login" --type=task --priority=P2 --description="Track login work"

# List issues
pb list

# Show issue details
pb show pb-abc12345

# Update status
pb update pb-abc12345 --status in_progress

# Close an issue
pb close pb-abc12345

# Rename an issue id
pb rename pb-abc12345 pb-new1234

# Rename open issues to a new prefix
pb rename-prefix peb --open

# Set the prefix for new issue IDs
pb prefix set peb

# Add a dependency
pb dep add pb-issue-a pb-issue-b

# Remove a dependency
pb dep rm pb-issue-a pb-issue-b

# Show dependency tree
pb dep tree pb-issue-a

# List ready issues (no open blockers)
pb ready
```

## Notes

- The event log is the source of truth. The SQLite cache is derived.
- Git merges are safe because events are append-only.
- Run `pb init` in the project root before using other commands.
