package cmd

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
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
		case r.Method == http.MethodGet && r.URL.Path == "/big.json":
			_, _ = w.Write([]byte(`{"id":9007199254740993}`))
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

// clearRedmineEnv unsets every REDMINE_* variable for the duration of the
// test so a developer's environment can never point a test at a real server.
func clearRedmineEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(key, "REDMINE_") {
			t.Setenv(key, "") // restores the original value on cleanup
			_ = os.Unsetenv(key)
		}
	}
}

// runRootWithJQ executes the full root command against serverURL with a
// throwaway config and returns stdout, stderr, and the execution error.
func runRootWithJQ(t *testing.T, serverURL string, args ...string) (string, string, error) {
	t.Helper()
	return runRootWithConfig(t, "server: "+serverURL+"\napi_key: test\nauth_method: apikey\n", args...)
}

// runRootWithConfig is runRootWithJQ with the given config file contents.
func runRootWithConfig(t *testing.T, cfg string, args ...string) (string, string, error) {
	t.Helper()
	clearRedmineEnv(t)
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
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
		{"api keeps large integers", twoIssues, []string{"api", "/big.json", "--jq", ".id"}, "9007199254740993\n"},
		{"api -o json keeps large integers", twoIssues, []string{"api", "/big.json", "-o", "json"}, "{\n  \"id\": 9007199254740993\n}\n"},
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

func TestJQ_EmptyExpressionIsRejected(t *testing.T) {
	srv, hits := jqTestServer(t, twoIssues)
	stdout, _, err := runRootWithJQ(t, srv.URL, "issues", "list", "--jq", "")
	var compileErr *output.JQCompileError
	if !errors.As(err, &compileErr) {
		t.Fatalf("expected *output.JQCompileError, got %v", err)
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
	var jqErr *output.JQRuntimeError
	if !errors.As(err, &jqErr) {
		t.Fatalf("expected *output.JQRuntimeError, got %T: %v", err, err)
	}
	if !strings.HasPrefix(err.Error(), "command succeeded, but --jq failed: ") {
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

// TestRemovedCSVOutput pins that an explicit --output csv, in any letter
// case, fails before any request, wherever the flag is given, and that the jq
// expression its error suggests produces CSV from the real issues list JSON.
func TestRemovedCSVOutput(t *testing.T) {
	const issues = `{"issues":[` +
		`{"id":1,"subject":"Crash, \"save\""},` +
		`{"id":2,"subject":"=SUM(A1)"}` +
		`],"total_count":2,"offset":0,"limit":100}`
	srv, hits := jqTestServer(t, issues)

	var errMsg string
	for _, args := range [][]string{
		{"-o", "csv", "issues", "list"},
		{"-o", "CSV", "issues", "list"},
		{"issues", "list", "--output", "csv"},
		{"issues", "list", "--output", "Csv"},
		{"--output", "csv", "issues", "list", "--jq", ".[].id"},
		// api registers its own local --output, which shadows the root flag.
		{"api", "/issues.json", "-o", "csv"},
		{"api", "/issues.json", "-o", "CSV"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			_, _, err := runRootWithJQ(t, srv.URL, args...)
			if err == nil || !strings.HasPrefix(err.Error(), "CSV output was removed; use -o json with --jq and jq's @csv") {
				t.Fatalf("err = %v, want the CSV removal error", err)
			}
			errMsg = err.Error()
		})
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("expected no API requests, got %d", n)
	}

	_, expr, ok := strings.Cut(errMsg, "--jq '")
	expr, _, ok2 := strings.Cut(expr, "'")
	if !ok || !ok2 {
		t.Fatalf("error %q does not contain a --jq example", errMsg)
	}
	stdout, _, err := runRootWithJQ(t, srv.URL, "issues", "list", "-o", "json", "--jq", expr)
	if err != nil {
		t.Fatalf("suggested --jq %q: %v", expr, err)
	}
	if want := "1,\"Crash, \"\"save\"\"\"\n2,\"=SUM(A1)\"\n"; stdout != want {
		t.Fatalf("suggested --jq %q printed %q, want %q", expr, stdout, want)
	}
}

// TestRemovedCSVOutputFormatConfig pins that output_format: csv, in any
// letter case, falls back to the table with one warning, and that the
// warning only appears when the fallback is used: -o/--output and --jq pick
// the format themselves.
func TestRemovedCSVOutputFormatConfig(t *testing.T) {
	srv, _ := jqTestServer(t, twoIssues)
	const warning = "warning: CSV output was removed, so output_format csv (profile or REDMINE_OUTPUT_FORMAT) falls back to table"

	tests := []struct {
		name     string
		format   string
		args     []string
		wantWarn bool
		wantOut  string
	}{
		{name: "fallback", format: "csv", args: []string{"issues", "list"}, wantWarn: true, wantOut: "First"},
		{name: "fallback upper case", format: "CSV", args: []string{"issues", "list"}, wantWarn: true, wantOut: "First"},
		{name: "-o json", format: "csv", args: []string{"issues", "list", "-o", "json"}, wantOut: `"subject": "First"`},
		{name: "--output table", format: "csv", args: []string{"issues", "list", "--output", "table"}, wantOut: "First"},
		{name: "--jq", format: "csv", args: []string{"issues", "list", "--jq", ".[0].id"}, wantOut: "1\n"},
		{name: "api -o json", format: "csv", args: []string{"api", "/issues/1.json", "-o", "json"}, wantOut: `"First"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := "server: " + srv.URL + "\napi_key: test\nauth_method: apikey\noutput_format: " + tt.format + "\n"
			stdout, stderr, err := runRootWithConfig(t, cfg, tt.args...)
			if err != nil {
				t.Fatalf("err = %v (stderr %q)", err, stderr)
			}
			if !strings.Contains(stdout, tt.wantOut) {
				t.Fatalf("stdout = %q, want it to contain %q", stdout, tt.wantOut)
			}
			if tt.wantWarn {
				if strings.Count(stderr, warning) != 1 || !strings.Contains(stderr, "@csv") {
					t.Fatalf("stderr = %q, want one CSV fallback warning with the @csv hint", stderr)
				}
				if strings.Contains(stdout, "1,First") {
					t.Fatalf("stdout is CSV, want a table:\n%s", stdout)
				}
			} else if stderr != "" {
				t.Fatalf("stderr = %q, want no warning", stderr)
			}
		})
	}
}

func TestJQ_ErrorsAreNotFiltered(t *testing.T) {
	srv, _ := jqTestServer(t, twoIssues)
	stdout, _, err := runRootWithJQ(t, srv.URL, "issues", "get", "999", "--jq", ".id")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if strings.Contains(err.Error(), "--jq") {
		t.Fatalf("API error must surface unfiltered, got %v", err)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestJQ_RejectedForCommandsWithoutJSONOutput(t *testing.T) {
	srv, hits := jqTestServer(t, twoIssues)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"completion", "bash", "--jq", "."}, "--jq is not supported for redmine completion"},
		{[]string{"mcp", "serve", "--jq", "."}, "--jq is not supported for redmine mcp serve"},
		{[]string{"mcp", "tools", "--jq", "."}, "--jq is not supported for redmine mcp tools"},
		{[]string{"attachments", "download", "1", "--path", "-", "--jq", "."}, "--jq is not supported with --path -"},
	} {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			stdout, _, err := runRootWithJQ(t, srv.URL, tc.args...)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("expected no API requests, got %d", n)
	}
}

func TestRootCmd_InteractiveCommandsRejectJQFlag(t *testing.T) {
	clearRedmineEnv(t)
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
