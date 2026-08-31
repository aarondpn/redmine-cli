package issue

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func newCreateIssueServer(t *testing.T, posted *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/17.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"project":{"id":17,"identifier":"demo","name":"Demo"}}`))
		case "/issues.json":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, posted)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"issue":{"id":1,"subject":"test","project":{"id":17,"name":"Demo"},"tracker":{"id":1,"name":"Bug"},"status":{"id":1,"name":"New"},"priority":{"id":2,"name":"Normal"}}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
}

// TestCmdIssueCreate_CustomFieldValuesWithSpaces pins that repeated
// --custom-field flags carry values containing spaces through to the API
// payload unchanged (issue #155).
func TestCmdIssueCreate_CustomFieldValuesWithSpaces(t *testing.T) {
	var posted map[string]any
	srv := newCreateIssueServer(t, &posted)
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdCreate(f)
	cmd.SetArgs([]string{
		"--project", "17", "--subject", "test",
		"--custom-field", "2=Value for field 2",
		"--custom-field", "61=Value for 61",
		"--output", "json",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	issue, ok := posted["issue"].(map[string]any)
	if !ok {
		t.Fatalf("posted body missing issue wrapper: %v", posted)
	}
	cfvs, ok := issue["custom_field_values"].(map[string]any)
	if !ok {
		t.Fatalf("posted body missing custom_field_values: %v", issue)
	}
	if cfvs["2"] != "Value for field 2" {
		t.Errorf("custom_field_values[2] = %q, want %q", cfvs["2"], "Value for field 2")
	}
	if cfvs["61"] != "Value for 61" {
		t.Errorf("custom_field_values[61] = %q, want %q", cfvs["61"], "Value for 61")
	}
}

// TestCmdIssueCreate_RejectsStrayPositionalArgs pins that a key=value pair
// not attached to a --custom-field flag fails loudly instead of being
// silently dropped. This was the actual failure mode behind issue #155:
// `--custom-field 2="a b" 61="c d"` created the issue with only field 2 set.
func TestCmdIssueCreate_RejectsStrayPositionalArgs(t *testing.T) {
	var posted map[string]any
	srv := newCreateIssueServer(t, &posted)
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdCreate(f)
	cmd.SetArgs([]string{
		"--project", "17", "--subject", "test",
		"--custom-field", "2=Value for field 2",
		"61=Value for 61",
		"--output", "json",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for stray positional arg, got nil (posted: %v)", posted)
	}
	if !strings.Contains(err.Error(), "unknown command") && !strings.Contains(err.Error(), "arg") {
		t.Errorf("error should mention the rejected argument, got: %v", err)
	}
	if posted != nil {
		t.Errorf("no issue should have been created, but API received: %v", posted)
	}
}
