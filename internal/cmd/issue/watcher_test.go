package issue

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestCmdWatchers_List(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !strings.Contains(r.URL.RawQuery, "watchers") {
			t.Errorf("expected include=watchers, got %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"issue":{"id":1,"project":{"id":1,"name":"P"},"tracker":{"id":1,"name":"T"},"status":{"id":1,"name":"New"},"priority":{"id":1,"name":"Normal"},"author":{"id":1,"name":"A"},"subject":"s","watchers":[{"id":7,"name":"Alice"},{"id":8,"name":"Bob"}]}}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdWatchers(f)
	cmd.SetArgs([]string{"list", "1", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := testutil.Stdout(f)
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "Bob") {
		t.Fatalf("stdout = %q, want Alice and Bob", out)
	}
}

func TestCmdWatchers_Add(t *testing.T) {
	var postPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			postPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdWatchers(f)
	cmd.SetArgs([]string{"add", "11", "22"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if postPath != "/issues/11/watchers.json" {
		t.Fatalf("path = %s, want /issues/11/watchers.json", postPath)
	}
}

func TestCmdWatchers_Remove(t *testing.T) {
	var delPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			delPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdWatchers(f)
	cmd.SetArgs([]string{"remove", "11", "22"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if delPath != "/issues/11/watchers/22.json" {
		t.Fatalf("path = %s, want /issues/11/watchers/22.json", delPath)
	}
}
