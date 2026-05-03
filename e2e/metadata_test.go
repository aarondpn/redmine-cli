//go:build e2e

package e2e

import (
	"bytes"
	"encoding/csv"
	"slices"
	"testing"
)

// TestMetadata_TrackersListCSV verifies the CSV output shape of `trackers
// list` since the existing `firstTrackerName` helper already covers the JSON
// path. Pinning the headers protects renames in the CSV column layout.
func TestMetadata_TrackersListCSV(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	stdout := r.run(t, "trackers", "list", "--output", "csv")
	rows, err := csv.NewReader(bytes.NewReader(stdout)).ReadAll()
	if err != nil {
		t.Fatalf("decode trackers CSV: %v\nstdout:\n%s", err, stdout)
	}
	if len(rows) < 2 {
		t.Fatalf("expected header + at least 1 tracker row, got %d rows\nstdout:\n%s", len(rows), stdout)
	}
	wantHeaders := []string{"ID", "Name", "Description"}
	if !slices.Equal(rows[0], wantHeaders) {
		t.Fatalf("trackers CSV header mismatch:\n got: %v\nwant: %v", rows[0], wantHeaders)
	}
}

// TestMetadata_StatusesList verifies the JSON shape of `statuses list` and
// that both an open and a closed status are present, mirroring the open/closed
// gating that issues_test.go relies on. Asserts on the is_closed flag round-
// trip rather than specific status names so the test is portable across
// Redmine default-data variations.
func TestMetadata_StatusesList(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	var statuses []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		IsClosed bool   `json:"is_closed"`
	}
	r.runJSON(t, &statuses, "statuses", "list")
	if len(statuses) == 0 {
		t.Fatal("statuses list returned no statuses")
	}

	var hasClosed, hasOpen bool
	for _, s := range statuses {
		if s.ID == 0 || s.Name == "" {
			t.Errorf("status entry missing id or name: %+v", s)
		}
		if s.IsClosed {
			hasClosed = true
		} else {
			hasOpen = true
		}
	}
	if !hasOpen {
		t.Error("statuses list contained no open status; expected at least one")
	}
	if !hasClosed {
		t.Error("statuses list contained no closed status; expected at least one")
	}
}
