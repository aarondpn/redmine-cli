---
title: Quick Start
description: Get up and running with redmine-cli in minutes.
---

## First-Time Setup

Configure your Redmine server connection:

```bash
redmine init
```

## Common Tasks

### Working with Issues

```bash
# List open issues for a project
redmine issues list --project myproject

# View a specific issue
redmine issues get 123

# Create a new issue
redmine issues create --project myproject --subject "Fix login bug" --tracker Bug

# Update an issue
redmine issues update 123 --status "In Progress" --assignee me

# Close an issue with a comment
redmine issues close 123 --notes "Fixed in commit abc123"

# Browse issues interactively
redmine issues browse --project myproject
```

### Logging Time

```bash
# Log time interactively
redmine time log

# Log time with flags
redmine time log --issue 123 --hours 2.5 --activity Development --comment "Code review"

# View your time entries for today
redmine time list --user me --from today
```

### Searching

```bash
# Search across all resources
redmine search "login bug"

# Search only issues
redmine search issues "login bug" --project myproject

# Search open issues only
redmine search "login bug" --open-issues
```

## Name Resolution

You don't need to memorize numeric IDs. The CLI resolves names automatically:

```bash
# Use names instead of IDs
redmine issues create --project myproject --tracker Bug --priority High --assignee "John Smith"

# Use "me" for the current user
redmine issues list --assignee me
```

## Output Formats

```bash
# Default table output
redmine issues list

# JSON for scripting
redmine issues list -o json

# CSV for spreadsheets
redmine issues list -o csv

# Wide table with extra columns
redmine issues list -o wide
```
