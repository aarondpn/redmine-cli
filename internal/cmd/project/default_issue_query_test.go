package project

import (
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

const queriesJSON = `{"queries":[{"id":3,"name":"Open bugs","is_public":true},{"id":8,"name":"Sprint","is_public":true,"project_id":7}],"total_count":2}`

func TestProjectCreate_DefaultIssueQueryByName(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{
		"GET /queries.json":   queriesJSON,
		"POST /projects.json": `{"project":{"id":9,"name":"Demo","identifier":"demo"}}`,
	}, &writes)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCreate(f)
	cmd.SetArgs([]string{"--name", "Demo", "--identifier", "demo", "--default-issue-query", "open bugs"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	testutil.RequireOneWrite(t, writes, map[string]any{
		"project": map[string]any{"name": "Demo", "identifier": "demo", "default_issue_query_id": 3.0},
	})
}

func TestProjectUpdate_DefaultIssueQuery(t *testing.T) {
	for _, tt := range []struct {
		name   string
		routes map[string]string
		value  string
		want   float64
	}{
		{"project query by name", map[string]string{
			"GET /queries.json":       queriesJSON,
			"GET /projects/demo.json": `{"project":{"id":7,"name":"Demo","identifier":"demo"}}`,
		}, "Sprint", 8},
		{"numeric ID without lookup", map[string]string{}, "42", 42},
		{"empty clears", map[string]string{}, "", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var writes []map[string]any
			tt.routes["PUT /projects/demo.json"] = ""
			srv := testutil.RouteServer(t, tt.routes, &writes)
			f := testutil.NewFactory(t, srv.URL)
			cmd := newCmdUpdate(f)
			cmd.SetArgs([]string{"demo", "--default-issue-query", tt.value})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			testutil.RequireOneWrite(t, writes, map[string]any{"project": map[string]any{"default_issue_query_id": tt.want}})
		})
	}
}
