---
title: Configuration
description: Configuring redmine-cli to connect to your Redmine server.
---

## Interactive Setup

The easiest way to configure redmine-cli is the interactive wizard:

```bash
redmine init
```

This walks you through setting your server URL, authentication method, and optionally selecting a default project. Configuration is saved to `~/.redmine-cli.yaml`.

## Configuration File

The config file (`~/.redmine-cli.yaml`) supports the following settings:

```yaml
server: https://redmine.example.com
auth_method: apikey        # "apikey" or "basic"
api_key: your-api-key
username: ""               # for basic auth
password: ""               # for basic auth
default_project: myproject # optional
output_format: table       # table, wide, json, csv
no_color: false
```

## Environment Variables

All settings can be overridden with environment variables prefixed with `REDMINE_`:

```bash
export REDMINE_SERVER=https://redmine.example.com
export REDMINE_API_KEY=your-api-key
export REDMINE_AUTH_METHOD=apikey
```

Additionally, the following variable controls the startup update check:

```bash
export REDMINE_NO_UPDATE_CHECK=1  # disable automatic update check
```

See [Self-Update](/commands/other/#startup-update-check) for details.

## Global Flags

These flags can be used with any command to override config values:

| Flag | Description |
|------|------------|
| `-s, --server` | Redmine server URL |
| `-k, --api-key` | API key for authentication |
| `--config` | Config file path (default `~/.redmine-cli.yaml`) |
| `--no-color` | Disable colored output |
| `-v, --verbose` | Enable debug logging |

## View Current Configuration

```bash
redmine config
```

Displays the active server, auth method, default project, and output format.
