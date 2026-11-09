package wiki

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

type recorded struct{ method, uri, body string }

func recordingServer(t *testing.T) (*httptest.Server, func() []recorded) {
	t.Helper()
	var (
		mu    sync.Mutex
		calls []recorded
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls = append(calls, recorded{r.Method, r.URL.RequestURI(), string(b)})
		mu.Unlock()
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"proj"}}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []recorded {
		mu.Lock()
		defer mu.Unlock()
		return append([]recorded(nil), calls...)
	}
}

func lastOf(calls []recorded, method string) (recorded, bool) {
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].method == method {
			return calls[i], true
		}
	}
	return recorded{}, false
}

func TestDelete_ChildrenFlags(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    []string
		wantURI string
		wantErr string
	}{
		{"default", nil, "/projects/proj/wiki/Page.json", ""},
		{"destroy", []string{"--children", "destroy"}, "/projects/proj/wiki/Page.json?todo=destroy", ""},
		{"nullify", []string{"--children", "nullify"}, "/projects/proj/wiki/Page.json?todo=nullify", ""},
		{"invalid", []string{"--children", "burn"}, "", "--children must be one of"},
		{"reassign needs target", []string{"--children", "reassign"}, "", "requires --reassign-to"},
		{"target conflicts with destroy", []string{"--children", "destroy", "--reassign-to", "X"}, "", "cannot be combined"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv, calls := recordingServer(t)
			f := testutil.NewFactory(t, srv.URL)
			cmd := newCmdDelete(f)
			cmd.SetArgs(append([]string{"Page", "--project", "proj", "--force"}, tt.args...))
			err := cmd.Execute()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				if _, ok := lastOf(calls(), http.MethodDelete); ok {
					t.Error("a DELETE was sent despite the invalid flags")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			del, ok := lastOf(calls(), http.MethodDelete)
			if !ok || del.uri != tt.wantURI {
				t.Fatalf("DELETE = %+v (sent=%v), want %s", del, ok, tt.wantURI)
			}
		})
	}
}

func TestDelete_DestroyAsksForConfirmation(t *testing.T) {
	srv, calls := recordingServer(t)
	f := testutil.NewFactory(t, srv.URL) // stdin is empty, so the prompt is declined
	cmd := newCmdDelete(f)
	cmd.SetArgs([]string{"Page", "--project", "proj", "--children", "destroy"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, ok := lastOf(calls(), http.MethodDelete); ok {
		t.Error("destroy must not run without confirmation or --force")
	}
}

func TestUpdate_OptionFlags(t *testing.T) {
	srv, calls := recordingServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{"Page", "--project", "proj", "--text", "body", "--title", "New",
		"--start-page", "--redirect-existing-links=false",
		"--remove-attachment", "4", "--remove-attachment", "9"})
	// The fake read-back returns a project payload, so verification output is
	// irrelevant; only the PUT body is asserted.
	_ = cmd.Execute()
	put, ok := lastOf(calls(), http.MethodPut)
	if !ok {
		t.Fatal("no PUT sent")
	}
	var body struct {
		WikiPage map[string]any `json:"wiki_page"`
	}
	if err := json.Unmarshal([]byte(put.body), &body); err != nil {
		t.Fatal(err)
	}
	wp := body.WikiPage
	if wp["is_start_page"] != true || wp["redirect_existing_links"] != "0" {
		t.Errorf("wiki_page = %v", wp)
	}
	if ids, _ := wp["deleted_attachment_ids"].([]any); len(ids) != 2 || ids[0] != float64(4) || ids[1] != float64(9) {
		t.Errorf("deleted_attachment_ids = %v", wp["deleted_attachment_ids"])
	}
}

func TestUpdate_OptionFlagErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"redirect without title", []string{"--redirect-existing-links=false"}, "requires --title"},
		{"bad attachment id", []string{"--remove-attachment", "0"}, "positive attachment ID"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv, calls := recordingServer(t)
			f := testutil.NewFactory(t, srv.URL)
			cmd := newCmdUpdate(f)
			cmd.SetArgs(append([]string{"Page", "--project", "proj", "--text", "x"}, tt.args...))
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
			if _, ok := lastOf(calls(), http.MethodPut); ok {
				t.Error("a PUT was sent despite the invalid flags")
			}
		})
	}
}
