# pb log UI Spec

Goal: make `pb log` a human-friendly, git-log-like view while preserving
machine-readable output options.

## Goals

- Default output is readable for humans (multi-line entries, clear labels).
- Use a pager (like `less`) when output is a TTY, so users can scroll with j/k.
- Keep current data fidelity (event timestamp, type, issue id, details).
- Preserve machine output via `--json`.

## Non-Goals

- Exact `git log` feature parity or color theming.
- Changes to the event schema.
- Background processes or caching beyond the existing SQLite cache.

## Output (Default: Pretty)

Each event is rendered as a compact, multi-line block with a blank line between
entries. Proposed format:

```
event <line> <type> <issue-id>
Title: <issue title or "unknown">
When:  <event timestamp>
Actor: <actor> (<actor_date>)
Details:
  <key>=<value>
```

Notes:
- `Actor` and `actor_date` come from git blame when available; otherwise
  `unknown`.
- `Details` is derived from payload key/values; empty payload prints
  `Details: (none)`.
- The log remains newest-first by timestamp (same as current behavior).

## Output (Table)

Keep the current columnar format available via a flag:

```
<actor> <actor_date> <event_time> <type> <issue_id> <title> [details]
```

This should remain stable for users who prefer a compact view.

## Pager Behavior

- When stdout is a TTY and output is not `--json`, pipe through a pager.
- Default pager: `PB_PAGER` env var, else `PAGER`, else `less -FRX`.
- Allow `--no-pager` to disable paging.
- When not a TTY, write directly to stdout.

## Flags

- `--no-pager`: disable paging
- `--table`: use column format (current output)
- `--json`: emit JSON lines (no pager)
- Existing filters remain: `--limit/-n`, `--since`, `--until`, `--no-git`

## Examples

```
$ pb log
# opens pager with pretty view

$ pb log --table --limit 20
# prints 20 events in column view

$ pb log --json
# JSON lines to stdout
```

## Implementation Notes

- Implement a formatter for the pretty view in `cmd/pb/log.go`.
- Add a small pager helper (TTY check, exec pager, fallback to stdout).
- Keep table and json paths intact, only change default selection.
- Update tests to cover pretty formatting and no-pager behavior.
- Update README to describe the new default and flags.
