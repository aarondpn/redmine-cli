package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// jqTestServer serves a tiny fixed Redmine API and counts the requests it
// receives so tests can assert that an invalid --jq never reaches the server.
func jqTestServer(t *testing.T, issuesBody string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/issues.json":
			_, _ = w.Write([]byte(issuesBody))
		case r.Method == http.MethodGet && r.URL.Path == "/issues/1.json":
			_, _ = w.Write([]byte(`{"issue":{"id":1,"subject":"First","status":{"id":1,"name":"New"}}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/issues/1.json":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":["Not found"]}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

const twoIssues = `{"issues":[` +
	`{"id":1,"subject":"First","status":{"id":1,"name":"New"}},` +
	`{"id":2,"subject":"Second","status":{"id":5,"name":"Closed"}}` +
	`],"total_count":2,"offset":0,"limit":100}`

// runRootWithJQ executes the full root command against serverURL with a
// throwaway config and returns stdout, stderr, and the execution error.
func runRootWithJQ(t *testing.T, serverURL string, args ...string) (string, string, error) {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := "server: " + serverURL + "\napi_key: test\nauth_method: apikey\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	root, f := NewRootCmdWithFactory("test")
	var out, errOut bytes.Buffer
	f.IOStreams = &cmdutil.IOStreams{In: strings.NewReader(""), Out: &out, ErrOut: &errOut}
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(append([]string{"--config", cfgPath}, args...))
	err := root.Execute()
	return out.String(), errOut.String(), err
}

func TestJQ_AppliesToJSONOutputPaths(t *testing.T) {
	tests := []struct {
		name   string
		issues string
		args   []string
		want   string
	}{
		{"list", twoIssues, []string{"issues", "list", "--jq", ".[].id"}, "1\n2\n"},
		{"list short flag", twoIssues, []string{"issues", "list", "-q", ".[].status.name"}, "New\nClosed\n"},
		{"empty list", `{"issues":[],"total_count":0}`, []string{"issues", "list", "--jq", "."}, "[]\n"},
		{"resource body", twoIssues, []string{"issues", "get", "1", "--jq", ".status.name"}, "New\n"},
		{"action envelope", twoIssues, []string{"issues", "delete", "1", "--force", "--jq", "{ok, action, id}"}, `{"action":"deleted","id":1,"ok":true}` + "\n"},
		{"api response body", twoIssues, []string{"api", "/issues.json", "--jq", ".issues[1].subject"}, "Second\n"},
		{"explicit json output", twoIssues, []string{"issues", "list", "-o", "json", "--jq", "length"}, "2\n"},
		{"config", twoIssues, []string{"config", "--jq", ".auth_method"}, "apikey\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := jqTestServer(t, tt.issues)
			stdout, stderr, err := runRootWithJQ(t, srv.URL, tt.args...)
			if err != nil {
				t.Fatalf("Execute: %v\nstderr:\n%s", err, stderr)
			}
			if stdout != tt.want {
				t.Fatalf("stdout = %q, want %q", stdout, tt.want)
			}
		})
	}
}

func TestJQ_InvalidExpressionFailsBeforeAnyRequest(t *testing.T) {
	srv, hits := jqTestServer(t, twoIssues)
	stdout, _, err := runRootWithJQ(t, srv.URL, "issues", "list", "--jq", ".[")
	if err == nil {
		t.Fatal("expected error for invalid --jq expression")
	}
	if !strings.Contains(err.Error(), "invalid --jq expression") {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("expected no API requests, got %d", n)
	}
}

func TestJQ_RuntimeErrorFailsWithEmptyStdout(t *testing.T) {
	srv, _ := jqTestServer(t, twoIssues)
	stdout, _, err := runRootWithJQ(t, srv.URL, "issues", "get", "1", "--jq", ".[0]")
	if err == nil {
		t.Fatal("expected jq runtime error")
	}
	if !strings.HasPrefix(err.Error(), "--jq: ") {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestJQ_RejectsNonJSONOutput(t *testing.T) {
	srv, hits := jqTestServer(t, twoIssues)
	for _, args := range [][]string{
		{"issues", "list", "-o", "table", "--jq", ".[].id"},
		{"--output", "csv", "issues", "list", "--jq", ".[].id"},
		// api registers its own local --output, which shadows the root flag.
		{"api", "/issues.json", "-o", "table", "--jq", ".issues"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			_, _, err := runRootWithJQ(t, srv.URL, args...)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "--jq requires JSON output") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("expected no API requests, got %d", n)
	}
}

func TestJQ_ErrorsAreNotFiltered(t *testing.T) {
	srv, _ := jqTestServer(t, twoIssues)
	stdout, _, err := runRootWithJQ(t, srv.URL, "issues", "get", "999", "--jq", ".id")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if strings.HasPrefix(err.Error(), "--jq") {
		t.Fatalf("API error must surface unfiltered, got %v", err)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestRootCmd_InteractiveCommandsRejectJQFlag(t *testing.T) {
	for _, args := range [][]string{
		{"auth", "login", "--jq", "."},
		{"auth", "logout", "--jq", "."},
		{"auth", "switch", "--jq", "."},
		{"issues", "browse", "--jq", "."},
		{"search", "browse", "--jq", "."},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			root := NewRootCmd("test")
			root.SetArgs(args)

			err := root.Execute()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "--jq is not supported") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRootCmd_JQShorthandIsFreeOnEveryCommand(t *testing.T) {
	root := NewRootCmd("test")
	jq := root.PersistentFlags().Lookup("jq")
	if jq == nil || jq.Shorthand != "q" {
		t.Fatal("root command missing persistent --jq/-q flag")
	}
	walk(root, func(c *cobra.Command) {
		// InheritedFlags merges the root's persistent flags into Flags(); a
		// local -q on any subcommand would either panic here or shadow --jq.
		_ = c.InheritedFlags()
		if got := c.Flags().ShorthandLookup("q"); got != jq {
			t.Errorf("%s: -q does not resolve to the root --jq flag", c.CommandPath())
		}
	})
}
