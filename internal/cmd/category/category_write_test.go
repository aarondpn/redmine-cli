package category

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

type seenCall struct{ method, uri, body string }

// categoryServer emulates the endpoints the category commands touch. Project
// 1 ("demo") has categories 4 "UI" (assigned to Alice) and 9 "Backend".
func categoryServer(t *testing.T) (*httptest.Server, *[]seenCall) {
	t.Helper()
	var calls []seenCall
	srv := httptest.NewServer(withProject(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, seenCall{r.Method, r.URL.RequestURI(), string(b)})
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && (r.URL.Path == "/projects/demo/issue_categories.json" || r.URL.Path == "/projects/1/issue_categories.json"):
			_, _ = w.Write([]byte(`{"issue_categories":[{"id":4,"name":"UI","project":{"id":1,"name":"Demo"}},{"id":9,"name":"Backend","project":{"id":1,"name":"Demo"}}],"total_count":2}`))
		case r.Method == "GET" && r.URL.Path == "/issue_categories/4.json":
			_, _ = w.Write([]byte(`{"issue_category":{"id":4,"name":"UI","project":{"id":1,"name":"Demo"},"assigned_to":{"id":2,"name":"Alice"}}}`))
		case r.Method == "GET" && r.URL.Path == "/issue_categories/9.json":
			_, _ = w.Write([]byte(`{"issue_category":{"id":9,"name":"Backend","project":{"id":1,"name":"Demo"}}}`))
		case r.Method == "GET" && r.URL.Path == "/users/current.json":
			_, _ = w.Write([]byte(`{"user":{"id":2,"login":"alice","firstname":"Alice","lastname":"A"}}`))
		case r.Method == "POST" && r.URL.Path == "/projects/demo/issue_categories.json":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"issue_category":{"id":12,"name":"New","project":{"id":1,"name":"Demo"},"assigned_to":{"id":2,"name":"Alice"}}}`))
		case r.Method == "PUT" || r.Method == "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.RequestURI())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func lastCall(calls *[]seenCall) seenCall { return (*calls)[len(*calls)-1] }

func TestCategoryGet_ByID_Table(t *testing.T) {
	srv, _ := categoryServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryGet(f)
	cmd.SetArgs([]string{"4"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := testutil.Stdout(f)
	for _, want := range []string{"UI", "Demo", "Alice"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestCategoryGet_ByName_JSON(t *testing.T) {
	srv, _ := categoryServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryGet(f)
	cmd.SetArgs([]string{"backend", "--project", "1", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out := testutil.Stdout(f); !strings.Contains(out, `"id": 9`) || !strings.Contains(out, `"Backend"`) {
		t.Errorf("json = %s", out)
	}
}

func TestCategoryGet_NameWithoutProject(t *testing.T) {
	srv, _ := categoryServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryGet(f)
	cmd.SetArgs([]string{"UI"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("err = %v, want --project hint", err)
	}
}

func TestCategoryCreate(t *testing.T) {
	srv, calls := categoryServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryCreate(f)
	cmd.SetArgs([]string{"--project", "1", "--name", "New", "--assignee", "me", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	c := lastCall(calls)
	if c.method != "POST" || c.body != `{"issue_category":{"name":"New","assigned_to_id":2}}` {
		t.Errorf("call = %+v", c)
	}
	if out := testutil.Stdout(f); !strings.Contains(out, `"id": 12`) {
		t.Errorf("json = %s", out)
	}
}

func TestCategoryCreate_NoneOmitsAssignee(t *testing.T) {
	srv, calls := categoryServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryCreate(f)
	cmd.SetArgs([]string{"--project", "1", "--name", "New", "--assignee", "none"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := lastCall(calls).body; got != `{"issue_category":{"name":"New"}}` {
		t.Errorf("body = %s", got)
	}
}

func TestCategoryCreate_RequiresName(t *testing.T) {
	srv, _ := categoryServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryCreate(f)
	cmd.SetArgs([]string{"--project", "1"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for missing --name")
	}
}

func TestCategoryUpdate_ChangedFieldsOnly(t *testing.T) {
	srv, calls := categoryServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryUpdate(f)
	cmd.SetArgs([]string{"4", "--name", "Interface"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	c := lastCall(calls)
	if c.method != "PUT" || c.uri != "/issue_categories/4.json" || c.body != `{"issue_category":{"name":"Interface"}}` {
		t.Errorf("call = %+v", c)
	}
}

func TestCategoryUpdate_AssigneeNoneClears(t *testing.T) {
	srv, calls := categoryServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryUpdate(f)
	cmd.SetArgs([]string{"4", "--assignee", "none"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := lastCall(calls).body; got != `{"issue_category":{"assigned_to_id":""}}` {
		t.Errorf("body = %s", got)
	}
}

func TestCategoryUpdate_Validation(t *testing.T) {
	srv, calls := categoryServer(t)
	for name, args := range map[string][]string{
		"nothing":     {"4"},
		"zero ID":     {"4", "--assignee", "0"},
		"negative ID": {"4", "--assignee", "-3"},
	} {
		f := testutil.NewFactory(t, srv.URL)
		cmd := newCmdCategoryUpdate(f)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("unexpected requests: %v", *calls)
	}
}

func TestCategoryDelete_Force(t *testing.T) {
	srv, calls := categoryServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryDelete(f)
	cmd.SetArgs([]string{"4", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	c := lastCall(calls)
	if c.method != "DELETE" || c.uri != "/issue_categories/4.json" {
		t.Errorf("call = %+v", c)
	}
}

func TestCategoryDelete_Cancelled(t *testing.T) {
	srv, calls := categoryServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryDelete(f)
	cmd.SetArgs([]string{"4"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, c := range *calls {
		if c.method == "DELETE" {
			t.Errorf("DELETE sent despite declined confirmation")
		}
	}
	if got := testutil.Stderr(f); !strings.Contains(got, "Delete cancelled") || !strings.Contains(got, "without a category") {
		t.Errorf("stderr = %q", got)
	}
}

func TestCategoryDelete_ReassignByName(t *testing.T) {
	srv, calls := categoryServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryDelete(f)
	cmd.SetArgs([]string{"UI", "--project", "1", "--reassign-to", "Backend", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	c := lastCall(calls)
	if c.method != "DELETE" || c.uri != "/issue_categories/4.json?reassign_to_id=9" {
		t.Errorf("call = %+v", c)
	}
}

func TestCategoryDelete_ReassignNameUsesDeletedCategorysProject(t *testing.T) {
	srv, calls := categoryServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryDelete(f)
	cmd.SetArgs([]string{"4", "--reassign-to", "Backend", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := lastCall(calls).uri; got != "/issue_categories/4.json?reassign_to_id=9" {
		t.Errorf("uri = %s", got)
	}
}

func TestCategoryDelete_ReassignToSelfRejected(t *testing.T) {
	srv, calls := categoryServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryDelete(f)
	cmd.SetArgs([]string{"4", "--reassign-to", "4", "--force"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error")
	}
	if len(*calls) != 0 {
		t.Errorf("unexpected requests: %v", *calls)
	}
}

func TestCategoryDelete_ConfirmMentionsReassignTarget(t *testing.T) {
	srv, _ := categoryServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryDelete(f)
	cmd.SetArgs([]string{"4", "--reassign-to", "9"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Stderr(f); !strings.Contains(got, "moved to category 9") {
		t.Errorf("stderr = %q", got)
	}
}
