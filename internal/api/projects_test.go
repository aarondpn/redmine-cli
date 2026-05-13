package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func TestProjectService_List_PropagatesIncludes(t *testing.T) {
	var gotInclude string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInclude = r.URL.Query().Get("include")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"projects":[],"total_count":0}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Projects = &ProjectService{client: c}

	if _, _, err := c.Projects.List(context.Background(), []string{"trackers", "enabled_modules"}, 0, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotInclude != "trackers,enabled_modules" {
		t.Errorf("include = %q, want %q", gotInclude, "trackers,enabled_modules")
	}
}

func TestProjectService_Get_PropagatesIncludes(t *testing.T) {
	var gotInclude string
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInclude = r.URL.Query().Get("include")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"project":{"id":1,"name":"Demo","identifier":"demo","enabled_modules":[{"id":1,"name":"issue_tracking"}],"trackers":[{"id":1,"name":"Bug"}]}}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Projects = &ProjectService{client: c}

	p, err := c.Projects.Get(context.Background(), "demo", []string{"trackers", "enabled_modules"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != "/projects/demo.json" {
		t.Errorf("path = %q, want /projects/demo.json", gotPath)
	}
	if gotInclude != "trackers,enabled_modules" {
		t.Errorf("include = %q, want %q", gotInclude, "trackers,enabled_modules")
	}
	if len(p.EnabledModules) != 1 || p.EnabledModules[0].Name != "issue_tracking" {
		t.Errorf("enabled_modules = %+v, want one module 'issue_tracking'", p.EnabledModules)
	}
	if len(p.Trackers) != 1 || p.Trackers[0].Name != "Bug" {
		t.Errorf("trackers = %+v, want one tracker 'Bug'", p.Trackers)
	}
}

func TestProjectService_Create_SerializesExtendedFields(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"project":{"id":42,"identifier":"demo","name":"Demo"}}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Projects = &ProjectService{client: c}

	pub := true
	_, err := c.Projects.Create(context.Background(), models.ProjectCreate{
		Name:                "Demo",
		Identifier:          "demo",
		Description:         "Test",
		Homepage:            "https://example.test",
		IsPublic:            &pub,
		ParentID:            7,
		InheritMembers:      true,
		DefaultAssignedToID: 3,
		TrackerIDs:          []int{1, 2},
		EnabledModuleNames:  []string{"issue_tracking", "wiki"},
		IssueCustomFieldIDs: []int{10},
		CustomFieldValues:   map[string]string{"5": "hello"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	proj, ok := gotBody["project"].(map[string]any)
	if !ok {
		t.Fatalf("body missing 'project' wrapper: %v", gotBody)
	}
	if proj["homepage"] != "https://example.test" {
		t.Errorf("homepage = %v, want https://example.test", proj["homepage"])
	}
	if proj["is_public"] != true {
		t.Errorf("is_public = %v, want true", proj["is_public"])
	}
	if proj["parent_id"].(float64) != 7 {
		t.Errorf("parent_id = %v, want 7", proj["parent_id"])
	}
	if proj["inherit_members"] != true {
		t.Errorf("inherit_members = %v, want true", proj["inherit_members"])
	}
	if proj["default_assigned_to_id"].(float64) != 3 {
		t.Errorf("default_assigned_to_id = %v, want 3", proj["default_assigned_to_id"])
	}
	trackerIDs := proj["tracker_ids"].([]any)
	if len(trackerIDs) != 2 || trackerIDs[0].(float64) != 1 || trackerIDs[1].(float64) != 2 {
		t.Errorf("tracker_ids = %v, want [1 2]", trackerIDs)
	}
	modules := proj["enabled_module_names"].([]any)
	if len(modules) != 2 || modules[0] != "issue_tracking" || modules[1] != "wiki" {
		t.Errorf("enabled_module_names = %v, want [issue_tracking wiki]", modules)
	}
	cfvs := proj["custom_field_values"].(map[string]any)
	if cfvs["5"] != "hello" {
		t.Errorf("custom_field_values[5] = %v, want hello", cfvs["5"])
	}
}

func TestProjectService_Update_OmitsUnsetFields(t *testing.T) {
	var gotBody map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Projects = &ProjectService{client: c}

	homepage := "https://updated.test"
	if err := c.Projects.Update(context.Background(), "demo", models.ProjectUpdate{
		Homepage: &homepage,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	proj := gotBody["project"].(map[string]any)
	if proj["homepage"] != "https://updated.test" {
		t.Errorf("homepage = %v, want https://updated.test", proj["homepage"])
	}
	// Sanity: untouched fields must be absent from the JSON body.
	for _, key := range []string{"name", "description", "is_public", "tracker_ids", "enabled_module_names"} {
		if _, ok := proj[key]; ok {
			t.Errorf("body unexpectedly contained %q: %v", key, proj[key])
		}
	}
}

func TestProjectService_Archive(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Projects = &ProjectService{client: c}

	if err := c.Projects.Archive(context.Background(), "demo"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if gotPath != "/projects/demo/archive.json" {
		t.Errorf("path = %s, want /projects/demo/archive.json", gotPath)
	}
}

func TestProjectService_Unarchive(t *testing.T) {
	var gotMethod, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Projects = &ProjectService{client: c}

	if err := c.Projects.Unarchive(context.Background(), "demo"); err != nil {
		t.Fatalf("unarchive: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if gotPath != "/projects/demo/unarchive.json" {
		t.Errorf("path = %s, want /projects/demo/unarchive.json", gotPath)
	}
}

func TestProjectService_Archive_PropagatesError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":["Not Found"]}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Projects = &ProjectService{client: c}

	err := c.Projects.Archive(context.Background(), "demo")
	if err == nil {
		t.Fatal("expected error from 404 response, got nil")
	}
	if !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("err = %v, want a 404/Not Found message", err)
	}
}
