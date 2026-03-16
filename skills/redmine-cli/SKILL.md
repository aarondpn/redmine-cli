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

The CLI must be configured first. Run `redmine config` to check. If not configured, run `redmine init` for interactive setup, or pass `--server` and `--api-key` flags.

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
```

## Time Entries

### Log time

```bash
redmine time log --issue 123 --hours 2.5 --activity 9 --comment "Worked on feature"
```

### List time entries

```bash
redmine time list --project myproject -o json
redmine time list --issue 123 -o json
```

## Other Commands

```bash
# Search across resources
redmine search "query string" --project myproject -o json

# List projects
redmine projects list -o json

# List users
redmine users list -o json

# List trackers (for --tracker filter)
redmine trackers list -o json

# List statuses (for --status filter)
redmine statuses list -o json

# Current config
redmine config
```

## Tips

- Always use `-o json` for programmatic access to avoid parsing table formatting.
- Use `--limit 0` to fetch all results when you need the complete dataset.
- The `--version` flag on `issues list` accepts either a version name (string) or numeric ID.
- Version status filters (`--open`, `--closed`, `--locked`) are applied client-side.
- Set a default project with `redmine init` to avoid `--project` on every command.
- Use `--project` or `-p` to override the default project per-command.