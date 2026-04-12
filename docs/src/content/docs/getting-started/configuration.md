---
title: Configuration
description: Configuring redmine-cli to connect to your Redmine server.
---

## Interactive Setup

The easiest way to configure redmine-cli is the interactive login wizard:

```bash
redmine auth login
```

This walks you through setting your server URL, authentication method, and optionally selecting a default project. The profile is saved to `~/.redmine-cli.yaml`.

## Multiple Profiles

You can authenticate to multiple Redmine instances. Each login creates a named profile:

```bash
# Login to a second instance
redmine auth login

# List all profiles
redmine auth list

# Switch active profile
redmine auth switch

# Use a specific profile for one command
redmine --profile work issues list

# Check current auth status
redmine auth status

# Remove a profile
redmine auth logout work
```

## Configuration File

The config file (`~/.redmine-cli.yaml`) uses a profile-based format:

```yaml
active_profile: work
profiles:
  work:
    server: https://redmine.work.com
    auth_method: apikey
    api_key: your-api-key
    default_project: myproject
    output_format: table
  personal:
    server: https://redmine.personal.com
    auth_method: apikey
    api_key: another-key
    output_format: wide
```

All settings are scoped per profile.

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
| `-p, --profile` | Use a specific auth profile |
| `--config` | Config file path (default `~/.redmine-cli.yaml`) |
| `--no-color` | Disable colored output |
| `-v, --verbose` | Enable debug logging |

## View Current Configuration

```bash
redmine config
```

Displays the active profile, server, auth method, default project, and output format.
