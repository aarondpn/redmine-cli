//go:build e2e

package e2e

import "testing"

// TestProjects_CRUD covers project create/get/list/delete (delete runs via
// the fixture's t.Cleanup).
func TestProjects_CRUD(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	proj := createTestProject(t, r)

	var fetched struct {
		ID         int    `json:"id"`
		Identifier string `json:"identifier"`
	}
	r.runJSON(t, &fetched, "projects", "get", proj.Identifier)
	if fetched.ID != proj.ID {
		t.Fatalf("projects get ID = %d, want %d", fetched.ID, proj.ID)
	}

	var listed []struct {
		Identifier string `json:"identifier"`
	}
	r.runJSON(t, &listed, "projects", "list")
	found := false
	for _, p := range listed {
		if p.Identifier == proj.Identifier {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("projects list did not include %q", proj.Identifier)
	}
}
