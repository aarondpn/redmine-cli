package project

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// TestCmdProjectCreate_ExtendedFieldsSerialize covers the full set of new
// flags: homepage, public, parent, inherit-members, default-assignee,
// tracker (with name resolution via /trackers.json), enable-module, and
// numeric --custom-field passthrough. The trackerHits counter pins the
// batch-resolver invariant: a single --tracker flag with N values must
// issue exactly one /trackers.json call.
func TestCmdProjectCreate_ExtendedFieldsSerialize(t *testing.T) {
	var posted map[string]any
	var trackerHits atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/trackers.json":
			trackerHits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"trackers":[{"id":11,"name":"Bug"},{"id":12,"name":"Feature"}]}`))
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

	if got := trackerHits.Load(); got != 1 {
		t.Errorf("/trackers.json hit %d times, want 1 (batch resolver should fetch once for N names)", got)
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

// TestCmdProjectCreate_IssueCustomFieldsBatchResolves exercises the parallel
// path to the tracker resolver: --issue-custom-field with N names must hit
// /custom_fields.json exactly once thanks to ResolveCustomFieldNames.
func TestCmdProjectCreate_IssueCustomFieldsBatchResolves(t *testing.T) {
	var posted map[string]any
	var cfHits atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/custom_fields.json":
			cfHits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"custom_fields":[
                {"id":21,"name":"Severity","customized_type":"issue","field_format":"string","is_required":false,"is_filter":true,"searchable":false,"multiple":false,"visible":true},
                {"id":22,"name":"Effort","customized_type":"issue","field_format":"int","is_required":false,"is_filter":false,"searchable":false,"multiple":false,"visible":true}
            ]}`))
		case "/projects.json":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &posted)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"demo","name":"Demo"}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCreate(f)
	cmd.SetArgs([]string{
		"--name", "Demo", "--identifier", "demo",
		"--issue-custom-field", "Severity,Effort",
		"--output", "json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if got := cfHits.Load(); got != 1 {
		t.Errorf("/custom_fields.json hit %d times, want 1 (batch resolver should fetch once)", got)
	}

	proj := posted["project"].(map[string]any)
	ids := proj["issue_custom_field_ids"].([]any)
	if len(ids) != 2 || ids[0].(float64) != 21 || ids[1].(float64) != 22 {
		t.Errorf("issue_custom_field_ids = %v, want [21 22]", ids)
	}
}

// TestCmdProjectCreate_CustomFieldNameKey covers the name-keyed branch of
// parseCustomFieldValues, which must resolve "Severity" against
// /custom_fields.json and emit the value under the resolved ID as a string
// key. The numeric-key short-circuit is exercised by
// TestCmdProjectCreate_ExtendedFieldsSerialize.
func TestCmdProjectCreate_CustomFieldNameKey(t *testing.T) {
	var posted map[string]any
	var cfHits atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/custom_fields.json":
			cfHits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"custom_fields":[
                {"id":99,"name":"Severity","customized_type":"project","field_format":"string","is_required":false,"is_filter":false,"searchable":false,"multiple":false,"visible":true}
            ]}`))
		case "/projects.json":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &posted)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"demo","name":"Demo"}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCreate(f)
	cmd.SetArgs([]string{
		"--name", "Demo", "--identifier", "demo",
		"--custom-field", "Severity=high",
		"--output", "json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if got := cfHits.Load(); got != 1 {
		t.Errorf("/custom_fields.json hit %d times, want 1", got)
	}

	proj := posted["project"].(map[string]any)
	cfvs := proj["custom_field_values"].(map[string]any)
	if cfvs["99"] != "high" {
		t.Errorf("custom_field_values[99] = %v, want high", cfvs["99"])
	}
	// Verify the name key did not leak through unresolved.
	if _, present := cfvs["Severity"]; present {
		t.Errorf("custom_field_values still contains unresolved name key: %v", cfvs)
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
