//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cliRunner executes the redmine CLI against a specific temporary config
// file. Each test gets its own runner so config state never leaks between
// tests. Prefer the factory constructors below over building a runner by hand;
// add a new constructor when introducing a new profile shape (OAuth, proxy
// auth, ...).
type cliRunner struct {
	configPath string
	repoRoot   string
}

// newCLIRunner returns a runner with a single API-key profile. This is the
// default factory used by most tests.
func newCLIRunner(t *testing.T, baseURL, apiKey string) *cliRunner {
	t.Helper()
	return newRunnerWithConfig(t, fmt.Sprintf(`active_profile: local-e2e
profiles:
  local-e2e:
    server: %s
    api_key: %s
    auth_method: apikey
    output_format: json
`, baseURL, apiKey))
}

// newCLIRunnerBasicAuth returns a runner configured for HTTP basic auth.
// Tests that want to verify the basic-auth code path use this.
func newCLIRunnerBasicAuth(t *testing.T, baseURL, username, password string) *cliRunner {
	t.Helper()
	return newRunnerWithConfig(t, fmt.Sprintf(`active_profile: local-e2e-basic
profiles:
  local-e2e-basic:
    server: %s
    username: %s
    password: %s
    auth_method: basic
    output_format: json
`, baseURL, username, password))
}

func newRunnerWithConfig(t *testing.T, config string) *cliRunner {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "redmine-cli-e2e.yaml")
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return &cliRunner{configPath: configPath, repoRoot: repoRootFromCaller()}
}

// run executes the CLI, fails the test on non-zero exit, and returns stdout.
func (r *cliRunner) run(t *testing.T, args ...string) []byte {
	t.Helper()
	stdout, _, err := r.runRaw(args...)
	if err != nil {
		t.Fatal(err)
	}
	return stdout
}

// runJSON runs the CLI and decodes stdout into dest.
func (r *cliRunner) runJSON(t *testing.T, dest any, args ...string) {
	t.Helper()
	stdout := r.run(t, args...)
	if err := json.Unmarshal(stdout, dest); err != nil {
		t.Fatalf("decode JSON for %q: %v\nstdout:\n%s", strings.Join(args, " "), err, stdout)
	}
}

// runExpectError runs the CLI and requires a non-zero exit. It returns
// stdout and stderr so tests can inspect the error envelope.
func (r *cliRunner) runExpectError(t *testing.T, args ...string) (stdout, stderr []byte) {
	t.Helper()
	stdout, stderr, err := r.runRaw(args...)
	if err == nil {
		t.Fatalf("expected non-zero exit for %q\nstdout:\n%s", strings.Join(args, " "), stdout)
	}
	return stdout, stderr
}

func (r *cliRunner) runRaw(args ...string) (stdout, stderr []byte, err error) {
	cmdArgs := append([]string{"--config", r.configPath, "--output", "json"}, args...)
	cmd := exec.Command(builtCLIPath, cmdArgs...)
	cmd.Dir = r.repoRoot
	cmd.Env = append(os.Environ(), "REDMINE_NO_UPDATE_CHECK=1")

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	if runErr != nil {
		runErr = fmt.Errorf("run %q: %w\nstdout:\n%s\nstderr:\n%s",
			strings.Join(cmdArgs, " "), runErr, outBuf.String(), errBuf.String())
	}
	return outBuf.Bytes(), errBuf.Bytes(), runErr
}
