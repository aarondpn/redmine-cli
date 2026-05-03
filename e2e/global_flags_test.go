//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGlobal_ServerOverride builds a runner whose embedded config points at
// an unreachable URL, then passes --server / --api-key on the command line
// to confirm that flag overrides take precedence over config values. If
// `users me` succeeds, the override path is wired up correctly.
func TestGlobal_ServerOverride(t *testing.T) {
	requireE2E(t)
	r := newRunnerWithConfig(t, `active_profile: bogus
profiles:
  bogus:
    server: http://127.0.0.1:1
    api_key: not-a-real-key
    auth_method: apikey
    output_format: json
`)

	var me struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
	}
	r.runJSON(t, &me,
		"--server", e2eBaseURL(),
		"--api-key", e2eAPIKey(),
		"users", "me")

	if me.ID == 0 {
		t.Fatalf("override-only users me returned empty user: %+v", me)
	}
}

// TestGlobal_VerboseLogs runs `users me --verbose` and verifies the debug
// stream lands on stderr. The debug logger prefixes lines with "[debug]" and
// the API client emits at least one "HTTP <method> <url>" log per request.
func TestGlobal_VerboseLogs(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	stdout, stderr, err := r.runRawWithEnv(nil, "users", "me", "--verbose")
	if err != nil {
		t.Fatalf("users me --verbose: %v\nstderr:\n%s", err, stderr)
	}
	if len(bytes.TrimSpace(stdout)) == 0 {
		t.Fatalf("expected stdout payload from users me --verbose")
	}

	stderrStr := string(stderr)
	for _, want := range []string{"[debug]", "HTTP", "/users/current"} {
		if !strings.Contains(stderrStr, want) {
			t.Fatalf("verbose stderr missing %q\nstderr:\n%s", want, stderrStr)
		}
	}
}

// TestGlobal_ConfigFlag_AlternatePath writes a profile config to a custom
// filename inside t.TempDir() and verifies --config picks it up. We bypass
// the runner's auto-injection of --config by exec-ing the built binary
// directly, since the runner always points at its own configPath.
func TestGlobal_ConfigFlag_AlternatePath(t *testing.T) {
	requireE2E(t)

	altPath := filepath.Join(t.TempDir(), "alt-profile.yaml")
	cfg := fmt.Sprintf(`active_profile: alt
profiles:
  alt:
    server: %s
    api_key: %s
    auth_method: apikey
    output_format: json
`, e2eBaseURL(), e2eAPIKey())
	if err := os.WriteFile(altPath, []byte(cfg), 0o600); err != nil {
		t.Fatalf("write alt config: %v", err)
	}

	cmd := exec.Command(builtCLIPath, "--config", altPath, "--output", "json", "users", "me")
	cmd.Dir = repoRootFromCaller()
	cmd.Env = append(os.Environ(), "REDMINE_NO_UPDATE_CHECK=1")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("users me with --config %s failed: %v\nstdout:\n%s\nstderr:\n%s",
			altPath, err, stdout.String(), stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"login"`)) {
		t.Fatalf("expected users me JSON to contain a login field; got:\n%s", stdout.String())
	}
}

// TestGlobal_MalformedConfig writes broken YAML to the runner's config path
// and verifies the CLI fails fast with a helpful parse error referencing
// either the config path or the parser. We do NOT assert the exact wording
// (it comes from gopkg.in/yaml.v3) but we require some substring tying the
// failure to config loading.
func TestGlobal_MalformedConfig(t *testing.T) {
	requireE2E(t)
	r := newRunnerWithConfig(t, "not: valid: yaml: at all\n  : : :\n")

	stdout, stderr := r.runExpectError(t, "users", "me")
	combined := strings.ToLower(string(stdout) + string(stderr))
	for _, marker := range []string{"yaml", "parsing", "config"} {
		if strings.Contains(combined, marker) {
			return
		}
	}
	t.Fatalf("malformed-config error did not mention yaml/parsing/config:\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
}
