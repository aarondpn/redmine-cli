package news

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

type seenCall struct{ method, uri, body string }

const (
	newsRow = `{"id":%ID%,"project":{"id":1,"name":"Demo"},"author":{"id":2,"name":"Alice"},"title":"Release","summary":"Sum","description":"Body","created_on":"2024-05-01T10:00:00Z"}`
)

func row(id string) string { return strings.ReplaceAll(newsRow, "%ID%", id) }

// newsServer emulates the news endpoints. Project 1 is "demo".
func newsServer(t *testing.T, listBody string) (*httptest.Server, *[]seenCall) {
	t.Helper()
	var calls []seenCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		calls = append(calls, seenCall{r.Method, r.URL.RequestURI(), string(b)})
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && (r.URL.Path == "/projects/1.json" || r.URL.Path == "/projects/demo.json"):
			_, _ = w.Write([]byte(`{"project":{"id":1,"name":"Demo","identifier":"demo"}}`))
		case r.Method == "GET" && r.URL.Path == "/projects/demo/news.json":
			_, _ = w.Write([]byte(listBody))
		case r.Method == "GET" && r.URL.Path == "/news.json":
			_, _ = w.Write([]byte(listBody))
		case r.Method == "GET" && r.URL.Path == "/news/5.json":
			_, _ = w.Write([]byte(`{"news":{"id":5,"project":{"id":1,"name":"Demo"},"author":{"id":2,"name":"Alice"},"title":"Release","summary":"Sum","description":"Body","created_on":"2024-05-01T10:00:00Z",` +
				`"comments":[{"id":11,"author":{"id":3,"name":"Bob"},"content":"Nice one"}],` +
				`"attachments":[{"id":21,"filename":"notes.txt","filesize":12,"content_type":"text/plain"}]}}`))
		case r.Method == "GET" && r.URL.Path == "/users/current.json":
			_, _ = w.Write([]byte(`{"user":{"id":2,"login":"alice","firstname":"Alice","lastname":"A"}}`))
		case r.Method == "POST" && r.URL.Path == "/uploads.json":
			_, _ = w.Write([]byte(`{"upload":{"token":"tok1"}}`))
		case r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE":
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

func findCall(calls *[]seenCall, method string) (seenCall, bool) {
	for _, c := range *calls {
		if c.method == method && !strings.HasPrefix(c.uri, "/uploads") {
			return c, true
		}
	}
	return seenCall{}, false
}

// --- list ---

func TestNewsList_Table(t *testing.T) {
	srv, _ := newsServer(t, `{"news":[`+row("5")+`],"total_count":1}`)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdList(f)
	cmd.SetArgs([]string{"--project", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := testutil.Stdout(f)
	for _, want := range []string{"ID", "Project", "Title", "Author", "Created", "Release", "Alice", "Demo"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestNewsList_JSON(t *testing.T) {
	srv, _ := newsServer(t, `{"news":[`+row("5")+`],"total_count":1}`)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdList(f)
	cmd.SetArgs([]string{"--project", "1", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(got) != 1 || got[0]["title"] != "Release" {
		t.Errorf("json = %v", got)
	}
}

func TestNewsList_AllProjectsWithoutDefault(t *testing.T) {
	srv, calls := newsServer(t, `{"news":[],"total_count":0}`)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdList(f)
	cmd.SetArgs([]string{"-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := lastCall(calls).uri; !strings.HasPrefix(got, "/news.json") {
		t.Errorf("uri = %s, want the global /news.json", got)
	}
	if got := testutil.Stdout(f); got != "[]\n" {
		t.Errorf("stdout = %q", got)
	}
}

func TestNewsList_DefaultProject(t *testing.T) {
	srv, calls := newsServer(t, `{"news":[],"total_count":0}`)
	f := testutil.NewFactoryWithConfig(t, srv.URL, "default_project: demo\n")
	cmd := newCmdList(f)
	cmd.SetArgs([]string{"-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := lastCall(calls).uri; !strings.HasPrefix(got, "/projects/demo/news.json") {
		t.Errorf("uri = %s, want the default project's news", got)
	}
}

func TestNewsList_PaginationWarning(t *testing.T) {
	srv, calls := newsServer(t, `{"news":[`+row("5")+`],"total_count":3,"offset":0,"limit":1}`)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdList(f)
	cmd.SetArgs([]string{"--project", "1", "--limit", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := lastCall(calls).uri; !strings.Contains(got, "limit=1") {
		t.Errorf("uri = %s", got)
	}
	if got := testutil.Stderr(f); !strings.Contains(got, "news") || !strings.Contains(got, "--offset") {
		t.Errorf("stderr = %q, want pagination warning", got)
	}
}

// --- get ---

func TestNewsGet_IncludesRendered(t *testing.T) {
	srv, calls := newsServer(t, "")
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGet(f)
	cmd.SetArgs([]string{"5", "--include", "comments,attachments"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := lastCall(calls).uri; got != "/news/5.json?include=comments%2Cattachments" {
		t.Errorf("uri = %s", got)
	}
	out := testutil.Stdout(f)
	for _, want := range []string{"Release", "Sum", "Body", "Bob", "Nice one", "notes.txt", "text/plain"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	for _, heading := range []string{"\nComments:\n", "\nAttachments:\n"} {
		if !strings.Contains(out, heading) {
			t.Errorf("output missing heading %q:\n%s", heading, out)
		}
	}
}

func TestNewsGet_JSON(t *testing.T) {
	srv, _ := newsServer(t, "")
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGet(f)
	cmd.SetArgs([]string{"5", "--include", "comments", "-o", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &got); err != nil {
		t.Fatal(err)
	}
	if c, _ := got["comments"].([]any); len(c) != 1 {
		t.Errorf("comments = %v", got["comments"])
	}
}

func TestNewsGet_UnknownIncludeRejected(t *testing.T) {
	srv, calls := newsServer(t, "")
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGet(f)
	cmd.SetArgs([]string{"5", "--include", "journals"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "journals") {
		t.Fatalf("err = %v, want unknown include error", err)
	}
	if len(*calls) != 0 {
		t.Errorf("unexpected requests: %v", *calls)
	}
}

func TestNewsGet_InvalidID(t *testing.T) {
	f := testutil.NewFactory(t, "http://unused")
	cmd := newCmdGet(f)
	cmd.SetArgs([]string{"abc"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "invalid news ID") {
		t.Fatalf("err = %v", err)
	}
}

// --- create ---

func TestNewsCreate_DescriptionFileAndEnvelopeWithID(t *testing.T) {
	srv, calls := newsServer(t, `{"news":[`+row("77")+`],"total_count":1}`)
	f := testutil.NewFactory(t, srv.URL)
	f.OutputFormat = "json"
	file := filepath.Join(t.TempDir(), "body.txt")
	if err := os.WriteFile(file, []byte("Body"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newCmdCreate(f)
	cmd.SetArgs([]string{"--project", "1", "--title", "Release", "--summary", "Sum", "--description-file", file})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	post, ok := findCall(calls, "POST")
	if !ok || post.uri != "/projects/demo/news.json" || post.body != `{"news":{"title":"Release","summary":"Sum","description":"Body"}}` {
		t.Errorf("post = %+v", post)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &env); err != nil {
		t.Fatalf("stdout %q: %v", testutil.Stdout(f), err)
	}
	if env["action"] != "created" || env["resource"] != "news" || env["id"] != float64(77) || env["ok"] != true {
		t.Errorf("envelope = %v", env)
	}
}

func TestNewsCreate_EnvelopeWithoutID(t *testing.T) {
	srv, _ := newsServer(t, `{"news":[],"total_count":0}`)
	f := testutil.NewFactory(t, srv.URL)
	f.OutputFormat = "json"
	cmd := newCmdCreate(f)
	cmd.SetArgs([]string{"--project", "1", "--title", "Release", "--description", "Body"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &env); err != nil {
		t.Fatal(err)
	}
	if _, has := env["id"]; has && env["id"] != nil {
		t.Errorf("id should be omitted, envelope = %v", env)
	}
	if env["ok"] != true || env["action"] != "created" {
		t.Errorf("envelope = %v", env)
	}
	if got := testutil.Stderr(f); !strings.Contains(got, "could not be determined") {
		t.Errorf("stderr = %q, want read-back warning", got)
	}
}

func TestNewsCreate_Attach(t *testing.T) {
	srv, calls := newsServer(t, `{"news":[],"total_count":0}`)
	f := testutil.NewFactory(t, srv.URL)
	file := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newCmdCreate(f)
	cmd.SetArgs([]string{"--project", "1", "--title", "T", "--description", "D", "--attach", file})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	post, _ := findCall(calls, "POST")
	if !strings.Contains(post.body, `"uploads":[{"token":"tok1","filename":"notes.txt"`) {
		t.Errorf("body = %s", post.body)
	}
}

func TestNewsCreate_RequiresTitleAndDescription(t *testing.T) {
	srv, calls := newsServer(t, "")
	for name, args := range map[string][]string{
		"no title":       {"--project", "1", "--description", "D"},
		"no description": {"--project", "1", "--title", "T"},
		"no project":     {"--title", "T", "--description", "D"},
	} {
		f := testutil.NewFactory(t, srv.URL)
		cmd := newCmdCreate(f)
		cmd.SetArgs(args)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SilenceUsage = true
		if err := cmd.Execute(); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	for _, c := range *calls {
		if c.method == "POST" {
			t.Errorf("POST sent despite invalid input: %+v", c)
		}
	}
}

// --- update ---

func TestNewsUpdate_ChangedFieldsOnly(t *testing.T) {
	srv, calls := newsServer(t, "")
	f := testutil.NewFactory(t, srv.URL)
	f.OutputFormat = "json"
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{"5", "--title", "New title"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	c := lastCall(calls)
	if c.method != "PUT" || c.uri != "/news/5.json" || c.body != `{"news":{"title":"New title"}}` {
		t.Errorf("call = %+v", c)
	}
	var env map[string]any
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &env); err != nil {
		t.Fatal(err)
	}
	if env["action"] != "updated" || env["resource"] != "news" || env["id"] != float64(5) {
		t.Errorf("envelope = %v", env)
	}
}

func TestNewsUpdate_ClearSummary(t *testing.T) {
	srv, calls := newsServer(t, "")
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{"5", "--summary", ""})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := lastCall(calls).body; got != `{"news":{"summary":""}}` {
		t.Errorf("body = %s", got)
	}
}

func TestNewsUpdate_NothingToChange(t *testing.T) {
	srv, calls := newsServer(t, "")
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{"5"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "nothing to change") {
		t.Fatalf("err = %v", err)
	}
	if len(*calls) != 0 {
		t.Errorf("unexpected requests: %v", *calls)
	}
}

// --- delete ---

func TestNewsDelete_Force(t *testing.T) {
	srv, calls := newsServer(t, "")
	f := testutil.NewFactory(t, srv.URL)
	f.OutputFormat = "json"
	cmd := newCmdDelete(f)
	cmd.SetArgs([]string{"5", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	c := lastCall(calls)
	if c.method != "DELETE" || c.uri != "/news/5.json" {
		t.Errorf("call = %+v", c)
	}
	if out := testutil.Stdout(f); !strings.Contains(out, `"action": "deleted"`) || !strings.Contains(out, `"ok": true`) {
		t.Errorf("stdout = %s", out)
	}
}

func TestNewsDelete_Cancelled(t *testing.T) {
	srv, calls := newsServer(t, "")
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdDelete(f)
	cmd.SetArgs([]string{"5"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 0 {
		t.Errorf("requests sent despite declined confirmation: %v", *calls)
	}
	got := testutil.Stderr(f)
	if !strings.Contains(got, "delete news 5") || !strings.Contains(got, "Delete cancelled") {
		t.Errorf("stderr = %q", got)
	}
}
