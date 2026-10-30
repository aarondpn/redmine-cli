package enumeration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func serve(t *testing.T, routes map[string]string) (*httptest.Server, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.RequestURI())
		body, ok := routes[r.URL.Path]
		if !ok {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func run(t *testing.T, build func(*cmdutil.Factory) *cobra.Command, srvURL string, args ...string) (string, error) {
	t.Helper()
	f := testutil.NewFactory(t, srvURL)
	cmd := build(f)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return testutil.Stdout(f), err
}

const prioBody = `{"issue_priorities":[{"id":1,"name":"Low","is_default":false,"active":true},{"id":2,"name":"Normal","is_default":true,"active":true}]}`

func TestPrioritiesList(t *testing.T) {
	srv, _ := serve(t, map[string]string{"/enumerations/issue_priorities.json": prioBody})

	out, err := run(t, NewCmdPriorities, srv.URL, "list", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("not a JSON array: %v\n%s", err, out)
	}
	if len(items) != 2 || items[1]["name"] != "Normal" || items[1]["is_default"] != true {
		t.Errorf("items = %v", items)
	}

	out, err = run(t, NewCmdPriorities, srv.URL, "list", "-o", "table")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ID", "Name", "Default", "Active", "Low", "Normal", "yes"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
}

func TestDocumentCategoriesList(t *testing.T) {
	srv, _ := serve(t, map[string]string{
		"/enumerations/document_categories.json": `{"document_categories":[{"id":3,"name":"Technical documentation","is_default":false,"active":true}]}`,
	})
	out, err := run(t, NewCmdDocumentCategories, srv.URL, "list", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"name": "Technical documentation"`) {
		t.Errorf("json missing name:\n%s", out)
	}
	out, err = run(t, NewCmdDocumentCategories, srv.URL, "list", "-o", "table")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Technical documentation") || !strings.Contains(out, "Default") {
		t.Errorf("table:\n%s", out)
	}
}

func TestActivitiesList_Global(t *testing.T) {
	srv, _ := serve(t, map[string]string{
		"/enumerations/time_entry_activities.json": `{"time_entry_activities":[{"id":9,"name":"Design","is_default":true,"active":true}]}`,
	})
	out, err := run(t, NewCmdActivities, srv.URL, "list", "-o", "table")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Design", "Default", "Active"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
}

func TestActivitiesList_Project(t *testing.T) {
	srv, seen := serve(t, map[string]string{
		"/projects/demo.json": `{"project":{"id":1,"identifier":"demo","name":"Demo","time_entry_activities":[{"id":50,"name":"Design override"}]}}`,
	})
	out, err := run(t, NewCmdActivities, srv.URL, "list", "--project", "demo", "-o", "table")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Design override") || !strings.Contains(out, "50") {
		t.Errorf("table:\n%s", out)
	}
	if strings.Contains(out, "Default") || strings.Contains(out, "Active") {
		t.Errorf("project table must only show ID and Name:\n%s", out)
	}
	found := false
	for _, u := range *seen {
		if strings.Contains(u, "include=time_entry_activities") {
			found = true
		}
	}
	if !found {
		t.Errorf("no include=time_entry_activities request: %v", *seen)
	}

	out, err = run(t, NewCmdActivities, srv.URL, "list", "--project", "demo", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"id": 50`) {
		t.Errorf("json:\n%s", out)
	}
}

func TestActivitiesList_EmptyJSON(t *testing.T) {
	srv, _ := serve(t, map[string]string{
		"/enumerations/time_entry_activities.json": `{"time_entry_activities":[]}`,
	})
	out, err := run(t, NewCmdActivities, srv.URL, "list", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "[]" {
		t.Errorf("out = %q, want []", out)
	}
}
