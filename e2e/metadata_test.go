//go:build e2e

package e2e

import "testing"

// TestMetadata_TrackersList verifies that the read-only `trackers list`
// command returns a non-empty collection with usable id/name fields.
func TestMetadata_TrackersList(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	var trackers []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	r.runJSON(t, &trackers, "trackers", "list")
	if len(trackers) == 0 {
		t.Fatal("trackers list returned no trackers")
	}
	for _, tr := range trackers {
		if tr.ID == 0 {
			t.Errorf("tracker has zero ID: %+v", tr)
		}
		if tr.Name == "" {
			t.Errorf("tracker has empty name: %+v", tr)
		}
	}
}

// TestMetadata_StatusesList verifies that `statuses list` returns the default
// issue statuses with both open and closed entries.
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

	hasClosed := false
	hasOpen := false
	for _, s := range statuses {
		if s.ID == 0 {
			t.Errorf("status has zero ID: %+v", s)
		}
		if s.Name == "" {
			t.Errorf("status has empty name: %+v", s)
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
