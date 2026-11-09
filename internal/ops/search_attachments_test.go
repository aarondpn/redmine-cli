package ops

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

func TestSearch_Attachments(t *testing.T) {
	for _, tt := range []struct {
		value   string
		wantSet bool
		wantErr bool
	}{
		{"", false, false},
		{"0", true, false},
		{"1", true, false},
		{"only", true, false},
		{"yes", false, true},
	} {
		t.Run(tt.value, func(t *testing.T) {
			var got map[string][]string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.URL.Query()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"results":[],"total_count":0}`))
			}))
			defer ts.Close()
			client := api.NewTestClient(ts.Client(), ts.URL, debug.New(nil))

			_, err := Search(t.Context(), client, SearchInput{Query: "foo", Attachments: tt.value})
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			vals, ok := got["attachments"]
			if ok != tt.wantSet || (ok && vals[0] != tt.value) {
				t.Errorf("attachments = %q (present=%v), want %q (present=%v)", vals, ok, tt.value, tt.wantSet)
			}
		})
	}
}
