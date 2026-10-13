package issue

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// issueStatusServer fakes the Redmine endpoints an issue update touches. Its
// PUT always answers 204, mirroring Redmine's behavior when the workflow
// silently drops status_id; the read-back GET returns issueBody.
type issueStatusServer struct {
	mu        sync.Mutex
	issueBody string
	puts      []map[string]any
	gets      int
}

func (s *issueStatusServer) start(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch {
		case r.URL.Path == "/issue_statuses.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"issue_statuses":[{"id":1,"name":"New","is_closed":false},{"id":5,"name":"Closed","is_closed":true},{"id":6,"name":"Wontfix","is_closed":true}]}`))
		case r.URL.Path == "/issues/7.json" && r.Method == http.MethodPut:
			raw, _ := io.ReadAll(r.Body)
			var body map[string]any
			_ = json.Unmarshal(raw, &body)
			s.puts = append(s.puts, body)
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/issues/7.json" && r.Method == http.MethodGet:
			s.gets++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(s.issueBody))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (s *issueStatusServer) lastIssue(t *testing.T) map[string]any {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.puts) == 0 {
		t.Fatal("no PUT received")
	}
	issue, _ := s.puts[len(s.puts)-1]["issue"].(map[string]any)
	return issue
}

const (
	issue7StillNew = `{"issue":{"id":7,"status":{"id":1,"name":"New"},"allowed_statuses":[{"id":1,"name":"New","is_closed":false},{"id":5,"name":"Closed","is_closed":true}]}}`
	issue7Wontfix  = `{"issue":{"id":7,"status":{"id":6,"name":"Wontfix"}}}`
	issue7Closed   = `{"issue":{"id":7,"status":{"id":5,"name":"Closed"}}}`
)

// quiet mirrors the root command's settings so a failing leaf command does
// not dump usage text into the test log.
func quiet(cmd *cobra.Command) *cobra.Command {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd
}

// requireStatusNotApplied asserts the command failed with the coded error the
// standard error path renders as a status_not_applied error envelope, and
// that nothing was printed by the command itself: main.go owns the output.
func requireStatusNotApplied(t *testing.T, f *cmdutil.Factory, err error) *cmdutil.CodedError {
	t.Helper()
	var coded *cmdutil.CodedError
	if !errors.As(err, &coded) {
		t.Fatalf("err = %v, want *cmdutil.CodedError", err)
	}
	if coded.Code != output.ErrCodeStatusNotApplied {
		t.Errorf("code = %q, want %q", coded.Code, output.ErrCodeStatusNotApplied)
	}
	if out := testutil.Stdout(f); out != "" {
		t.Errorf("stdout = %q, want empty (no success envelope on failure)", out)
	}
	if stderr := testutil.Stderr(f); stderr != "" {
		t.Errorf("stderr = %q, want empty (the error path prints the message once)", stderr)
	}
	return coded
}

func decodeEnvelope(t *testing.T, stdout string) output.ActionEnvelope {
	t.Helper()
	var env output.ActionEnvelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout is not an action envelope: %v\n%s", err, stdout)
	}
	return env
}

func TestCmdIssueUpdate_StatusNotApplied(t *testing.T) {
	fake := &issueStatusServer{issueBody: issue7StillNew}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(NewCmdUpdate(f))
	cmd.SetArgs([]string{"7", "--status", "6", "--note", "closing as wontfix"})
	f.OutputFormat = output.FormatJSON

	err := cmd.Execute()
	requireStatusNotApplied(t, f, err)

	env := cmdutil.BuildErrorEnvelope(err)
	if env.Error.Code != "status_not_applied" {
		t.Errorf("envelope code = %q", env.Error.Code)
	}
	if !strings.Contains(env.Error.Message, `Issue #7: status was not changed to "Wontfix" (ID 6); it is now "New" (ID 1)`) ||
		!strings.Contains(env.Error.Message, "workflow") {
		t.Errorf("envelope message = %q", env.Error.Message)
	}
	wantDetails := []string{
		"Allowed statuses: New (1), Closed (5).",
		"The other changes in this request (fields, notes, attachments) may already have been applied; do not resend them blindly.",
	}
	if strings.Join(env.Error.Details, "|") != strings.Join(wantDetails, "|") {
		t.Errorf("envelope details = %q, want %q", env.Error.Details, wantDetails)
	}

	// Text mode prints FormatError(err) once: the message plus the details.
	text := cmdutil.FormatError(err)
	if !strings.HasPrefix(text, "Issue #7: status was not changed") || !strings.Contains(text, "\n  - Allowed statuses: New (1), Closed (5).") {
		t.Errorf("text error = %q", text)
	}
}

func TestCmdIssueUpdate_StatusApplied(t *testing.T) {
	fake := &issueStatusServer{issueBody: issue7Wontfix}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(NewCmdUpdate(f))
	cmd.SetArgs([]string{"7", "--status", "6"})
	f.OutputFormat = output.FormatJSON

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	env := decodeEnvelope(t, testutil.Stdout(f))
	if !env.Ok || env.Message != "Updated issue #7" {
		t.Errorf("unexpected envelope: %+v", env)
	}
	if fake.gets != 1 {
		t.Errorf("read-back GETs = %d, want 1", fake.gets)
	}
	if stderr := testutil.Stderr(f); stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestCmdIssueUpdate_NoStatusNoReadBack(t *testing.T) {
	fake := &issueStatusServer{issueBody: issue7StillNew}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(NewCmdUpdate(f))
	cmd.SetArgs([]string{"7", "--subject", "renamed"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if fake.gets != 0 {
		t.Errorf("GETs = %d, want 0 without --status", fake.gets)
	}
}

func TestCmdIssueClose_StatusNotApplied(t *testing.T) {
	fake := &issueStatusServer{issueBody: issue7StillNew}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(NewCmdClose(f))
	cmd.SetArgs([]string{"7"})

	coded := requireStatusNotApplied(t, f, cmd.Execute())
	if !strings.Contains(coded.Message, "open subtasks") {
		t.Errorf("message should mention the subtask/blocker case: %q", coded.Message)
	}
	// A status-only request has no other changes to warn about.
	if len(coded.Details) != 1 {
		t.Errorf("details = %q, want only the allowed statuses", coded.Details)
	}
}

func TestCmdIssueClose_StatusApplied(t *testing.T) {
	fake := &issueStatusServer{issueBody: issue7Closed}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(NewCmdClose(f))
	cmd.SetArgs([]string{"7"})
	f.OutputFormat = output.FormatJSON

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if env := decodeEnvelope(t, testutil.Stdout(f)); !env.Ok || env.Action != output.ActionClosed {
		t.Errorf("unexpected envelope: %+v", env)
	}
}

// TestCmdIssueClose_AlreadyClosedOtherStatus pins that closing an issue that
// is already in another closed status succeeds, with a note on stderr.
func TestCmdIssueClose_AlreadyClosedOtherStatus(t *testing.T) {
	fake := &issueStatusServer{issueBody: issue7Wontfix}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(NewCmdClose(f))
	cmd.SetArgs([]string{"7"})
	f.OutputFormat = output.FormatJSON

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v (an issue that is already closed must not fail)", err)
	}
	if env := decodeEnvelope(t, testutil.Stdout(f)); !env.Ok || env.Action != output.ActionClosed {
		t.Errorf("unexpected envelope: %+v", env)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, `Issue #7 is already closed with status "Wontfix"`) {
		t.Errorf("stderr missing already-closed note:\n%s", stderr)
	}
}

func TestCmdIssueReopen_StatusNotApplied(t *testing.T) {
	fake := &issueStatusServer{issueBody: issue7Closed}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(NewCmdReopen(f))
	cmd.SetArgs([]string{"7"})

	requireStatusNotApplied(t, f, cmd.Execute())
}

// TestCmdIssueCreate_StatusIgnoredWarns pins that a create whose requested
// status Redmine replaced with the default only warns: the issue exists, so
// a non-zero exit would invite a duplicate-creating retry.
func TestCmdIssueCreate_StatusIgnoredWarns(t *testing.T) {
	var posted map[string]any
	srv := newCreateIssueServer(t, &posted)
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(NewCmdCreate(f))
	// The fake server always answers with status 1 ("New").
	cmd.SetArgs([]string{"--project", "17", "--subject", "test", "--status", "3", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, `was created with status "New" instead of the requested status ID 3:`) {
		t.Errorf("stderr missing status warning:\n%s", stderr)
	}
}
