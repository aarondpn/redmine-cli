package extension

import (
	"os"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
)

// scaffoldFile is one file written by `extension create`.
type scaffoldFile struct {
	Path    string
	Mode    os.FileMode
	Content string
}

// scaffoldFiles returns the files of a new extension. lang is "" for a
// script extension or "go" for a precompiled Go one. Templates use
// __NAME__ for the extension name; they cannot use text/template because
// the release workflow contains GitHub Actions ${{ }} expressions.
func scaffoldFiles(name, lang string) []scaffoldFile {
	exe := "redmine-cli-" + name
	files := []scaffoldFile{
		{Path: spec.FileName, Mode: 0o644, Content: manifestTemplate},
		{Path: ".vscode/settings.json", Mode: 0o644, Content: vscodeTemplate},
	}
	if lang == "go" {
		files = append(files,
			scaffoldFile{Path: "main.go", Mode: 0o644, Content: goMainTemplate},
			scaffoldFile{Path: "go.mod", Mode: 0o644, Content: goModTemplate},
			scaffoldFile{Path: ".gitignore", Mode: 0o644, Content: "/" + exe + "\n/" + exe + ".exe\n/dist/\n"},
			scaffoldFile{Path: ".github/workflows/release.yml", Mode: 0o644, Content: releaseWorkflowTemplate},
			scaffoldFile{Path: "README.md", Mode: 0o644, Content: readmeTemplate + readmeGoTemplate},
		)
	} else {
		files = append(files,
			scaffoldFile{Path: exe, Mode: 0o755, Content: scriptTemplate},
			scaffoldFile{Path: "README.md", Mode: 0o644, Content: readmeTemplate},
		)
	}
	r := strings.NewReplacer("__NAME__", name, "__SCHEMA__", spec.SchemaID, "__MANIFEST__", spec.FileName)
	for i := range files {
		files[i].Content = r.Replace(files[i].Content)
	}
	return files
}

const manifestTemplate = `# yaml-language-server: $schema=__SCHEMA__
schema_version: 1
name: __NAME__
description: The __NAME__ extension for redmine-cli
commands:
  - name: whoami
    description: Show the current Redmine user
    output:
      detail:
        - { key: ID, path: id }
        - { key: Login, path: login }
        - { key: Name, jq: '.firstname + " " + .lastname' }
  - name: echo
    description: Print the input object, a starting point for your own commands
    params:
      - { name: message, type: string, required: true, positional: true, description: Text to echo }
      - { name: tag, type: string, array: true, description: Tags to attach (repeatable) }
    output:
      detail:
        - { key: Message, path: message }
        - { key: Tags, path: tag }
`

const vscodeTemplate = `{
  "yaml.schemas": {
    "__SCHEMA__": "__MANIFEST__"
  }
}
`

const scriptTemplate = `#!/bin/sh
# redmine-cli extension "__NAME__".
#
# Each command in __MANIFEST__ runs this script with REDMINE_EXT_COMMAND set
# to its name and its validated input as a JSON object on stdin. Print the
# result as JSON on stdout; on failure, write the reason to stderr and exit
# with a non-zero status.
#
# Call Redmine through "$REDMINE_CLI_BIN" api: it uses the profile, server
# and headers of the redmine invocation, so this script never handles
# credentials. Commands without "writes: true" run read-only.
set -eu

input=$(cat)

case "${REDMINE_EXT_COMMAND:-}" in
whoami)
	exec "$REDMINE_CLI_BIN" api /users/current.json --jq .user
	;;
echo)
	printf '%s\n' "$input"
	;;
*)
	echo "unknown command: ${REDMINE_EXT_COMMAND:-<none>}" >&2
	exit 2
	;;
esac
`

const goMainTemplate = `// Command redmine-cli-__NAME__ is a redmine-cli extension.
//
// Each command in __MANIFEST__ runs this program with REDMINE_EXT_COMMAND
// set to its name and its validated input as a JSON object on stdin. It
// prints the result as JSON on stdout; on failure it writes the reason to
// stderr and exits with a non-zero status.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	input := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &input); err != nil {
			return fmt.Errorf("invalid input: %w", err)
		}
	}

	switch command := os.Getenv("REDMINE_EXT_COMMAND"); command {
	case "whoami":
		return whoami()
	case "echo":
		return printJSON(input)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func whoami() error {
	out, err := redmine("api", "/users/current.json")
	if err != nil {
		return err
	}
	var resp struct {
		User json.RawMessage ` + "`json:\"user\"`" + `
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return err
	}
	_, err = os.Stdout.Write(resp.User)
	return err
}

// redmine runs the redmine binary that started the extension. It uses the
// profile, server and headers of that invocation, so the extension never
// handles credentials. Commands without "writes: true" run read-only.
func redmine(args ...string) ([]byte, error) {
	cmd := exec.Command(os.Getenv("REDMINE_CLI_BIN"), args...)
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
`

const goModTemplate = `module redmine-cli-__NAME__

go 1.24
`

const releaseWorkflowTemplate = `# Publishes the release assets 'redmine extension install' looks for:
# one binary per platform named redmine-cli-__NAME__-<os>-<arch>[.exe], the
# manifest, and checksums.txt, which the install verifies.
name: release

on:
  push:
    tags: ["v*"]

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Build
        run: |
          set -euo pipefail
          mkdir -p dist
          for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
            goos=${target%/*}
            goarch=${target#*/}
            suffix=""
            if [ "$goos" = windows ]; then suffix=".exe"; fi
            CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -trimpath -ldflags="-s -w" \
              -o "dist/redmine-cli-__NAME__-$goos-$goarch$suffix" .
          done
          cp __MANIFEST__ dist/
          (cd dist && sha256sum * > checksums.txt)
      - name: Publish
        env:
          GH_TOKEN: ${{ github.token }}
        run: gh release create "$GITHUB_REF_NAME" dist/* --title "$GITHUB_REF_NAME" --generate-notes
`

const readmeTemplate = `# redmine-cli-__NAME__

An extension for [redmine-cli](https://github.com/aarondpn/redmine-cli) that
adds the ` + "`redmine __NAME__`" + ` command.

## Install

` + "```sh" + `
redmine extension install <owner>/redmine-cli-__NAME__
redmine __NAME__ --help
` + "```" + `

## Commands

The commands, their parameters and how results are shown are declared in
` + "`__MANIFEST__`" + `. redmine-cli turns every command into a CLI command and,
unless it sets ` + "`mcp: false`" + `, an MCP tool. Editors with the YAML language
server validate the manifest through the schema named on its first line.

## Develop

` + "```sh" + `
redmine extension install .     # link this directory as a local extension
redmine extension check .       # validate the manifest
redmine extension check . --command echo --input '{"message": "hi"}'
redmine __NAME__ echo hello --tag a --tag b
` + "```" + `

Each command receives its validated input as a JSON object on stdin and
` + "`REDMINE_EXT_COMMAND`" + ` names it. Print the result as JSON on stdout and
call Redmine with ` + "`\"$REDMINE_CLI_BIN\" api ...`" + `; the extension never
sees credentials.

Add the GitHub topic ` + "`redmine-cli-extension`" + ` so ` + "`redmine extension search`" + `
finds this repository.
`

const readmeGoTemplate = `
## Release

Build once (` + "`go build`" + `) before linking it locally. Pushing a tag such as
` + "`v1.0.0`" + ` runs ` + "`.github/workflows/release.yml`" + `, which publishes the
binaries, the manifest and ` + "`checksums.txt`" + ` that
` + "`redmine extension install`" + ` downloads.
`
