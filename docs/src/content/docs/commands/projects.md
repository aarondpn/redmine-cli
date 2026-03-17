---
title: Projects
description: Manage Redmine projects.
---

The `projects` command (alias: `p`) manages Redmine projects.

## List Projects

```bash
redmine projects list [flags]
```

| Flag | Description |
|------|------------|
| `--limit` | Maximum number of results |
| `--offset` | Result offset for pagination |
| `-o, --output` | Output format: `table`, `wide`, `json`, `csv` |

## View a Project

```bash
redmine projects get <identifier> [flags]
```

| Flag | Description |
|------|------------|
| `--include` | Extra data to include: `trackers`, `issue_categories`, `enabled_modules`, `time_entry_activities` |
| `-o, --output` | Output format |

## Create a Project

```bash
redmine projects create [flags]
```

| Flag | Description |
|------|------------|
| `--name` | Project name — **required** |
| `--identifier` | URL-friendly identifier — **required** |
| `--description` | Project description |
| `--public` | Make the project public |
| `--parent` | Parent project ID |

## Update a Project

```bash
redmine projects update <identifier> [flags]
```

| Flag | Description |
|------|------------|
| `--name` | New project name |
| `--description` | New description |
| `--public` | Set public visibility |

## Delete a Project

```bash
redmine projects delete <identifier> [flags]
```

| Flag | Description |
|------|------------|
| `-f, --force` | Skip confirmation prompt |

## List Members

```bash
redmine projects members <identifier> [flags]
```

Lists all members and their roles for a project.
