---
title: AI Agent Integration
description: Using redmine-cli with AI coding agents like Claude Code and Cursor.
---

redmine-cli ships with an [agent skill](https://github.com/anthropics/skills) that teaches AI coding agents (Claude Code, Cursor, etc.) how to use the CLI effectively. The skill covers output formats, pagination, filtering, name resolution, and common workflows.

## Install the Skill

```bash
# Install globally (available in all projects)
redmine install-skill --global

# Or install for the current project only
redmine install-skill
```

This uses the [skills](https://github.com/anthropics/skills) CLI under the hood (`npx skills add`), which requires Node.js.

## What the Agent Learns

Once installed, the agent will:

- Use `-o json` for all commands to get machine-readable output
- Query available options (trackers, statuses, versions, etc.) before creating or updating issues, rather than guessing values
- Present options to the user for selection when values are ambiguous
- Handle pagination with `--limit` and `--offset`
- Use name resolution (e.g. `--assignee "John Smith"` instead of `--assignee 42`)
- Use the `me` shorthand for `--assignee me`
- Use `redmine auth login` for initial setup and `redmine auth switch` to change servers
- Use `redmine --profile <name>` to target a specific Redmine instance when multiple are configured

## Manual Setup

If you prefer not to use the skill installer, you can add the skill reference directly to your agent configuration.

### Claude Code

Add to `.claude/settings.json`:

```json
{
  "skills": ["aarondpn/redmine-cli:redmine-cli"]
}
```

### Other Agents

Copy the contents of [`skills/redmine-cli/SKILL.md`](https://github.com/aarondpn/redmine-cli/blob/main/skills/redmine-cli/SKILL.md) into your project's agent instructions file (e.g. `CLAUDE.md`, `.cursorrules`, etc.).
