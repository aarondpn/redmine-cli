//go:build e2e

package e2e

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
)

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

// TestProjects_CreateExtendedFields covers the new flags on `projects create`:
// homepage, --enable-module, --tracker. We re-fetch with includes and assert
// the server stored the values.
func TestProjects_CreateExtendedFields(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	tracker := firstTrackerName(t, r)

	identifier := uniqueIdentifier(t)
	homepage := "https://example.test/" + identifier
	var created struct {
		ID         int    `json:"id"`
		Identifier string `json:"identifier"`
	}
	r.runJSON(t, &created, "projects", "create",
		"--name", "CLI E2E Extended "+identifier,
		"--identifier", identifier,
		"--homepage", homepage,
		"--tracker", tracker,
		"--enable-module", "issue_tracking,wiki")
	t.Cleanup(func() {
		var deleted actionEnvelope
		r.runJSON(t, &deleted, "projects", "delete", identifier, "--force")
		if !deleted.Ok {
			t.Errorf("cleanup delete failed: %+v", deleted)
		}
	})

	var fetched struct {
		Homepage       string `json:"homepage"`
		EnabledModules []struct {
			Name string `json:"name"`
		} `json:"enabled_modules"`
		Trackers []struct {
			Name string `json:"name"`
		} `json:"trackers"`
	}
	r.runJSON(t, &fetched, "projects", "get", identifier,
		"--include", "enabled_modules,trackers")

	if fetched.Homepage != homepage {
		t.Errorf("homepage = %q, want %q", fetched.Homepage, homepage)
	}

	for _, want := range []string{"issue_tracking", "wiki"} {
		if !slices.ContainsFunc(fetched.EnabledModules, func(m struct {
			Name string `json:"name"`
		}) bool {
			return m.Name == want
		}) {
			t.Errorf("enabled module %q missing from %+v", want, fetched.EnabledModules)
		}
	}

	if !slices.ContainsFunc(fetched.Trackers, func(item struct {
		Name string `json:"name"`
	}) bool {
		return item.Name == tracker
	}) {
		t.Errorf("tracker %q missing from %+v", tracker, fetched.Trackers)
	}
}

// TestProjects_UpdateExtendedFields verifies that the new --homepage flag on
// update round-trips. We deliberately avoid clearing default_assignee here
// because the API treats 0 as "no change" on some Redmine versions, which
// would make the assertion flaky across the matrix.
func TestProjects_UpdateExtendedFields(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	updated := "https://example.test/" + proj.Identifier + "/updated"
	var env actionEnvelope
	r.runJSON(t, &env, "projects", "update", proj.Identifier,
		"--homepage", updated)
	if !env.Ok || env.Action != "updated" || env.Resource != "project" {
		t.Fatalf("unexpected update envelope: %+v", env)
	}

	var fetched struct {
		Homepage string `json:"homepage"`
	}
	r.runJSON(t, &fetched, "projects", "get", proj.Identifier)
	if fetched.Homepage != updated {
		t.Errorf("homepage = %q, want %q", fetched.Homepage, updated)
	}
}

// TestProjects_ArchiveLifecycle archives and unarchives a project. The
// archive endpoint requires Redmine 5.0+; on older instances the route
// returns 404, in which case the test skips with a clear message.
func TestProjects_ArchiveLifecycle(t *testing.T) {
	requireE2E(t)
	skipIfArchiveUnsupported(t)

	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	stdout, stderr, err := r.runRaw("projects", "archive", proj.Identifier, "--force")
	if err != nil {
		if isProbablyUnsupportedArchive(stderr) {
			t.Skipf("server rejected archive request (likely Redmine <5.0); stdout=%s stderr=%s", stdout, stderr)
		}
		t.Fatalf("archive failed: %v", err)
	}

	// Register the unarchive fallback immediately on archive success: if any
	// later assertion calls t.Fatalf, createTestProject's delete cleanup runs
	// against an active project. Redmine 5+ refuses to delete archived
	// projects via the standard endpoint. Errors are tolerated (the explicit
	// unarchive below normally makes this a no-op).
	t.Cleanup(func() {
		_, _, _ = r.runRaw("projects", "unarchive", proj.Identifier)
	})

	var archived actionEnvelope
	if err := json.Unmarshal(stdout, &archived); err != nil {
		t.Fatalf("decode archive envelope: %v\n%s", err, stdout)
	}
	if !archived.Ok || archived.Action != "archived" || archived.Resource != "project" {
		t.Fatalf("unexpected archive envelope: %+v", archived)
	}

	// Skip a get-while-archived assertion: Redmine 5+ hides archived projects
	// behind 403 on /projects/<id>.json, so the round-trip would fail on the
	// versions where this test actually runs. The archive envelope above is
	// the contract we ship; the post-unarchive get below proves the lifecycle
	// is reversible.

	var unarchived actionEnvelope
	r.runJSON(t, &unarchived, "projects", "unarchive", proj.Identifier)
	if !unarchived.Ok || unarchived.Action != "unarchived" || unarchived.Resource != "project" {
		t.Fatalf("unexpected unarchive envelope: %+v", unarchived)
	}

	var fetched struct {
		Status int `json:"status"`
	}
	r.runJSON(t, &fetched, "projects", "get", proj.Identifier)
	if fetched.Status != 1 {
		t.Errorf("status after unarchive = %d, want 1 (active)", fetched.Status)
	}
}

// skipIfArchiveUnsupported skips the calling test when the e2e matrix is
// running against a Redmine version known to lack the archive endpoint, which
// landed in Redmine 5.0.
func skipIfArchiveUnsupported(t *testing.T) {
	t.Helper()
	skipBelowRedmine(t, 5, 0, "project archive")
}

// isProbablyUnsupportedArchive inspects an error envelope on stderr and
// returns true when the failure looks like the archive route is missing
// (404 / not_found). Used as a defensive fallback when REDMINE_E2E_VERSION
// is not set but the live server still refuses the call.
func isProbablyUnsupportedArchive(stderr []byte) bool {
	var env errorEnvelope
	if err := json.Unmarshal(lastLine(stderr), &env); err != nil {
		return false
	}
	if env.Error.Code == "not_found" {
		return true
	}
	msg := strings.ToLower(env.Error.Message)
	return strings.Contains(msg, "not found") || strings.Contains(msg, "404")
}

// TestProjects_DefaultAssignee guards the default_assignee decoding: Redmine
// 4.1+ returns the key "default_assignee" (not "default_assigned_to"), and the
// CLI must surface it in both JSON and the table view.
func TestProjects_DefaultAssignee(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	user := createTestUser(t, r)
	addProjectMember(t, r, proj.Identifier, "--user-id", user.ID, roleIDByName(t, r, "Developer"))

	var updated actionEnvelope
	r.runJSON(t, &updated, "projects", "update", proj.Identifier, "--default-assignee", strconv.Itoa(user.ID))
	if !updated.Ok {
		t.Fatalf("projects update --default-assignee failed: %+v", updated)
	}

	var got struct {
		DefaultAssignee *struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"default_assignee"`
	}
	r.runJSON(t, &got, "projects", "get", proj.Identifier)
	if got.DefaultAssignee == nil || got.DefaultAssignee.ID != user.ID || got.DefaultAssignee.Name != user.Name {
		t.Fatalf("default_assignee = %+v, want id=%d name=%q", got.DefaultAssignee, user.ID, user.Name)
	}

	table := string(r.run(t, "projects", "get", proj.Identifier, "--output", "table"))
	if !strings.Contains(table, "Default Assignee") || !strings.Contains(table, user.Name) {
		t.Fatalf("table output missing default assignee %q\n%s", user.Name, table)
	}
}
