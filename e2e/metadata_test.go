//go:build e2e

package e2e

import (
	"testing"
)

// TestMetadata_TrackerGetJSON checks that `trackers get` resolves a tracker by
// the name `trackers list` reported. The shape of the payload (default status,
// enabled standard fields, ...) is asserted by the contract tests.
func TestMetadata_TrackerGetJSON(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	type tracker struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	var trackers []tracker
	r.runJSON(t, &trackers, "trackers", "list")
	if len(trackers) == 0 {
		t.Fatal("trackers list returned no trackers")
	}

	var got tracker
	r.runJSON(t, &got, "trackers", "get", trackers[0].Name)

	if got.ID != trackers[0].ID || got.Name != trackers[0].Name {
		t.Fatalf("trackers get returned %+v, want id=%d name=%q", got, trackers[0].ID, trackers[0].Name)
	}
}
