---
title: Search
description: Search across Redmine resources.
---

The `search` command lets you search for issues, news, documents, wiki pages, messages, and projects.

## General Search

```bash
redmine search <query> [flags]
```

| Flag | Description |
|------|------------|
| `--project` | Limit to a project identifier |
| `--scope` | Search scope: `all`, `my_projects`, `subprojects` |
| `--all-words` | Match all query words |
| `--titles-only` | Search titles only |
| `--open-issues` | Only return open issues |
| `--attachments` | Attachment search: `0`, `1`, `only` |
| `--issues` | Include issues |
| `--news` | Include news |
| `--documents` | Include documents |
| `--changesets` | Include changesets |
| `--wiki-pages` | Include wiki pages |
| `--messages` | Include forum messages |
| `--projects` | Include projects |
| `--limit` | Maximum results |
| `--offset` | Result offset |
| `-o, --output` | Output format |

```bash
# Search everything
redmine search "login bug"

# Search only in a project
redmine search "login bug" --project myproject --open-issues
```

## Typed Search

Search specific resource types directly:

```bash
redmine search issues <query> [flags]
redmine search projects <query> [flags]
redmine search wiki <query> [flags]
redmine search news <query> [flags]
redmine search messages <query> [flags]
```

## Interactive Browse

```bash
redmine search browse <query> [flags]
```

Opens search results in an interactive terminal browser.
