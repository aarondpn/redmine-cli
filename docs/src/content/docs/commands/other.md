---
title: Other Commands
description: Categories, trackers, statuses, and utility commands.
---

## API

Make authenticated requests to any Redmine REST API endpoint:

```bash
# GET the current user
redmine api /users/current.json

# GET with query parameters
redmine api /issues.json -f project_id=myproject -f limit=5

# POST with JSON fields (method auto-detected)
redmine api /issues.json -F 'issue[subject]=Bug report' -F 'issue[project_id]=1'

# POST from a file (use - for stdin)
redmine api /issues.json --input body.json

# Explicit HTTP method
redmine api -X DELETE /issues/123.json

# Show response headers
redmine api /issues.json -i

# Suppress output
redmine api -X PUT /issues/123.json -F 'issue[status_id]=5' --silent
```

| Flag | Short | Description |
|------|-------|-------------|
| `--method` | `-X` | HTTP method (default: GET, or POST when body provided) |
| `--field` | `-f` | Query parameter as `key=value` (repeatable) |
| `--raw-field` | `-F` | JSON body field as `key=value` (repeatable) |
| `--input` | | Read request body from file (`-` for stdin) |
| `--include` | `-i` | Show response status line and headers |
| `--silent` | | Suppress response output |

JSON responses are pretty-printed when stdout is a terminal. Non-2xx responses still print the body but exit with code 1.

## Categories

List issue categories for a project:

```bash
redmine categories list --project <identifier>
```

## Trackers

List all available trackers:

```bash
redmine trackers list
```

## Statuses

List all issue statuses:

```bash
redmine statuses list
```

## Shell Completion

Generate completion scripts for your shell:

```bash
# Bash
redmine completion bash > /etc/bash_completion.d/redmine

# Zsh
redmine completion zsh > "${fpath[1]}/_redmine"

# Fish
redmine completion fish > ~/.config/fish/completions/redmine.fish
```

## Self-Update

```bash
redmine update
```

Checks GitHub for the latest release, downloads it with SHA256 checksum verification, and replaces the current binary. If installed via Homebrew, delegates to `brew upgrade`.
