package ops

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

type respSpec struct {
	status int
	body   string
}

// respServer answers "METHOD /path" with the given status and body, 404 otherwise.
func respServer(t *testing.T, responses map[string]respSpec) (*api.Client, *[]string) {
	t.Helper()
	var seen []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.RequestURI())
		resp, ok := responses[r.Method+" "+r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.status)
		_, _ = w.Write([]byte(resp.body))
	}))
	t.Cleanup(ts.Close)
	return api.NewTestClient(ts.Client(), ts.URL, debug.New(nil)), &seen
}

func TestGetNews_Validation(t *testing.T) {
	client, calls := opsServer(t, nil)
	for name, in := range map[string]GetNewsInput{
		"zero id":         {},
		"negative id":     {ID: -3},
		"unknown include": {ID: 1, Includes: []string{"comments", "bogus"}},
	} {
		if _, err := GetNews(context.Background(), client, in); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("unexpected requests: %v", *calls)
	}
}

func TestGetNews_AcceptsKnownIncludes(t *testing.T) {
	client, calls := opsServer(t, map[string]string{
		"GET /news/5.json": `{"news":{"id":5,"title":"T"}}`,
	})
	if _, err := GetNews(context.Background(), client, GetNewsInput{ID: 5, Includes: []string{"attachments", " comments ", ""}}); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 {
		t.Errorf("calls = %v", *calls)
	}
}

func TestUpdateJournal_HintOnUnsupportedEndpoint(t *testing.T) {
	for status, want := range map[int]string{
		http.StatusNotFound:     "Redmine 5.0",
		http.StatusUnauthorized: "Redmine 5.0",
		http.StatusForbidden:    "Redmine 5.0+",
	} {
		client, _ := respServer(t, map[string]respSpec{"PUT /journals/9.json": {status, `{"errors":["nope"]}`}})
		_, err := UpdateJournal(context.Background(), client, UpdateJournalInput{ID: 9, Notes: ptr("x")})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("status %d: err = %v, want hint containing %q", status, err, want)
			continue
		}
		var he *HintedError
		if !errors.As(err, &he) || he.Hint == "" {
			t.Errorf("status %d: expected HintedError, got %T", status, err)
		}
	}
}

func TestUpdateJournal_NoHintOnOtherErrors(t *testing.T) {
	client, _ := respServer(t, map[string]respSpec{"PUT /journals/9.json": {http.StatusUnprocessableEntity, `{"errors":["bad"]}`}})
	_, err := UpdateJournal(context.Background(), client, UpdateJournalInput{ID: 9, Notes: ptr("x")})
	var he *HintedError
	if err == nil || errors.As(err, &he) {
		t.Errorf("err = %v, want plain error", err)
	}
}

func TestDeleteIssueCategory_ReassignTargetNotFound(t *testing.T) {
	client, calls := respServer(t, map[string]respSpec{
		"GET /issue_categories/4.json": {200, `{"issue_category":{"id":4,"name":"A","project":{"id":1,"name":"Demo"}}}`},
	})
	_, err := DeleteIssueCategory(context.Background(), client, DeleteIssueCategoryInput{ID: 4, ReassignToID: 99})
	if err == nil || !strings.Contains(err.Error(), "reassign target category 99 not found") {
		t.Fatalf("err = %v", err)
	}
	for _, c := range *calls {
		if strings.HasPrefix(c, "DELETE") {
			t.Errorf("DELETE must not be sent: %s", c)
		}
	}
}
