package project

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// TestCmdProjectCreate_ExtendedFieldsSerialize covers the full set of new
// flags: homepage, public, parent, inherit-members, default-assignee,
// tracker (with name resolution via /trackers.json), enable-module, and
// numeric --custom-field passthrough.
func TestCmdProjectCreate_ExtendedFieldsSerialize(t *testing.T) {
	var posted map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/trackers.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"trackers":[{"id":11,"name":"Bug"},{"id":12,"name":"Feature"}]}`))
			return
		case "/users/current.json":
			// Resolver.ResolveAssignee accepts a numeric ID directly so this is
			// only hit if the user types --default-assignee me. We pass an
			// integer to avoid the dependency in this test.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"user":{"id":42}}`))
			return
		case "/projects.json":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &posted)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"demo","name":"Demo"}}`))
			return
		}
		t.Fatalf("unexpected path %s", r.URL.Path)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCreate(f)
	cmd.SetArgs([]string{
		"--name", "Demo",
		"--identifier", "demo",
		"--description", "x",
		"--homepage", "https://example.test",
		"--public",
		"--parent", "7",
		"--inherit-members",
		"--default-assignee", "42",
		"--tracker", "Bug,Feature",
		"--enable-module", "issue_tracking,wiki",
		"--custom-field", "5=hello",
		"--output", "json",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	proj, ok := posted["project"].(map[string]any)
	if !ok {
		t.Fatalf("posted body missing project wrapper: %v", posted)
	}

	if proj["name"] != "Demo" || proj["identifier"] != "demo" {
		t.Errorf("core fields = %+v", proj)
	}
	if proj["homepage"] != "https://example.test" {
		t.Errorf("homepage = %v", proj["homepage"])
	}
	if proj["is_public"] != true {
		t.Errorf("is_public = %v, want true", proj["is_public"])
	}
	if proj["parent_id"].(float64) != 7 {
		t.Errorf("parent_id = %v", proj["parent_id"])
	}
	if proj["inherit_members"] != true {
		t.Errorf("inherit_members = %v", proj["inherit_members"])
	}
	if proj["default_assigned_to_id"].(float64) != 42 {
		t.Errorf("default_assigned_to_id = %v", proj["default_assigned_to_id"])
	}
	trackers := proj["tracker_ids"].([]any)
	if len(trackers) != 2 || trackers[0].(float64) != 11 || trackers[1].(float64) != 12 {
		t.Errorf("tracker_ids = %v, want [11 12]", trackers)
	}
	mods := proj["enabled_module_names"].([]any)
	if len(mods) != 2 || mods[0] != "issue_tracking" || mods[1] != "wiki" {
		t.Errorf("enabled_module_names = %v", mods)
	}
	cfvs := proj["custom_field_values"].(map[string]any)
	if cfvs["5"] != "hello" {
		t.Errorf("custom_field_values[5] = %v", cfvs["5"])
	}
}

func TestCmdProjectCreate_UnknownModuleRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be called; got %s", r.URL.Path)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCreate(f)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{
		"--name", "X", "--identifier", "x",
		"--enable-module", "not_a_real_module",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown module")
	}
}

func TestCmdProjectCreate_MinimalBackwardCompatible(t *testing.T) {
	var posted map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &posted)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"demo","name":"Demo"}}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCreate(f)
	cmd.SetArgs([]string{"--name", "Demo", "--identifier", "demo", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	proj := posted["project"].(map[string]any)
	for _, k := range []string{"homepage", "is_public", "parent_id", "inherit_members", "default_assigned_to_id", "tracker_ids", "enabled_module_names", "custom_field_values"} {
		if _, ok := proj[k]; ok {
			t.Errorf("unexpected %q in minimal body: %v", k, proj[k])
		}
	}
}
