---
title: Wiki
description: Manage project wiki pages.
---

The `wiki` command (alias: `w`) manages wiki pages within a project.

All subcommands require a project — either via `--project` or the configured default project.

## List Wiki Pages

```bash
redmine wiki list --project <identifier> [flags]
```

| Flag | Description |
|------|------------|
| `--project` | Project identifier — **required** if no default |
| `--limit` | Maximum number of results |
| `--offset` | Result offset |
| `-o, --output` | Output format |

```bash
# List wiki pages
redmine wiki list --project myproject

# JSON output
redmine wiki list --project myproject -o json
```

## View a Wiki Page

```bash
redmine wiki get <page-title> --project <identifier> [flags]
```

| Flag | Description |
|------|------------|
| `--project` | Project identifier — **required** if no default |
| `--version` | Page version number (default: latest) |
| `--include` | Include additional data (e.g. `attachments`) |
| `-o, --output` | Output format |

```bash
# View a wiki page
redmine wiki get WikiStart --project myproject

# View a specific version
redmine wiki get WikiStart --project myproject --version 3

# Include attachments
redmine wiki get WikiStart --project myproject --include attachments
```

## Create a Wiki Page

```bash
redmine wiki create <page-title> --project <identifier> --text "content" [flags]
```

| Flag | Description |
|------|------------|
| `--project` | Project identifier — **required** if no default |
| `-t, --text` | Page content in Textile/Markdown — **required** |
| `--title` | Display title (defaults to page name) |
| `--comments` | Change comment |
| `--attach` | Path to file to attach (repeatable) |
| `-o, --output` | Output format |

```bash
# Create a new wiki page
redmine wiki create MyPage --project myproject --text "h1. Hello World"

# With title and comment
redmine wiki create MyPage --project myproject --text "Content here" --title "My Page" --comments "Initial draft"

# Attach a file
redmine wiki create MyPage --project myproject --text "See diagram" --attach ./diagram.png
```

## Update a Wiki Page

```bash
redmine wiki update <page-title> --project <identifier> [flags]
```

Only flags you explicitly pass are sent — omitted flags are not changed.

| Flag | Description |
|------|------------|
| `--project` | Project identifier — **required** if no default |
| `-t, --text` | Page content in Textile/Markdown |
| `--title` | Display title |
| `--comments` | Change comment |
| `--attach` | Path to file to attach (repeatable) |

```bash
# Update page content
redmine wiki update MyPage --project myproject --text "Updated content"

# Update with a change comment
redmine wiki update MyPage --project myproject --text "New text" --comments "Fixed typo"

# Attach a file
redmine wiki update MyPage --project myproject --comments "Added diagram" --attach ./diagram.png
```

## Delete a Wiki Page

```bash
redmine wiki delete <page-title> --project <identifier> [flags]
```

This also removes all attachments and the page history. Any child pages will be re-parented to the wiki root.

| Flag | Description |
|------|------------|
| `--project` | Project identifier — **required** if no default |
| `--force` | Skip confirmation prompt |

```bash
# Delete with confirmation
redmine wiki delete MyPage --project myproject

# Skip confirmation
redmine wiki delete MyPage --project myproject --force
```
