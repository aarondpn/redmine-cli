//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// newRunnerWithMultiProfile constructs a cliRunner whose config file holds two
// API-key profiles, with "primary" marked active. Both profiles point at the
// same baseURL/apiKey so either is usable against the live Redmine container.
func newRunnerWithMultiProfile(t *testing.T, baseURL, apiKey string) (r *cliRunner, primary, secondary string) {
	t.Helper()
	primary = "primary"
	secondary = "secondary"
	cfg := fmt.Sprintf(`active_profile: %s
profiles:
  %s:
    server: %s
    api_key: %s
    auth_method: apikey
    output_format: json
  %s:
    server: %s
    api_key: %s
    auth_method: apikey
    output_format: json
`, primary, primary, baseURL, apiKey, secondary, baseURL, apiKey)
	return newRunnerWithConfig(t, cfg), primary, secondary
}

func TestAuth_StatusActive(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	var status map[string]any
	r.runJSON(t, &status, "auth", "status")

	if active, _ := status["active"].(bool); !active {
		t.Fatalf("auth status active = %v, want true; payload=%+v", status["active"], status)
	}
	if got, want := status["server"], e2eBaseURL(); got != want {
		t.Fatalf("auth status server = %v, want %v", got, want)
	}
	if got, want := status["profile"], defaultRunnerProfile; got != want {
		t.Fatalf("auth status profile = %v, want %v", got, want)
	}
}

func TestAuth_StatusNoProfile(t *testing.T) {
	requireE2E(t)
	r := newRunnerWithConfig(t, "active_profile: \"\"\nprofiles: {}\n")

	var status map[string]any
	r.runJSON(t, &status, "auth", "status")

	if active, _ := status["active"].(bool); active {
		t.Fatalf("auth status active = true, want false; payload=%+v", status)
	}
	if got, want := status["reason"], "no_profiles_configured"; got != want {
		t.Fatalf("auth status reason = %v, want %v", got, want)
	}
}

func TestAuth_StatusBadKey(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), "not-a-real-api-key-0000000000000000000000")

	var status map[string]any
	r.runJSON(t, &status, "auth", "status")

	if active, _ := status["active"].(bool); active {
		t.Fatalf("auth status active = true, want false; payload=%+v", status)
	}
	if user, _ := status["user"].(string); !strings.Contains(strings.ToLower(user), "authentication failed") {
		t.Fatalf("auth status user = %q, want authentication-failure indication", user)
	}
}

func TestAuth_List(t *testing.T) {
	requireE2E(t)
	r, primary, secondary := newRunnerWithMultiProfile(t, e2eBaseURL(), e2eAPIKey())

	var profiles []struct {
		Name   string `json:"name"`
		Server string `json:"server"`
		Active bool   `json:"active"`
	}
	r.runJSON(t, &profiles, "auth", "list")

	if len(profiles) != 2 {
		t.Fatalf("auth list returned %d profiles, want 2: %+v", len(profiles), profiles)
	}

	seen := map[string]bool{}
	activeCount := 0
	for _, p := range profiles {
		seen[p.Name] = true
		if p.Server != e2eBaseURL() {
			t.Errorf("profile %q server = %q, want %q", p.Name, p.Server, e2eBaseURL())
		}
		if p.Active {
			activeCount++
			if p.Name != primary {
				t.Errorf("active profile = %q, want %q", p.Name, primary)
			}
		}
	}
	if !seen[primary] || !seen[secondary] {
		t.Fatalf("auth list missing expected profiles, got: %+v", profiles)
	}
	if activeCount != 1 {
		t.Fatalf("auth list marked %d profiles active, want exactly 1", activeCount)
	}
}

func TestAuth_Switch(t *testing.T) {
	requireE2E(t)
	r, _, secondary := newRunnerWithMultiProfile(t, e2eBaseURL(), e2eAPIKey())

	var env actionEnvelope
	r.runJSON(t, &env, "auth", "switch", secondary)
	if !env.Ok || env.Action != "switched" || env.Resource != "profile" {
		t.Fatalf("unexpected switch envelope: %+v", env)
	}
	if id, _ := env.ID.(string); id != secondary {
		t.Fatalf("switch envelope ID = %v, want %q", env.ID, secondary)
	}

	data, err := os.ReadFile(r.configPath)
	if err != nil {
		t.Fatalf("read config back: %v", err)
	}
	if !strings.Contains(string(data), "active_profile: "+secondary) {
		t.Fatalf("config file does not show active_profile=%s after switch:\n%s", secondary, data)
	}
}

// TestAuth_LogoutNoArgsRequiresInteractive checks that logout without a TTY
// errors out cleanly. The runner always passes --output json, which logout
// rejects via PrepareInteractiveCommand because it has no flag-only path.
func TestAuth_LogoutNoArgsRequiresInteractive(t *testing.T) {
	requireE2E(t)
	r, primary, secondary := newRunnerWithMultiProfile(t, e2eBaseURL(), e2eAPIKey())

	stdout, stderr := r.runExpectError(t, "auth", "logout")
	combined := string(stdout) + string(stderr)
	if !strings.Contains(combined, "--output is not supported") {
		t.Fatalf("expected logout to fail with --output rejection, got:\nstdout:%s\nstderr:%s", stdout, stderr)
	}

	data, err := os.ReadFile(r.configPath)
	if err != nil {
		t.Fatalf("read config back: %v", err)
	}
	cfg := string(data)
	if !strings.Contains(cfg, primary+":") || !strings.Contains(cfg, secondary+":") {
		t.Fatalf("expected both profiles to remain after failed logout, got:\n%s", cfg)
	}
}
