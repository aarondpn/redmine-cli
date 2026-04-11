---
title: Wiki
description: Manage project wiki pages.
---

The `wiki` command (alias: `w`) manages wiki pages within a project.

All subcommands require a project — either via `--project`/`-p` or the configured default project.

## List Wiki Pages

```bash
redmine wiki list --project <identifier> [flags]
```

| Flag | Description |
|------|------------|
| `--project` | `-p` Project identifier — **required** if no default |
| `--limit` | Maximum number of results |
| `--offset` | Result offset |
| `-o, --output` | Output format |

```bash
# List wiki pages
redmine wiki list -p myproject

# JSON output
redmine wiki list -p myproject -o json
```

## View a Wiki Page

```bash
redmine wiki get <page-title> --project <identifier> [flags]
```

| Flag | Description |
|------|------------|
| `--project` | `-p` Project identifier — **required** if no default |
| `--version` | Page version number (default: latest) |
| `--include` | Include additional data (e.g. `attachments`) |
| `-o, --output` | Output format |

```bash
# View a wiki page
redmine wiki get WikiStart -p myproject

# View a specific version
redmine wiki get WikiStart -p myproject --version 3

# Include attachments
redmine wiki get WikiStart -p myproject --include attachments
```

## Create a Wiki Page

```bash
redmine wiki create <page-title> --project <identifier> --text "content" [flags]
```

| Flag | Description |
|------|------------|
| `--project` | `-p` Project identifier — **required** if no default |
| `--text` | `-t` Page content in Textile/Markdown — **required** |
| `--title` | Display title (defaults to page name) |
| `--comments` | Change comment |
| `-o, --output` | Output format |

```bash
# Create a new wiki page
redmine wiki create MyPage -p myproject -t "h1. Hello World"

# With title and comment
redmine wiki create MyPage -p myproject -t "Content here" --title "My Page" --comments "Initial draft"
```

## Update a Wiki Page

```bash
redmine wiki update <page-title> --project <identifier> [flags]
```

Only flags you explicitly pass are sent — omitted flags are not changed.

| Flag | Description |
|------|------------|
| `--project` | `-p` Project identifier — **required** if no default |
| `--text` | `-t` Page content in Textile/Markdown |
| `--title` | Display title |
| `--comments` | Change comment |

```bash
# Update page content
redmine wiki update MyPage -p myproject --text "Updated content"

# Update with a change comment
redmine wiki update MyPage -p myproject -t "New text" --comments "Fixed typo"
```

## Delete a Wiki Page

```bash
redmine wiki delete <page-title> --project <identifier> [flags]
```

| Flag | Description |
|------|------------|
| `--project` | `-p` Project identifier — **required** if no default |
| `--force` | `-f` Skip confirmation prompt |

```bash
# Delete with confirmation
redmine wiki delete MyPage -p myproject

# Skip confirmation
redmine wiki delete MyPage -p myproject --force
```
