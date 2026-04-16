//go:build e2e

package e2e

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestErrors_NotFound covers 404 handling on both issues and projects: the
// CLI must exit non-zero and print an error envelope with code=not_found on
// stdout (since --output json is active).
func TestErrors_NotFound(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	t.Run("issues get missing", func(t *testing.T) {
		stdout, _ := r.runExpectError(t, "issues", "get", "2147483600")
		assertErrorCode(t, stdout, "not_found")
	})

	t.Run("projects get missing", func(t *testing.T) {
		stdout, _ := r.runExpectError(t, "projects", "get", "definitely-does-not-exist-12345")
		assertErrorCode(t, stdout, "not_found")
	})
}

// TestErrors_AuthFailed verifies that an invalid API key surfaces as
// code=auth_failed rather than being swallowed.
func TestErrors_AuthFailed(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), "invalid-api-key-for-e2e-auth-failure-test")

	stdout, _ := r.runExpectError(t, "users", "me")
	assertErrorCode(t, stdout, "auth_failed")
}

func assertErrorCode(t *testing.T, stdout []byte, want string) {
	t.Helper()
	var env errorEnvelope
	if err := json.Unmarshal(stdout, &env); err != nil {
		t.Fatalf("decode error envelope: %v\nstdout:\n%s", err, stdout)
	}
	if env.Error.Code != want {
		t.Fatalf("error code = %q, want %q\nstdout:\n%s", env.Error.Code, want, stdout)
	}
	if strings.TrimSpace(env.Error.Message) == "" {
		t.Fatalf("error envelope missing message\nstdout:\n%s", stdout)
	}
}
