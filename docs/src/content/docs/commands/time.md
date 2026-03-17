---
title: Time Entries
description: Log, list, and manage time entries.
---

The `time` command (alias: `t`) manages time entries.

## Log Time

```bash
redmine time log [flags]
```

When called without flags, opens an interactive form.

| Flag | Description |
|------|------------|
| `--issue` | Issue ID |
| `--project` | Project (name or identifier) |
| `--hours` | Hours spent |
| `--activity` | Activity type (name or ID) |
| `--comment` | Description of work |
| `--date` | Date (YYYY-MM-DD, default: today) |

```bash
# Log time interactively
redmine time log

# Log time with flags
redmine time log --issue 123 --hours 1.5 --activity Development --comment "Bug fix"
```

## List Time Entries

```bash
redmine time list [flags]
```

| Flag | Description |
|------|------------|
| `--project` | Filter by project |
| `--user` | Filter by user: `me`, name, or ID |
| `--issue` | Filter by issue ID |
| `--activity` | Filter by activity (name or ID) |
| `--from` | Start date (YYYY-MM-DD or `today`) |
| `--to` | End date (YYYY-MM-DD or `today`) |
| `--limit` | Maximum number of results |
| `--offset` | Result offset for pagination |
| `-o, --output` | Output format |

```bash
# Your time entries this week
redmine time list --user me --from 2024-01-15 --to 2024-01-19

# All time on an issue
redmine time list --issue 123
```

## View a Time Entry

```bash
redmine time get <id> [flags]
```

## Update a Time Entry

```bash
redmine time update <id> [flags]
```

| Flag | Description |
|------|------------|
| `--hours` | New hours |
| `--activity` | New activity |
| `--comment` | New comment |
| `--date` | New date |

## Delete a Time Entry

```bash
redmine time delete <id> [flags]
```

| Flag | Description |
|------|------------|
| `-f, --force` | Skip confirmation prompt |

## Time Summary

```bash
redmine time summary [flags]
```

Displays a summary of time entries grouped by project and activity. Accepts the same filter flags as `list`.
