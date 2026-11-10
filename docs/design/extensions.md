# Design: `redmine extension` (CLI + MCP extensions)

Status: accepted, in implementation (branch `feat/extensions`).

## Goal

Users install additional top-level commands (e.g. `redmine pr list 1234`) from
GitHub repositories, similar to `gh extension`. An extension author describes
each command **once** in a manifest; redmine-cli derives from that single
definition:

- a cobra command (flags, positionals, help, completion, validation, rendering),
- an MCP tool (`input_schema`, validation, write gating),
- a published JSON Schema for IDE validation of the manifest.

The extension itself only implements a handler: read JSON from stdin, write
JSON to stdout, call Redmine through `redmine api`.

## Decisions

| Topic | Decision |
|---|---|
| Repo naming | `owner/redmine-cli-<name>`; command is `redmine <name>` |
| Executable name | `redmine-cli-<name>` (`.exe` on Windows) |
| Binary release assets | `redmine-cli-<name>-<goos>-<goarch>[.exe]` (raw binary, not archived) |
| Discovery topic | `redmine-cli-extension` |
| Extension name rule | `^[a-z0-9][a-z0-9-]*$`, max 32 chars, must not equal a core command or alias, or `extension`/`ext`/`help`/`completion` |
| Credentials | Never injected into the extension. Extensions call `$REDMINE_CLI_BIN api ...`. No opt-in. |
| Core vs extension name clash | Core always wins; `redmine extension exec <name>` is the escape hatch |
| Subcommand depth | One level: `redmine <ext> <command>` |
| Table columns | `path` (dot notation) **or** `jq` expression per column |
| MCP tool names | `<ext>_<command>` with `-` → `_`; max 64 chars |
| MCP groups | per extension `ext-<name>`, plus umbrella group `extensions` |
| MCP default | stdio: on. HTTP: off unless explicitly enabled |
| Attestation for extensions | Out of scope for this iteration (checksums are in scope) |
| SchemaStore registration | Out of scope (external PR); modeline + `.vscode` scaffold are in scope |

## Storage

- Root: `$REDMINE_EXTENSIONS_DIR` if set (tests), else
  `$XDG_DATA_HOME/redmine-cli/extensions`, else `~/.local/share/redmine-cli/extensions`;
  on Windows `%LOCALAPPDATA%\redmine-cli\extensions`.
- One directory per extension: `<root>/redmine-cli-<name>/`.
  - **binary**: `redmine-cli-<name>[.exe]`, optional `redmine-extension.yml`.
  - **git** (script): a git clone; executable `redmine-cli-<name>` at repo root;
    optional `redmine-extension.yml` at repo root.
  - Install metadata for every kind (owner, repo, tag, sha256, pinned,
    installed_at, kind, checksum_verified) lives in `<root>/.meta/<name>.yml`,
    never inside the extension directory: a file there could come from the
    repository and forge the extension's origin.
  - **local**: `<root>/redmine-cli-<name>` is a symlink to a local directory
    (Windows: a small link file containing the path). The manifest is re-read on
    every start.
- Writes are atomic (temp dir + rename). Removal deletes the directory / link only.
- Checksums: verified when the release publishes a checksums file. Otherwise
  the install warns. Upgrading from a checksum-verified version to one that
  cannot be verified fails unless `--force`. A release without a manifest asset
  installs as passthrough with a warning.

## Commands (`redmine extension`, alias `ext`)

| Command | Behaviour |
|---|---|
| `install <owner/repo \| https URL \| .>` | `--pin <tag>`, `--force`, `--yes`. Binary if the release (latest or pinned) has a matching asset; otherwise `git clone` (requires `git`, `--pin` checks out the tag). `.` installs the current directory as a local extension (dir name must be `redmine-cli-<name>`). Before installing a third-party extension, shows a trust prompt (see "Trust prompt"). Validates the manifest (if any) and prints the CLI commands and MCP tools it adds. |
| `list` | name, repo, version/ref, kind, pinned, #commands, #mcp tools. Structured output (`-o json`, `--jq`). |
| `upgrade <name> \| --all` | `--dry-run`, `--force`. Pinned and local extensions are skipped with a note. Binary: compare tag with latest. Git: `git pull --ff-only`. |
| `remove <name>` | |
| `exec <name> [args...]` | Runs an extension even if its name is shadowed by a core command. |
| `search [query]` | GitHub search for repos with topic `redmine-cli-extension`; marks installed ones. Structured output. |
| `create <name>` | `--precompiled=go` or default script. Scaffolds `redmine-cli-<name>/` with executable/handler, `redmine-extension.yml` (with `yaml-language-server` modeline), `.vscode/settings.json` (`yaml.schemas` mapping), README, and for Go a `.github/workflows/release.yml` producing the asset names above. |
| `check [name \| .]` | Validates the manifest (schema + semantic rules) and, with `--command <c> --input '<json>'`, runs one command through the exact CLI/MCP protocol and prints the result. |

All new commands must be registered in `internal/cmd/output_contract_test.go`
and declare `Args`.

Trust prompt: on a TTY ask "Extensions are not verified by redmine-cli and run
with your user permissions (also as MCP tools). Install owner/repo? [y/N]".
Non-TTY: require `--yes` (consistent with existing confirm handling in
`cmdutil/confirm.go`). Local installs (`.`) do not prompt.

## Manifest `redmine-extension.yml`

```yaml
# yaml-language-server: $schema=https://redmine-cli.dev/schema/redmine-extension.v1.json
schema_version: 1
name: pr
description: Work with linked pull requests
min_cli_version: 2.9.0          # optional
commands:
  - name: list
    description: List PRs/MRs linked to a Redmine issue
    writes: false               # default false
    destructive: false          # default false; implies confirmation in CLI
    mcp: true                   # default true; false = CLI only
    timeout: 30s                # default 60s; > 0 and ≤ 5m (ms, s, m, or NmMs)
    params:
      - { name: issue_id, type: integer, required: true, positional: true, description: Redmine issue ID }
      - { name: state, type: string, enum: [open, merged, closed], default: open }
      - { name: label, type: string, array: true }
      - { name: extra, type: json }
    output:
      table:                    # for array results (CLI table format)
        - { header: ID, path: id }
        - { header: Title, path: title }
        - { header: Author, jq: '.author.login' }
      detail:                   # for object results (CLI table format)
        - { key: State, path: state }
  - name: shell
    raw: true                   # gh-style passthrough, CLI only, no params/output
    description: Interactive shell
```

Parameter types: `string`, `integer`, `number`, `boolean`, `json`; `array: true`
makes a list (repeatable flag); `enum` for string/integer; `default`;
`required`; `positional` (positionals are ordered as declared; an array
positional may only be last). `json` accepts a JSON string or `@file` in the CLI
and any JSON value over MCP.

Semantic rules (enforced in Go, beyond the JSON Schema):
unique command names and param names; command names match
`^[a-z][a-z0-9-]*$`, param names `^[a-z][a-z0-9_]*$`; no two MCP-enabled
commands map to the same tool name;
`default` within `enum` and of the right type; positional ordering rules; raw
commands have no `params`/`output` and are never MCP tools; MCP tool name ≤ 64
chars; `output` paths non-empty and `jq` expressions compile;
`schema_version` supported; `min_cli_version` ≤ running version (dev builds
skip this check); `name` equals the extension name; `required` is not combined
with `default`; boolean params are not arrays; enum values are unique.
`schema_version` and `min_cli_version` are checked first, so an older CLI
reports the version it needs rather than unknown fields. Schema and semantic
errors are reported together, sorted by line.

Strict vs lenient: `install`, `upgrade` and `extension check` reject unknown
fields. Loading an installed extension at runtime (dispatch, MCP) ignores them
with one warning per invocation, so an extension written for a newer v1 keeps
working on older CLIs.

Extensions **without** a manifest are pure passthrough (`gh`-style): `redmine
<name> args...` runs the executable with the args, CLI only.

## Execution protocol (shared by CLI and MCP)

Declarative command:
- argv: none beyond the executable (script extensions on Windows go through
  `sh` from Git for Windows, as gh does).
- stdin: the validated input object as JSON (param names as keys; defaults
  applied; absent optional params omitted).
- env (in addition to the inherited environment):
  - `REDMINE_CLI_BIN` – absolute path of the running redmine binary
  - `REDMINE_EXT_NAME`, `REDMINE_EXT_COMMAND`, `REDMINE_EXT_SURFACE=cli|mcp`
  - `REDMINE_MCP=1` and `REDMINE_MCP_TOOL=<tool>` on the MCP surface
  - `REDMINE_PROFILE` (effective profile), `REDMINE_CONFIG` (if `--config` set),
    `REDMINE_SERVER` (if `--server` set), `REDMINE_HEADERS` (if `--header`
    flags set; newline-separated `Name: value`), `REDMINE_READ_ONLY=1` (if
    read-only is active **or** the command has `writes: false`),
    `REDMINE_NO_COLOR=1` (if `--no-color`), `REDMINE_NO_UPDATE_CHECK=1` (always)
  - `--api-key` / `--api-key-file` are never forwarded; when given together
    with an extension command, warn once on stderr. Inherited `REDMINE_API_KEY`
    etc. in the environment are neither added nor stripped.
- stdout: JSON (object/array/scalar). Non-JSON stdout is shown as text.
- exit ≠ 0: failure; stderr is the error message (sanitized via
  `output.SanitizeTerminal`/`textsafe`, truncated).
- signals: SIGINT, SIGTERM and (Unix) SIGHUP kill the process group; on Linux
  `Pdeathsig` is a backstop. `mcp serve` stops in-flight extension runs before
  it exits.
- limits: timeout per command (process group killed; Windows: process),
  stdout ≤ 10 MiB, captured stderr ≤ 64 KiB, context cancellation kills.

CLI surface specifics: stderr of the extension is streamed live to the user's
stderr (still bounded for the error message); stdout is captured and rendered
via the Factory printer: JSON format / `--jq` → JSON; table format → `table`
spec for arrays, `detail` spec for objects, otherwise pretty JSON. Exit codes
of failing extensions are propagated via `cmdutil.SilentError{Code}`.
`destructive: true` adds `--yes` and asks for confirmation.

Raw command / manifest-less extension: argv `[<command>] args...`, stdio
attached directly, `REDMINE_EXT_COMMAND` set for raw commands, exit code
propagated. Same env contract otherwise.

## Prerequisites in core

- `REDMINE_PROFILE` and `REDMINE_CONFIG` env vars honoured by config loading /
  global flags (flags still win).
- `REDMINE_HEADERS` env var (newline-separated `Name: value`) merged like
  `--header` (flags win over env, env wins over profile headers).
- Extract the GitHub release helpers from `internal/cmd/update` into a shared
  package (e.g. `internal/ghrelease`): limited downloads, checksum verification,
  token lookup (`GITHUB_TOKEN`/`GH_TOKEN`), HTTP client with timeouts. `update`
  keeps identical behaviour and tests.
- The startup update check in `cmd/redmine/main.go` must not run for
  `extension upgrade` and is skipped for extension invocations' nested calls
  (`REDMINE_NO_UPDATE_CHECK=1`).

## Root wiring

- Extension discovery is local-only (no network) and **injectable**: only
  `cmd/redmine/main.go` enables it (e.g. `NewRootCmdWithOptions`); `NewRootCmd`
  used by tests does not load the developer's installed extensions.
- Extensions are registered under a cobra group "Extension commands"; a broken
  manifest does not break the CLI: the extension is registered as passthrough-less
  stub that reports the manifest error when invoked, and `extension list` shows it.
- Shell completion: extension names, subcommands, flags and enum values.
- Extension upgrade notice: when an installed (non-local, non-pinned)
  extension runs on a TTY, a 24h-cached background check may print
  "A new release of <name> is available" (respects `REDMINE_NO_UPDATE_CHECK`).

## MCP integration

- `internal/mcpserver` gets a catalog built from generated tools + extension
  tools; `AllTools`/`IsKnownTool`/`ParseTools`/`ParseGroup(s)` validate
  against the catalog (dynamic groups `ext-<name>`, `extensions`).
- `Options.ExtensionTools` carries the tools; `registerTools` registers them via
  raw `srv.AddTool` with the derived `input_schema`; input is validated with
  `jsonschema-go` before exec; the result becomes `structuredContent` (+ text
  block) for JSON objects, text otherwise; failure → `isError` result.
- `writes: true` tools only with `--enable-writes`; `writes: false` tools run with
  `REDMINE_READ_ONLY=1`. `destructive` adds "Destructive." to the description.
- Results go through secret redaction: keys naming secrets (`*apikey`,
  `*password`, `token` variants, `Authorization`) are dropped
  case-insensitively, and every string value goes through text redaction.
- Enablement: `--enable-extensions` flag / `mcp.enable_extensions` (*bool) /
  `REDMINE_MCP_ENABLE_EXTENSIONS`. Unset: stdio on, HTTP off. Explicit false:
  off everywhere. The setup server (unconfigured Redmine) has no extension tools.
- Name clash with a core tool or another extension tool: skip with a stderr
  warning, server still starts.
- `redmine mcp tools` lists extension groups/tools with origin
  (`pr_list (read, extension pr@v1.2.0)`).
- The MCP process forwards its own profile/config/server/headers to the
  extension exactly like the CLI does (shared env builder).

Security note (must be in docs): read-only enforcement protects against buggy
extensions that use `redmine api`, not against malicious ones; extensions run
with the user's permissions. Install-time trust is the security boundary.

## JSON Schema for the manifest

- Single source: the Go manifest structs in `internal/extension/spec`.
- `cmd/schemagen` (go:generate) writes the schema with descriptions/enums/
  defaults/examples to:
  - `internal/extension/spec/redmine-extension.v1.json` (embedded via go:embed,
    used for validation at install/check time)
  - `docs/public/schema/redmine-extension.v1.json` (published at
    `https://redmine-cli.dev/schema/redmine-extension.v1.json`)
- A test fails if either file differs from freshly generated output.
- Conditional rules expressible as `if/then` (e.g. `enum` only for
  string/integer, raw commands without params) go into the schema; everything
  else is a semantic check.
- Verify the emitted draft works with the YAML language server; if 2020-12
  constructs are a problem, post-process to a draft-07 compatible schema.
- Validation errors carry file:line using `yaml.v3` node positions.

## Packages

- `internal/extension/spec` – manifest model, YAML parsing with positions,
  schema generation + embedded schema validation, semantic validation,
  params → JSON Schema, params → cobra flags/positionals + input object,
  output rendering spec evaluation (path / jq).
- `internal/extension/exec` (or inside `internal/extension`) – process
  execution protocol, env builder, limits, sh lookup on Windows.
- `internal/extension` – Manager: dirs, List/Install/Upgrade/Remove, GitHub
  client (base URL injectable), git runner interface (fakeable), install
  metadata, update-notice cache.
- `internal/cmd/extension` – cobra commands.
- `internal/mcpserver` – catalog + extension tool registration.

## Tests

- Unit: spec (parsing, schema, semantic rules with line numbers, flag
  derivation, input building, rendering), exec (env contract, timeout kill,
  output caps, exit codes, non-JSON), manager (temp `REDMINE_EXTENSIONS_DIR`,
  `httptest` GitHub, fake git, checksums, pin, upgrade, remove, local link,
  name validation, core-name clash).
- Dispatch: flag passthrough of global flags into env, exit-code
  propagation, raw passthrough, manifest-less passthrough, broken manifest stub,
  core-name shadowing + `exec`.
- MCP: in-memory transport with a fake script extension: tools/list, schema
  rejection, structured/text results, isError, timeout, name clash skip, write
  gating, forced read-only env, HTTP default off, groups filtering, `mcp tools`.
- Schema drift test.
- E2E (`e2e/`): local fixture extension calling `redmine api /users/current.json`
  via CLI and via MCP against the Docker Redmine; read-only enforcement.

## Docs

- `docs/src/content/docs/commands/extensions.mdx` (+ `zh-cn`)
- Guide `docs/src/content/docs/guides/writing-extensions.mdx` (manifest,
  protocol, schema/IDE setup, MCP behaviour, security note)
- MCP section in `guides/ai-agents.mdx`; `mcp serve` flag docs
- Short section in `README.md`, `README.ja.md`, `README.zh-CN.md`
- `skills/redmine-cli` mention

## Out of scope (follow-ups)

Sigstore attestation for extensions, public `pkg/extkit` helper for Go
extensions, SchemaStore registration, extension-provided MCP prompts/resources,
nested subcommands, GitHub Enterprise hosts.
