package wiki

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// wikiParentServer fakes the endpoints a wiki create/update with --parent
// touches. Its PUT answers like Redmine whether or not parent_title was
// applied (201 with putResp, or 204 when putResp is empty); wiki GETs return
// pageBody (500 when empty).
type wikiParentServer struct {
	mu       sync.Mutex
	putResp  string
	pageBody string
	puts     []map[string]any
	wikiGets int
}

func (s *wikiParentServer) start(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/wiki/"):
			s.wikiGets++
			if s.pageBody == "" {
				http.Error(w, "boom", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(s.pageBody))
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"proj"}}`))
		case r.Method == http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			var body map[string]any
			_ = json.Unmarshal(b, &body)
			s.puts = append(s.puts, body)
			if s.putResp == "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(s.putResp))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (s *wikiParentServer) sentParent(t *testing.T) (any, bool) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.puts) != 1 {
		t.Fatalf("PUTs = %d, want 1", len(s.puts))
	}
	wp, _ := s.puts[0]["wiki_page"].(map[string]any)
	v, present := wp["parent_title"]
	return v, present
}

// quiet mirrors the root command's settings so a failing leaf command does
// not dump usage text into the test log.
func quiet(cmd *cobra.Command) *cobra.Command {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd
}

// requireParentNotApplied asserts the command failed with the coded error the
// standard error path renders as a parent_not_applied error envelope, and
// that the command itself printed nothing: main.go owns the output.
func requireParentNotApplied(t *testing.T, f *cmdutil.Factory, err error) {
	t.Helper()
	var coded *cmdutil.CodedError
	if !errors.As(err, &coded) {
		t.Fatalf("err = %v, want *cmdutil.CodedError", err)
	}
	if coded.Code != output.ErrCodeParentNotApplied {
		t.Errorf("code = %q, want %q", coded.Code, output.ErrCodeParentNotApplied)
	}
	if out := testutil.Stdout(f); out != "" {
		t.Errorf("stdout = %q, want empty (no success envelope on failure)", out)
	}
	if stderr := testutil.Stderr(f); stderr != "" {
		t.Errorf("stderr = %q, want empty (the error path prints the message once)", stderr)
	}
}

func TestUpdate_Parent_SendsAndVerifies(t *testing.T) {
	fake := &wikiParentServer{pageBody: `{"wiki_page":{"title":"MyPage","text":"existing","parent":{"title":"User_Guide"},"version":2}}`}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(newCmdUpdate(f))
	cmd.SetArgs([]string{"MyPage", "--project", "proj", "--text", "body", "--parent", "User Guide"})
	f.OutputFormat = output.FormatJSON

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got, _ := fake.sentParent(t); got != "User Guide" {
		t.Errorf("parent_title = %v, want User Guide", got)
	}
	if fake.wikiGets != 1 {
		t.Errorf("wiki GETs = %d, want 1 read-back", fake.wikiGets)
	}
	var env output.ActionEnvelope
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &env); err != nil || !env.Ok {
		t.Errorf("stdout = %q, want an ok action envelope", testutil.Stdout(f))
	}
}

// TestUpdate_EmptyParent_MovesToRoot checks that --parent "" is sent as an
// explicit empty parent_title rather than being dropped as unset.
func TestUpdate_EmptyParent_MovesToRoot(t *testing.T) {
	fake := &wikiParentServer{pageBody: `{"wiki_page":{"title":"MyPage","text":"existing","version":2}}`}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(newCmdUpdate(f))
	cmd.SetArgs([]string{"MyPage", "--project", "proj", "--parent", ""})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got, present := fake.sentParent(t)
	if !present || got != "" {
		t.Errorf("parent_title = %v (present %v), want an explicit empty string", got, present)
	}
}

func TestUpdate_WithoutParent_OmitsParent(t *testing.T) {
	fake := &wikiParentServer{}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(newCmdUpdate(f))
	cmd.SetArgs([]string{"MyPage", "--project", "proj", "--text", "body"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, present := fake.sentParent(t); present {
		t.Error("parent_title sent although --parent was not given")
	}
	if fake.wikiGets != 0 {
		t.Errorf("wiki GETs = %d, want 0 without --parent", fake.wikiGets)
	}
}

func TestUpdate_ParentNotApplied(t *testing.T) {
	fake := &wikiParentServer{pageBody: `{"wiki_page":{"title":"MyPage","text":"existing","version":2}}`}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(newCmdUpdate(f))
	cmd.SetArgs([]string{"MyPage", "--project", "proj", "--text", "body", "--parent", "Guide"})
	f.OutputFormat = output.FormatJSON

	err := cmd.Execute()
	requireParentNotApplied(t, f, err)

	env := cmdutil.BuildErrorEnvelope(err)
	if env.Error.Code != "parent_not_applied" {
		t.Errorf("envelope code = %q", env.Error.Code)
	}
	if !strings.Contains(env.Error.Message, `Wiki page "MyPage": parent was not changed to "Guide"; it is now none (wiki root).`) ||
		!strings.Contains(env.Error.Message, "rename_wiki_pages") {
		t.Errorf("envelope message = %q", env.Error.Message)
	}
	if len(env.Error.Details) != 1 || !strings.Contains(env.Error.Details[0], "may already have been applied") {
		t.Errorf("envelope details = %q, want the other-changes note", env.Error.Details)
	}
}

func TestUpdate_ParentReadBackFailureWarns(t *testing.T) {
	fake := &wikiParentServer{}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(newCmdUpdate(f))
	cmd.SetArgs([]string{"MyPage", "--project", "proj", "--text", "body", "--parent", "Guide"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v (a failed read-back must not fail an applied update)", err)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "could not verify") {
		t.Errorf("stderr = %q, want the verification warning", stderr)
	}
}

func TestCreate_Parent_SendsParentTitle(t *testing.T) {
	fake := &wikiParentServer{putResp: `{"wiki_page":{"title":"Child","parent":{"title":"Guide"},"version":1}}`}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(newCmdCreate(f))
	cmd.SetArgs([]string{"Child", "--project", "proj", "--text", "body", "--parent", "Guide"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got, _ := fake.sentParent(t); got != "Guide" {
		t.Errorf("parent_title = %v, want Guide", got)
	}
}

// TestCreate_OverwriteParentNotApplied covers create over an existing page,
// which Redmine handles as an update and may silently keep the old parent.
func TestCreate_OverwriteParentNotApplied(t *testing.T) {
	fake := &wikiParentServer{pageBody: `{"wiki_page":{"title":"Child","parent":{"title":"Old"},"version":2}}`}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(newCmdCreate(f))
	cmd.SetArgs([]string{"Child", "--project", "proj", "--text", "body", "--parent", "Guide"})
	f.OutputFormat = output.FormatJSON

	requireParentNotApplied(t, f, cmd.Execute())
}

func TestParentNotAppliedError_PassesOtherErrorsThrough(t *testing.T) {
	if parentNotAppliedError(errors.New("boom")) != nil {
		t.Error("parentNotAppliedError converted an unrelated error")
	}
	wrapped := fmt.Errorf("update: %w", &ops.WikiParentNotAppliedError{Page: "P", Requested: "A", Current: "B"})
	if parentNotAppliedError(wrapped) == nil {
		t.Error("parentNotAppliedError ignored a wrapped WikiParentNotAppliedError")
	}
}

func TestList_ShowsParentAndVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/wiki/index.json") {
			_, _ = w.Write([]byte(`{"wiki_pages":[` +
				`{"title":"Guide","version":3,"updated_on":"2026-01-02T00:00:00Z"},` +
				`{"title":"Install","parent":{"title":"Guide"},"version":1,"updated_on":"2026-01-04T00:00:00Z"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"proj"}}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(newCmdList(f))
	cmd.SetArgs([]string{"--project", "proj", "--output", "table"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"Parent", "Version"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("table header missing %q\n%s", want, stdout)
		}
	}
	var installLine string
	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, "Install") {
			installLine = line
		}
	}
	if !strings.Contains(installLine, "Guide") {
		t.Errorf("Install row does not show its parent: %q\n%s", installLine, stdout)
	}

	f = testutil.NewFactory(t, srv.URL)
	cmd = quiet(newCmdList(f))
	cmd.SetArgs([]string{"--project", "proj", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var pages []models.WikiPageIndex
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &pages); err != nil {
		t.Fatalf("decode json output: %v\n%s", err, testutil.Stdout(f))
	}
	if len(pages) != 2 || pages[1].Parent == nil || pages[1].Parent.Title != "Guide" || pages[0].Version != 3 {
		t.Errorf("json pages = %+v, want parent and version carried through", pages)
	}
}

// TestUpdate_ParentOnly_NoOtherChangesNote: a bare --parent resends the
// current text unchanged, so a dropped parent must not claim that other
// changes may have been applied.
func TestUpdate_ParentOnly_NoOtherChangesNote(t *testing.T) {
	fake := &wikiParentServer{pageBody: `{"wiki_page":{"title":"MyPage","text":"existing","version":2}}`}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(newCmdUpdate(f))
	cmd.SetArgs([]string{"MyPage", "--project", "proj", "--parent", "Guide"})
	f.OutputFormat = output.FormatJSON

	err := cmd.Execute()
	requireParentNotApplied(t, f, err)
	if env := cmdutil.BuildErrorEnvelope(err); len(env.Error.Details) != 0 {
		t.Errorf("envelope details = %q, want none for a parent-only update", env.Error.Details)
	}
	fake.mu.Lock()
	wp, _ := fake.puts[0]["wiki_page"].(map[string]any)
	fake.mu.Unlock()
	if wp["text"] != "existing" {
		t.Errorf("sent text = %v, want the current text resent", wp["text"])
	}
}
