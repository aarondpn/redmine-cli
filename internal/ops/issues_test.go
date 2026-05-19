package ops

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

type captureIssueUpdateHandler struct {
	mu     sync.Mutex
	bodies []map[string]any
}

func (h *captureIssueUpdateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "unexpected method "+r.Method, http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(r.Body)
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	h.mu.Lock()
	h.bodies = append(h.bodies, parsed)
	h.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (h *captureIssueUpdateHandler) lastIssue(t *testing.T) map[string]any {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.bodies) == 0 {
		t.Fatal("no PUT bodies captured")
	}
	issue, ok := h.bodies[len(h.bodies)-1]["issue"].(map[string]any)
	if !ok {
		t.Fatalf("body missing issue key: %+v", h.bodies[len(h.bodies)-1])
	}
	return issue
}

func newTestAPIClient(t *testing.T, handler http.Handler) (*api.Client, func()) {
	t.Helper()
	ts := httptest.NewServer(handler)
	cfg := &config.Config{Server: ts.URL, APIKey: "test-key", AuthMethod: "apikey"}
	client, err := api.NewClient(cfg, debug.New(nil))
	if err != nil {
		ts.Close()
		t.Fatalf("api.NewClient: %v", err)
	}
	return client, ts.Close
}

// AddIssueComment must not send the issue-level is_private flag (which would
// silently flip issue privacy on every comment).
func TestAddIssueComment_DoesNotTouchIssuePrivacy(t *testing.T) {
	cap := &captureIssueUpdateHandler{}
	client, closeTS := newTestAPIClient(t, cap)
	defer closeTS()

	if _, err := AddIssueComment(context.Background(), client, AddIssueCommentInput{ID: 42, Notes: "hello"}); err != nil {
		t.Fatalf("AddIssueComment: %v", err)
	}

	issue := cap.lastIssue(t)
	if _, ok := issue["is_private"]; ok {
		t.Errorf("default comment unexpectedly sent is_private: %+v", issue)
	}
	if _, ok := issue["private_notes"]; ok {
		t.Errorf("default comment unexpectedly sent private_notes: %+v", issue)
	}
	if got, _ := issue["notes"].(string); got != "hello" {
		t.Errorf("notes = %v, want hello", issue["notes"])
	}
}

// CreateIssue propagates the new fields (start_date, due_date,
// watcher_user_ids, custom_field_values) to the underlying POST body.
func TestCreateIssue_PropagatesNewFields(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/issues.json" {
			http.Error(w, "unexpected request "+r.Method+" "+r.URL.Path, http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issue":{"id":1}}`))
	}))
	defer srv.Close()
	cfg := &config.Config{Server: srv.URL, APIKey: "k", AuthMethod: "apikey"}
	client, err := api.NewClient(cfg, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := CreateIssue(context.Background(), client, CreateIssueInput{
		ProjectID:         5,
		Subject:           "Hi",
		StartDate:         "2026-05-01",
		DueDate:           "2026-05-15",
		WatcherUserIDs:    []int{2, 3},
		CustomFieldValues: map[string]string{"7": "urgent"},
	}); err != nil {
		t.Fatal(err)
	}
	issue := got["issue"].(map[string]any)
	if issue["start_date"] != "2026-05-01" {
		t.Errorf("start_date = %v, want 2026-05-01", issue["start_date"])
	}
	if issue["due_date"] != "2026-05-15" {
		t.Errorf("due_date = %v, want 2026-05-15", issue["due_date"])
	}
	ids := issue["watcher_user_ids"].([]any)
	if len(ids) != 2 || ids[0].(float64) != 2 || ids[1].(float64) != 3 {
		t.Errorf("watcher_user_ids = %v, want [2 3]", ids)
	}
	cfvs := issue["custom_field_values"].(map[string]any)
	if cfvs["7"] != "urgent" {
		t.Errorf("custom_field_values[7] = %v, want urgent", cfvs["7"])
	}
}

// UpdateIssue propagates the new pointer fields (start_date, private_notes,
// custom_field_values) only when they are set.
func TestUpdateIssue_PropagatesNewFields(t *testing.T) {
	cap := &captureIssueUpdateHandler{}
	client, closeTS := newTestAPIClient(t, cap)
	defer closeTS()

	start := "2026-06-01"
	priv := true
	notes := "internal"
	if _, err := UpdateIssue(context.Background(), client, UpdateIssueInput{
		ID:                42,
		StartDate:         &start,
		Notes:             &notes,
		PrivateNotes:      &priv,
		CustomFieldValues: map[string]string{"8": "yes"},
	}); err != nil {
		t.Fatal(err)
	}
	issue := cap.lastIssue(t)
	if issue["start_date"] != "2026-06-01" {
		t.Errorf("start_date = %v, want 2026-06-01", issue["start_date"])
	}
	if issue["private_notes"] != true {
		t.Errorf("private_notes = %v, want true", issue["private_notes"])
	}
	cfvs := issue["custom_field_values"].(map[string]any)
	if cfvs["8"] != "yes" {
		t.Errorf("custom_field_values[8] = %v, want yes", cfvs["8"])
	}
}

// AddIssueComment must wire private_notes (journal-level) when requested,
// not is_private (issue-level).
func TestAddIssueComment_SetsPrivateNotesWhenRequested(t *testing.T) {
	cap := &captureIssueUpdateHandler{}
	client, closeTS := newTestAPIClient(t, cap)
	defer closeTS()

	if _, err := AddIssueComment(context.Background(), client, AddIssueCommentInput{ID: 42, Notes: "shh", PrivateNotes: true}); err != nil {
		t.Fatalf("AddIssueComment: %v", err)
	}

	issue := cap.lastIssue(t)
	if got, _ := issue["private_notes"].(bool); !got {
		t.Errorf("private_notes = %v, want true", issue["private_notes"])
	}
	if _, ok := issue["is_private"]; ok {
		t.Errorf("unexpectedly sent is_private along with private_notes: %+v", issue)
	}
}
