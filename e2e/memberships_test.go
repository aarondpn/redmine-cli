//go:build e2e

package e2e

import (
	"strconv"
	"strings"
	"testing"
)

// TestMemberships_UserCRUD drives the full user-membership life cycle:
// create (with --user-id) -> list -> get -> update roles -> delete ->
// verify gone via list.
func TestMemberships_UserCRUD(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	user := createTestUser(t, r)
	roleID := firstRoleID(t, r)

	var created struct {
		ID      int `json:"id"`
		Project struct {
			ID int `json:"id"`
		} `json:"project"`
		User *struct {
			ID int `json:"id"`
		} `json:"user"`
		Group *struct {
			ID int `json:"id"`
		} `json:"group"`
		Roles []struct {
			ID int `json:"id"`
		} `json:"roles"`
	}
	r.runJSON(t, &created, "memberships", "create",
		"--project", proj.Identifier,
		"--user-id", strconv.Itoa(user.ID),
		"--role-ids", strconv.Itoa(roleID))
	if created.ID == 0 {
		t.Fatalf("created membership has zero ID: %+v", created)
	}
	if created.Project.ID != proj.ID {
		t.Fatalf("created membership project.id = %d, want %d", created.Project.ID, proj.ID)
	}
	if created.User == nil || created.User.ID != user.ID {
		t.Fatalf("created membership user = %+v, want id=%d", created.User, user.ID)
	}
	if created.Group != nil {
		t.Fatalf("user-membership unexpectedly set group = %+v", created.Group)
	}
	if !containsRoleID(created.Roles, roleID) {
		t.Fatalf("created membership roles = %+v, want to contain %d", created.Roles, roleID)
	}

	if !listIncludesMembership(t, r, proj.Identifier, created.ID) {
		t.Fatalf("memberships list did not include created membership %d", created.ID)
	}

	var got struct {
		ID   int `json:"id"`
		User *struct {
			ID int `json:"id"`
		} `json:"user"`
	}
	r.runJSON(t, &got, "memberships", "get", strconv.Itoa(created.ID))
	if got.ID != created.ID {
		t.Fatalf("get membership id = %d, want %d", got.ID, created.ID)
	}
	if got.User == nil || got.User.ID != user.ID {
		t.Fatalf("get membership user = %+v, want id=%d", got.User, user.ID)
	}

	// pickAlternateRoleIDs falls back to {roleID} on single-role servers; in
	// that case the update is a self-replace but still exercises the request
	// path and envelope, and the role-set assertion below still holds.
	newRoleIDs := pickAlternateRoleIDs(t, r, roleID)
	var updated actionEnvelope
	r.runJSON(t, &updated, "memberships", "update", strconv.Itoa(created.ID),
		"--role-ids", joinInts(newRoleIDs))
	if !updated.Ok || updated.Action != "updated" || updated.Resource != "membership" {
		t.Fatalf("unexpected update envelope: %+v", updated)
	}
	if envelopeIntID(updated.ID) != created.ID {
		t.Fatalf("update envelope id = %v, want %d", updated.ID, created.ID)
	}

	var afterUpdate struct {
		Roles []struct {
			ID int `json:"id"`
		} `json:"roles"`
	}
	r.runJSON(t, &afterUpdate, "memberships", "get", strconv.Itoa(created.ID))
	if len(afterUpdate.Roles) != len(newRoleIDs) {
		t.Fatalf("after update roles count = %d, want %d (%+v)", len(afterUpdate.Roles), len(newRoleIDs), afterUpdate.Roles)
	}
	for _, want := range newRoleIDs {
		if !containsRoleID(afterUpdate.Roles, want) {
			t.Fatalf("after update roles missing role %d: %+v", want, afterUpdate.Roles)
		}
	}

	var deleted actionEnvelope
	r.runJSON(t, &deleted, "memberships", "delete", strconv.Itoa(created.ID), "--force")
	if !deleted.Ok || deleted.Action != "deleted" || deleted.Resource != "membership" {
		t.Fatalf("unexpected delete envelope: %+v", deleted)
	}
	if envelopeIntID(deleted.ID) != created.ID {
		t.Fatalf("delete envelope id = %v, want %d", deleted.ID, created.ID)
	}

	if listIncludesMembership(t, r, proj.Identifier, created.ID) {
		t.Fatalf("membership %d still present after delete", created.ID)
	}
}

// TestMemberships_GroupCreate covers the group-membership branch of
// `memberships create`. We assert the response envelope carries the group
// (not user) and the membership shows up in list.
func TestMemberships_GroupCreate(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	grp := createTestGroup(t, r)
	roleID := firstRoleID(t, r)

	var created struct {
		ID      int `json:"id"`
		Project struct {
			ID int `json:"id"`
		} `json:"project"`
		User *struct {
			ID int `json:"id"`
		} `json:"user"`
		Group *struct {
			ID int `json:"id"`
		} `json:"group"`
	}
	r.runJSON(t, &created, "memberships", "create",
		"--project", proj.Identifier,
		"--group-id", strconv.Itoa(grp.ID),
		"--role-ids", strconv.Itoa(roleID))
	if created.ID == 0 {
		t.Fatalf("group membership has zero ID: %+v", created)
	}
	if created.Group == nil || created.Group.ID != grp.ID {
		t.Fatalf("group membership group = %+v, want id=%d", created.Group, grp.ID)
	}
	if created.User != nil {
		t.Fatalf("group membership unexpectedly set user = %+v", created.User)
	}
	if created.Project.ID != proj.ID {
		t.Fatalf("group membership project.id = %d, want %d", created.Project.ID, proj.ID)
	}

	if !listIncludesMembership(t, r, proj.Identifier, created.ID) {
		t.Fatalf("memberships list did not include group membership %d", created.ID)
	}
}

func TestMemberships_RoleNameResolution(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	user := createTestUser(t, r)
	role := firstRole(t, r)

	var created struct {
		ID    int `json:"id"`
		Roles []struct {
			ID int `json:"id"`
		} `json:"roles"`
	}
	r.runJSON(t, &created, "memberships", "create",
		"--project", proj.Identifier,
		"--user-id", strconv.Itoa(user.ID),
		"--roles", role.Name)
	if !containsRoleID(created.Roles, role.ID) {
		t.Fatalf("created membership roles = %+v, want role %d", created.Roles, role.ID)
	}

	var updated actionEnvelope
	r.runJSON(t, &updated, "memberships", "update", strconv.Itoa(created.ID),
		"--roles", strconv.Itoa(role.ID))
	if !updated.Ok {
		t.Fatalf("unexpected update envelope: %+v", updated)
	}
}

// TestMemberships_CreateRequiresUserOrGroup verifies the mutual-exclusivity
// validation at internal/cmd/membership/create.go:33: omitting both
// --user-id and --group-id must exit non-zero with the membership cmd's own
// message. With --output json, the message lands in the stdout error envelope.
func TestMemberships_CreateRequiresUserOrGroup(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	roleID := firstRoleID(t, r)

	stdout, stderr := r.runExpectError(t, "memberships", "create",
		"--project", proj.Identifier,
		"--role-ids", strconv.Itoa(roleID))
	const want = "either --user-id or --group-id is required"
	if !strings.Contains(string(stdout), want) && !strings.Contains(string(stderr), want) {
		t.Fatalf("expected error to mention missing --user-id/--group-id\nstdout:\n%s\nstderr:\n%s",
			stdout, stderr)
	}
}

// listIncludesMembership returns true when `memberships list --project` for
// projectID contains the given membership ID. Centralized so the create and
// post-delete checks share one decode shape.
func listIncludesMembership(t *testing.T, r *cliRunner, projectID string, membershipID int) bool {
	t.Helper()
	var listed []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &listed, "memberships", "list", "--project", projectID, "--limit", "100")
	for _, m := range listed {
		if m.ID == membershipID {
			return true
		}
	}
	return false
}

func containsRoleID(roles []struct {
	ID int `json:"id"`
}, id int) bool {
	for _, role := range roles {
		if role.ID == id {
			return true
		}
	}
	return false
}

// pickAlternateRoleIDs returns {role.ID} for the first non-builtin role that
// differs from current. Falls back to {current} when the server only exposes
// one assignable role.
func pickAlternateRoleIDs(t *testing.T, r *cliRunner, current int) []int {
	t.Helper()
	type role struct {
		ID        int  `json:"id"`
		Builtin   bool `json:"builtin"`
		IsBuiltin bool `json:"is_builtin"`
	}
	var resp struct {
		Roles []role `json:"roles"`
	}
	r.runJSON(t, &resp, "api", "/roles.json")
	for _, role := range resp.Roles {
		if role.Builtin || role.IsBuiltin {
			continue
		}
		if role.ID != current {
			return []int{role.ID}
		}
	}
	return []int{current}
}

func joinInts(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, ",")
}
