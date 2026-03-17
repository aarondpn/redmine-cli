---
name: redmine-cli
description: Guide for using the redmine CLI tool to interact with Redmine project management. Covers output formats, pagination, filtering, and common workflows for issues, versions, time entries, and search.
---

# Redmine CLI

This skill teaches you how to use the `redmine` CLI to interact with a Redmine project management server.

## When to Use This Skill

Use this skill when the user asks you to:

- List, view, or search Redmine issues
- List or view project versions (milestones)
- Log or view time entries
- Query projects, users, groups, trackers, or statuses
- Perform any task involving Redmine project management data

## Prerequisites

The CLI must be configured first. Run `redmine config` to check. If not configured, run `redmine init` for interactive setup.

## Output Formats

Always use `-o json` for machine-readable output:

```bash
redmine issues list -o json
redmine versions list --project myproject -o json
```

Other formats: `-o csv` (tabular), `-o table` (default, human-readable).

## Pagination

All list commands support `--limit` and `--offset`:

```bash
--limit N     # Return at most N results
--limit 0     # Fetch ALL results (no limit) - use this when you need the complete dataset
--offset N    # Skip the first N results
```

Example: page through results:

```bash
redmine issues list --project myproject --limit 25 --offset 0
redmine issues list --project myproject --limit 25 --offset 25
```

## Issues

### List issues

```bash
# Open issues (default)
redmine issues list --project myproject -o json

# All issues regardless of status
redmine issues list --project myproject --status "*" --limit 0 -o json

# Closed issues assigned to me
redmine issues list --status closed --assignee me -o json

# Issues for a specific version (by name or ID)
redmine issues list --project myproject --version "v1.0" -o json
redmine issues list --version 42 -o json

# Sort by field
redmine issues list --project myproject --sort updated_on:desc -o json
```

### Get a single issue

```bash
redmine issues get 123 -o json

# With comments/journals
redmine issues get 123 --include journals -o json

# With all related data
redmine issues get 123 --include journals,children,relations -o json
```

### Create an issue

All flags that reference other resources (project, tracker, priority, status, assignee, version) accept **names or numeric IDs**. The CLI resolves names automatically.

```bash
# Create using human-readable names
redmine issues create --project myproject --tracker Bug --priority High --subject "Fix login page"

# Assign to the current user
redmine issues create --project myproject --subject "My task" --assignee me

# Full example with all fields
redmine issues create --project myproject --tracker Feature --priority Normal \
  --subject "Add search" --description "Implement full-text search" \
  --assignee "John Smith" --status "In Progress" --category "Development" \
  --version "v2.0" --parent 100 --estimated-hours 8 --private

# Numeric IDs still work
redmine issues create --project 1 --tracker 1 --priority 2 --subject "Test"

# Output as JSON
redmine issues create --project myproject --subject "New bug" --tracker Bug -o json
```

Available flags: `--project`, `--tracker`, `--subject` (required), `--description`, `--priority`, `--assignee`, `--status`, `--category`, `--version`, `--parent`, `--estimated-hours`, `--private`, `-o`.

If `--project` is omitted, the configured default project is used.

### Update an issue

Same name resolution as create. Only changed flags are sent to the server.

```bash
# Update status and priority by name
redmine issues update 123 --status Closed --priority Low

# Reassign with a note
redmine issues update 123 --assignee me --note "Taking over"

# Change category
redmine issues update 123 --category "Development"

# Set version and estimated hours
redmine issues update 123 --version "v2.0" --estimated-hours 4.5

# Mark as private
redmine issues update 123 --private
```

Available flags: `--subject`, `--description`, `--tracker`, `--status`, `--priority`, `--assignee`, `--category`, `--version`, `--parent`, `--estimated-hours`, `--private`, `--done-ratio`, `--note`.

### Name resolution errors

If a name doesn't match, the CLI provides helpful suggestions:

- **Small lists** (≤10 options): all available options are shown
- **Large lists** (>10 options): fuzzy "Did you mean?" suggestions based on typo similarity
- **Ambiguous matches**: all exact matches are listed with their IDs

```bash
# Will show all available trackers if "NonExistent" doesn't match
redmine issues create --project myproject --tracker NonExistent --subject "Test"

# Typos get "Did you mean?" suggestions (e.g. "Featrue" -> "Feature")
redmine issues create --project myproject --tracker Featrue --subject "Test"

# Will show matching users if the name is ambiguous
redmine issues update 123 --assignee "John"
```

## Versions

### List versions

```bash
# All versions for a project
redmine versions list --project myproject -o json

# Filter by status
redmine versions list --project myproject --open -o json
redmine versions list --project myproject --closed -o json
redmine versions list --project myproject --locked -o json

# Or use --status flag
redmine versions list --project myproject --status open -o json
```

### Get a single version

```bash
redmine versions get 42 -o json

# By name (uses default project, or pass --project)
redmine versions get "v1.0" -o json
redmine versions get "v1.0" --project myproject -o json
```

## Time Entries

### Log time

```bash
redmine time log --issue 123 --hours 2.5 --activity Development --comment "Worked on feature"
```

### List time entries

```bash
redmine time list --project myproject -o json
redmine time list --issue 123 -o json
```

## Users

```bash
# List users
redmine users list -o json

# Get user by ID, login, name, or "me"
redmine users get jsmith -o json
redmine users get "John Smith" -o json
redmine users get me -o json

# Filter users by group (name or ID)
redmine users list --group Developers -o json
```

## Groups

```bash
# List groups
redmine groups list -o json

# Get group by name or ID
redmine groups get Developers -o json

# Add/remove users by name
redmine groups add-user Developers jsmith
redmine groups remove-user Developers "John Smith"
```

## Other Commands

```bash
# Search across resources
redmine search "query string" --project myproject -o json

# List projects
redmine projects list -o json

# List trackers (for --tracker filter)
redmine trackers list -o json

# List issue categories (for --category filter)
redmine categories list --project myproject -o json

# List statuses (for --status filter)
redmine statuses list -o json

# Current config
redmine config
```

## Resolving Ambiguous Values Interactively

When you need to specify a project, tracker, version, assignee, priority, or status but are **not sure of the exact name or ID**, do NOT guess. Instead:

1. **Query the available options first** using the appropriate list command:
   ```bash
   redmine projects list -o json        # available projects
   redmine trackers list -o json        # available trackers
   redmine statuses list -o json        # available statuses
   redmine categories list -o json              # available categories
   redmine versions list -o json               # available versions
   redmine users list -o json           # available users
   ```
2. **Present the options to the user** in a clear, numbered list or selection prompt using your interactive tools (e.g. AskUserQuestion with a formatted list of choices). Let the user pick from the actual available options rather than asking them to type a free-form name.
3. **Then use the confirmed value** in the create/update command.

This pattern applies broadly — whenever a command requires a value from a fixed set (tracker, status, priority, category, version, assignee, project), prefer querying and presenting options over asking the user to remember or look up exact names. This makes the experience intuitive and avoids resolution errors.

## Tips

- Always use `-o json` for programmatic access to avoid parsing table formatting.
- Use `--limit 0` to fetch all results when you need the complete dataset.
- All resource flags (`--project`, `--tracker`, `--priority`, `--status`, `--assignee`, `--category`, `--version`, `--activity`, `--group`) accept human-readable names or numeric IDs.
- Commands that take user/group/version IDs as arguments also accept names (e.g., `redmine users get jsmith`, `redmine groups get Developers`).
- The `--assignee` flag and user arguments support the special value `me` to refer to the current API user.
- Version status filters (`--open`, `--closed`, `--locked`) are applied client-side.
- Set a default project with `redmine init` to avoid `--project` on every command.
- Use `--project` or `-p` to override the default project per-command.
- If a name doesn't resolve, the CLI shows all available options — use this to discover valid values.