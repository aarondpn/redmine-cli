//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// actionEnvelope matches the JSON shape emitted by no-body mutators.
type actionEnvelope struct {
	Ok       bool   `json:"ok"`
	Action   string `json:"action"`
	Resource string `json:"resource"`
	ID       any    `json:"id"`
	Message  string `json:"message"`
}

// errorEnvelope matches the JSON shape written to stdout on failure when
// --output json is active (see output.ErrorEnvelope).
type errorEnvelope struct {
	Error struct {
		Message string   `json:"message"`
		Code    string   `json:"code"`
		Details []string `json:"details"`
	} `json:"error"`
}

// envelopeIntID coerces the envelope ID (which JSON decodes as float64) to an
// int for comparisons.
func envelopeIntID(v any) int {
	switch id := v.(type) {
	case float64:
		return int(id)
	case int:
		return id
	default:
		return 0
	}
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// The following accessors centralize env-var lookups so renames stay local to
// this file. Tests should prefer these over os.Getenv.
func e2eBaseURL() string  { return getenvDefault("REDMINE_E2E_BASE_URL", "http://127.0.0.1:3000") }
func e2eUsername() string { return getenvDefault("REDMINE_E2E_USERNAME", "admin") }
func e2eAPIKey() string   { return os.Getenv("REDMINE_E2E_API_KEY") }
func e2ePassword() string { return os.Getenv("REDMINE_E2E_PASSWORD") }

// requireErrorEnvelopeMessage decodes stdout as an error envelope and requires
// a non-empty message. Returns the decoded envelope so callers can perform
// additional code-level assertions.
func requireErrorEnvelopeMessage(t *testing.T, stdout []byte) errorEnvelope {
	t.Helper()
	var env errorEnvelope
	if err := json.Unmarshal(stdout, &env); err != nil {
		t.Fatalf("decode error envelope: %v\nstdout:\n%s", err, stdout)
	}
	if strings.TrimSpace(env.Error.Message) == "" {
		t.Fatalf("error envelope missing message\nstdout:\n%s", stdout)
	}
	return env
}

// updateGoldens reports whether tests should overwrite golden files instead
// of asserting against them. Set UPDATE_GOLDENS=1 to refresh.
func updateGoldens() bool { return os.Getenv("UPDATE_GOLDENS") == "1" }

// assertGoldenJSON re-marshals got into stable indented JSON and compares it
// against the file at path (relative to the e2e directory). When
// UPDATE_GOLDENS=1, the file is rewritten instead. Used for snapshotting
// stable response shapes (MCP tool catalogs, output-format samples) so an
// accidental rename or schema drift fails CI.
//
// The golden file is required: a missing file fails the test with a clear
// instruction to run with UPDATE_GOLDENS=1. Goldens must be committed.
func assertGoldenJSON(t *testing.T, path string, got any) {
	t.Helper()

	want, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal golden value: %v", err)
	}
	want = append(want, '\n')

	abs := path
	if !filepath.IsAbs(path) {
		abs = filepath.Join(repoRootFromCaller(), "e2e", path)
	}

	if updateGoldens() {
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(abs, want, 0o600); err != nil {
			t.Fatalf("write golden %s: %v", abs, err)
		}
		return
	}

	have, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with UPDATE_GOLDENS=1 to create)", abs, err)
	}
	if !bytes.Equal(want, have) {
		t.Fatalf("golden %s mismatch:\n--- want ---\n%s\n--- have ---\n%s", abs, string(want), string(have))
	}
}
