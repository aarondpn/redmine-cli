# redmine-cli

A command-line interface for [Redmine](https://www.redmine.org/) project management.

## Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/aarondpn/redmine-cli/main/install.sh | bash
```

This auto-detects your OS and architecture, downloads the latest release, and installs the `redmine` binary to `/usr/local/bin`.

## Manual Download

Grab the latest release for your platform from [GitHub Releases](https://github.com/aarondpn/redmine-cli/releases/latest):

| Platform      | Architecture | Download |
|---------------|-------------|----------|
| Linux         | x86_64      | `redmine-linux-amd64.tar.gz` |
| Linux         | ARM64       | `redmine-linux-arm64.tar.gz` |
| macOS         | Intel       | `redmine-darwin-amd64.tar.gz` |
| macOS         | Apple Silicon | `redmine-darwin-arm64.tar.gz` |
| Windows       | x86_64      | `redmine-windows-amd64.zip` |

## Install with Go

```bash
go install github.com/aarondpn/redmine-cli@latest
```

## Getting Started

```bash
# Configure your Redmine server and API key
redmine init

# List issues
redmine issue list

# View a specific issue
redmine issue view 123

# Log time
redmine time log
```

Run `redmine --help` to see all available commands.
