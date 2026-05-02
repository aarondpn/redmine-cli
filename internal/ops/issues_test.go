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
