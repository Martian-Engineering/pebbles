# Pebbles Workflow

*NOTE!!! Wherever `pb` is referenced below, substitute `go run ./cmd/pb` instead — we're still in development and have
not yet created a binary.*

We track work in Pebbles instead of Markdown. Run `pb help` to see how. Always start a session with this command.

## Development Workflow

We track work in Pebbles instead of Markdown. Here are some of the key commands:

```bash
pb help                    # Show CLI usage
pb list                    # List all issues
pb ready                   # Show issues ready to work (no blockers)
pb show <issue-id>         # Show issue details
pb update <issue-id> --status in_progress
pb close <issue-id>
pb dep tree <issue-id>     # Visualize dependencies
```

You should generally begin by running `pb help`.

Current epics are tracked with dependencies. Check `pb list` to see all issues and `pb ready` for unblocked work.

### Beginning Work

IMPORTANT: Before doing any work, always create a Pebbles issue or, if appropriate, epic.

### Plan Mode Workflow

When working on complex features that require planning (via `EnterPlanMode`), follow this workflow:

1. **During plan mode**: Explore the codebase, understand patterns, and write a detailed plan to the plan file
2. **Before exiting plan mode**: Ensure the plan captures all subtasks, configuration decisions, and dependencies
3. **First implementation step**: After exiting plan mode, **always** create Pebbles issues before writing any code:
   - Create an **epic** for the overall feature/deployment
   - Create **subtasks** as individual Pebbles issues, one per discrete piece of work
   - Each issue description should capture relevant context from the plan (what to implement, key decisions, file paths)
   - Set the epic to depend on each subtask using `pb dep add <epic-id> <subtask-id>`
   - Add **execution dependencies** between subtasks using `pb dep add <task-a> <task-b>` (task-a depends on task-b)

4. **Then proceed with implementation**: Work through the subtasks using the normal task workflow

**Dependency direction**:
- `pb dep add A B` means "A depends on B" (B blocks A)
- For epic/subtask relationship: `pb dep add <epic-id> <subtask-id>`

**Example**:
```bash
# After exiting plan mode for "RDS Deployment" epic:
pb create --title="RDS Deployment & pg-sync Service" --type=epic --description="..."
pb create --title="Create RDS Terraform module" --type=task --description="..."  # Returns task1-id
pb create --title="Add RDS to dev environment" --type=task --description="..."   # Returns task2-id

# Make epic depend on each subtask (subtasks block the epic)
pb dep add <epic-id> <task1-id>
pb dep add <epic-id> <task2-id>

# Add execution dependency (task2 waits for task1)
pb dep add <task2-id> <task1-id>
```

### Prioritizing Work

Always start by checking for in-progress work:

1. **Check for in-progress epics**: Run `pb list` and look for epics with status `in_progress`
2. **Focus on epic tasks**: If an epic is in-progress, prioritize tasks that block that epic
3. **Check ready work**: Use `pb ready` to see unblocked tasks, then choose tasks related to in-progress epics
4. **Mark epics in-progress**: When starting work on an epic's tasks, mark the epic as `in_progress` if not already marked

### Task Workflow

When working on individual issues, follow this workflow:

1. **Start work**: `pb update <issue-id> --status in_progress` (for both epics and tasks)
2. **Complete the work**: Implement the feature/fix
3. **Close the issue**: `pb close <issue-id>`
4. **Commit immediately**: Create a git commit after closing each issue with:
   - Summary of completed issue(s) in the commit message
   - List of changes made
   - Reference to issue IDs that were closed
   - **Regeneration prompt** for non-trivial changes (see below)

### Regeneration Prompts in Commits

For non-trivial commits, add a `Regeneration-Prompt:` block that captures intent:

```
feat(telegram): add caption splitting

<body>

Regeneration-Prompt: |
  <Goal>: What problem are we solving?
  <Constraints>: What must be preserved or followed?
  <Context>: What influenced decisions?
  <Scope>: What files/areas are affected?
```

The prompt should be sufficient for a fresh agent to recreate similar code without seeing the diff. Skip for trivial commits (typos, version bumps).

This ensures a clean audit trail where commits map directly to completed work items.

### Discovering Issues During Development

When bugs, inconsistencies, or improvements are discovered during development:

1. **Create an issue immediately**: Use `pb create` to document the problem as soon as it's discovered
2. **Commit issue additions**: Always commit `.pebbles/events.jsonl` after adding issues
3. **Err on the side of creating issues**: Better to have tracked issues than forgotten problems
4. **Link to relevant epics**: Use `pb dep add <epic-id> <issue-id>` so the epic depends on the newly discovered issue
5. **Don't let issues block current work**: If the discovered issue isn't critical, create it and continue with the current task
6. **Document what was found**: Include enough detail in the issue description for someone else (or future you) to understand the problem

Examples of when to create issues:
- User testing reveals unexpected behavior
- Inconsistent UI/UX patterns across screens
- Missing functionality that should be added for completeness
- Technical debt or code that needs refactoring
- Documentation that needs updating
