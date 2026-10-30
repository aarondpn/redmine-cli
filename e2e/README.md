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

| File                     | Coverage |
|--------------------------|----------|
| `projects_test.go`       | project create / get / list / delete |
| `issues_test.go`         | issue lifecycle (create/list/close/reopen), update, comment, assign, attachment upload |
| `issues_list_test.go`    | list filter round-trips (`--status`, `--assignee me`, `--tracker`, bogus tracker error) |
| `assignee_test.go`       | group assignment by name, `--assignee none` (unassign + list filter), MCP unassign, non-admin name resolution via project memberships |
| `time_entries_test.go`   | time log / list / update / delete with activity resolution |
| `search_test.go`         | `search --issues` and `search --projects` scope |
| `auth_test.go`           | basic-auth profile against `/users/current.json` (needs `REDMINE_E2E_PASSWORD`) |
| `api_test.go`            | raw `api` passthrough GET + POST + PUT with `--input` |
| `errors_test.go`         | error envelope codes: `not_found`, `auth_failed` |
| `mcp_test.go`            | MCP `serve` over stdio + HTTP, group filter, write gating, `--auth-token` 401 |
| `custom_fields_test.go`  | `custom-fields list` / `get` plus the Redmine 7.0 scope and role metadata |
| `custom_field_input_test.go` | `--custom-field` on issues, projects, time entries, versions, users, my-account, groups (multi-value, clearing, type-scoped name resolution) and MCP `custom_field_values` |
| `contract_*_test.go`     | raw Redmine JSON vs `-o json` vs MCP, and the table of version-gated keys (see below) |
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

It also seeds the fixtures for `custom_field_input_test.go`: a multi-value
issue field (`E2E Tags`), a name shared by an issue and a time entry field
(`E2E Shared Code`, for type-scoped name resolution), `E2E Entry Tags`
(time entry), `E2E Project Labels`, `E2E Version Owner`, `E2E User Skills`
(editable) and `E2E Group Code`. None of them is required: a required time
entry custom field would make every other test that logs time fail.

It then runs `seed-contract.rb`, which seeds the data the contract tests
need (it is idempotent; re-run it on a live stack with
`docker compose -f e2e/compose.yaml exec -T redmine bundle exec rails runner - < e2e/seed-contract.rb`).

### Contract tests

`TestContract` fetches each resource's raw JSON from Redmine and diffs it
against `redmine ... -o json` and the matching MCP tool. A key Redmine sends
that the CLI drops or changes fails the test. The raw payload defines what is
expected, so the same test works on every Redmine version. Fix failures in
`internal/models/`; allowlist entries need a `Reason`. Run only these with
`make e2e-test E2E_RUN=TestContract`; failing payloads land in `e2e/_artifacts/`.

### Version-related tests

- A JSON key that exists from Redmine N on: add a row to the gate table in
  `contract_gates_test.go`, no hand-written test.
- Behaviour that differs by version: gate with `skipBelowRedmine(t, 7, 0, "…")`
  or `redmineAtLeast(5, 0)` from `helpers_test.go`. An unset
  `REDMINE_E2E_VERSION` counts as the newest version.
- Behaviour that depends on configuration or plugins: probe the capability,
  not the version.

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
