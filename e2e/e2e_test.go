//go:build e2e

// Package e2e contains end-to-end tests that drive the built `redmine` CLI
// against a real Redmine instance started via docker-compose (see
// `make e2e-up`). The package is split into topical test files:
//
//   - e2e_test.go       - TestMain (builds the CLI) and the requireE2E gate.
//   - runner_test.go    - cliRunner and profile-specific constructors.
//   - helpers_test.go   - envelope types and small utilities shared by tests.
//   - fixtures_test.go  - reusable test fixtures (project, issue, ...).
//   - projects_test.go  - project CRUD.
//   - issues_test.go    - issue lifecycle, mutation, attachment, assign.
//   - issues_list_test.go - list filter round-trips.
//   - time_entries_test.go - time entry CRUD.
//   - search_test.go    - search across resources.
//   - auth_test.go      - basic-auth profile.
//   - api_test.go       - raw `api` passthrough (GET/POST).
//   - errors_test.go    - error envelope and exit code paths.
//
// When adding a new feature area, create a new topical file and reuse the
// helpers here; do not grow a single large test function.
package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// builtCLIPath points to the freshly-built CLI binary used by all tests.
var builtCLIPath string

func TestMain(m *testing.M) {
	repoRoot := repoRootFromCaller()
	builtCLIPath = filepath.Join(os.TempDir(), fmt.Sprintf("redmine-e2e-%d", time.Now().UnixNano()))

	buildCmd := exec.Command("go", "build", "-o", builtCLIPath, "./cmd/redmine")
	buildCmd.Dir = repoRoot
	buildCmd.Env = append(os.Environ(), "REDMINE_NO_UPDATE_CHECK=1")
	out, err := buildCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build e2e binary: %v\n%s", err, out)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.Remove(builtCLIPath)
	os.Exit(code)
}

// requireE2E skips the test unless REDMINE_E2E=1 and REDMINE_E2E_API_KEY are
// set. Every e2e test must call this first so the suite cleanly no-ops in
// environments without a running Redmine.
func requireE2E(t *testing.T) {
	t.Helper()
	if os.Getenv("REDMINE_E2E") != "1" {
		t.Skip("set REDMINE_E2E=1 to run local Redmine end-to-end tests")
	}
	if os.Getenv("REDMINE_E2E_API_KEY") == "" {
		t.Fatal("set REDMINE_E2E_API_KEY to run local Redmine end-to-end tests")
	}
}

func repoRootFromCaller() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("failed to resolve e2e test file path")
	}
	return filepath.Dir(filepath.Dir(file))
}
