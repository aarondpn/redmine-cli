//go:build e2e

package e2e

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestErrors_NotFound covers 404 handling on both issues and projects: the
// CLI must exit non-zero and print an error envelope with code=not_found on
// stderr (since --output json is active).
func TestErrors_NotFound(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	t.Run("issues get missing", func(t *testing.T) {
		stderr := r.runExpectJSONError(t, "issues", "get", "2147483600")
		assertErrorCode(t, stderr, "not_found")
	})

	t.Run("projects get missing", func(t *testing.T) {
		stderr := r.runExpectJSONError(t, "projects", "get", "definitely-does-not-exist-12345")
		assertErrorCode(t, stderr, "not_found")
	})
}

// TestErrors_AuthFailed verifies that an invalid API key surfaces as
// code=auth_failed rather than being swallowed.
func TestErrors_AuthFailed(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), "invalid-api-key-for-e2e-auth-failure-test")

	stderr := r.runExpectJSONError(t, "users", "me")
	assertErrorCode(t, stderr, "auth_failed")
}

func assertErrorCode(t *testing.T, stderr []byte, want string) {
	t.Helper()
	var env errorEnvelope
	if err := json.Unmarshal(stderr, &env); err != nil {
		t.Fatalf("decode error envelope: %v\nstderr:\n%s", err, stderr)
	}
	if env.Error.Code != want {
		t.Fatalf("error code = %q, want %q\nstderr:\n%s", env.Error.Code, want, stderr)
	}
	if strings.TrimSpace(env.Error.Message) == "" {
		t.Fatalf("error envelope missing message\nstderr:\n%s", stderr)
	}
}

// TestErrors_ConnectionFailure points a runner at a port with no listener and
// verifies the CLI surfaces a populated error envelope. Connection failures
// are not *api.APIError, so the code falls back to "unknown" (see
// internal/cmdutil/errors.go BuildErrorEnvelope).
func TestErrors_ConnectionFailure(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, "http://127.0.0.1:1", "fake-key")

	stderr := r.runExpectJSONError(t, "users", "me")
	env := requireErrorEnvelopeMessage(t, stderr)
	if env.Error.Code != "unknown" {
		t.Fatalf("connection failure code = %q, want unknown\nstderr:\n%s", env.Error.Code, stderr)
	}
}
