package search

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// Redmine treats a missing all_words as true and an empty value as false, so
// the CLI must always send an explicit value.
func TestCmdSearch_AllWordsQuery(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"default is all words", []string{"foo bar"}, "1"},
		{"explicit flag", []string{"foo bar", "--all-words"}, "1"},
		{"any word", []string{"foo bar", "--all-words=false"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery map[string][]string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.Query()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"results":[],"total_count":0}`))
			}))
			defer srv.Close()

			f := testutil.NewFactory(t, srv.URL)
			cmd := NewCmdSearch(f)
			cmd.SetArgs(append(tt.args, "--scope", "all", "--output", "json"))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			vals, ok := gotQuery["all_words"]
			if !ok || len(vals) != 1 || vals[0] != tt.want {
				t.Errorf("all_words = %q (present=%v), want %q", vals, ok, tt.want)
			}
		})
	}
}

func TestCmdSearchWiki_AnyWordQuery(t *testing.T) {
	var gotQuery map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdSearchWiki(f)
	cmd.SetArgs([]string{"foo", "--scope", "all", "--all-words=false", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	vals, ok := gotQuery["all_words"]
	if !ok || len(vals) != 1 || vals[0] != "" {
		t.Errorf("all_words = %q (present=%v), want empty", vals, ok)
	}
}
