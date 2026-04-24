//go:build e2e

package e2e

import (
	"strconv"
	"testing"
	"time"
)

// TestVersions_Lifecycle drives the full project-version life cycle against
// a real Redmine instance: create → list → get (by ID and by name) →
// update → verify → delete (by name) → verify gone.
func TestVersions_Lifecycle(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	const (
		versionName = "v1.0-e2e"
		newName     = "v1.0.1-e2e"
	)

	var created struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Status  string `json:"status"`
		DueDate string `json:"due_date"`
		Project struct {
			ID int `json:"id"`
		} `json:"project"`
	}
	r.runJSON(t, &created, "versions", "create",
		"--project", proj.Identifier,
		"--name", versionName,
		"--status", "open",
		"--sharing", "none",
		"--due-date", "2026-12-31",
		"--description", "Created by e2e lifecycle test")
	if created.ID == 0 {
		t.Fatalf("created version has zero ID: %+v", created)
	}
	if created.Name != versionName {
		t.Fatalf("created version name = %q, want %q", created.Name, versionName)
	}
	if created.Status != "open" {
		t.Fatalf("created version status = %q, want open", created.Status)
	}
	if created.DueDate != "2026-12-31" {
		t.Fatalf("created version due_date = %q, want 2026-12-31", created.DueDate)
	}
	if created.Project.ID != proj.ID {
		t.Fatalf("created version project.id = %d, want %d", created.Project.ID, proj.ID)
	}

	var listed []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	r.runJSON(t, &listed, "versions", "list", "--project", proj.Identifier)
	foundInList := false
	for _, v := range listed {
		if v.ID == created.ID && v.Name == versionName {
			foundInList = true
			break
		}
	}
	if !foundInList {
		t.Fatalf("versions list did not include created version %d (%q): %+v", created.ID, versionName, listed)
	}

	// Get by numeric ID.
	var byID struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	r.runJSON(t, &byID, "versions", "get", strconv.Itoa(created.ID))
	if byID.ID != created.ID || byID.Name != versionName {
		t.Fatalf("get by ID = %+v, want id=%d name=%q", byID, created.ID, versionName)
	}

	// Get by name exercises the resolveVersionID name-lookup branch.
	var byName struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	r.runJSON(t, &byName, "versions", "get", versionName, "--project", proj.Identifier)
	if byName.ID != created.ID {
		t.Fatalf("get by name = %+v, want id=%d", byName, created.ID)
	}

	// Update: rename, flip status, clear description, and push due-date
	// forward via the `today` keyword so the shared keyword-resolver is
	// exercised through a real request.
	var updated actionEnvelope
	r.runJSON(t, &updated, "versions", "update", strconv.Itoa(created.ID),
		"--name", newName,
		"--status", "locked",
		"--description", "",
		"--due-date", "today")
	if !updated.Ok || updated.Action != "updated" || updated.Resource != "version" {
		t.Fatalf("unexpected update envelope: %+v", updated)
	}
	if envelopeIntID(updated.ID) != created.ID {
		t.Fatalf("update envelope id = %v, want %d", updated.ID, created.ID)
	}

	var afterUpdate struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Status      string `json:"status"`
		Description string `json:"description"`
		DueDate     string `json:"due_date"`
	}
	r.runJSON(t, &afterUpdate, "versions", "get", strconv.Itoa(created.ID))
	if afterUpdate.Name != newName {
		t.Errorf("after update name = %q, want %q", afterUpdate.Name, newName)
	}
	if afterUpdate.Status != "locked" {
		t.Errorf("after update status = %q, want locked", afterUpdate.Status)
	}
	if afterUpdate.Description != "" {
		t.Errorf("after update description = %q, want empty", afterUpdate.Description)
	}
	today := time.Now().Format("2006-01-02")
	if afterUpdate.DueDate != today {
		t.Errorf("after update due_date = %q, want %q (today keyword resolved server-side)", afterUpdate.DueDate, today)
	}

	// Delete by the new name — exercises resolveVersionID on the delete path.
	var deleted actionEnvelope
	r.runJSON(t, &deleted, "versions", "delete", newName,
		"--project", proj.Identifier,
		"--force")
	if !deleted.Ok || deleted.Action != "deleted" || deleted.Resource != "version" {
		t.Fatalf("unexpected delete envelope: %+v", deleted)
	}
	if envelopeIntID(deleted.ID) != created.ID {
		t.Fatalf("delete envelope id = %v, want %d", deleted.ID, created.ID)
	}

	r.runJSON(t, &listed, "versions", "list", "--project", proj.Identifier)
	for _, v := range listed {
		if v.ID == created.ID {
			t.Fatalf("version %d still listed after delete: %+v", created.ID, listed)
		}
	}
}
