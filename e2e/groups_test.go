//go:build e2e

package e2e

import (
	"strconv"
	"testing"
)

// TestGroups_CRUD covers list → get (by ID and by name) → update on a group
// produced by the createTestGroup fixture (which exercises create + delete via
// its t.Cleanup).
func TestGroups_CRUD(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	grp := createTestGroup(t, r)

	var listed []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	r.runJSON(t, &listed, "groups", "list", "--limit", "100")
	foundInList := false
	for _, g := range listed {
		if g.ID == grp.ID {
			foundInList = true
			break
		}
	}
	if !foundInList {
		t.Fatalf("groups list did not include group %d (%q): %+v", grp.ID, grp.Name, listed)
	}

	var byID struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	r.runJSON(t, &byID, "groups", "get", strconv.Itoa(grp.ID))
	if byID.ID != grp.ID || byID.Name != grp.Name {
		t.Fatalf("get by ID = %+v, want id=%d name=%q", byID, grp.ID, grp.Name)
	}

	// Get by name exercises the resolver's name-lookup branch.
	var byName struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	r.runJSON(t, &byName, "groups", "get", grp.Name)
	if byName.ID != grp.ID {
		t.Fatalf("get by name = %+v, want id=%d", byName, grp.ID)
	}

	newName := grp.Name + "-renamed"
	var updated actionEnvelope
	r.runJSON(t, &updated, "groups", "update", strconv.Itoa(grp.ID), "--name", newName)
	if !updated.Ok || updated.Action != "updated" || updated.Resource != "group" {
		t.Fatalf("unexpected update envelope: %+v", updated)
	}
	if envelopeIntID(updated.ID) != grp.ID {
		t.Fatalf("update envelope id = %v, want %d", updated.ID, grp.ID)
	}

	var afterUpdate struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	r.runJSON(t, &afterUpdate, "groups", "get", strconv.Itoa(grp.ID))
	if afterUpdate.Name != newName {
		t.Fatalf("after update name = %q, want %q", afterUpdate.Name, newName)
	}
	// Keep fixture metadata in sync; the cleanup deletes by ID but logs name.
	grp.Name = newName
}

// TestGroups_DeleteExplicit creates a group without the fixture (which would
// double-delete on cleanup) and asserts the explicit delete envelope plus a
// subsequent 404 on get.
func TestGroups_DeleteExplicit(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	name := "e2e-grp-" + uniqueShortSuffix(t)
	var created struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	r.runJSON(t, &created, "groups", "create", "--name", name)
	if created.ID == 0 || created.Name != name {
		t.Fatalf("create returned %+v, want id>0 name=%q", created, name)
	}

	var deleted actionEnvelope
	r.runJSON(t, &deleted, "groups", "delete", strconv.Itoa(created.ID), "--force")
	if !deleted.Ok || deleted.Action != "deleted" || deleted.Resource != "group" {
		t.Fatalf("unexpected delete envelope: %+v", deleted)
	}
	if envelopeIntID(deleted.ID) != created.ID {
		t.Fatalf("delete envelope id = %v, want %d", deleted.ID, created.ID)
	}

	stdout, _ := r.runExpectError(t, "groups", "get", strconv.Itoa(created.ID))
	assertErrorCode(t, stdout, "not_found")
}

// TestGroups_AddRemoveUser verifies that add-user/remove-user mutate the
// membership list visible via `groups get --include-users`. Both subcommands
// emit user_added/user_removed envelopes.
func TestGroups_AddRemoveUser(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	grp := createTestGroup(t, r)
	user := createTestUser(t, r)

	groupArg := strconv.Itoa(grp.ID)
	userArg := strconv.Itoa(user.ID)

	var added actionEnvelope
	r.runJSON(t, &added, "groups", "add-user", groupArg, userArg)
	if !added.Ok || added.Action != "user_added" || added.Resource != "group" {
		t.Fatalf("unexpected add-user envelope: %+v", added)
	}
	if envelopeIntID(added.ID) != grp.ID {
		t.Fatalf("add-user envelope id = %v, want %d", added.ID, grp.ID)
	}

	if !groupHasUser(t, r, grp.ID, user.ID) {
		t.Fatalf("user %d (%q) not present in group %d after add-user", user.ID, user.Login, grp.ID)
	}

	var removed actionEnvelope
	r.runJSON(t, &removed, "groups", "remove-user", groupArg, userArg)
	if !removed.Ok || removed.Action != "user_removed" || removed.Resource != "group" {
		t.Fatalf("unexpected remove-user envelope: %+v", removed)
	}
	if envelopeIntID(removed.ID) != grp.ID {
		t.Fatalf("remove-user envelope id = %v, want %d", removed.ID, grp.ID)
	}

	if groupHasUser(t, r, grp.ID, user.ID) {
		t.Fatalf("user %d still present in group %d after remove-user", user.ID, grp.ID)
	}
}

func groupHasUser(t *testing.T, r *cliRunner, groupID, userID int) bool {
	t.Helper()
	var got struct {
		Users []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"users"`
	}
	r.runJSON(t, &got, "groups", "get", strconv.Itoa(groupID), "--include-users")
	for _, u := range got.Users {
		if u.ID == userID {
			return true
		}
	}
	return false
}
