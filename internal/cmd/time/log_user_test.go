package time

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// TestTimeLog_UserNameResolvesViaIssueProject pins that `time log --issue N
// --user <name>` without --project matches the name against the issue's
// project members, so it works without admin rights (/users.json is 403).
func TestTimeLog_UserNameResolvesViaIssueProject(t *testing.T) {
	var capturedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/issues/42.json":
			_, _ = w.Write([]byte(`{"issue":{"id":42,"project":{"id":7,"name":"Demo"}}}`))
		case "/projects/7/memberships.json":
			_, _ = w.Write([]byte(`{"memberships":[{"id":1,"project":{"id":7,"name":"Demo"},"user":{"id":3,"name":"Alice Smith"},"roles":[]},{"id":2,"project":{"id":7,"name":"Demo"},"group":{"id":12,"name":"Alice Smith"},"roles":[]}],"total_count":2}`))
		case "/users.json":
			w.WriteHeader(http.StatusForbidden)
		case "/time_entries.json":
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &capturedBody)
			_, _ = w.Write([]byte(`{"time_entry":{"id":99,"hours":1,"spent_on":"2025-06-15","project":{"id":7,"name":"Demo"},"user":{"id":3,"name":"Alice Smith"},"activity":{"id":9,"name":"Development"}}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeLog(f)
	cmd.SetArgs([]string{"--hours", "1", "--issue", "42", "--date", "2025-06-15", "--user", "Alice Smith"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	te, _ := capturedBody["time_entry"].(map[string]any)
	if te == nil {
		t.Fatal("request body missing time_entry wrapper")
	}
	// The group of the same name is ignored: time entries belong to users.
	if te["user_id"] != float64(3) {
		t.Errorf("payload user_id = %v, want 3", te["user_id"])
	}
	if v, ok := te["project_id"]; ok {
		t.Errorf("project_id must stay absent when logging against an issue, got %v", v)
	}
}
