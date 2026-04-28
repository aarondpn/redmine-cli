# MCP Server Roadmap

A plan for evolving `redmine-cli`'s MCP subcommand into a full-featured server with low ongoing maintenance cost.

## Current state (snapshot)

- SDK: `github.com/modelcontextprotocol/go-sdk v1.5.0`
- Transport: stdio only (`internal/cmd/mcp/serve.go:51`)
- Registered tool groups (`internal/mcpserver/register.go:12`): issues, projects, time entries, users, search, meta, wikis, memberships
- Resources: present, no pagination, limited templates
- Prompts: none
- Auth: single profile per process start
- Tests: 5 MCP-specific files

## Goal

A "fully fledged" MCP server with broad Redmine coverage, prompts, HTTP transport, and a design that is materially easier to maintain and extend. Adding a new endpoint should be mechanical, schemas should stay aligned with implementation, and docs should be derived from the same source of truth wherever practical.

---

## Strategy: shared op layer + codegen

Each existing MCP tool is a thin wrapper: arg struct -> `client.Service.Method` -> result struct. ~90% mechanical. The maintenance pain is duplicating this 60-100 times across tools, CLI commands, tests, and docs.

### Target architecture

```
internal/models       types + filter structs (already exists)
internal/api          HTTP client (already exists)
internal/ops          NEW: business operations, returns plain Go values
internal/cmd          cobra wrappers (call ops, format output)
internal/mcpserver    MCP wrappers (call ops, return JSON)
prompts/*.md          MCP prompt templates (data, not code)
```

`internal/cmd/issue/list.go` and `internal/mcpserver/tools_issues.go` both call `client.Issues.List` today. Extracting an `ops` layer removes duplicated business logic and payload-shaping, even though CLI commands will still own interactive concerns like flag handling, resolution, completions, and output formatting.

### Codegen pass

Annotate `ops` functions with directive comments. A `go generate` pass walks `ops/`, reads function signatures + struct tags + directives, and emits `internal/mcpserver/zz_generated_tools.go`.

```go
// ops/issues.go

//mcp:tool name=list_issues writes=false
//mcp:desc "List Redmine issues matching filters."
func ListIssues(ctx context.Context, c *api.Client, f IssueFilter) (IssueListResult, error) { ... }
```

Inputs to the generator:
- function signature -> arg struct + result schema
- existing `jsonschema:"..."` struct tags -> per-field descriptions
- `//mcp:` directives -> tool name, write-mode flag, prompt bindings

Adding an endpoint becomes: write the `ops` function + tag it. MCP tool registration and MCP-facing docs can then be generated automatically. CLI flags and command ergonomics remain handwritten, because they currently include resolution, completions, defaults, and presentation concerns that do not map cleanly to MCP.

### Prompts as data

Load `prompts/*.md` at startup with frontmatter:

```markdown
---
name: triage_issue
arguments: [issue_id]
---
Body template referencing {{.issue_id}}
```

Adding a prompt = add a file. Zero code change.

### Resources via registry

Single slice `[]ResourceTemplate{ {URI, Handler} }`. Each resource = ~5 lines, no per-resource boilerplate. This phase is about reducing registration and parsing duplication for existing singular resources. If collection-style resources are added later, pagination should be designed as part of that separate resource model instead of being forced into the current singular templates.

### Testing via record/replay

`httptest` + golden fixtures captured against a real Redmine instance. Tool tests reduce to: "call tool X with args Y, assert response matches `testdata/X.golden.json`". The generator can scaffold these too, but the fixtures need normalization rules for unstable fields such as IDs, timestamps, and ordering so replay stays maintainable.

---

## Rollout phases

### Phase 1 — Extract `ops/` layer

Migrate the highest-churn domains (issues, time entries, projects). Both `cmd/` and `mcpserver/` call into `ops`. Proves the pattern before generalising.

Acceptance: existing tests pass; LOC in `tools_issues.go` and `cmd/issue/*.go` shrinks materially.

### Phase 2 — Codegen for MCP tool registration

Build the generator. Replace handwritten `tools_*.go` with `zz_generated_tools.go`. Keep handwritten escape hatches for any tool that needs custom behaviour.

Acceptance: `go generate ./...` regenerates the registration file; diff against the previous handwritten file is empty in behaviour (verified by integration tests).

### Phase 3 — File-based prompts + resource registry

Add `prompts/` loader. Convert resources to a registry pattern for singular resources.

Acceptance: server advertises prompts in `prompts/list`; existing singular resources move to the registry pattern without changing their URI model or behavior.

### Phase 4 — Apply to remaining domains

Roll out to: groups, attachments, relations, watchers, journals, custom queries, versions (verify wiring), wiki pages (already exists, harmonise).

Acceptance: tool coverage expands substantially across the remaining Redmine surface without reintroducing handwritten per-tool boilerplate.

### Phase 5 — Transport + docs

- Add `--http :8080` flag wiring `StreamableHTTPHandler` (go-sdk v1.5).
- README section: Claude Desktop `claude_desktop_config.json` snippet, `claude mcp add redmine ...` example.
- Auto-generate `docs/mcp/tools.md` from the same codegen pass.

Acceptance: stdio and HTTP transports both pass an integration smoke test against the MCP Inspector.

---

## Why not the alternatives

- **OpenAPI spec ingestion**: Redmine's spec is unofficial and incomplete. Generator brittleness > savings.
- **Pure reflection at runtime** (no codegen): loses jsonschema descriptions, degrades the model's ability to pick correct args.
- **Status quo + discipline**: linear maintenance cost. Every new tool keeps its schema, wiring, tests, and docs spread across multiple files. The codegen path concentrates the MCP-facing definition in one place.

---

## Out of scope (separate decisions)

- OAuth / per-user API keys (only meaningful once HTTP transport ships).
- Sampling / elicitation flows for destructive writes.
- Multi-instance routing via `X-Redmine-Profile` header.
- Collection-style MCP resources with pagination semantics.

---

## Success metrics

- New MCP endpoints are added by implementing one `ops` function and a small amount of metadata, rather than duplicating schema and registration glue.
- Handwritten MCP code is limited to true edge cases rather than the default path.
- Docs and schema stay in sync without manual intervention wherever they describe MCP tools.
- Tool coverage expands from the current core groups toward the broader Redmine surface without a linear increase in maintenance cost.
