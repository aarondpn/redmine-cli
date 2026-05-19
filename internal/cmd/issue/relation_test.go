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

func TestCmdRelations_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/issues/1/relations.json" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"relations":[{"id":7,"issue_id":1,"issue_to_id":2,"relation_type":"blocks"}]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdRelations(f)
	cmd.SetArgs([]string{"list", "1", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Stdout(f); !strings.Contains(got, "blocks") {
		t.Fatalf("stdout = %q, want blocks", got)
	}
}

func TestCmdRelations_Add(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/issues/1/relations.json" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		_, _ = w.Write([]byte(`{"relation":{"id":7,"issue_id":1,"issue_to_id":2,"relation_type":"blocks"}}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdRelations(f)
	cmd.SetArgs([]string{"add", "1", "--to", "2", "--type", "blocks", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	rel := gotBody["relation"].(map[string]any)
	if rel["issue_to_id"].(float64) != 2 || rel["relation_type"] != "blocks" {
		t.Fatalf("relation body = %v", rel)
	}
}

func TestCmdRelations_AddRejectsInvalidType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("API should not be called: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdRelations(f)
	cmd.SetArgs([]string{"add", "1", "--to", "2", "--type", "nonsense"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid relation_type") {
		t.Fatalf("err = %v, want invalid relation_type error", err)
	}
}

func TestCmdRelations_Remove(t *testing.T) {
	var delPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			delPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdRelations(f)
	cmd.SetArgs([]string{"remove", "7"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if delPath != "/relations/7.json" {
		t.Fatalf("path = %s, want /relations/7.json", delPath)
	}
}
