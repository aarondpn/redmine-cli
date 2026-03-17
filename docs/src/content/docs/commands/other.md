---
title: Other Commands
description: Categories, trackers, statuses, and utility commands.
---

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
