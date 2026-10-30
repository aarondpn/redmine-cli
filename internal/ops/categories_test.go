package ops

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

type opsCall struct{ method, uri, body string }

// opsServer records every request and answers with the canned response for
// "METHOD /path" (status 204 when none is registered).
func opsServer(t *testing.T, responses map[string]string) (*api.Client, *[]opsCall) {
	t.Helper()
	var (
		mu    sync.Mutex
		calls []opsCall
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls = append(calls, opsCall{r.Method, r.URL.RequestURI(), string(b)})
		mu.Unlock()
		if resp, ok := responses[r.Method+" "+r.URL.Path]; ok {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(ts.Close)
	return api.NewTestClient(ts.Client(), ts.URL, debug.New(nil)), &calls
}

func ptr[T any](v T) *T { return &v }

func TestCreateIssueCategory_Validation(t *testing.T) {
	client, calls := opsServer(t, nil)
	for _, in := range []CreateIssueCategoryInput{
		{ProjectID: "demo"},
		{ProjectID: "demo", Name: "  "},
		{Name: "UI"},
		{ProjectID: "demo", Name: "UI", AssignedToID: -1},
	} {
		if _, err := CreateIssueCategory(context.Background(), client, in); err == nil {
			t.Errorf("input %+v: expected error", in)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("unexpected requests: %v", *calls)
	}
}

func TestCreateIssueCategory_Success(t *testing.T) {
	client, calls := opsServer(t, map[string]string{
		"POST /projects/demo/issue_categories.json": `{"issue_category":{"id":3,"name":"UI","project":{"id":1,"name":"Demo"}}}`,
	})
	cat, err := CreateIssueCategory(context.Background(), client, CreateIssueCategoryInput{ProjectID: "demo", Name: "UI", AssignedToID: 5})
	if err != nil || cat.ID != 3 {
		t.Fatalf("cat=%+v err=%v", cat, err)
	}
	if (*calls)[0].body != `{"issue_category":{"name":"UI","assigned_to_id":5}}` {
		t.Errorf("body = %s", (*calls)[0].body)
	}
}

func TestUpdateIssueCategory_Validation(t *testing.T) {
	client, calls := opsServer(t, nil)
	for name, in := range map[string]UpdateIssueCategoryInput{
		"no id":          {Name: ptr("x")},
		"no fields":      {ID: 1},
		"empty name":     {ID: 1, Name: ptr(" ")},
		"negative owner": {ID: 1, AssignedToID: ptr(-2)},
	} {
		if _, err := UpdateIssueCategory(context.Background(), client, in); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("unexpected requests: %v", *calls)
	}
}

func TestUpdateIssueCategory_ClearsAssignee(t *testing.T) {
	client, calls := opsServer(t, nil)
	res, err := UpdateIssueCategory(context.Background(), client, UpdateIssueCategoryInput{ID: 4, AssignedToID: ptr(0)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Message != "Updated issue category 4" {
		t.Errorf("message = %q", res.Message)
	}
	c := (*calls)[0]
	if c.method != "PUT" || c.uri != "/issue_categories/4.json" || c.body != `{"issue_category":{"assigned_to_id":""}}` {
		t.Errorf("call = %+v", c)
	}
}

func TestDeleteIssueCategory_Validation(t *testing.T) {
	client, calls := opsServer(t, nil)
	for name, in := range map[string]DeleteIssueCategoryInput{
		"no id":    {},
		"negative": {ID: 1, ReassignToID: -1},
		"self":     {ID: 1, ReassignToID: 1},
	} {
		if _, err := DeleteIssueCategory(context.Background(), client, in); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("unexpected requests: %v", *calls)
	}
}

func TestDeleteIssueCategory_PlainDelete(t *testing.T) {
	client, calls := opsServer(t, nil)
	res, err := DeleteIssueCategory(context.Background(), client, DeleteIssueCategoryInput{ID: 4})
	if err != nil {
		t.Fatal(err)
	}
	if res.Message != "Deleted issue category 4" {
		t.Errorf("message = %q", res.Message)
	}
	if len(*calls) != 1 || (*calls)[0].method != "DELETE" || (*calls)[0].uri != "/issue_categories/4.json" {
		t.Errorf("calls = %v", *calls)
	}
}

func TestDeleteIssueCategory_ReassignSameProject(t *testing.T) {
	client, calls := opsServer(t, map[string]string{
		"GET /issue_categories/4.json": `{"issue_category":{"id":4,"name":"A","project":{"id":1,"name":"Demo"}}}`,
		"GET /issue_categories/9.json": `{"issue_category":{"id":9,"name":"B","project":{"id":1,"name":"Demo"}}}`,
	})
	res, err := DeleteIssueCategory(context.Background(), client, DeleteIssueCategoryInput{ID: 4, ReassignToID: 9})
	if err != nil {
		t.Fatal(err)
	}
	if res.Message != "Deleted issue category 4, issues moved to category 9" {
		t.Errorf("message = %q", res.Message)
	}
	last := (*calls)[len(*calls)-1]
	if last.method != "DELETE" || last.uri != "/issue_categories/4.json?reassign_to_id=9" {
		t.Errorf("last call = %+v", last)
	}
}

func TestDeleteIssueCategory_ReassignOtherProjectRejected(t *testing.T) {
	client, calls := opsServer(t, map[string]string{
		"GET /issue_categories/4.json": `{"issue_category":{"id":4,"name":"A","project":{"id":1,"name":"Demo"}}}`,
		"GET /issue_categories/9.json": `{"issue_category":{"id":9,"name":"B","project":{"id":2,"name":"Other"}}}`,
	})
	_, err := DeleteIssueCategory(context.Background(), client, DeleteIssueCategoryInput{ID: 4, ReassignToID: 9})
	if err == nil || !strings.Contains(err.Error(), "belongs to project") {
		t.Fatalf("err = %v, want cross-project rejection", err)
	}
	for _, c := range *calls {
		if c.method == "DELETE" {
			t.Errorf("DELETE must not be sent: %+v", c)
		}
	}
}

func TestGetIssueCategory_RejectsBadID(t *testing.T) {
	client, _ := opsServer(t, nil)
	if _, err := GetIssueCategory(context.Background(), client, GetIssueCategoryInput{}); err == nil {
		t.Error("expected error")
	}
}
