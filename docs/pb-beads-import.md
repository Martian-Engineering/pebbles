# Beads to Pebbles Import Spec

Goal: initialize a Pebbles project from an existing Beads repo, preserving issues,
status, priorities, dependencies, and comments from .beads/issues.jsonl.

## Goals

- Import Beads issues into `.pebbles/events.jsonl` with stable issue IDs.
- Preserve title, description, issue_type, priority, status, deps, and comments.
- Make the import repeatable and safe (dry-run, backups, no partial writes).
- Provide clear warnings when data cannot be mapped.

## Non-Goals

- Reconstruct Beads internal history beyond current state fields.
- Import Beads daemon state, sync metadata, or sqlite DB.
- Support non-Beads sources.

## Inputs

Primary input is `.beads/issues.jsonl` from a Beads repo. Each line is a JSON
issue document. The importer should not read or mutate `beads.db`.

## Proposed Command

```
pb import beads [--from <repo>] [--prefix <prefix>] [--include-tombstones]
               [--dry-run] [--backup] [--force]
```

- `--from`: repo root (default: current directory).
- `--prefix`: override prefix detection for `.pebbles/config.json`.
- `--include-tombstones`: import deleted issues (default: on).
- `--dry-run`: no writes, only a summary and warnings.
- `--backup`: move existing `.pebbles` to `.pebbles.backup-<timestamp>`.
- `--force`: allow overwrite without backup (discouraged).

## Mapping: Beads -> Pebbles Events

Pebbles event schema:

```
{ "type": "create", "timestamp": "...", "issue_id": "...", "payload": { ... } }
```

### Core Fields

- `id` -> Pebbles `issue_id` (preserve exactly).
- `title`, `description` -> create payload.
- `issue_type` -> create payload `type`.
- `priority` -> create payload `priority` (int 0-4).
- `created_at` -> create event timestamp.

### Status

Beads status -> Pebbles events:

- `open`: no status update (default in Pebbles).
- `in_progress`: add `status_update` at `updated_at`.
- `closed`: add `close` at `closed_at` if present, else `updated_at`.
- `tombstone`: add `close` at `deleted_at` if present, else `updated_at`.

If `close_reason` or `delete_reason` is set, add a `comment` event at the same
timestamp with a body like:

```
Close reason: <text>
Deleted by: <deleted_by>
Deleted at: <deleted_at>
```

### Comments

Beads comments are arrays of `{author, text, created_at}`.
Pebbles comment events have no author field, so prefix the body:

```
Author: <author>
<text>
```

Timestamp should be `created_at` when present.

### Dependencies

Each Beads dependency becomes a Pebbles `dep_add` event:

- `type = blocks` -> `dep_type = blocks`
- `type = parent-child` -> `dep_type = parent-child`
- other types (example: `relates-to`) -> skip and warn

Use dependency `created_at` as the event timestamp.

Parent-child dependencies should be written directly to the log to preserve
existing issue IDs. Do not call `pb dep add` during import, because the CLI
renames children to `<parent>.<N>` when they are missing the suffix.

### Ordering

Pebbles applies events sequentially and requires referenced issues to exist.
Importer should ensure:

1. All `create` events are written first (sorted by `created_at`).
2. Dependency and comment events are written next (sorted by timestamp).
3. Status updates and closes are written last (sorted by timestamp).

If multiple events share the same timestamp, keep a stable order:
`create` -> `dep_add` -> `comment` -> `status_update` -> `close`.

### Prefix Detection

- Default: detect the most common prefix before the first `-` in issue IDs.
- If multiple prefixes are present, require `--prefix`.
- Set `.pebbles/config.json` to the chosen prefix so future issues match.

### Validation and Warnings

- If any dependency references a missing issue, warn and skip the dep.
- If a priority is outside 0-4, clamp and warn.
- If a timestamp is missing, fall back to `updated_at`, then `created_at`,
  then `now()`.
- If `.pebbles` exists, require `--backup` or `--force`.

## Expected Workflow

1. `pb import beads --dry-run`
2. Review summary (counts, warnings, prefixes).
3. `pb import beads --backup` (or `--force`).
4. Run `pb list` and `pb ready` to verify.

## Repo Checks

### /Users/phaedrus/Projects/claude-team

- Issues: 143 total
- Statuses: tombstone 125, open 17, closed 1
- Issue types: task, feature, chore, bug, epic
- Priorities: 0-3
- Dependencies: blocks only
- Comments: present (example comment fields: author, text, created_at)
- Prefix: `cic`

Implications: tombstone handling and comments are required. No parent-child
edges here.

### /Users/phaedrus/Projects/luckystrike/connected-play

- Issues: 589 total
- Statuses: tombstone 426, closed 108, open 51, in_progress 4
- Issue types: task, bug, epic, feature, molecule
- Priorities: 0-4
- Dependencies: blocks (475), parent-child (105), relates-to (1)
- Comments: none found in sample scan
- Prefix: `cp`
- IDs with dot suffixes (examples): `cp-4use.7`, `cp-xrpr.3`

Implications: parent-child deps must be preserved without renaming, and unknown
dependency types (relates-to) must be skipped or annotated.

## Limitations

- Beads does not expose full event history in issues.jsonl; import is a
  reconstruction from current state only.
- Pebbles has no deleted status, so tombstones become closed issues.
- Comment authors are stored in the comment body.
- Unknown dependency types are skipped (reported as warnings).
- Prefixes are single-valued in Pebbles; multi-prefix Beads repos need manual
  selection.
