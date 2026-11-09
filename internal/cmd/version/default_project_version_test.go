package version

import (
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestVersionCreate_DefaultProjectVersion(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{
		"GET /projects/demo.json":           `{"project":{"id":1,"name":"Demo","identifier":"demo"}}`,
		"POST /projects/demo/versions.json": `{"version":{"id":9,"name":"1.0","project":{"id":1,"name":"Demo"}}}`,
	}, &writes)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionCreate(f)
	cmd.SetArgs([]string{"--project", "demo", "--name", "1.0", "--default-project-version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	testutil.RequireOneWrite(t, writes, map[string]any{"version": map[string]any{"name": "1.0", "default_project_version": true}})
}

func TestVersionUpdate_DefaultProjectVersion(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{"PUT /versions/9.json": ""}, &writes)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionUpdate(f)
	cmd.SetArgs([]string{"9", "--default-project-version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	testutil.RequireOneWrite(t, writes, map[string]any{"version": map[string]any{"default_project_version": true}})
}

func TestVersionUpdate_DefaultProjectVersionFalseIsRejected(t *testing.T) {
	var writes []map[string]any
	srv := testutil.RouteServer(t, map[string]string{}, &writes)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionUpdate(f)
	cmd.SetArgs([]string{"9", "--default-project-version=false"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--default-version") {
		t.Fatalf("err = %v, want a pointer to projects update --default-version", err)
	}
}
