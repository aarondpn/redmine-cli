<p align="center">
  <img src="docs/public/favicon.svg" alt="redmine-cli logo" width="120" />
</p>

<h1 align="center">redmine-cli</h1>

<p align="center">
  A command-line interface for <a href="https://www.redmine.org/">Redmine</a> project management.
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
go install github.com/aarondpn/redmine-cli@latest
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
