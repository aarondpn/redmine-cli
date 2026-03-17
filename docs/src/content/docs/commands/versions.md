---
title: Versions
description: Manage project versions (milestones).
---

The `versions` command (alias: `v`) manages project versions.

## List Versions

```bash
redmine versions list --project <identifier> [flags]
```

| Flag | Description |
|------|------------|
| `--project` | Project identifier — **required** |
| `--status` | Filter by status: `open`, `locked`, `closed` |
| `--limit` | Maximum number of results |
| `--offset` | Result offset |
| `-o, --output` | Output format |

```bash
# List open versions
redmine versions list --project myproject --status open
```

## View a Version

```bash
redmine versions get <id> [flags]
```
