package membership

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

const (
	usersJSON       = `{"users":[{"id":5,"login":"ann","firstname":"Ann","lastname":"Lee"},{"id":6,"login":"bob","firstname":"Bob","lastname":"Ray"}],"total_count":2}`
	annMembership   = `{"id":11,"project":{"id":1,"name":"Demo"},"user":{"id":5,"name":"Ann Lee"},"roles":[{"id":3,"name":"Dev"}]}`
	bobMembership   = `{"id":12,"project":{"id":1,"name":"Demo"},"user":{"id":6,"name":"Bob Ray"},"roles":[{"id":3,"name":"Dev"}]}`
	demoProjectJSON = `{"project":{"id":1,"name":"Demo","identifier":"demo"}}`
)

func TestMembershipCreate_RepeatedUserSendsUserIDs(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{
		"GET /projects/demo.json":              demoProjectJSON,
		"GET /users.json":                      usersJSON,
		"POST /projects/demo/memberships.json": `{"membership":` + annMembership + `}`,
		"GET /projects/demo/memberships.json":  `{"memberships":[` + bobMembership + `,` + annMembership + `],"total_count":2}`,
	}, &writes)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdMembershipCreate(f)
	cmd.SetArgs([]string{"--project", "demo", "--user", "ann", "--user", "6", "--role-ids", "3", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	testutil.RequireOneWrite(t, writes, map[string]any{
		"membership": map[string]any{"user_ids": []any{5.0, 6.0}, "role_ids": []any{3.0}},
	})
	var got []struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &got); err != nil || len(got) != 2 || got[0].ID != 11 || got[1].ID != 12 {
		t.Errorf("stdout = %s (%v), want both memberships in flag order", testutil.Stdout(f), err)
	}
}

func TestMembershipCreate_RepeatedUserTable(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{
		"GET /projects/demo.json":              demoProjectJSON,
		"POST /projects/demo/memberships.json": `{"membership":` + annMembership + `}`,
		"GET /projects/demo/memberships.json":  `{"memberships":[` + annMembership + `,` + bobMembership + `],"total_count":2}`,
	}, &writes)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdMembershipCreate(f)
	cmd.SetArgs([]string{"--project", "demo", "--user", "5", "--user", "6", "--role-ids", "3"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := testutil.Stdout(f) + testutil.Stderr(f)
	for _, want := range []string{"Added 2 members to project Demo", "Ann Lee (membership 11)", "Bob Ray (membership 12)"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestMembershipCreate_OutputShape(t *testing.T) {
	for _, tt := range []struct {
		args      []string
		wantArray bool
	}{
		{[]string{"--project", "demo", "--user-id", "5", "--role-ids", "3", "-o", "json"}, false},
		{[]string{"--project", "demo", "--user", "ann", "--role-ids", "3", "-o", "json"}, true},
		{[]string{"--project", "demo", "--user", "ann", "--user", "5", "--role-ids", "3", "-o", "json"}, true},
	} {
		var writes []map[string]any
		srv := testutil.RouteServer(t, map[string]string{
			"GET /projects/demo.json":              demoProjectJSON,
			"GET /users.json":                      usersJSON,
			"POST /projects/demo/memberships.json": `{"membership":` + annMembership + `}`,
		}, &writes)
		f := testutil.NewFactory(t, srv.URL)
		cmd := newCmdMembershipCreate(f)
		cmd.SetArgs(tt.args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		// A single principal (also a repeated one) is sent as user_id.
		testutil.RequireOneWrite(t, writes, map[string]any{
			"membership": map[string]any{"user_id": 5.0, "role_ids": []any{3.0}},
		})
		out := []byte(testutil.Stdout(f))
		if tt.wantArray {
			var got []struct {
				ID int `json:"id"`
			}
			if err := json.Unmarshal(out, &got); err != nil || len(got) != 1 || got[0].ID != 11 {
				t.Errorf("%v: stdout = %s, want an array with membership 11", tt.args, out)
			}
		} else {
			var got struct {
				ID int `json:"id"`
			}
			if err := json.Unmarshal(out, &got); err != nil || got.ID != 11 {
				t.Errorf("%v: stdout = %s, want a single membership object", tt.args, out)
			}
		}
	}
}

func TestMembershipCreate_PrincipalFlagsAreExclusive(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{}, &writes)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdMembershipCreate(f)
	cmd.SetArgs([]string{"--project", "demo", "--user", "5", "--group-id", "9", "--role-ids", "3"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "none of the others can be") {
		t.Fatalf("err = %v", err)
	}

	cmd = newCmdMembershipCreate(f)
	cmd.SetArgs([]string{"--project", "demo", "--role-ids", "3"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--user, --user-id or --group-id") {
		t.Fatalf("err = %v", err)
	}
}
