package user

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestUserCreate_StatusLanguagePref(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{"POST /users.json": `{"user":{"id":3,"login":"jdoe"}}`}, &writes)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserCreate(f)
	cmd.SetArgs([]string{"--login", "jdoe", "--firstname", "J", "--lastname", "D", "--mail", "j@x.y", "--generate-password",
		"--status", "3", "--language", "de", "--mail-notification", "selected",
		"--pref", "hide_mail=1", "--pref", "time_zone=Eastern Time (US & Canada)", "--pref", "hide_mail=0"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	testutil.RequireOneWrite(t, writes, map[string]any{
		"user": map[string]any{"login": "jdoe", "firstname": "J", "lastname": "D", "mail": "j@x.y",
			"generate_password": true, "status": 3.0, "language": "de", "mail_notification": "selected"},
		"pref": map[string]any{"hide_mail": "0", "time_zone": "Eastern Time (US & Canada)"},
	})
}

func TestUserCreate_RejectsMalformedPref(t *testing.T) {
	for _, bad := range []string{"hide_mail", "=1", "hide mail=1"} {
		var writes []map[string]any
		srv := testutil.RouteServer(t, map[string]string{}, &writes)
		f := testutil.NewFactory(t, srv.URL)
		cmd := newCmdUserCreate(f)
		cmd.SetArgs([]string{"--login", "jdoe", "--firstname", "J", "--lastname", "D", "--mail", "j@x.y", "--generate-password", "--pref", bad})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "--pref") {
			t.Errorf("--pref %q: err = %v", bad, err)
		}
	}
}

func TestUserUpdate_PasswordFileFromStdin(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{"PUT /users/7.json": ""}, &writes)
	f := testutil.NewFactory(t, srv.URL)
	f.IOStreams.In = bytes.NewBufferString("n3w secret\n")
	cmd := newCmdUserUpdate(f)
	cmd.SetArgs([]string{"7", "--password-file", "-"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	testutil.RequireOneWrite(t, writes, map[string]any{
		"user": map[string]any{"password": "n3w secret", "password_confirmation": "n3w secret"},
	})
}

func TestUserUpdate_PasswordFileValidation(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{}, &writes)

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserUpdate(f)
	cmd.SetArgs([]string{"7", "--password-file", "-"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "standard input is empty") {
		t.Errorf("empty stdin: err = %v", err)
	}

	f = testutil.NewFactory(t, srv.URL)
	f.IOStreams.In = bytes.NewBufferString("pw\n")
	cmd = newCmdUserUpdate(f)
	cmd.SetArgs([]string{"7", "--password-file", "-", "--generate-password"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "none of the others can be") {
		t.Errorf("with --generate-password: err = %v", err)
	}
	if len(writes) != 0 {
		t.Errorf("no request may be sent: %v", writes)
	}
}

func TestUserUpdate_GroupsNotifiedProjectsLanguagePref(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{
		"GET /groups.json":       `{"groups":[{"id":4,"name":"Developers"},{"id":5,"name":"QA"}],"total_count":2}`,
		"GET /projects/web.json": `{"project":{"id":12,"name":"Web","identifier":"web"}}`,
		"PUT /users/7.json":      "",
	}, &writes)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserUpdate(f)
	cmd.SetArgs([]string{"7", "--group", "developers", "--group", "5", "--notified-project", "web", "--notified-project", "3",
		"--mail-notification", "selected", "--language", "", "--pref", "no_self_notified=1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	testutil.RequireOneWrite(t, writes, map[string]any{
		"user": map[string]any{"group_ids": []any{4.0, 5.0}, "notified_project_ids": []any{12.0, 3.0},
			"mail_notification": "selected", "language": ""},
		"pref": map[string]any{"no_self_notified": "1"},
	})
}

func TestUserUpdate_EmptyGroupAndNotifiedProjectClear(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{"PUT /users/7.json": ""}, &writes)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserUpdate(f)
	cmd.SetArgs([]string{"7", "--group", "", "--notified-project", ""})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	testutil.RequireOneWrite(t, writes, map[string]any{
		"user": map[string]any{"group_ids": []any{}, "notified_project_ids": []any{}},
	})
}

func TestValidateMailNotification_Selected(t *testing.T) {
	if err := ValidateMailNotification("selected"); err != nil {
		t.Errorf("selected: %v", err)
	}
	if err := ValidateMailNotification("bogus"); err == nil || !strings.Contains(err.Error(), "selected") {
		t.Errorf("bogus: err = %v, want the allowed list to name selected", err)
	}
}
