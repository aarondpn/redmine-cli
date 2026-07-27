//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
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

// e2eVersion returns the Redmine line under test ("4.2", "6.1", "7.0", ...).
// The Makefile e2e-test target sets REDMINE_E2E_VERSION; it is empty for
// ad-hoc runs against an instance of unknown version.
func e2eVersion() string { return os.Getenv("REDMINE_E2E_VERSION") }

// redmineAtLeast reports whether the Redmine line under test is at least
// major.minor. An unset or unparseable REDMINE_E2E_VERSION counts as "new
// enough" so ad-hoc runs still exercise the newest assertions; version-gated
// tests should therefore stay tolerant of a server that turns out to be older.
func redmineAtLeast(major, minor int) bool {
	gotMajor, gotMinor, ok := parseRedmineVersion(e2eVersion())
	if !ok {
		return true
	}
	if gotMajor != major {
		return gotMajor > major
	}
	return gotMinor >= minor
}

// skipBelowRedmine skips the calling test when the version under test predates
// major.minor. feature names the capability for the skip message.
func skipBelowRedmine(t *testing.T, major, minor int, feature string) {
	t.Helper()
	if !redmineAtLeast(major, minor) {
		t.Skipf("REDMINE_E2E_VERSION=%s does not support %s (requires %d.%d+)", e2eVersion(), feature, major, minor)
	}
}

// parseRedmineVersion extracts the major and minor components from a version
// string such as "6.1" or "7.0.0". ok is false when the string is empty or
// does not start with a numeric major component.
func parseRedmineVersion(v string) (major, minor int, ok bool) {
	parts := strings.Split(strings.TrimSpace(v), ".")
	if len(parts) == 0 || parts[0] == "" {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	if len(parts) > 1 {
		if minor, err = strconv.Atoi(parts[1]); err != nil {
			minor = 0
		}
	}
	return major, minor, true
}

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
