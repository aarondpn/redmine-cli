//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Writable user, my-account, group, project, version and membership fields
// (#206). Most of them are write-only in the Redmine API (language, pref,
// notified projects, twofa_required, default_issue_query_id), so the tests
// read them back through their visible effect where one exists.

// TestUserUpdate_PasswordStdin sets a new password from standard input and
// logs in with it.
func TestUserUpdate_PasswordStdin(t *testing.T) {
	requireE2E(t)
	admin := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	user := createTestUser(t, admin)
	newPassword := "New-" + uniqueShortSuffix(t) + "-9z"

	stdout, stderr, err := runWithStdin(admin, newPassword+"\n", "users", "update", strconv.Itoa(user.ID), "--password-stdin")
	if err != nil {
		t.Fatalf("users update --password-stdin: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	var me struct {
		Login string `json:"login"`
	}
	newCLIRunnerBasicAuth(t, e2eBaseURL(), user.Login, newPassword).runJSON(t, &me, "users", "me")
	if me.Login != user.Login {
		t.Fatalf("login with the new password returned %q, want %q", me.Login, user.Login)
	}
	if _, stderr := newCLIRunnerBasicAuth(t, e2eBaseURL(), user.Login, user.Password).runExpectError(t, "users", "me"); !strings.Contains(string(stderr), "auth_failed") {
		t.Errorf("the old password still works or fails differently:\n%s", stderr)
	}

	_, stderr, err = runWithStdin(admin, "", "users", "update", strconv.Itoa(user.ID), "--password-stdin")
	if err == nil || !strings.Contains(string(stderr), "standard input is empty") {
		t.Errorf("empty stdin: err = %v\nstderr:\n%s", err, stderr)
	}
	// Redmine's password rules surface as a validation error.
	_, stderr, err = runWithStdin(admin, "short\n", "users", "update", strconv.Itoa(user.ID), "--password-stdin")
	if err == nil || !strings.Contains(string(stderr), "validation_failed") {
		t.Errorf("too short a password: err = %v\nstderr:\n%s", err, stderr)
	}
}

// TestUserCreate_StatusLanguagePref creates users with --status, --language
// and --pref and reads each back through its visible effect.
func TestUserCreate_StatusLanguagePref(t *testing.T) {
	requireE2E(t)
	admin := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	locked := createTestUser(t, admin, "--status", "3")
	var got struct {
		Status int `json:"status"`
	}
	admin.runJSON(t, &got, "users", "get", strconv.Itoa(locked.ID))
	if got.Status != 3 {
		t.Errorf("status of a user created with --status 3 = %d", got.Status)
	}

	// hide_mail decides whether other non-admin users see the address. A
	// shared project makes the two users visible to each other.
	hidden := createTestUser(t, admin, "--language", "de", "--pref", "hide_mail=1")
	viewer := createTestUser(t, admin)
	shared := createTestProject(t, admin)
	addProjectMember(t, admin, shared.Identifier, "--user-id", hidden.ID, firstRoleID(t, admin))
	addProjectMember(t, admin, shared.Identifier, "--user-id", viewer.ID, firstRoleID(t, admin))
	viewerRunner := newCLIRunnerBasicAuth(t, e2eBaseURL(), viewer.Login, viewer.Password)
	mailSeen := func() string {
		var u struct {
			Mail string `json:"mail"`
		}
		viewerRunner.runJSON(t, &u, "users", "get", strconv.Itoa(hidden.ID))
		return u.Mail
	}
	if mail := mailSeen(); mail != "" {
		t.Errorf("--pref hide_mail=1: another user still sees %q", mail)
	}
	var env actionEnvelope
	admin.runJSON(t, &env, "users", "update", strconv.Itoa(hidden.ID), "--pref", "hide_mail=0")
	if mail := mailSeen(); mail != hidden.Mail {
		t.Errorf("--pref hide_mail=0: another user sees %q, want %q", mail, hidden.Mail)
	}

	// The language shows in the user's own validation messages.
	_, stderr := newCLIRunnerBasicAuth(t, e2eBaseURL(), hidden.Login, hidden.Password).runExpectError(t, "my-account", "update", "--firstname", "")
	if !strings.Contains(string(stderr), "Vorname") {
		t.Errorf("--language de: validation message is not German:\n%s", stderr)
	}
}

// TestUserUpdate_GroupsNotifiedProjectsLanguage covers --group (replace and
// clear), --notified-project with --mail-notification selected and
// --language on update.
func TestUserUpdate_GroupsNotifiedProjectsLanguage(t *testing.T) {
	requireE2E(t)
	admin := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	user := createTestUser(t, admin)
	g1 := createTestGroup(t, admin)
	g2 := createTestGroup(t, admin)
	proj := createTestProject(t, admin)
	addProjectMember(t, admin, proj.Identifier, "--user-id", user.ID, firstRoleID(t, admin))

	groupIDs := func() []int {
		var u struct {
			Groups []struct {
				ID int `json:"id"`
			} `json:"groups"`
		}
		admin.runJSON(t, &u, "users", "get", strconv.Itoa(user.ID), "--include", "groups")
		ids := []int{}
		for _, g := range u.Groups {
			ids = append(ids, g.ID)
		}
		return ids
	}

	var env actionEnvelope
	admin.runJSON(t, &env, "users", "update", strconv.Itoa(user.ID),
		"--group", g1.Name, "--group", strconv.Itoa(g2.ID),
		"--mail-notification", "selected", "--notified-project", proj.Identifier,
		"--language", "fr")
	if got := groupIDs(); len(got) != 2 || !containsInt(got, g1.ID) || !containsInt(got, g2.ID) {
		t.Errorf("groups after --group %s --group %d = %v", g1.Name, g2.ID, got)
	}

	admin.runJSON(t, &env, "users", "update", strconv.Itoa(user.ID), "--group", strconv.Itoa(g2.ID))
	if got := groupIDs(); len(got) != 1 || got[0] != g2.ID {
		t.Errorf("groups after replacing with %d = %v", g2.ID, got)
	}
	admin.runJSON(t, &env, "users", "update", strconv.Itoa(user.ID), "--group", "", "--notified-project", "")
	if got := groupIDs(); len(got) != 0 {
		t.Errorf(`groups after --group "" = %v, want none`, got)
	}

	_, stderr := admin.runExpectError(t, "users", "update", strconv.Itoa(user.ID), "--group", "no-such-group-"+uniqueShortSuffix(t))
	if !strings.Contains(string(stderr), "resolve --group") {
		t.Errorf("unknown group name: stderr =\n%s", stderr)
	}
}

// TestMyAccountUpdate_LanguagePrefNotifiedProjects runs my-account update as
// a regular user.
func TestMyAccountUpdate_LanguagePrefNotifiedProjects(t *testing.T) {
	requireE2E(t)
	admin := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	user := createTestUser(t, admin)
	proj := createTestProject(t, admin)
	addProjectMember(t, admin, proj.Identifier, "--user-id", user.ID, firstRoleID(t, admin))
	r := newCLIRunnerBasicAuth(t, e2eBaseURL(), user.Login, user.Password)

	var env actionEnvelope
	r.runJSON(t, &env, "my-account", "update",
		"--mail-notification", "selected", "--notified-project", proj.Identifier,
		"--pref", "no_self_notified=1", "--pref", "hide_mail=1", "--language", "de")
	if !env.Ok {
		t.Fatalf("envelope = %+v", env)
	}
	var got struct {
		MailNotification string `json:"mail_notification"`
	}
	r.runJSON(t, &got, "my-account", "get")
	if got.MailNotification != "" && got.MailNotification != "selected" {
		t.Errorf("mail_notification = %q, want selected", got.MailNotification)
	}
	_, stderr := r.runExpectError(t, "my-account", "update", "--firstname", "")
	if !strings.Contains(string(stderr), "Vorname") {
		t.Errorf("--language de: validation message is not German:\n%s", stderr)
	}

	// A pref-only update sends an empty user object, which Redmine accepts.
	r.runJSON(t, &env, "my-account", "update", "--pref", "comments_sorting=desc")
	if !env.Ok {
		t.Fatalf("pref-only update envelope = %+v", env)
	}
}

// TestGroups_TwofaRequired: Redmine 5.0+ stores twofa_required, older
// versions ignore it (the docs promise both). No API response returns it.
func TestGroups_TwofaRequired(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	g := createTestGroup(t, r, "--twofa-required")
	var env actionEnvelope
	r.runJSON(t, &env, "groups", "update", strconv.Itoa(g.ID), "--twofa-required=false")
	if !env.Ok {
		t.Fatalf("envelope = %+v", env)
	}
}

// TestProjects_DefaultIssueQuery sets the seeded public query as the default
// issue query by name, then clears it. Redmine 5.0+ stores it, older versions
// ignore it; no API response returns it.
func TestProjects_DefaultIssueQuery(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	identifier := uniqueIdentifier(t)
	var created struct {
		Identifier string `json:"identifier"`
	}
	r.runJSON(t, &created, "projects", "create", "--name", "CLI E2E "+identifier, "--identifier", identifier,
		"--default-issue-query", seededQueryName)
	t.Cleanup(func() {
		var deleted actionEnvelope
		r.runJSON(t, &deleted, "projects", "delete", identifier, "--force")
	})
	if created.Identifier != identifier {
		t.Fatalf("created = %+v", created)
	}

	var env actionEnvelope
	r.runJSON(t, &env, "projects", "update", identifier, "--default-issue-query", strconv.Itoa(requireSeededQueryID(t, r)))
	r.runJSON(t, &env, "projects", "update", identifier, "--default-issue-query", "")
	if !env.Ok {
		t.Fatalf("envelope = %+v", env)
	}

	_, stderr := r.runExpectError(t, "projects", "update", identifier, "--default-issue-query", "No such query "+uniqueShortSuffix(t))
	if !strings.Contains(string(stderr), "resolve --default-issue-query") {
		t.Errorf("unknown query name: stderr =\n%s", stderr)
	}
}

// TestVersions_DefaultProjectVersion makes a version the project default on
// create and on update and reads it back from the project.
func TestVersions_DefaultProjectVersion(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	defaultVersion := func() int {
		var p struct {
			DefaultVersion *struct {
				ID int `json:"id"`
			} `json:"default_version"`
		}
		r.runJSON(t, &p, "projects", "get", proj.Identifier)
		if p.DefaultVersion == nil {
			return 0
		}
		return p.DefaultVersion.ID
	}

	var v1, v2 struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &v1, "versions", "create", "--project", proj.Identifier, "--name", "v1", "--default-project-version")
	if got := defaultVersion(); got != v1.ID {
		t.Fatalf("default version after create = %d, want %d", got, v1.ID)
	}
	r.runJSON(t, &v2, "versions", "create", "--project", proj.Identifier, "--name", "v2")
	if got := defaultVersion(); got != v1.ID {
		t.Fatalf("a version created without the flag changed the default to %d", got)
	}
	var env actionEnvelope
	r.runJSON(t, &env, "versions", "update", strconv.Itoa(v2.ID), "--default-project-version")
	if got := defaultVersion(); got != v2.ID {
		t.Fatalf("default version after update = %d, want %d", got, v2.ID)
	}

	_, stderr := r.runExpectError(t, "versions", "update", strconv.Itoa(v2.ID), "--default-project-version=false")
	if !strings.Contains(string(stderr), "--default-version") {
		t.Errorf("--default-project-version=false: stderr =\n%s", stderr)
	}
}

// TestMemberships_CreateSeveralUsers adds two users in one request and checks
// that a principal Redmine did not add is reported.
func TestMemberships_CreateSeveralUsers(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	u1 := createTestUser(t, r)
	u2 := createTestUser(t, r)
	u3 := createTestUser(t, r)
	roleID := strconv.Itoa(firstRoleID(t, r))

	var created []struct {
		ID   int `json:"id"`
		User *struct {
			ID int `json:"id"`
		} `json:"user"`
	}
	r.runJSON(t, &created, "memberships", "create", "--project", proj.Identifier,
		"--user", u1.Login, "--user", strconv.Itoa(u2.ID), "--role-ids", roleID)
	if len(created) != 2 || created[0].User == nil || created[0].User.ID != u1.ID || created[1].User == nil || created[1].User.ID != u2.ID {
		t.Fatalf("created = %+v, want memberships of %d and %d in flag order", created, u1.ID, u2.ID)
	}
	for _, m := range created {
		if !listIncludesMembership(t, r, proj.Identifier, m.ID) {
			t.Errorf("membership %d missing from the list", m.ID)
		}
	}

	// A single --user still prints one membership object.
	var single struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &single, "memberships", "create", "--project", proj.Identifier, "--user", u3.Login, "--role-ids", roleID)
	if single.ID == 0 {
		t.Fatalf("single --user output = %+v", single)
	}

	stderr := r.runExpectJSONError(t, "memberships", "create", "--project", proj.Identifier,
		"--user", strconv.Itoa(u1.ID), "--user", "999999", "--role-ids", roleID)
	if !strings.Contains(string(stderr), "999999") || !strings.Contains(string(stderr), "were not added") {
		t.Errorf("unknown principal in a bulk request: stderr =\n%s", stderr)
	}
}

// TestMCPStdio_WritableFields drives the new MCP fields and reads them back
// where Redmine returns them.
func TestMCPStdio_WritableFields(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	user := createTestUser(t, r)
	other := createTestUser(t, r)
	group := createTestGroup(t, r)
	roleID := firstRoleID(t, r)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	session := r.startMCPStdio(t, ctx, "--enable-writes")

	t.Run("update_user rejects password", func(t *testing.T) {
		out, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "update_user", Arguments: map[string]any{"id": user.ID, "password": "Not-Allowed-123"}})
		if err == nil && !out.IsError {
			t.Fatalf("update_user accepted a password: %s", structuredJSON(out))
		}
		var me struct {
			Login string `json:"login"`
		}
		newCLIRunnerBasicAuth(t, e2eBaseURL(), user.Login, user.Password).runJSON(t, &me, "users", "me")
	})

	t.Run("update_user groups, language, pref", func(t *testing.T) {
		shared := createTestProject(t, r)
		addProjectMember(t, r, shared.Identifier, "--user-id", user.ID, firstRoleID(t, r))
		addProjectMember(t, r, shared.Identifier, "--user-id", other.ID, firstRoleID(t, r))
		mcpCallOK(t, ctx, session, "update_user", map[string]any{
			"id": user.ID, "group_ids": []int{group.ID}, "language": "de",
			"pref": map[string]any{"hide_mail": "1", "time_zone": "Berlin"},
		})
		var u struct {
			Groups []struct {
				ID int `json:"id"`
			} `json:"groups"`
		}
		r.runJSON(t, &u, "users", "get", strconv.Itoa(user.ID), "--include", "groups")
		if len(u.Groups) != 1 || u.Groups[0].ID != group.ID {
			t.Errorf("groups = %+v, want [%d]", u.Groups, group.ID)
		}
		var seen struct {
			Mail string `json:"mail"`
		}
		newCLIRunnerBasicAuth(t, e2eBaseURL(), other.Login, other.Password).runJSON(t, &seen, "users", "get", strconv.Itoa(user.ID))
		if seen.Mail != "" {
			t.Errorf("pref hide_mail=1 via MCP: mail still visible (%q)", seen.Mail)
		}
	})

	t.Run("create_membership user_ids", func(t *testing.T) {
		out := mcpCallOK(t, ctx, session, "create_membership", map[string]any{
			"project_id": proj.Identifier, "user_ids": []int{user.ID, other.ID}, "role_ids": []int{roleID},
		})
		var first struct {
			User struct {
				ID int `json:"id"`
			} `json:"user"`
		}
		raw, _ := json.Marshal(out.StructuredContent)
		_ = json.Unmarshal(raw, &first)
		if first.User.ID != user.ID {
			t.Errorf("result = %s, want the first principal's membership", raw)
		}
		var list []struct {
			User *struct {
				ID int `json:"id"`
			} `json:"user"`
		}
		r.runJSON(t, &list, "memberships", "list", "--project", proj.Identifier)
		found := map[int]bool{}
		for _, m := range list {
			if m.User != nil {
				found[m.User.ID] = true
			}
		}
		if !found[user.ID] || !found[other.ID] {
			t.Errorf("memberships = %+v, want both users", list)
		}
	})

	t.Run("update_my_account", func(t *testing.T) {
		mcpCallOK(t, ctx, session, "update_my_account", map[string]any{"pref": map[string]any{"comments_sorting": "asc"}})
	})

	t.Run("versions default_project_version", func(t *testing.T) {
		out := mcpCallOK(t, ctx, session, "create_version", map[string]any{
			"project_id": proj.Identifier, "name": "mcp-v1", "default_project_version": true,
		})
		id := mcpExtIssueIDFromResult(t, out)
		var p struct {
			DefaultVersion *struct {
				ID int `json:"id"`
			} `json:"default_version"`
		}
		r.runJSON(t, &p, "projects", "get", proj.Identifier)
		if p.DefaultVersion == nil || p.DefaultVersion.ID != id {
			t.Errorf("default_version = %+v, want %d", p.DefaultVersion, id)
		}
		mcpCallOK(t, ctx, session, "update_version", map[string]any{"id": id, "default_project_version": true})
	})

	t.Run("groups twofa_required and project default_issue_query_id", func(t *testing.T) {
		mcpCallOK(t, ctx, session, "update_group", map[string]any{"id": group.ID, "twofa_required": true})
		mcpCallOK(t, ctx, session, "update_project", map[string]any{
			"identifier": proj.Identifier, "default_issue_query_id": requireSeededQueryID(t, r),
		})
	})
}
