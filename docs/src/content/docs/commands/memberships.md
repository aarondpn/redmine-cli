---
title: Memberships
description: Manage Redmine project memberships.
---

The `memberships` command (alias: `m`) manages project memberships.

## List Memberships

```bash
redmine memberships list --project <identifier> [flags]
```

| Flag | Description |
|------|------------|
| `--project` | Project name, identifier, or ID -- **required** |
| `--limit` | Maximum number of results |
| `--offset` | Result offset for pagination |
| `-o, --output` | Output format: `table`, `json`, `csv` |

## View a Membership

```bash
redmine memberships get <id> [flags]
```

| Flag | Description |
|------|------------|
| `-o, --output` | Output format |

## Create a Membership

```bash
redmine memberships create [flags]
```

Adds a user or group to a project with specified roles.

| Flag | Description |
|------|------------|
| `--project` | Project name, identifier, or ID -- **required** |
| `--user-id` | User ID to add |
| `--group-id` | Group ID to add |
| `--role-ids` | Comma-separated role IDs -- **required** |
| `-o, --output` | Output format |

Either `--user-id` or `--group-id` must be provided. They are mutually exclusive.

## Update a Membership

```bash
redmine memberships update <id> [flags]
```

Only the assigned roles can be changed on an existing membership.

| Flag | Description |
|------|------------|
| `--role-ids` | Comma-separated role IDs -- **required** |

## Delete a Membership

```bash
redmine memberships delete <id> [flags]
```

| Flag | Description |
|------|------------|
| `-f, --force` | Skip confirmation prompt |
