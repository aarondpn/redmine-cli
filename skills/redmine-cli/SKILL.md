---
name: redmine-cli
description: Use the `redmine` CLI to interact with Redmine. Activate when the user asks to create, list, update, close, or search issues, epics, or subtasks, log or view time entries, read or edit wiki pages, manage versions or memberships, query projects/users/groups, or perform any Redmine project management task. Also activate when the user says "redmine", "issue", "ticket", "time entry", "wiki", pastes a Redmine URL (e.g. `.../issues/123`), or references Redmine workflows.
---

# Redmine CLI

A CLI for the Redmine REST API. Use `redmine <command> --help` for detailed flags and examples — this skill only covers what `--help` cannot tell you.

## Available Commands

Only these top-level commands exist. Do NOT invent subcommands that aren't listed here — run `redmine <command> --help` to discover subcommands.

| Command | Purpose |
|---------|---------|
| `issues` | Create, list, get, update, close, reopen, assign, comment, delete, search, browse issues; manage watchers and relations (`issues watchers …`, `issues relations …`) |
| `attachments` | Inspect attachment metadata (`attachments get <id>`) and download attachment files (`attachments download <id>`) using the active profile's auth |
| `queries` | List Redmine saved queries; reuse them via `issues list --query` / `--query-id` |
| `projects` | List, get, create, update, archive, unarchive, delete projects; list project members. `--include` on list/get exposes trackers, modules, categories, custom fields, and time-entry activities (Redmine 5.0+ for archive). |
| `time` | Log, list, get, update, delete, summarize time entries |
| `versions` | Create, list, get, update, delete project versions (milestones) |
| `files` | List and upload project-level files (release artifacts) |
| `memberships` | List, get, create, update, delete project memberships |
| `users` | List, get, create, update, delete users |
| `my-account` | Get and update your own Redmine account (works without admin) |
| `groups` | List, get, create, update, delete groups; add/remove users |
| `roles` | List and get roles, including their permissions |
| `categories` | List issue categories |
| `trackers` | List and get trackers |
| `statuses` | List issue statuses |
| `custom-fields` | List and get custom field definitions (admin-only endpoint) |
| `search` | Search issues, wiki, news, messages, or browse results |
| `auth` | Login, logout, list, switch, and check status of authentication profiles |
| `wiki` | List, get, create, update, delete wiki pages |
| `api` | Make raw authenticated API requests |

## Setup

If the `redmine` command is not found, install it:

```bash
curl -fsSL https://raw.githubusercontent.com/aarondpn/redmine-cli/main/install.sh | bash
```

Then run `redmine auth login` for interactive configuration. Use `redmine config` to verify an existing setup.

## Critical Rules

- **Use `--jq` (`-q`) to extract fields** — e.g. `redmine issues get 123 --jq '.status.name'`. It is built in (no external `jq` needed) and implies `-o json`. If the flag is unknown, run `redmine update`.
- **Errors go to stderr with a non-zero exit code**; on failure stdout is empty. Check the exit code — never merge stderr into stdout (`2>&1`) or discard it (`2>/dev/null`). Error code `jq_failed` means the command itself succeeded and only the filter failed — don't re-run writes.
- **Use `--limit 0`** to fetch ALL results. The default limit is 100.
- **Inspect attachments** before answering about an issue that has any (see below).
- **All name-accepting flags** (--project, --tracker, --status, --priority, --assignee, --category, --version, --activity) resolve human-readable names automatically. Don't look up IDs or option lists first — only when a name fails to resolve, check the valid values (e.g. `redmine projects get <project> --include trackers,issue_categories`) and ask the user if it's still unclear.
- **`--assignee me`** refers to the current API user.
- **`--status "*"`** shows all issues regardless of status (default is `open`).

## When Something Doesn't Work: Stop and Use `--help`

**Do NOT guess, loop, or retry with invented flags/subcommands.** If a command fails or you're unsure about the correct syntax:

1. **Run `redmine <command> --help`** (or `redmine <command> <subcommand> --help`) to see the actual available options, flags, and subcommands.
2. **Read the help output carefully** — it is authoritative and always up to date. Trust it over your own assumptions.
3. **Never invent flags or subcommands** that aren't shown in `--help`. If you think an option should exist but it doesn't appear in the help, it doesn't exist.
4. **Do not loop** — if the same command fails twice, stop and re-read the help output. Do not keep retrying with slight variations hoping one will work.
5. **Parse output with `--jq`** — never use Python scripts, awk hacks, or regex to parse CLI output. The CLI's JSON output is well-structured; use it.
6. **Ask the user** if the help output doesn't clarify things — that's better than spiraling through failed attempts.

## Permission Errors

- **Never retry with other credentials** (other profiles, API keys from env vars, CI settings, or config files). Permissions come from the user's Redmine role — stop and ask the user.
- User and group names resolve through the project's members, which works without admin. Names of people outside the project, and logins, need admin; on a permission error use `me`, a numeric ID, or a project member's name instead.

## After Creating Resources

When you create an issue, project, user, or other resource, the CLI returns the new ID. Offer the user a clickable URL so they can open it in the browser:

- **Issues**: `redmine issues open <id>` opens the issue directly. You can also provide the URL: `<server>/issues/<id>`
- **Projects**: `<server>/projects/<identifier>`
- **Users**: `<server>/users/<id>`
- **Time entries**: `<server>/time_entries/<id>/edit`

Get the server URL from `redmine config` (or from the JSON output's hints). Always mention the URL or the `open` command after a successful create so the user can quickly navigate to the new resource.

## Attachments: Always Download and Inspect Them

Issues often carry attachments (screenshots, diagrams, logs, PDFs) that contain
information not present in the text. **Whenever an issue has attachments,
download them and inspect their contents before answering** - especially
images, which frequently hold the actual error, mockup, or detail the ticket is
about.

1. **Discover attachment IDs**: `redmine issues get <id> --attachments` lists each
   attachment's `id`, `filename`, `size`, and `content_type`. With `-o json` the
   issue's `attachments[]` array is included in the output.
2. **Download one file**: `redmine attachments download <att-id> -d <dir>` saves it
   under its real filename (or `--path <file>` for an exact path, `--path -` to
   stream to stdout). No `curl`, no manual API-key handling - it reuses the active
   profile's auth.
3. **Download everything at once**: `redmine issues get <id> --download-attachments <dir>`
   pulls every attachment of the issue into `<dir>` in one step.
4. **Inspect metadata only** (no download): `redmine attachments get <att-id>`.

After downloading an image, open/read it and use what it shows. Do not answer a
question about a ticket with attachments without first looking at them.

## Non-Obvious Behaviors

- `redmine issues list` defaults to `--status open`. Use `--status closed`, `--status "*"`, or a specific status name.
- `redmine issues get <id> --journals` includes comments/history. Also available: `--children`, `--relations`, `--attachments`.
- `redmine issues update` only sends flags you explicitly pass — omitted flags are not changed.
- If `--project` is omitted, the configured default project is used (set via `redmine auth login`).
- Projects can accumulate hundreds of versions, most of them closed or locked. When you need a version for a new issue, time entry, or similar workflow, always start from `redmine versions list --open` so the shortlist stays small and you don't pick a version that can no longer accept work.
- Any date flag (`--due-date`, `--date`, `--from`, `--to`) accepts the literal keyword `today` as a shortcut for the current date.
