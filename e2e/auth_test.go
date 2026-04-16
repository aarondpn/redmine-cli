//go:build e2e

package e2e

import "testing"

// TestAuth_BasicAuth verifies the basic-auth code path: a profile with
// username + password successfully authenticates against /users/current.json.
//
// This test is skipped unless REDMINE_E2E_PASSWORD is set. The Makefile's
// e2e-test target sources this via e2e/admin-password.sh (which also resets
// the admin password on the running container).
func TestAuth_BasicAuth(t *testing.T) {
	requireE2E(t)

	password := e2ePassword()
	if password == "" {
		t.Skip("set REDMINE_E2E_PASSWORD to run basic-auth tests (see e2e/README.md)")
	}

	r := newCLIRunnerBasicAuth(t, e2eBaseURL(), e2eUsername(), password)

	var me struct {
		Login string `json:"login"`
		Admin bool   `json:"admin"`
	}
	r.runJSON(t, &me, "users", "me")
	if me.Login != e2eUsername() {
		t.Fatalf("basic auth users me login = %q, want %q", me.Login, e2eUsername())
	}
	if !me.Admin {
		t.Fatalf("expected basic auth user %q to be an admin", me.Login)
	}
}
