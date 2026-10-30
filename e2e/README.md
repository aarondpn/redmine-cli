# Local Redmine E2E

This directory contains a local end-to-end harness for `redmine-cli`.

It uses:

- Docker Official Image `redmine:7.0` by default
- `postgres:16-alpine`
- The default Redmine admin account to derive an API key for CLI tests

The harness is version-aware. The supported matrix in this repo is:

- `4.2`
- `5.1`
- `6.1`
- `7.0`

If you want to test a custom image, set `E2E_IMAGE` when you start the stack.

## Start Redmine

```bash
make e2e-up
```

That starts Docker Compose from [compose.yaml](compose.yaml), waits until Redmine is reachable, and bootstraps the instance for CLI testing by enabling the Redmine REST API.

To choose a supported Redmine line explicitly:

```bash
make e2e-up E2E_VERSION=4.2
make e2e-up E2E_VERSION=5.1
make e2e-up E2E_VERSION=6.1
make e2e-up E2E_VERSION=7.0
```

To override the Redmine image:

```bash
make e2e-up E2E_IMAGE=your-registry/redmine:7.0-custom
```

It has to be a make variable, not an environment variable: the recipe passes
`REDMINE_IMAGE=$(E2E_IMAGE)` to Docker Compose itself, and that assignment
wins over anything inherited from the environment.

## Write a local CLI config

```bash
make e2e-config
```

By default this writes `./.redmine-cli.e2e.yaml` in the repo root using the admin user's Redmine API key against `http://127.0.0.1:3000`.

## Run the Go e2e suite

```bash
make e2e-test
```

The tests build the CLI once, then each test provisions its own project/issue
fixtures (cleaned up via `t.Cleanup`) and drives the CLI against the local
Redmine instance. The suite is split into topical files:

| File                          | Coverage |
|-------------------------------|----------|
| `contract_cases_test.go`      | `TestContract`: raw Redmine JSON vs `-o json` vs MCP for ~40 resources (see below) |
| `contract_gates_test.go`      | `TestContract_Coverage` (table of version-gated keys), `TestContract_Enumerations`, `TestContract_EveryReadToolHasACase` |
| `contract_helpers_test.go`    | raw fetch, untyped CLI/MCP runners, seeded-world discovery |
| `projects_test.go`            | project create / get / list / delete |
| `issues_test.go`, `issues_*_test.go`, `issue_*_test.go` | issue lifecycle, update, comment, assign, attachments, status, delete, list filters, edge cases |
| `assignee_test.go`            | group assignment by name, `--assignee none`, MCP unassign, non-admin name resolution |
| `time_entries_test.go`, `time_summary_test.go` | time log / list / update / delete, summaries |
| `search_test.go`              | `search --issues` and `search --projects` scope |
| `wiki_test.go`                | wiki pages: create / get / update (sections, parents, versions) / delete |
| `versions_test.go`, `categories_test.go`, `files_test.go`, `attachments_test.go` | version, category, project file and attachment commands |
| `users_test.go`, `groups_test.go`, `memberships_test.go`, `my_account_test.go`, `roles_test.go` | user / group / membership / account / role commands |
| `metadata_test.go`, `custom_fields_test.go`, `queries_test.go` | trackers, statuses, custom fields, saved queries |
| `json_collections_test.go`    | `[]` vs absent for included collections that are empty |
| `output_formats_test.go`, `jq_test.go`, `text_file_flags_test.go`, `global_flags_test.go`, `completion_test.go` | output formats, `--jq`, file flags, global flags, shell completion |
| `auth_test.go`, `auth_lifecycle_test.go` | basic-auth profile, `auth` commands |
| `api_test.go`                 | raw `api` passthrough |
| `errors_test.go`              | error envelope codes |
| `mcp_test.go`, `mcp_extended_test.go`, `mcp_wiki_test.go`, `mcp_secrets_test.go` | MCP `serve` over stdio + HTTP, group filter, write gating, auth token, secret redaction, tool catalogue golden |
| `version_gate_test.go`, `contract_tables_test.go` | server-free tests of the version comparator and the contract tables |
| (API decoding regressions, issue #201) | Spread over the topical files: `projects_test.go` default assignee, `users_test.go` / `my_account_test.go` nullable `admin` / `status` (admin vs non-admin viewer, `users list` before / after 6.0), `groups_test.go` group memberships with project + roles, `roles_test.go` no builtin columns, `issues_test.go` always-present `is_private` / `private_notes`, `files_test.go` `downloads: 0`, `search_test.go` + `mcp_extended_test.go` `all_words` default vs `=false` |

Shared infrastructure lives in `e2e_test.go` (TestMain + `requireE2E`),
`runner_test.go` (CLI runner + profile constructors), `helpers_test.go`
(envelope types + env accessors) and `fixtures_test.go` (project / issue
fixtures, tracker / activity lookups).

`bootstrap-redmine.sh` seeds the fixtures that have no REST endpoint: a public
saved query, an issue custom field (`E2E Severity`, applied to all projects)
and a role-restricted time entry custom field (`E2E Billing Code`, seeded
`visible: false` so the role restriction has meaning). The seeding uses `save!`
throughout: a fixture that cannot be created should stop the bootstrap, not
degrade quietly into a confusing test failure later.

It then runs `seed-contract.rb` (piped to `rails runner -`) to build the
"contract world" described below. A stack created before that script existed
has to be recreated (`make e2e-down e2e-up`); the contract tests fail with an
instruction to do so rather than testing less. To re-seed a running stack
without recreating it, pipe the script in by hand
(`docker compose -f e2e/compose.yaml exec -T redmine bundle exec rails runner - < e2e/seed-contract.rb`);
it is idempotent.

Run only the contract tests with `make e2e-test E2E_RUN=TestContract`.

### Contract tests

`TestContract` guards against Redmine returning data the CLI silently drops.
For every case in `contract_cases_test.go` it

1. fetches the raw payload straight from Redmine over HTTP (not through the
   CLI's own `api` command, which runs through the code under test),
2. runs the matching `redmine ... -o json` command,
3. calls the matching MCP tool,

and diffs raw against each of the other two with `internal/testutil/contract`.
What Redmine returned decides what must be there, so the same test adapts to
each Redmine line without any version knowledge: a key 4.2 does not send is not
expected, a key 7.0 sends and the models lack fails the test.

A finding is one of `MISSING` (key dropped, the issue #202 bug class),
`ZERO-DROP` (a zero value removed by `omitempty`), `VALUE` (changed value or
type), `EXTRA` (invented non-zero data), `ARRAY-LEN` or `LEAK` (a redacted key
is present). Numbers are compared numerically (`8.0` equals `8`); a string `"5"`
is not the number `5`. A raw `null` counts as an absent key.

If a contract case fails, fix the model in `internal/models/`. The allowlist is
only for deliberate reshaping, and every rule needs a `Reason`:

- the CLI prints lists as bare arrays, so the envelope (`total_count`,
  `offset`, `limit`) is only compared on the MCP side, by construction;
- MCP redacts `api_key` (`redactedAPIKey`, with a positive assertion that the
  key really is absent);
- `knownZeroDrops` lists fields whose `omitempty` predates the contract tests
  (`is_private`, `private_notes`, `homepage`, `inherit_members`, `regexp`,
  `comments`, `downloads`). Remove an entry when its `omitempty` goes away.

A rule that matches nothing fails the test (unless `Optional`), so the list
cannot go stale. The payloads of failing cases are written to
`e2e/_artifacts/<version>/<case>/` (raw.json, cli.json, mcp.json) and uploaded
by CI.

To cover a new resource add a `contractCase` (raw path with a superset
`include=` list, the CLI args, the MCP tool). Placeholders such as `{parent}`
or `{P}` resolve to the seeded world. `TestContract_EveryReadToolHasACase`
fails for a read-only MCP tool without a case or a reasoned `contractNoCase`
entry. Enumerations have neither a command nor a tool and are checked through
the API client by `TestContract_Enumerations`.

### The contract world and the gate table

A diff over a payload that never contains the interesting key passes
vacuously, so `seed-contract.rb` creates one of everything: custom fields on
versions, groups, users, time entries and activities, a three-level issue tree,
a closed issue, an edited journal note, time entries (so versions and parents
have `spent_hours`), a user with two-factor, `passwd_changed_on`, an LDAP auth
source and a membership inherited through a group, builtin-group memberships, a
changeset linked to an issue, an issue status description, a wiki page with a
parent, and reactions where the model exists. It uses fixed names
(`e2e-contract`, `e2econtract`, ...) that the tests look up through the REST
API. The script must run unchanged on Ruby 2.7 / Rails 5.2 (Redmine 4.2) up to
Rails 8: plain syntax, and capabilities probed with `column_names` /
`defined?` rather than the version string.

`contract_gates_test.go` holds the single table of version-gated keys. Each
row has the case, the path below its root (`[]` fans out over arrays), the
first Redmine line that sends it (`Since`), the upstream reference and the
fixture condition. `TestContract_Coverage` asserts both directions: from
`Since` on, the raw payload must contain a non-null value at the path
(otherwise the fixture does not exercise it); below `Since` it must not
(otherwise the table is stale). A wrong `Since` therefore fails loudly on the
matrix instead of rotting. With `CI=true` an unset or unparseable
`REDMINE_E2E_VERSION` is a failure, not a skip.

### Writing version-related tests

Some behaviour only exists from a given Redmine line onwards. Pick the
mechanism by what is being asserted:

- **A JSON key appears from release N on**: do not write a test. Add a row to
  the gate table in `contract_gates_test.go` (and, if the resource is not yet
  covered, a `contractCase`). Never assert "the JSON has key X" behind an
  inline `redmineAtLeast`; the table is the one place that records which
  release introduced which key, and the coverage test keeps it honest.
- **Behaviour differs by release** (Markdown vs Textile, a command's
  behaviour, an error message): a hand-written test gated with the helpers in
  `helpers_test.go` rather than parsing `REDMINE_E2E_VERSION` inline.
  `skipBelowRedmine(t, 7, 0, "...")` skips on older servers and
  `redmineAtLeast(5, 0)` branches. Pair `skipBelowRedmine` with
  `skipIfVersionUnknown`: an unset `REDMINE_E2E_VERSION` counts as "new
  enough", and an unknown old server would otherwise report a missing new
  field as a Redmine bug. Name these `Test<Feature>_Redmine<N>_<What>`.
- **Behaviour depends on configuration, permissions or plugins** (group
  assignment, SCM, LDAP, gravatar): gate on the capability, not the release.
  In fixtures that means probing in Ruby (`Journal.column_names.include?(...)`,
  `defined?(Reaction)`); in tests, probe through the REST API.

Skip only when the feature cannot exist on the running version. A missing
fixture, an unreachable endpoint or a key that is absent although the table
says the release sends it is a `Fatal`, never a skip.

Adding a Redmine line to the matrix (`SUPPORTED_E2E_VERSIONS` in the Makefile
and the workflow) also needs a run of the whole suite on it, a review of every
row in the gate table, and a line in this README.

### Adding a new test

1. Create `<feature>_test.go` with `//go:build e2e` at the top.
2. Call `requireE2E(t)` first.
3. Build a runner with `newCLIRunner` (API key) or `newCLIRunnerBasicAuth`.
4. Use `createTestProject` / `createTestIssue` for any write fixtures so
   cleanup is handled automatically.
5. Decode CLI JSON output with `runner.runJSON(t, &dest, ...)` and assert on
   the decoded struct rather than the raw bytes.
6. For commands that must fail, use `runner.runExpectError(t, ...)` and
   decode into `errorEnvelope`.
7. Do not re-assert that decoded fields show up in `-o json`; that is what
   the contract tests are for. Test behaviour: writes, filters, name
   resolution, errors, rendering.

To run the full supported-version matrix sequentially:

```bash
make e2e-matrix
```

The matrix uses separate ports per Redmine line to avoid collisions:

- `4.2` -> `http://127.0.0.1:3402`
- `5.1` -> `http://127.0.0.1:3501`
- `6.1` -> `http://127.0.0.1:3601`
- `7.0` -> `http://127.0.0.1:3700`

## Tear it down

```bash
make e2e-down
```

## Environment overrides

- `REDMINE_E2E_BASE_URL`
- `REDMINE_E2E_USERNAME`
- `REDMINE_E2E_API_KEY`
- `REDMINE_E2E_PASSWORD` - admin password forced by `bootstrap-redmine.sh` and used by basic-auth tests. `make e2e-up` / `make e2e-test` default this to `admintest123`. Tests that need basic auth are skipped when this is empty.
- `REDMINE_E2E_TIMEOUT_SECONDS`
- `REDMINE_E2E_CONFIG_PATH`
- `REDMINE_E2E_PROFILE_NAME`
- `E2E_IMAGE` - Makefile-level override for the Redmine image. `REDMINE_IMAGE` is set by the recipe and cannot be overridden from the environment.
- `E2E_VERSION`
- `E2E_PORT`
- `E2E_RUN` - `go test -run` pattern for `make e2e-test`, e.g. `E2E_RUN=TestContract`.
- `E2E_PASSWORD` - Makefile-level override for the forced admin password (propagates into `REDMINE_E2E_PASSWORD`).
