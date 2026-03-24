---
name: redmine-cli
description: Use the `redmine` CLI to interact with Redmine. Activate when the user asks to create, list, update, close, or search issues, log or view time entries, manage versions or memberships, query projects/users/groups, or perform any Redmine project management task. Also activate when the user says "redmine", "issue", "ticket", "time entry", or references Redmine workflows.
---

# Redmine CLI

A CLI for the Redmine REST API. Run `redmine --help` and `redmine <command> --help` to discover available commands and flags — this skill only covers what `--help` cannot tell you.

## Setup

If the `redmine` command is not found, install it:

```bash
curl -fsSL https://raw.githubusercontent.com/aarondpn/redmine-cli/main/install.sh | bash
```

Then run `redmine init` for interactive configuration. Use `redmine config` to verify an existing setup.

## Critical Rules

- **Always use `-o json`** when you need to parse output programmatically. JSON goes to stdout only; stderr is separate.
- **Use `--limit 0`** to fetch ALL results. The default limit is 100.
- **All name-accepting flags** (--project, --tracker, --status, --priority, --assignee, --category, --version, --activity) resolve human-readable names automatically. You don't need to look up IDs first.
- **`--assignee me`** refers to the current API user.
- **`--status "*"`** shows all issues regardless of status (default is `open`).

## Permission Gotcha: Users & Groups

Resolving users and groups **by name requires admin privileges**. If you get a permission error:
- Do NOT retry with the same name
- Use `me` for the current user
- To discover user IDs without admin access, extract them from other sources:
  - `redmine issues list --project <project> -o json` — the `assigned_to` and `author` fields contain user IDs and names
  - `redmine memberships list --project <project> -o json` — lists all project members with their IDs
  - `redmine issues get <id> --journals -o json` — journal entries contain user references

## Workflow: Resolving Ambiguous Values

When a command needs a value from a fixed set (tracker, status, priority, category, version, assignee) and you're not sure of the exact name:

1. **Query options first**: `redmine trackers list -o json`, `redmine statuses list -o json`, etc.
2. **Present choices to the user** via AskUserQuestion with a formatted list
3. **Use the confirmed value** in the command

For users/groups, if the list endpoint fails with a permission error, use the workarounds from the section above instead.

## Non-Obvious Behaviors

- `redmine issues list` defaults to `--status open`. Use `--status closed`, `--status "*"`, or a specific status name.
- `redmine issues get <id> --journals` includes comments/history. Also available: `--children`, `--relations`.
- `redmine issues update` only sends flags you explicitly pass — omitted flags are not changed.
- If `--project` is omitted, the configured default project is used (set via `redmine init`).
- Version status filters (`--open`, `--closed`, `--locked`) on `redmine versions list` are applied client-side.
