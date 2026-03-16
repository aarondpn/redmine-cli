# redmine-cli

A command-line interface for [Redmine](https://www.redmine.org/) project management.

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
| Linux         | x86_64      | `redmine-linux-amd64.tar.gz` |
| Linux         | ARM64       | `redmine-linux-arm64.tar.gz` |
| macOS         | Intel       | `redmine-darwin-amd64.tar.gz` |
| macOS         | Apple Silicon | `redmine-darwin-arm64.tar.gz` |
| Windows       | x86_64      | `redmine-windows-amd64.zip` |

### Updating

```bash
redmine update
```

Downloads and verifies the latest release via SHA256 checksum before replacing the binary.

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
