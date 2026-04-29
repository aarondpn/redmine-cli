package time

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	stdtime "time"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

const timeEntryJSON = `{
	"id": 42,
	"project": {"id": 1, "name": "Demo"},
	"issue": {"id": 10},
	"user": {"id": 2, "name": "Alice"},
	"activity": {"id": 9, "name": "Development"},
	"hours": 2.5,
	"comments": "Fixed bug",
	"spent_on": "2025-06-15",
	"created_on": "2025-06-15T10:00:00Z",
	"updated_on": "2025-06-15T10:00:00Z"
}`

func timeListHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"time_entries":[` + timeEntryJSON + `],"total_count":5}`))
}

func emptyTimeListHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"time_entries":[],"total_count":0}`))
}

// --- list ---

func TestTimeList_EmptyJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(emptyTimeListHandler))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeList(f)
	cmd.SetArgs([]string{"--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Stdout(f); got != "[]\n" {
		t.Fatalf("stdout = %q, want %q", got, "[]\n")
	}
	if got := testutil.Stderr(f); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}

func TestTimeList_EmptyTableWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(emptyTimeListHandler))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeList(f)
	cmd.SetArgs([]string{"--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "No time entries found") {
		t.Fatalf("stderr = %q, want warning about no time entries", stderr)
	}
}

func TestTimeList_WithData_Table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"time_entries":[` + timeEntryJSON + `],"total_count":1}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeList(f)
	cmd.SetArgs([]string{"--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"Demo", "2.50", "Alice", "2025-06-15"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestTimeList_PaginationWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(timeListHandler))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeList(f)
	cmd.SetArgs([]string{"--output", "table", "--limit", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "Showing 1 of 5 entries") {
		t.Fatalf("stderr = %q, want pagination warning", stderr)
	}
}

func TestTimeList_NoPaginationWarningJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(timeListHandler))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeList(f)
	cmd.SetArgs([]string{"--output", "json", "--limit", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Stderr(f); got != "" {
		t.Fatalf("stderr = %q, want empty (no warning for JSON)", got)
	}
}

func TestTimeList_CSV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"time_entries":[` + timeEntryJSON + `],"total_count":1}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeList(f)
	cmd.SetArgs([]string{"--output", "csv"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	// CSV should contain headers and data
	for _, want := range []string{"ID", "Hours", "42", "2.50"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("csv output missing %q:\n%s", want, stdout)
		}
	}
}

func TestTimeList_EmptyCSVPrintsHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(emptyTimeListHandler))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeList(f)
	cmd.SetArgs([]string{"--output", "csv"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if got := testutil.Stderr(f); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}

	stdout := testutil.Stdout(f)
	if strings.Contains(stdout, "\x1b[") {
		t.Fatalf("csv output contains ANSI escapes:\n%q", stdout)
	}
	if got := stdout; got != "ID,Date,Project,Issue,Hours,Activity,User,Comments\n" {
		t.Fatalf("stdout = %q, want csv headers only", got)
	}
}

// --- get ---

func TestTimeGet_Detail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/time_entries/42.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"time_entry":` + timeEntryJSON + `}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeGet(f)
	cmd.SetArgs([]string{"42"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"Demo", "Alice", "2.50", "#10"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("detail output missing %q:\n%s", want, stdout)
		}
	}
}

func TestTimeGet_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"time_entry":` + timeEntryJSON + `}`))
	}))
	defer srv.Close()

	f := testutil.NewFactoryWithConfig(t, srv.URL, "output_format: json\n")
	cmd := newCmdTimeGet(f)
	cmd.SetArgs([]string{"42"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &entry); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, stdout)
	}
	if entry["id"].(float64) != 42 {
		t.Errorf("expected id=42, got %v", entry["id"])
	}
}

func TestTimeGet_InvalidID(t *testing.T) {
	f := testutil.NewFactory(t, "http://unused")
	cmd := newCmdTimeGet(f)
	cmd.SetArgs([]string{"abc"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-numeric ID")
	}
	if !strings.Contains(err.Error(), "invalid time entry ID") {
		t.Errorf("error = %q, want 'invalid time entry ID'", err.Error())
	}
}

// --- log ---

func TestTimeLog_Success(t *testing.T) {
	var capturedMethod string
	var capturedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"time_entry":{"id":99,"hours":2.5,"spent_on":"2025-06-15","project":{"id":1,"name":"Demo"},"user":{"id":2,"name":"Alice"},"activity":{"id":9,"name":"Development"}}}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeLog(f)
	cmd.SetArgs([]string{"--hours", "2.5", "--issue", "10", "--date", "2025-06-15", "--comment", "Fixed bug"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if capturedMethod != "POST" {
		t.Errorf("expected POST, got %s", capturedMethod)
	}

	// Verify the request payload contains the CLI flag values.
	te, _ := capturedBody["time_entry"].(map[string]interface{})
	if te == nil {
		t.Fatal("request body missing time_entry wrapper")
	}
	if te["hours"] != 2.5 {
		t.Errorf("payload hours = %v, want 2.5", te["hours"])
	}
	if te["issue_id"] != float64(10) {
		t.Errorf("payload issue_id = %v, want 10", te["issue_id"])
	}
	if te["spent_on"] != "2025-06-15" {
		t.Errorf("payload spent_on = %v, want 2025-06-15", te["spent_on"])
	}
	if te["comments"] != "Fixed bug" {
		t.Errorf("payload comments = %v, want 'Fixed bug'", te["comments"])
	}

	stderr := testutil.Stderr(f)
	if !strings.Contains(stderr, "Time entry #99 created") {
		t.Errorf("stderr = %q, want success message", stderr)
	}
}

func TestTimeLog_JSONReturnsCreatedEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"time_entry":{"id":99,"hours":2.5,"spent_on":"2025-06-15","comments":"Fixed bug","project":{"id":1,"name":"Demo"},"user":{"id":2,"name":"Alice"},"activity":{"id":9,"name":"Development"}}}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeLog(f)
	cmd.SetArgs([]string{"--hours", "2.5", "--issue", "10", "--date", "2025-06-15", "--comment", "Fixed bug", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var entry map[string]any
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &entry); err != nil {
		t.Fatalf("expected JSON output, got: %v", err)
	}
	if entry["id"] != float64(99) {
		t.Fatalf("id = %v, want 99", entry["id"])
	}
	if entry["hours"] != 2.5 {
		t.Fatalf("hours = %v, want 2.5", entry["hours"])
	}
	if entry["comments"] != "Fixed bug" {
		t.Fatalf("comments = %v, want Fixed bug", entry["comments"])
	}
	if got := testutil.Stderr(f); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}

func TestTimeLog_MissingHours(t *testing.T) {
	f := testutil.NewFactory(t, "http://unused")
	cmd := newCmdTimeLog(f)
	cmd.SetArgs([]string{"--issue", "10"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --hours")
	}
	if !strings.Contains(err.Error(), "hours") {
		t.Errorf("error = %q, want mention of hours", err.Error())
	}
}

// --- update ---

func TestTimeUpdate_Success(t *testing.T) {
	var capturedMethod, capturedPath string
	var capturedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeUpdate(f)
	cmd.SetArgs([]string{"42", "--hours", "3.0", "--comment", "Updated estimate"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if capturedMethod != "PUT" {
		t.Errorf("expected PUT, got %s", capturedMethod)
	}
	if capturedPath != "/time_entries/42.json" {
		t.Errorf("expected /time_entries/42.json, got %s", capturedPath)
	}

	// Verify the request payload contains the CLI flag values.
	te, _ := capturedBody["time_entry"].(map[string]interface{})
	if te == nil {
		t.Fatal("request body missing time_entry wrapper")
	}
	if te["hours"] != 3.0 {
		t.Errorf("payload hours = %v, want 3.0", te["hours"])
	}
	if te["comments"] != "Updated estimate" {
		t.Errorf("payload comments = %v, want 'Updated estimate'", te["comments"])
	}

	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "Time entry #42 updated") {
		t.Errorf("stderr = %q, want success message", stderr)
	}
}

// --- delete ---

func TestTimeDelete_Force(t *testing.T) {
	var capturedMethod, capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeDelete(f)
	cmd.SetArgs([]string{"42", "--force"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if capturedMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", capturedMethod)
	}
	if capturedPath != "/time_entries/42.json" {
		t.Errorf("expected /time_entries/42.json, got %s", capturedPath)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "Time entry #42 deleted") {
		t.Errorf("stderr = %q, want success message", stderr)
	}
}

func TestTimeDelete_Cancelled(t *testing.T) {
	f := testutil.NewFactory(t, "http://unused")
	// Write "n" to stdin to decline confirmation
	f.IOStreams.In = strings.NewReader("n\n")

	cmd := newCmdTimeDelete(f)
	cmd.SetArgs([]string{"42"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "Delete cancelled") {
		t.Errorf("stderr = %q, want 'Delete cancelled'", stderr)
	}
}

func TestTimeDelete_Cancelled_JSONEnvelope(t *testing.T) {
	f := testutil.NewFactory(t, "http://unused")
	f.IOStreams.In = strings.NewReader("n\n")
	f.OutputFormat = "json"

	cmd := newCmdTimeDelete(f)
	cmd.SetArgs([]string{"42"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var env struct {
		Ok       bool   `json:"ok"`
		Action   string `json:"action"`
		Resource string `json:"resource"`
		ID       int    `json:"id"`
		Message  string `json:"message"`
	}
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &env); err != nil {
		t.Fatalf("expected JSON output, got: %v", err)
	}
	if env.Ok {
		t.Fatalf("expected ok=false envelope, got %+v", env)
	}
	if env.Action != "deleted" || env.Resource != "time_entry" || env.ID != 42 || env.Message != "Delete cancelled" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if stderr := testutil.Stderr(f); strings.Contains(stderr, "Delete cancelled") {
		t.Errorf("stderr = %q, did not expect duplicate warning in JSON mode", stderr)
	}
}

// --- summary ---

func TestTimeSummary_Table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"time_entries":[
			{"id":1,"project":{"id":1,"name":"Demo"},"user":{"id":2,"name":"Alice"},"activity":{"id":9,"name":"Development"},"hours":2.5,"comments":"","spent_on":"2025-06-15","created_on":"","updated_on":""},
			{"id":2,"project":{"id":1,"name":"Demo"},"user":{"id":2,"name":"Alice"},"activity":{"id":9,"name":"Development"},"hours":1.5,"comments":"","spent_on":"2025-06-16","created_on":"","updated_on":""}
		],"total_count":2}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeSummary(f)
	cmd.SetArgs([]string{"--output", "table", "--from", "2025-06-15", "--to", "2025-06-16"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	if !strings.Contains(stdout, "2025-06-15") || !strings.Contains(stdout, "2025-06-16") {
		t.Errorf("summary missing dates:\n%s", stdout)
	}
	stderr := testutil.Stderr(f)
	if !strings.Contains(stderr, "4.00") {
		t.Errorf("stderr = %q, want total hours", stderr)
	}
}

// --- today keyword ---

// TestTimeLog_DateTodayKeyword pins that --date today resolves to today's
// YYYY-MM-DD in the outgoing payload.
func TestTimeLog_DateTodayKeyword(t *testing.T) {
	var capturedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"time_entry":{"id":99,"hours":1,"spent_on":"","project":{"id":1,"name":"Demo"},"user":{"id":2,"name":"Alice"},"activity":{"id":9,"name":"Development"}}}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeLog(f)
	cmd.SetArgs([]string{"--hours", "1", "--issue", "10", "--date", "today"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	te, _ := capturedBody["time_entry"].(map[string]interface{})
	if te == nil {
		t.Fatal("request body missing time_entry wrapper")
	}
	got, _ := te["spent_on"].(string)
	want := stdtime.Now().Format("2006-01-02")
	if got != want {
		t.Errorf("spent_on = %q, want %q (today)", got, want)
	}
}

// TestTimeUpdate_DateTodayKeyword pins that the keyword is also resolved on
// the update path.
func TestTimeUpdate_DateTodayKeyword(t *testing.T) {
	var capturedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &capturedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeUpdate(f)
	cmd.SetArgs([]string{"42", "--date", "today"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	te, _ := capturedBody["time_entry"].(map[string]interface{})
	if te == nil {
		t.Fatal("request body missing time_entry wrapper")
	}
	got, _ := te["spent_on"].(string)
	want := stdtime.Now().Format("2006-01-02")
	if got != want {
		t.Errorf("spent_on = %q, want %q (today)", got, want)
	}
}

// TestTimeList_RangeTodayKeyword pins that --from today and --to today are
// forwarded to the filter as resolved ISO dates.
func TestTimeList_RangeTodayKeyword(t *testing.T) {
	var capturedQuery url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"time_entries":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeList(f)
	cmd.SetArgs([]string{"--from", "today", "--to", "today", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	want := stdtime.Now().Format("2006-01-02")
	if got := capturedQuery.Get("from"); got != want {
		t.Errorf("from query = %q, want %q", got, want)
	}
	if got := capturedQuery.Get("to"); got != want {
		t.Errorf("to query = %q, want %q", got, want)
	}
}
