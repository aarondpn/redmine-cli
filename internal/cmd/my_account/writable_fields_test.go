package myaccount

import (
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestMyAccountUpdate_LanguagePrefNotifiedProjects(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{
		"GET /projects/web.json": `{"project":{"id":12,"name":"Web","identifier":"web"}}`,
		"PUT /my/account.json":   "",
	}, &writes)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdMyAccountUpdate(f)
	cmd.SetArgs([]string{"--language", "de", "--mail-notification", "selected", "--notified-project", "web",
		"--pref", "no_self_notified=1", "--pref", "comments_sorting=desc"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	testutil.RequireOneWrite(t, writes, map[string]any{
		"user": map[string]any{"language": "de", "mail_notification": "selected", "notified_project_ids": []any{12.0}},
		"pref": map[string]any{"no_self_notified": "1", "comments_sorting": "desc"},
	})
}

func TestMyAccountUpdate_PrefOnly(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{"PUT /my/account.json": ""}, &writes)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdMyAccountUpdate(f)
	cmd.SetArgs([]string{"--pref", "hide_mail=1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	testutil.RequireOneWrite(t, writes, map[string]any{
		"user": map[string]any{},
		"pref": map[string]any{"hide_mail": "1"},
	})
}

func TestMyAccountUpdate_RejectsMalformedPref(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{}, &writes)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdMyAccountUpdate(f)
	cmd.SetArgs([]string{"--pref", "no_self_notified"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("want an error for a --pref without '='")
	}
}
