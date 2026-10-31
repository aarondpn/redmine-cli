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
  <a href="https://www.redmine.org/projects/redmine/wiki/changelog"><img src="https://img.shields.io/badge/Redmine-7.x-B32024?style=for-the-badge&logo=redmine&logoColor=white" alt="Redmine 7.x"></a>
</p>

<p align="center">
  <a href="#installation">Installation</a> ·
  <a href="#getting-started">Getting Started</a> ·
  <a href="#ai-agents">AI Agents</a>
</p>

## Installation

**Homebrew** (macOS, Linux):

```bash
brew install aarondpn/tap/redmine
```

**Install script** (macOS, Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/aarondpn/redmine-cli/main/install.sh | bash
```

**PowerShell** (Windows):

```powershell
irm https://raw.githubusercontent.com/aarondpn/redmine-cli/main/install.ps1 | iex
```

Or download a binary from [GitHub Releases](https://github.com/aarondpn/redmine-cli/releases/latest). Update with `redmine update`. More options: [installation docs](https://redmine-cli.dev/getting-started/installation/).

## Getting Started

```bash
# Configure your Redmine server and API key
redmine auth login

# Or log in with OAuth in the browser (Redmine 6.1+)
redmine auth login --oauth

# List issues
redmine issues list

# View a specific issue
redmine issues view 123

# Log time
redmine time log

# Script it: JSON output filtered with the built-in --jq
redmine issues list --project myproject --jq '.[].id'
redmine issues get 123 --jq '.status.name'
```

Run `redmine --help` to see all available commands.

`-o json` prints machine-readable JSON, and `-q/--jq <expression>` filters it (strings are printed raw, everything else as JSON). Errors always go to stderr; in JSON mode they are a single-line `{"error": {...}}` envelope on the last line of stderr, stdout stays empty and the exit code is non-zero. See [Output formats](https://redmine-cli.dev/guides/output-formats/).

<a name="ai-agents"></a>

## Using with AI Agents

redmine-cli ships an [Agent Skill](https://agentskills.io) (the open `SKILL.md` standard, supported by 35+ agents) and a built-in MCP server. Both are vendor-neutral - pick the one-liner that matches your tool:

| Tool | One-line install |
|---|---|
| **Any agent (skill)** | `npx skills add aarondpn/redmine-cli` |
| **Any MCP host** | `redmine mcp serve` (then add to host config) |
| **Docker (MCP only)** | `docker run -i --rm -e REDMINE_SERVER -e REDMINE_API_KEY ghcr.io/aarondpn/redmine-mcp` |
| **Claude Code** | `/plugin marketplace add aarondpn/redmine-cli` then `/plugin install redmine` |
| **Codex CLI** | `codex plugin marketplace add aarondpn/redmine-cli` then `/plugins` (install **Redmine** from the added marketplace) |
| **Gemini CLI** | `gemini extensions install https://github.com/aarondpn/redmine-cli` |

The skill teaches the agent the non-obvious parts of the CLI (output formats, name resolution, pagination, common workflows). The MCP server exposes the same operations as typed tool calls, read-only by default, with per-group / per-tool allow- and deny-lists.

Full configuration, host-specific snippets (Claude Desktop, Cursor, Zed, VS Code), and write-tool gating: see the [AI Agent Integration guide](https://redmine-cli.dev/guides/ai-agents/).

## Development

Local E2E testing against a real Redmine instance (Docker, supported on Redmine `4.2`, `5.1`, `6.1`, `7.0`): see [e2e/README.md](e2e/README.md).
