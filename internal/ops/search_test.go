package ops

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

func TestSearch_AllWordsQuery(t *testing.T) {
	f := false
	tr := true
	tests := []struct {
		name     string
		allWords *bool
		want     string
	}{
		{"omitted defaults to all words", nil, "1"},
		{"true", &tr, "1"},
		{"false sends empty value for any word", &f, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery map[string][]string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.Query()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"results":[],"total_count":0}`))
			}))
			defer ts.Close()
			client := api.NewTestClient(ts.Client(), ts.URL, debug.New(nil))

			if _, err := Search(t.Context(), client, SearchInput{Query: "foo bar", AllWords: tt.allWords}); err != nil {
				t.Fatal(err)
			}
			vals, ok := gotQuery["all_words"]
			if !ok || len(vals) != 1 || vals[0] != tt.want {
				t.Errorf("all_words = %q (present=%v), want %q", vals, ok, tt.want)
			}
		})
	}
}
