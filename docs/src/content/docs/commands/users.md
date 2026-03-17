---
title: Users
description: Manage Redmine users.
---

The `users` command (alias: `u`) manages Redmine users.

## List Users

```bash
redmine users list [flags]
```

| Flag | Description |
|------|------------|
| `--status` | Filter by status: `active`, `registered`, `locked` |
| `--name` | Filter by name or login |
| `--group` | Filter by group name or ID |
| `--limit` | Maximum number of results |
| `--offset` | Result offset for pagination |
| `-o, --output` | Output format |

## View a User

```bash
redmine users get <id-or-name> [flags]
```

Accepts a numeric ID, login, full name, or `me`.

## Current User

```bash
redmine users me [flags]
```

Shows the authenticated user's details.

## Create a User

```bash
redmine users create [flags]
```

| Flag | Description |
|------|------------|
| `--login` | Login name — **required** |
| `--password` | Password — **required** |
| `--firstname` | First name |
| `--lastname` | Last name |
| `--mail` | Email address |
| `--admin` | Grant admin privileges |

## Update a User

```bash
redmine users update <id-or-name> [flags]
```

Accepts the same flags as `create` (all optional). The user argument accepts a numeric ID, login, full name, or `me`.

## Delete a User

```bash
redmine users delete <id-or-name> [flags]
```

| Flag | Description |
|------|------------|
| `-f, --force` | Skip confirmation prompt |
