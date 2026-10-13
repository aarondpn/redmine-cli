package issue

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// newHintServer answers project lookups for "demo" and returns the given
// status/body for the issue endpoints and uploads.
func newHintServer(t *testing.T, issueStatus, uploadStatus int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/projects/demo.json":
			_, _ = w.Write([]byte(`{"project":{"id":17,"identifier":"demo","name":"Demo Project"}}`))
		case r.URL.Path == "/projects/Demo Project.json":
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == "/projects.json":
			_, _ = w.Write([]byte(`{"projects":[{"id":17,"identifier":"demo","name":"Demo Project"}],"total_count":1}`))
		case r.URL.Path == "/uploads.json":
			w.WriteHeader(uploadStatus)
			_, _ = w.Write([]byte(body))
		case r.URL.Path == "/issues.json" || strings.HasPrefix(r.URL.Path, "/issues/"):
			w.WriteHeader(issueStatus)
			_, _ = w.Write([]byte(body))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
}

func runIssueCmd(t *testing.T, f *cmdutil.Factory, args ...string) error {
	t.Helper()
	cmd := NewCmdIssue(f)
	cmd.SetArgs(args)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	return cmd.Execute()
}

const validationBody = `{"errors":["Category cannot be blank"]}`

func TestIssueCreate_ValidationErrorHint(t *testing.T) {
	cases := []struct {
		name   string
		config string
		args   []string
	}{
		{name: "explicit project", args: []string{"--project", "demo"}},
		{name: "default project", config: "default_project: demo\n"},
		{name: "project name resolves to identifier", args: []string{"--project", "Demo Project"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newHintServer(t, http.StatusUnprocessableEntity, http.StatusCreated, validationBody)
			defer srv.Close()

			f := testutil.NewFactoryWithConfig(t, srv.URL, tc.config)
			args := append([]string{"create", "--subject", "x"}, tc.args...)
			err := runIssueCmd(t, f, args...)
			if err == nil {
				t.Fatal("expected error")
			}

			wantCmd := "redmine projects get demo --include trackers,issue_categories,issue_custom_fields"
			text := cmdutil.FormatError(err)
			if !strings.Contains(text, "Category cannot be blank") {
				t.Errorf("text error lost the server message: %q", text)
			}
			if !strings.Contains(text, "\nHint: ") || !strings.Contains(text, wantCmd) {
				t.Errorf("text error missing hint with %q: %q", wantCmd, text)
			}

			env := cmdutil.BuildErrorEnvelope(err)
			if env.Error.Code != "validation_failed" {
				t.Errorf("code = %q", env.Error.Code)
			}
			if !strings.Contains(env.Error.Hint, wantCmd) ||
				!strings.Contains(env.Error.Hint, "redmine versions list --project demo --open") ||
				!strings.Contains(env.Error.Hint, "redmine memberships list --project demo") {
				t.Errorf("JSON hint = %q", env.Error.Hint)
			}
			if strings.Contains(env.Error.Message, "Hint:") {
				t.Errorf("JSON message should not embed the hint: %q", env.Error.Message)
			}
		})
	}
}

func TestIssueCreate_UploadValidationHasNoIssueHint(t *testing.T) {
	srv := newHintServer(t, http.StatusCreated, http.StatusUnprocessableEntity, `{"errors":["File is too big"]}`)
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "big.bin")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	f := testutil.NewFactory(t, srv.URL)
	err := runIssueCmd(t, f, "create", "--project", "demo", "--subject", "x", "--attach", path)
	if err == nil {
		t.Fatal("expected error")
	}
	if hint := cmdutil.ErrorHint(err); hint != "" {
		t.Errorf("upload failure should not get the issue-field hint, got %q", hint)
	}
}

func TestIssueUpdate_ValidationErrorHint(t *testing.T) {
	srv := newHintServer(t, http.StatusUnprocessableEntity, http.StatusCreated, validationBody)
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	err := runIssueCmd(t, f, "update", "123", "--subject", "x")
	if err == nil {
		t.Fatal("expected error")
	}
	hint := cmdutil.BuildErrorEnvelope(err).Error.Hint
	for _, want := range []string{"redmine issues get 123 -o json", "project.id", "redmine projects get PROJECT --include trackers,issue_categories,issue_custom_fields"} {
		if !strings.Contains(hint, want) {
			t.Errorf("hint %q missing %q", hint, want)
		}
	}
}

func TestIssueGet_NotFoundHint(t *testing.T) {
	srv := newHintServer(t, http.StatusNotFound, http.StatusCreated, ``)
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	err := runIssueCmd(t, f, "get", "999", "--journals")
	if err == nil {
		t.Fatal("expected error")
	}
	env := cmdutil.BuildErrorEnvelope(err)
	if env.Error.Code != "not_found" {
		t.Errorf("code = %q", env.Error.Code)
	}
	for _, want := range []string{"Issue #999 does not exist on this server", "--scope all"} {
		if !strings.Contains(env.Error.Hint, want) {
			t.Errorf("hint %q missing %q", env.Error.Hint, want)
		}
	}
	if !strings.Contains(cmdutil.FormatError(err), "\nHint: Issue #999") {
		t.Errorf("text error missing hint: %q", cmdutil.FormatError(err))
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("demo"); got != "demo" {
		t.Errorf("shellQuote(demo) = %q", got)
	}
	if got := shellQuote("My Project"); got != `'My Project'` {
		t.Errorf("shellQuote(My Project) = %q", got)
	}
	if got := shellQuote("it's"); got != `'it'\''s'` {
		t.Errorf("shellQuote(it's) = %q", got)
	}
}

func TestIssue_ForbiddenHints(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "get", args: []string{"get", "42"}, want: "Issue #42 exists but is not visible to the current user"},
		{name: "update", args: []string{"update", "42", "--subject", "x"}, want: "Issue #42 is either not visible to the current user"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newHintServer(t, http.StatusForbidden, http.StatusCreated, ``)
			defer srv.Close()

			f := testutil.NewFactory(t, srv.URL)
			err := runIssueCmd(t, f, tc.args...)
			if err == nil {
				t.Fatal("expected error")
			}
			env := cmdutil.BuildErrorEnvelope(err)
			if env.Error.Code != "forbidden" {
				t.Errorf("code = %q", env.Error.Code)
			}
			if !strings.Contains(env.Error.Hint, tc.want) || !strings.Contains(env.Error.Hint, "Don't retry with other credentials") {
				t.Errorf("hint = %q, want %q", env.Error.Hint, tc.want)
			}
			if strings.Count(cmdutil.FormatError(err), "Hint:") != 1 {
				t.Errorf("text error should carry exactly one hint: %q", cmdutil.FormatError(err))
			}
		})
	}
}
