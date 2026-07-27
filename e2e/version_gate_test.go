//go:build e2e

package e2e

import "testing"

// TestParseRedmineVersion is a pure unit test for the version comparator every
// version gate in this suite depends on. It deliberately does not call
// requireE2E: the parser must be verifiable without a running server.
func TestParseRedmineVersion(t *testing.T) {
	tests := []struct {
		in        string
		wantMajor int
		wantMinor int
		wantOK    bool
	}{
		{"7.0", 7, 0, true},
		{"6.1", 6, 1, true},
		{"4.2", 4, 2, true},
		{"7.0.0", 7, 0, true},
		{"6.1.3", 6, 1, true},
		{"  7.0  ", 7, 0, true},
		{"7", 7, 0, true},
		{"10.2", 10, 2, true},
		{"", 0, 0, false},
		{"   ", 0, 0, false},
		{".", 0, 0, false},
		{"x", 0, 0, false},
		{"6.x", 0, 0, false},
		{"latest", 0, 0, false},
	}

	for _, tc := range tests {
		major, minor, ok := parseRedmineVersion(tc.in)
		if ok != tc.wantOK || major != tc.wantMajor || minor != tc.wantMinor {
			t.Errorf("parseRedmineVersion(%q) = (%d, %d, %v), want (%d, %d, %v)",
				tc.in, major, minor, ok, tc.wantMajor, tc.wantMinor, tc.wantOK)
		}
	}
}

// TestRedmineAtLeast pins the comparison semantics, including the deliberate
// "unknown counts as new enough" default.
func TestRedmineAtLeast(t *testing.T) {
	tests := []struct {
		version string
		major   int
		minor   int
		want    bool
	}{
		{"7.0", 7, 0, true},
		{"7.0.0", 7, 0, true},
		{"6.1", 7, 0, false},
		{"4.2", 5, 0, false},
		{"5.1", 5, 0, true},
		{"6.1", 5, 0, true},
		{"10.0", 7, 0, true},
		{"", 7, 0, true},
		{"6.x", 7, 0, true},
	}

	for _, tc := range tests {
		t.Setenv("REDMINE_E2E_VERSION", tc.version)
		if got := redmineAtLeast(tc.major, tc.minor); got != tc.want {
			t.Errorf("redmineAtLeast(%d, %d) with REDMINE_E2E_VERSION=%q = %v, want %v",
				tc.major, tc.minor, tc.version, got, tc.want)
		}
	}
}
