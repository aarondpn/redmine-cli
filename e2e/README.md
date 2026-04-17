# Local Redmine E2E

This directory contains a local end-to-end harness for `redmine-cli`.

It uses:

- Docker Official Image `redmine:6.1` by default
- `postgres:16-alpine`
- The default Redmine admin account to derive an API key for CLI tests

The harness is version-aware. The supported matrix in this repo is:

- `4.2`
- `5.1`
- `6.1`

If you want to test a custom image, set `REDMINE_IMAGE` when you start the stack.

## Start Redmine

```bash
make e2e-up
```

That starts Docker Compose from [compose.yaml](/Users/aarond/Documents/Projects/github/redmine-cli/e2e/compose.yaml:1), waits until Redmine is reachable, and bootstraps the instance for CLI testing by enabling the Redmine REST API.

To choose a supported Redmine line explicitly:

```bash
make e2e-up E2E_VERSION=4.2
make e2e-up E2E_VERSION=5.1
make e2e-up E2E_VERSION=6.1
```

To override the Redmine image:

```bash
REDMINE_IMAGE=your-registry/redmine:7.0-rc make e2e-up
```

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
| `time_entries_test.go`   | time log / list / update / delete with activity resolution |
| `search_test.go`         | `search --issues` and `search --projects` scope |
| `auth_test.go`           | basic-auth profile against `/users/current.json` (needs `REDMINE_E2E_PASSWORD`) |
| `api_test.go`            | raw `api` passthrough GET + POST + PUT with `--input` |
| `errors_test.go`         | error envelope codes: `not_found`, `auth_failed` |

Shared infrastructure lives in `e2e_test.go` (TestMain + `requireE2E`),
`runner_test.go` (CLI runner + profile constructors), `helpers_test.go`
(envelope types + env accessors) and `fixtures_test.go` (project / issue
fixtures, tracker / activity lookups).

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

To run the full supported-version matrix sequentially:

```bash
make e2e-matrix
```

The matrix uses separate ports per Redmine line to avoid collisions:

- `4.2` -> `http://127.0.0.1:3402`
- `5.1` -> `http://127.0.0.1:3501`
- `6.1` -> `http://127.0.0.1:3601`

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
- `REDMINE_IMAGE`
- `E2E_VERSION`
- `E2E_PORT`
- `E2E_PASSWORD` - Makefile-level override for the forced admin password (propagates into `REDMINE_E2E_PASSWORD`).
