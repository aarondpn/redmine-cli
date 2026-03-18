---
title: Output Formats
description: Controlling how redmine-cli displays data.
---

Most commands support the `-o` / `--output` flag to control the output format.

## Table (default)

The default format, optimized for human readability:

```bash
redmine issues list --project myproject
```

```
ID    TRACKER  STATUS  PRIORITY  SUBJECT
123   Bug      Open    High      Fix login timeout
124   Feature  Open    Normal    Add dark mode
```

## Wide

Extended table with additional columns:

```bash
redmine issues list --project myproject -o wide
```

Includes extra fields like assignee, updated date, and done ratio.

## JSON

Machine-readable output, ideal for scripting and piping:

```bash
redmine issues list --project myproject -o json
```

```json
[
  {
    "id": 123,
    "tracker": {"id": 1, "name": "Bug"},
    "status": {"id": 1, "name": "Open"},
    "priority": {"id": 2, "name": "High"},
    "subject": "Fix login timeout"
  }
]
```

Useful with tools like `jq`:

```bash
# Get IDs of all open bugs
redmine issues list --tracker Bug -o json | jq '.[].id'

# Count issues by status
redmine issues list --project myproject -o json | jq 'group_by(.status.name) | map({status: .[0].status.name, count: length})'
```

When `-o json` is selected, the CLI emits JSON only on `stdout`. Human-readable pagination hints are suppressed in this mode, so piping to tools like `jq` is safe. Keep `stderr` separate if you want to capture actual errors.

## CSV

Comma-separated values for spreadsheets and data analysis:

```bash
redmine issues list --project myproject -o csv
```

```
ID,Tracker,Status,Priority,Subject
123,Bug,Open,High,Fix login timeout
124,Feature,Open,Normal,Add dark mode
```

## Setting a Default Format

You can set a default output format in your config file:

```yaml
# ~/.redmine-cli.yaml
output_format: table
```

The `-o` flag always overrides the default.
