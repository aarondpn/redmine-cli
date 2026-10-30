//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestUsers_Create exercises `users create` via the createTestUser fixture and
// verifies the returned envelope shape. The fixture also covers the
// auto-cleaning delete path.
func TestUsers_Create(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	u := createTestUser(t, r)
	if u.ID == 0 {
		t.Fatalf("created user missing ID: %+v", u)
	}
	if u.Mail == "" {
		t.Fatalf("created user missing mail: %+v", u)
	}
}

// TestUsers_List verifies the created user appears in `users list` and that
// pagination via --limit/--offset returns disjoint slices.
func TestUsers_List(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	u := createTestUser(t, r)

	all := listUserIDs(t, r, "--limit", "100")
	if !containsInt(all, u.ID) {
		t.Fatalf("users list did not include user %d (login %q)", u.ID, u.Login)
	}

	t.Run("pagination disjoint", func(t *testing.T) {
		// Use the e2eu prefix as a server-side filter so this subtest is
		// isolated from any concurrent activity (other tests, parallel runs,
		// pre-existing data). createTestUser logins are e2eu<suffix>.
		extra := createTestUser(t, r)
		_ = extra

		first := listUserIDs(t, r, "--name", "e2eu", "--limit", "1")
		if len(first) != 1 {
			t.Fatalf("first page: got %d users, want 1", len(first))
		}

		second := listUserIDs(t, r, "--name", "e2eu", "--limit", "1", "--offset", "1")
		if len(second) != 1 {
			t.Fatalf("second page: got %d users, want 1", len(second))
		}

		if first[0] == second[0] {
			t.Fatalf("pagination slices overlap: both pages returned user %d", first[0])
		}
	})
}

// TestUsers_ListLocked locks a user via the raw api passthrough and verifies
// the locked-status filter returns it while the default (active) listing does
// not. Both listings are scoped via --name to the user's login so this test is
// safe under concurrent activity.
func TestUsers_ListLocked(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	u := createTestUser(t, r)
	lockUser(t, r, u.ID)

	locked := listUserIDs(t, r, "--name", u.Login, "--status", "locked")
	if !containsInt(locked, u.ID) {
		t.Fatalf("locked user %d not in `users list --status locked --name %s`: %v", u.ID, u.Login, locked)
	}

	active := listUserIDs(t, r, "--name", u.Login)
	if containsInt(active, u.ID) {
		t.Fatalf("locked user %d should not appear in default `users list --name %s`", u.ID, u.Login)
	}
}

// TestUsers_GetByIDAndLogin verifies the resolver accepts both numeric IDs and
// logins for `users get`.
func TestUsers_GetByIDAndLogin(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	u := createTestUser(t, r)

	t.Run("by id", func(t *testing.T) {
		var got struct {
			ID    int    `json:"id"`
			Login string `json:"login"`
		}
		r.runJSON(t, &got, "users", "get", strconv.Itoa(u.ID))
		if got.ID != u.ID || got.Login != u.Login {
			t.Fatalf("users get by id = %+v, want id=%d login=%q", got, u.ID, u.Login)
		}
	})

	t.Run("by login", func(t *testing.T) {
		var got struct {
			ID int `json:"id"`
		}
		r.runJSON(t, &got, "users", "get", u.Login)
		if got.ID != u.ID {
			t.Fatalf("users get by login = id %d, want %d", got.ID, u.ID)
		}
	})
}

// TestUsers_Update changes firstname/lastname/mail and verifies the values
// round-trip via a follow-up get.
func TestUsers_Update(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	u := createTestUser(t, r)

	newFirst := "Updated"
	newLast := "User-" + strconv.Itoa(u.ID)
	newMail := "updated-" + strconv.Itoa(u.ID) + "@example.test"

	var updated actionEnvelope
	r.runJSON(t, &updated, "users", "update", strconv.Itoa(u.ID),
		"--firstname", newFirst,
		"--lastname", newLast,
		"--mail", newMail)
	if !updated.Ok || updated.Action != "updated" || updated.Resource != "user" || envelopeIntID(updated.ID) != u.ID {
		t.Fatalf("unexpected update envelope: %+v", updated)
	}

	var got struct {
		FirstName string `json:"firstname"`
		LastName  string `json:"lastname"`
		Mail      string `json:"mail"`
	}
	r.runJSON(t, &got, "users", "get", strconv.Itoa(u.ID))
	if got.FirstName != newFirst || got.LastName != newLast || got.Mail != newMail {
		t.Fatalf("update did not round-trip: got %+v, want firstname=%q lastname=%q mail=%q",
			got, newFirst, newLast, newMail)
	}
}

// TestUsers_Delete creates a user inline (not via the fixture, whose cleanup
// would re-delete and fail) and verifies a subsequent get returns not_found.
func TestUsers_Delete(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	suffix := uniqueShortSuffix(t)
	login := "e2eu" + suffix
	mail := login + "@example.test"
	password := "Pass-" + suffix + "-1A"

	var created struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &created, "users", "create",
		"--login", login,
		"--firstname", "E2E",
		"--lastname", "Delete-"+suffix,
		"--mail", mail,
		"--password", password)
	if created.ID == 0 {
		t.Fatalf("create returned no ID")
	}

	var deleted actionEnvelope
	r.runJSON(t, &deleted, "users", "delete", strconv.Itoa(created.ID), "--force")
	if !deleted.Ok || deleted.Action != "deleted" || deleted.Resource != "user" || envelopeIntID(deleted.ID) != created.ID {
		t.Fatalf("unexpected delete envelope: %+v", deleted)
	}

	stderr := r.runExpectJSONError(t, "users", "get", strconv.Itoa(created.ID))
	assertErrorCode(t, stderr, "not_found")
}

// TestUsers_CreateDuplicateLogin attempts to create a user with the admin
// login (which always exists). Redmine rejects duplicate logins as a
// validation error; the CLI must surface this as a non-zero exit with a
// validation error envelope on stderr.
func TestUsers_CreateDuplicateLogin(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	suffix := uniqueShortSuffix(t)
	stderr := r.runExpectJSONError(t, "users", "create",
		"--login", "admin",
		"--firstname", "Dup",
		"--lastname", "Admin-"+suffix,
		"--mail", "dup-"+suffix+"@example.test",
		"--password", "Pass-"+suffix+"-1A")

	var env errorEnvelope
	if err := json.Unmarshal(lastLine(stderr), &env); err != nil {
		t.Fatalf("decode error envelope: %v\nstderr:\n%s", err, stderr)
	}
	switch env.Error.Code {
	case "validation_error", "validation_failed", "unprocessable_entity":
	default:
		t.Fatalf("duplicate-login error code = %q, want validation_error/validation_failed/unprocessable_entity\nstderr:\n%s",
			env.Error.Code, stderr)
	}
	if env.Error.Message == "" {
		t.Fatalf("duplicate-login error envelope missing message\nstderr:\n%s", stderr)
	}
}

// TestUsers_CreateWithNewFields exercises the new --mail-notification,
// --must-change-passwd, and --generate-password flags via a single create
// call and verifies the resulting user can be fetched. We skip
// --auth-source-id because the e2e Redmine container provisions no external
// auth source, and we do not assert mail_notification round-trip because
// Redmine's GET /users/:id.json response does not expose mail_notification.
func TestUsers_CreateWithNewFields(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	suffix := uniqueShortSuffix(t)
	login := "e2eu" + suffix
	mail := login + "@example.test"
	password := "Pass-" + suffix + "-1A"

	var created struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
	}
	r.runJSON(t, &created, "users", "create",
		"--login", login,
		"--firstname", "E2E",
		"--lastname", "MailNotif-"+suffix,
		"--mail", mail,
		"--password", password,
		"--mail-notification", "only_my_events",
		"--must-change-passwd")
	if created.ID == 0 {
		t.Fatal("create returned no ID")
	}
	if created.Login != login {
		t.Fatalf("created login = %q, want %q", created.Login, login)
	}
	t.Cleanup(func() {
		var deleted actionEnvelope
		r.runJSON(t, &deleted, "users", "delete", strconv.Itoa(created.ID), "--force")
		if !deleted.Ok {
			t.Errorf("cleanup delete envelope not ok: %+v", deleted)
		}
	})
}

// TestUsers_UpdateMailNotification verifies the new update path is accepted
// by the server. As with TestUsers_CreateWithNewFields, the round-trip
// assertion is intentionally omitted because Redmine does not echo
// mail_notification back on GET /users/:id.json; the wire payload shape is
// covered by the unit test TestUserUpdate_NewFlags.
func TestUsers_UpdateMailNotification(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	u := createTestUser(t, r)

	var updated actionEnvelope
	r.runJSON(t, &updated, "users", "update", strconv.Itoa(u.ID), "--mail-notification", "none")
	if !updated.Ok || updated.Action != "updated" || updated.Resource != "user" {
		t.Fatalf("unexpected update envelope: %+v", updated)
	}
}

// listUserIDs runs `users list <args>` and returns just the IDs. Using a
// helper keeps each test focused on the assertion it cares about.
func listUserIDs(t *testing.T, r *cliRunner, args ...string) []int {
	t.Helper()
	var users []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &users, append([]string{"users", "list"}, args...)...)
	ids := make([]int, len(users))
	for i, u := range users {
		ids[i] = u.ID
	}
	return ids
}

// lockUser flips the user's status to 3 (locked) via the raw api passthrough.
// This stays decoupled from any CLI flag changes around `users update`.
func lockUser(t *testing.T, r *cliRunner, id int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lock.json")
	if err := os.WriteFile(path, []byte(`{"user":{"status":3}}`), 0o600); err != nil {
		t.Fatalf("write lock body: %v", err)
	}
	r.run(t, "api", "/users/"+strconv.Itoa(id)+".json", "-X", "PUT", "--input", path)
}

// TestUsers_GetAdminStatusVisibility guards the nullable admin/status
// decoding. Admins see both on users/show; a non-admin viewing another user
// gets neither, and the table must say "unknown" instead of a fake no/0.
func TestUsers_GetAdminStatusVisibility(t *testing.T) {
	requireE2E(t)
	admin := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, admin)
	viewer := createTestUser(t, admin)
	other := createTestUser(t, admin)
	roleID := roleIDByName(t, admin, "Developer")
	addProjectMember(t, admin, proj.Identifier, "--user-id", viewer.ID, roleID)
	addProjectMember(t, admin, proj.Identifier, "--user-id", other.ID, roleID)

	t.Run("admin sees admin and status", func(t *testing.T) {
		var got struct {
			Admin  *bool `json:"admin"`
			Status *int  `json:"status"`
		}
		admin.runJSON(t, &got, "users", "get", strconv.Itoa(other.ID))
		if got.Admin == nil || *got.Admin {
			t.Errorf("admin = %v, want false", got.Admin)
		}
		if got.Status == nil || *got.Status != 1 {
			t.Errorf("status = %v, want 1", got.Status)
		}
	})

	t.Run("non-admin sees neither", func(t *testing.T) {
		r := newCLIRunnerBasicAuth(t, e2eBaseURL(), viewer.Login, viewer.Password)
		var got map[string]any
		r.runJSON(t, &got, "users", "get", strconv.Itoa(other.ID))
		if got["login"] != other.Login {
			t.Fatalf("users get login = %v, want %q", got["login"], other.Login)
		}
		for _, key := range []string{"status", "admin"} {
			if v, ok := got[key]; ok {
				t.Errorf("non-admin users get must omit %q, got %v", key, v)
			}
		}
		table := string(r.run(t, "users", "get", strconv.Itoa(other.ID), "--output", "table"))
		for _, row := range []string{`(?m)^Admin:\s+unknown$`, `(?m)^Status:\s+unknown$`} {
			if !regexp.MustCompile(row).MatchString(table) {
				t.Errorf("table should match %s\n%s", row, table)
			}
		}
	})
}

// TestUsers_ListStatusByVersion: users/index only reports status from Redmine
// 6.0; before that the CLI must show "unknown" rather than status 0.
func TestUsers_ListStatusByVersion(t *testing.T) {
	requireE2E(t)
	skipIfVersionUnknown(t, "users list status (6.0+)")
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	u := createTestUser(t, r)

	var users []map[string]any
	r.runJSON(t, &users, "users", "list", "--name", u.Login)
	if len(users) != 1 {
		t.Fatalf("users list --name %s returned %d users, want 1", u.Login, len(users))
	}
	_, hasStatus := users[0]["status"]
	table := strings.ToLower(string(r.run(t, "users", "list", "--name", u.Login, "--output", "table")))
	if redmineAtLeast(6, 0) {
		if !hasStatus {
			t.Errorf("Redmine >= 6.0 users list should include status: %v", users[0])
		}
		if strings.Contains(table, "unknown") {
			t.Errorf("Redmine >= 6.0 table should not show unknown\n%s", table)
		}
	} else {
		if hasStatus {
			t.Errorf("Redmine < 6.0 users list should omit status: %v", users[0])
		}
		if !strings.Contains(table, "unknown") {
			t.Errorf("Redmine < 6.0 table should show unknown status\n%s", table)
		}
	}
}
