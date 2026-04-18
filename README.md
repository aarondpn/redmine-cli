<p align="center">
  <img src="docs/public/favicon.svg" alt="redmine-cli logo" width="120" />
</p>

<h1 align="center">redmine-cli</h1>

<p align="center">
  A command-line interface for <a href="https://www.redmine.org/">Redmine</a> project management.
</p>

<p align="center">
  <b>English</b> ·
  <a href="README.zh-CN.md">简体中文</a> ·
  <a href="README.ja.md">日本語</a>
</p>

<p align="center">
  <a href="https://github.com/aarondpn/redmine-cli/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/aarondpn/redmine-cli/ci.yml?style=for-the-badge&logo=githubactions&logoColor=white&label=CI" alt="CI"></a>
  <a href="https://github.com/aarondpn/redmine-cli/releases/latest"><img src="https://img.shields.io/github/v/release/aarondpn/redmine-cli?style=for-the-badge&logo=github&logoColor=white" alt="Release"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/aarondpn/redmine-cli?style=for-the-badge&logo=go&logoColor=white" alt="Go"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge&logo=opensourceinitiative&logoColor=white" alt="License"></a>
</p>

<p align="center">
  <a href="https://www.redmine.org/projects/redmine/wiki/changelog"><img src="https://img.shields.io/badge/Redmine-4.x-B32024?style=for-the-badge&logo=redmine&logoColor=white" alt="Redmine 4.x"></a>
  <a href="https://www.redmine.org/projects/redmine/wiki/changelog"><img src="https://img.shields.io/badge/Redmine-5.x-B32024?style=for-the-badge&logo=redmine&logoColor=white" alt="Redmine 5.x"></a>
  <a href="https://www.redmine.org/projects/redmine/wiki/changelog"><img src="https://img.shields.io/badge/Redmine-6.x-B32024?style=for-the-badge&logo=redmine&logoColor=white" alt="Redmine 6.x"></a>
</p>

## Installation

### Homebrew (macOS & Linux)

```bash
brew tap aarondpn/tap
brew install redmine
```

This also installs shell completions for bash, zsh, and fish.

### Quick Install Script

```bash
curl -fsSL https://raw.githubusercontent.com/aarondpn/redmine-cli/main/install.sh | bash
```

Auto-detects your OS and architecture, downloads the latest release with checksum verification, and installs to `~/.local/bin`.

### Install with Go

```bash
go install github.com/aarondpn/redmine-cli/v2/cmd/redmine@latest
```

### Manual Download

Grab the latest release for your platform from [GitHub Releases](https://github.com/aarondpn/redmine-cli/releases/latest):

| Platform      | Architecture | Download |
|---------------|-------------|----------|
| Linux         | x86_64      | `redmine-cli-linux-amd64.tar.gz` |
| Linux         | ARM64       | `redmine-cli-linux-arm64.tar.gz` |
| macOS         | Intel       | `redmine-cli-darwin-amd64.tar.gz` |
| macOS         | Apple Silicon | `redmine-cli-darwin-arm64.tar.gz` |
| Windows       | x86_64      | `redmine-cli-windows-amd64.zip` |

### Updating

```bash
redmine update
```

Downloads and verifies the latest release via SHA256 checksum before replacing the binary.

## Getting Started

```bash
# Configure your Redmine server and API key
redmine auth login

# List issues
redmine issues list

# View a specific issue
redmine issues view 123

# Log time
redmine time log
```

Run `redmine --help` to see all available commands.

## Using with AI Agents

redmine-cli ships with an [agent skill](https://github.com/anthropics/skills) that teaches AI coding agents (Claude Code, Cursor, etc.) how to use the CLI effectively. The skill covers output formats, pagination, filtering, name resolution, and common workflows -- so the agent knows to use `-o json`, resolve ambiguous values by querying first, and use the right flags without guessing.

### Install the skill

```bash
# Install globally (available in all projects)
redmine install-skill --global

# Or install for the current project only
redmine install-skill
```

This uses the [skills](https://github.com/anthropics/skills) CLI under the hood (`npx skills add`), which requires Node.js.

### What the agent learns

Once installed, the agent will:

- Use `-o json` for all commands to get machine-readable output
- Keep `stderr` separate when capturing `-o json`; JSON is written only to `stdout`
- Query available options (trackers, statuses, versions, etc.) before creating or updating issues, rather than guessing values
- Present options to the user for selection when values are ambiguous
- Handle pagination with `--limit` and `--offset`
- Use name resolution (e.g. `--assignee "John Smith"` instead of `--assignee 42`)
- Use the `me` shorthand for `--assignee me`

### Manual setup

If you prefer not to use the skill installer, you can add the skill reference directly to your agent configuration. For Claude Code, add to `.claude/settings.json`:

```json
{
  "skills": ["aarondpn/redmine-cli:redmine-cli"]
}
```

Or copy the contents of [`skills/redmine-cli/SKILL.md`](skills/redmine-cli/SKILL.md) into your project's `CLAUDE.md` or equivalent agent instructions file.

## MCP Server

`redmine mcp serve` exposes the CLI as a [Model Context Protocol](https://modelcontextprotocol.io) server over stdio, so MCP-aware hosts (Claude Desktop, Claude Code, Cursor, ...) can drive Redmine with the same profile-backed authentication the CLI already uses.

### Features

- **Tools** mapped 1:1 onto the CLI: `list_issues`, `get_issue`, `list_projects`, `get_project`, `list_project_members`, `list_time_entries`, `summary_time_entries`, `list_users`, `me`, `search`, `list_versions`, `list_trackers`, `list_statuses`, `list_categories`, `list_wiki_pages`, `get_wiki_page`, `list_memberships`, and more.
- **Resources** for URI-addressable reads: `redmine://issue/{id}`, `redmine://project/{id-or-identifier}`, `redmine://user/{id|me}`, `redmine://time-entry/{id}`, `redmine://wiki/{project}/{page}`, `redmine://version/{id}`.
- **Read-only by default.** Mutating tools (`create_*`, `update_*`, `delete_*`, `add_issue_comment`, `assign_issue`, `close_issue`, `reopen_issue`) are only registered when `--enable-writes` is passed; without the flag they never appear in `tools/list`.
- **Authentication reuses the active profile** (or `--profile`, `--server/--api-key`, `REDMINE_*` env vars) just like every other `redmine` command.

### Configure Claude Desktop / Claude Code

Add to the host's `mcpServers` config block:

```json
{
  "mcpServers": {
    "redmine": {
      "command": "redmine",
      "args": ["mcp", "serve", "--profile", "work"]
    }
  }
}
```

To also allow the assistant to create, update, and delete issues / time entries / wiki pages, enable writes explicitly:

```json
{
  "mcpServers": {
    "redmine-write": {
      "command": "redmine",
      "args": ["mcp", "serve", "--profile", "work", "--enable-writes"]
    }
  }
}
```

Write tools are destructive; prefer leaving them disabled unless the host surfaces a per-call approval UI you trust.

## Local E2E Testing

If you want to exercise the CLI against a real Redmine instance locally, the repo now includes a Docker-based e2e harness under [e2e/README.md](/Users/aarond/Documents/Projects/github/redmine-cli/e2e/README.md:1).

The setup uses Docker Official Images with Postgres and can target the supported Redmine lines `4.2`, `5.1`, and `6.1`. By default it uses `6.1` on `http://127.0.0.1:3000`. If you want a specific supported line, set `E2E_VERSION=...` before the Make target. If you want to point the harness at a custom image later, set `REDMINE_IMAGE=...`.

```bash
make e2e-up
make e2e-config
make e2e-test
make e2e-down
```

Or run the full supported-version matrix:

```bash
make e2e-matrix
```

The Go e2e suite creates a real project and issue, checks list/get flows, and verifies close/reopen behavior against the local instance.
