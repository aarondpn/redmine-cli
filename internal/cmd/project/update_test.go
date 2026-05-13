package project

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestCmdProjectUpdate_OnlySetFlagsAreSent(t *testing.T) {
	var putBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/demo.json" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &putBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{"demo", "--homepage", "https://updated.test"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	proj := putBody["project"].(map[string]any)
	if proj["homepage"] != "https://updated.test" {
		t.Errorf("homepage = %v", proj["homepage"])
	}
	for _, k := range []string{"name", "description", "is_public", "parent_id", "inherit_members", "tracker_ids", "enabled_module_names"} {
		if _, ok := proj[k]; ok {
			t.Errorf("body unexpectedly contained %q", k)
		}
	}
}

func TestCmdProjectUpdate_TrackerReplacement(t *testing.T) {
	var putBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/trackers.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"trackers":[{"id":11,"name":"Bug"}]}`))
		case "/projects/demo.json":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &putBody)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{"demo", "--tracker", "Bug"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	proj := putBody["project"].(map[string]any)
	trackers := proj["tracker_ids"].([]any)
	if len(trackers) != 1 || trackers[0].(float64) != 11 {
		t.Errorf("tracker_ids = %v, want [11]", trackers)
	}
}

func TestCmdProjectUpdate_ParentDetachAndResolve(t *testing.T) {
	// Sub-test A: empty --parent must send parent_id=0 (detach) without
	// hitting any resolver endpoint.
	t.Run("EmptyDetaches", func(t *testing.T) {
		var putBody map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/projects/demo.json" {
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &putBody)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		f := testutil.NewFactory(t, srv.URL)
		cmd := newCmdUpdate(f)
		cmd.SetArgs([]string{"demo", "--parent", ""})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
		proj := putBody["project"].(map[string]any)
		if proj["parent_id"].(float64) != 0 {
			t.Errorf("parent_id = %v, want 0 (detach)", proj["parent_id"])
		}
	})

	// Sub-test B: --parent <identifier> resolves to a numeric ID.
	t.Run("ResolvesByIdentifier", func(t *testing.T) {
		var putBody map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/projects/parentproj.json":
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"project":{"id":99,"identifier":"parentproj","name":"Parent"}}`))
			case "/projects/demo.json":
				raw, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(raw, &putBody)
				w.WriteHeader(http.StatusNoContent)
			default:
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
		}))
		defer srv.Close()

		f := testutil.NewFactory(t, srv.URL)
		cmd := newCmdUpdate(f)
		cmd.SetArgs([]string{"demo", "--parent", "parentproj"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
		proj := putBody["project"].(map[string]any)
		if proj["parent_id"].(float64) != 99 {
			t.Errorf("parent_id = %v, want 99", proj["parent_id"])
		}
	})
}

func TestCmdProjectUpdate_DefaultAssigneeEmptyClears(t *testing.T) {
	var putBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/demo.json" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &putBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{"demo", "--default-assignee", ""})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	proj := putBody["project"].(map[string]any)
	if proj["default_assigned_to_id"].(float64) != 0 {
		t.Errorf("default_assigned_to_id = %v, want 0 (clear)", proj["default_assigned_to_id"])
	}
}
