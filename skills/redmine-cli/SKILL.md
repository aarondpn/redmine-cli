---
name: redmine-cli
description: Use the `redmine` CLI to interact with Redmine. Activate when the user asks to create, list, update, close, or search issues, log or view time entries, manage versions or memberships, query projects/users/groups, or perform any Redmine project management task. Also activate when the user says "redmine", "issue", "ticket", "time entry", or references Redmine workflows.
---

# Redmine CLI

A CLI for the Redmine REST API. Use `redmine <command> --help` for detailed flags and examples — this skill only covers what `--help` cannot tell you.

## Available Commands

Only these top-level commands exist. Do NOT invent subcommands that aren't listed here — run `redmine <command> --help` to discover subcommands.

| Command | Purpose |
|---------|---------|
| `issues` | Create, list, get, update, close, reopen, assign, comment, delete, search, browse issues |
| `projects` | List, get, create, update, delete projects; list project members |
| `time` | Log, list, get, update, delete, summarize time entries |
| `versions` | List, get project versions (milestones) |
| `memberships` | List, get, create, update, delete project memberships |
| `users` | List, get, create, update, delete users |
| `groups` | List, get, create, update, delete groups; add/remove users |
| `categories` | List issue categories |
| `trackers` | List trackers |
| `statuses` | List issue statuses |
| `search` | Search issues, wiki, news, messages, or browse results |
| `auth` | Login, logout, list, switch, and check status of authentication profiles |
| `api` | Make raw authenticated API requests |

## Setup

If the `redmine` command is not found, install it:

```bash
curl -fsSL https://raw.githubusercontent.com/aarondpn/redmine-cli/main/install.sh | bash
```

Then run `redmine auth login` for interactive configuration. Use `redmine config` to verify an existing setup.

## Critical Rules

- **Always use `-o json`** when you need to parse output programmatically. JSON goes to stdout only; stderr is separate.
- **Use `--limit 0`** to fetch ALL results. The default limit is 100.
- **All name-accepting flags** (--project, --tracker, --status, --priority, --assignee, --category, --version, --activity) resolve human-readable names automatically. You don't need to look up IDs first.
- **`--assignee me`** refers to the current API user.
- **`--status "*"`** shows all issues regardless of status (default is `open`).

## When Something Doesn't Work: Stop and Use `--help`

**Do NOT guess, loop, or retry with invented flags/subcommands.** If a command fails or you're unsure about the correct syntax:

1. **Run `redmine <command> --help`** (or `redmine <command> <subcommand> --help`) to see the actual available options, flags, and subcommands.
2. **Read the help output carefully** — it is authoritative and always up to date. Trust it over your own assumptions.
3. **Never invent flags or subcommands** that aren't shown in `--help`. If you think an option should exist but it doesn't appear in the help, it doesn't exist.
4. **Do not loop** — if the same command fails twice, stop and re-read the help output. Do not keep retrying with slight variations hoping one will work.
5. **Parse output with `-o json` and standard JSON tools (jq)** — never use Python scripts, awk hacks, or regex to parse CLI output. The CLI's JSON output is well-structured; use it.
6. **Ask the user** if the help output doesn't clarify things — that's better than spiraling through failed attempts.

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

## After Creating Resources

When you create an issue, project, user, or other resource, the CLI returns the new ID. Offer the user a clickable URL so they can open it in the browser:

- **Issues**: `redmine issues open <id>` opens the issue directly. You can also provide the URL: `<server>/issues/<id>`
- **Projects**: `<server>/projects/<identifier>`
- **Users**: `<server>/users/<id>`
- **Time entries**: `<server>/time_entries/<id>/edit`

Get the server URL from `redmine config` (or from the JSON output's hints). Always mention the URL or the `open` command after a successful create so the user can quickly navigate to the new resource.

## Non-Obvious Behaviors

- `redmine issues list` defaults to `--status open`. Use `--status closed`, `--status "*"`, or a specific status name.
- `redmine issues get <id> --journals` includes comments/history. Also available: `--children`, `--relations`.
- `redmine issues update` only sends flags you explicitly pass — omitted flags are not changed.
- If `--project` is omitted, the configured default project is used (set via `redmine auth login`).
- Version status filters (`--open`, `--closed`, `--locked`) on `redmine versions list` are applied client-side.
