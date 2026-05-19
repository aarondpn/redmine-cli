package ops

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

func newRelationsTestClient(t *testing.T, srv *httptest.Server) *api.Client {
	t.Helper()
	cfg := &config.Config{Server: srv.URL, APIKey: "k", AuthMethod: "apikey"}
	client, err := api.NewClient(cfg, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestCreateIssueRelation_ValidatesInputs(t *testing.T) {
	tests := []struct {
		name    string
		in      CreateIssueRelationInput
		wantErr string
	}{
		{"missing issue", CreateIssueRelationInput{IssueToID: 2}, "issue_id"},
		{"missing target", CreateIssueRelationInput{IssueID: 1}, "issue_to_id"},
		{"self-relation", CreateIssueRelationInput{IssueID: 1, IssueToID: 1}, "itself"},
		{"bad type", CreateIssueRelationInput{IssueID: 1, IssueToID: 2, RelationType: "nope"}, "invalid relation_type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CreateIssueRelation(context.Background(), nil, tt.in)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestCreateIssueRelation_RejectsDelayOnWrongType(t *testing.T) {
	delay := 3
	_, err := CreateIssueRelation(context.Background(), nil, CreateIssueRelationInput{
		IssueID: 1, IssueToID: 2, RelationType: "blocks", Delay: &delay,
	})
	if err == nil || !strings.Contains(err.Error(), "delay is only valid") {
		t.Fatalf("err = %v, want delay-rejection message", err)
	}
}

func TestCreateIssueRelation_SendsPayload(t *testing.T) {
	var gotBody map[string]any
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"relation":{"id":7,"issue_id":1,"issue_to_id":2,"relation_type":"precedes","delay":3}}`))
	}))
	defer srv.Close()
	client := newRelationsTestClient(t, srv)

	delay := 3
	rel, err := CreateIssueRelation(context.Background(), client, CreateIssueRelationInput{
		IssueID: 1, IssueToID: 2, RelationType: "precedes", Delay: &delay,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/issues/1/relations.json" {
		t.Errorf("path = %s, want /issues/1/relations.json", gotPath)
	}
	rel2 := gotBody["relation"].(map[string]any)
	if rel2["issue_to_id"].(float64) != 2 || rel2["relation_type"] != "precedes" {
		t.Errorf("relation body = %v", rel2)
	}
	if rel.ID != 7 {
		t.Errorf("returned relation ID = %d, want 7", rel.ID)
	}
}

func TestDeleteIssueRelation_RejectsNonPositiveID(t *testing.T) {
	_, err := DeleteIssueRelation(context.Background(), nil, DeleteIssueRelationInput{ID: 0})
	if err == nil || !strings.Contains(err.Error(), "positive relation ID") {
		t.Fatalf("err = %v, want positive-ID message", err)
	}
}

func TestAddIssueWatcher_SendsUserID(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	client := newRelationsTestClient(t, srv)

	if _, err := AddIssueWatcher(context.Background(), client, AddIssueWatcherInput{ID: 11, UserID: 22}); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/issues/11/watchers.json" {
		t.Errorf("path = %s, want /issues/11/watchers.json", gotPath)
	}
	if v, _ := gotBody["user_id"].(float64); v != 22 {
		t.Errorf("user_id = %v, want 22", gotBody["user_id"])
	}
}

func TestRemoveIssueWatcher_HitsExpectedPath(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	client := newRelationsTestClient(t, srv)

	if _, err := RemoveIssueWatcher(context.Background(), client, RemoveIssueWatcherInput{ID: 11, UserID: 22}); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
	if gotPath != "/issues/11/watchers/22.json" {
		t.Errorf("path = %s, want /issues/11/watchers/22.json", gotPath)
	}
}
