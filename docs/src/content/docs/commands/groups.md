---
title: Groups
description: Manage Redmine groups.
---

The `groups` command (alias: `g`) manages Redmine groups.

## List Groups

```bash
redmine groups list [flags]
```

| Flag | Description |
|------|------------|
| `--limit` | Maximum number of results |
| `--offset` | Result offset for pagination |
| `-o, --output` | Output format |

## View a Group

```bash
redmine groups get <id-or-name> [flags]
```

Accepts a numeric ID or group name.

## Create a Group

```bash
redmine groups create [flags]
```

| Flag | Description |
|------|------------|
| `--name` | Group name — **required** |

## Update a Group

```bash
redmine groups update <id-or-name> [flags]
```

| Flag | Description |
|------|------------|
| `--name` | New group name |

## Add a User to a Group

```bash
redmine groups add-user <group-id-or-name> <user-id-or-name>
```

Both arguments accept a numeric ID or name.

## Remove a User from a Group

```bash
redmine groups remove-user <group-id-or-name> <user-id-or-name>
```

Both arguments accept a numeric ID or name.

## Delete a Group

```bash
redmine groups delete <id-or-name> [flags]
```

| Flag | Description |
|------|------------|
| `-f, --force` | Skip confirmation prompt |
