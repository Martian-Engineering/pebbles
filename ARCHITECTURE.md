# Pebbles Architecture

Pebbles is a deliberately minimal, git-friendly issue tracker. This document
captures the project spirit and the architectural guardrails that keep Pebbles
small and maintainable.

## Project Spirit

- Pebbles is a simpler cousin of Beads, not a clone with every feature.
- The event log is the source of truth. Everything else is derived.
- Favor clarity and determinism over cleverness.
- Avoid background services, hooks, and hidden automation.

## Core Goals

1. Local-first: always works offline.
2. Git-friendly: data is plain text and easy to merge.
3. Deterministic: state is derived from an append-only log.
4. Minimal UX: a small CLI surface that is easy to learn.

## Non-Goals

These are intentionally out of scope unless the project goals change:

- Daemons, background services, or long-running processes.
- Complex merge drivers or custom git tooling.
- Rich metadata (labels, comments, attachments, etc.).
- Sync servers or external services.
- Multi-user permissions or authentication.
- Complex search or analytics.

## Data Model

### Source of Truth

- `.pebbles/events.jsonl` is the only authoritative data store.
- The log is append-only. Events are never edited in place.

### Cache

- `.pebbles/pebbles.db` is a derived cache for query speed.
- The cache is rebuilt from the event log and is never committed.
- The cache is ignored via `.pebbles/.gitignore`.

### Config

- `.pebbles/config.json` stores the project prefix.

## Event Schema

Each line in `events.jsonl` is a JSON object:

```
{
  "type": "create|status_update|close|dep_add|dep_rm",
  "timestamp": "RFC3339Nano",
  "issue_id": "<prefix>-<hash>",
  "payload": { "string": "string" }
}
```

Event payloads are intentionally small. They should remain simple strings to
avoid schema churn. If the payload grows, consider adding a new event type
instead of overloading existing ones.

## Issue State

Issues are derived from events into a single row in SQLite. Current fields:

- id, title, description, issue_type
- status (open, in_progress, closed)
- priority (P0-P4)
- created_at, updated_at, closed_at

## CLI Surface

The CLI intentionally matches a small subset of Beads:

- init
- create, list, show, update, close, ready
- log
- dep add, dep rm, dep tree
- help

Avoid expanding beyond these unless there is a clear need that fits the
minimal philosophy.

## Merge Behavior

- Git merges are safe because the log is append-only.
- Conflicts should be resolved by keeping both event lines.
- There is no merge driver and no need for one.

## Invariants

- The event log is append-only.
- The cache is disposable.
- The CLI should remain small and predictable.
- No background automation.

## When to Say No

Say no to features that:

- Require a daemon or background tasks.
- Require tracking additional state outside the event log.
- Add complex cross-issue interactions or views.
- Make the data format harder to merge or inspect.

## If Change Is Required

If a change threatens simplicity, document why the exception is necessary.
Favor adding a new tool or separate project over bloating Pebbles.
