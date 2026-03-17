---
title: Issues
description: Create, list, update, and manage Redmine issues.
---

The `issues` command (alias: `i`) manages Redmine issues.

## List Issues

```bash
redmine issues list [flags]
```

| Flag | Description |
|------|------------|
| `--project` | Filter by project (name or ID) |
| `--tracker` | Filter by tracker (name or ID) |
| `--status` | Filter by status: `open`, `closed`, `*`, name, or ID |
| `--assignee` | Filter by assignee: `me`, name, or ID |
| `--version` | Filter by target version (name or ID) |
| `--sort` | Sort order, e.g. `updated_on:desc` |
| `--limit` | Maximum number of results (0 for all) |
| `--offset` | Result offset for pagination |
| `-o, --output` | Output format: `table`, `wide`, `json`, `csv` |

```bash
# List open issues assigned to you
redmine issues list --assignee me

# List all bugs for a project
redmine issues list --project myproject --tracker Bug

# List closed issues sorted by update date
redmine issues list --status closed --sort updated_on:desc
```

## View an Issue

```bash
redmine issues get <id> [flags]
```

| Flag | Description |
|------|------------|
| `--journals` | Include issue history/comments |
| `-o, --output` | Output format |

## Create an Issue

```bash
redmine issues create [flags]
```

| Flag | Description |
|------|------------|
| `--project` | Project (name or ID) — **required** |
| `--subject` | Issue subject — **required** |
| `--tracker` | Tracker (name or ID) |
| `--status` | Status (name or ID) |
| `--priority` | Priority (name or ID) |
| `--assignee` | Assignee: `me`, name, or ID |
| `--description` | Issue description |
| `--parent` | Parent issue ID |
| `--category` | Category (name or ID) |
| `--version` | Target version (name or ID) |
| `--estimated-hours` | Estimated hours |
| `--private` | Mark as private |

```bash
redmine issues create --project myproject --subject "Add search" --tracker Feature --priority High
```

## Update an Issue

```bash
redmine issues update <id> [flags]
```

Accepts the same flags as `create` (except `--project`) plus:

| Flag | Description |
|------|------------|
| `--notes` | Add a comment |
| `--done-ratio` | Completion percentage (0-100) |
| `--due-date` | Due date (YYYY-MM-DD) |

## Close an Issue

```bash
redmine issues close <id> [flags]
```

| Flag | Description |
|------|------------|
| `--notes` | Add a closing comment |

## Reopen an Issue

```bash
redmine issues reopen <id> [flags]
```

## Assign an Issue

```bash
redmine issues assign <id> --to <user> [flags]
```

## Add a Comment

```bash
redmine issues comment <id> --notes "Your comment" [flags]
```

## Delete an Issue

```bash
redmine issues delete <id> [flags]
```

| Flag | Description |
|------|------------|
| `-f, --force` | Skip confirmation prompt |

## Browse Issues (TUI)

```bash
redmine issues browse [flags]
```

Opens an interactive terminal browser for issues. Accepts the same filter flags as `list`.
