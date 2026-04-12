---
title: Authentication
description: Manage multiple Redmine authentication profiles.
---

The `auth` command manages authentication profiles for connecting to multiple Redmine instances.

## login

Start an interactive login wizard to authenticate with a Redmine server:

```bash
redmine auth login
```

If you work with multiple Redmine instances, run this command again to create additional profiles.

| Flag | Description |
|------|-------------|
| `--name` | Profile name (default: derived from server hostname) |

## list

List all saved authentication profiles:

```bash
redmine auth list
```

Shows each profile's server URL and which one is currently active.

## switch

Switch the active authentication profile:

```bash
redmine auth switch
```

Opens an interactive selector to choose which profile to use. The selected profile becomes the default for all subsequent commands.

## status

Check the current authentication status:

```bash
redmine auth status
```

Displays the active profile name, server URL, and authentication method.

## logout

Remove an authentication profile:

```bash
redmine auth logout [profile-name]
```

If no profile name is given, removes the currently active profile. You'll be prompted to confirm before removal.

## Quick Reference

```bash
# Login to a new instance
redmine auth login

# Login with a specific profile name
redmine auth login --name work

# List all profiles
redmine auth list

# Switch active profile
redmine auth switch

# Check current status
redmine auth status

# Logout (remove current profile)
redmine auth logout

# Logout a specific profile
redmine auth logout work

# Use a specific profile for one command
redmine --profile work issues list
```