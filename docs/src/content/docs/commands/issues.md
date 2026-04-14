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
| `--project` | Filter by project (name, identifier, or ID) |
| `--tracker` | Filter by tracker (name or ID) |
| `--status` | Filter by status: `open`, `closed`, `*`, name, or ID |
| `--assignee` | Filter by assignee: `me`, name, or ID |
| `--version` | Filter by target version (name or ID) |
| `--sort` | Sort order, e.g. `updated_on:desc` |
| `--include` | Include related data: `attachments`, `relations` |
| `--attachments` | Shorthand for `--include attachments` |
| `--relations` | Shorthand for `--include relations` |
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
| `--include` | Include related data: `journals`, `children`, `relations` |
| `--journals` | Shorthand for `--include journals` |
| `--children` | Shorthand for `--include children` |
| `--relations` | Shorthand for `--include relations` |
| `-o, --output` | Output format |

## Create an Issue

```bash
redmine issues create [flags]
```

| Flag | Description |
|------|------------|
| `--project` | Project (name, identifier, or ID) — **required** |
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
| `--attach` | File path to attach (repeatable) |

```bash
redmine issues create --project myproject --subject "Add search" --tracker Feature --priority High

# Create an issue with attachments
redmine issues create --project myproject --subject "Bug report" --attach /path/to/screenshot.png --attach /path/to/log.txt
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
| `--attach` | File path to attach (repeatable) |

```bash
# Update an issue and add an attachment
redmine issues update 123 --notes "Fixed the bug" --attach /path/to/fixed_code.patch
```

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
redmine issues assign <id> <user-id-or-name> [flags]
```

The user argument accepts a numeric ID, login, full name, or `me`.

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

## Open in Browser

```bash
redmine issues open <id>
```

Opens the issue in your default web browser. Constructs the URL from your configured Redmine server.

```bash
# Opens https://redmine.example.com/issues/123 in the browser
redmine issues open 123
```

## Browse Issues (TUI)

```bash
redmine issues browse [flags]
```

Opens an interactive terminal browser for issues. Accepts the same filter flags as `list`.
