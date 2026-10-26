package auth

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// TestWarnCleartextServer pins that auth login judges a server with
// api.IsCleartextServer and words the hint with api.CleartextServerWarning,
// including the userinfo scrubbing and the *.localhost exemption.
func TestWarnCleartextServer(t *testing.T) {
	cases := []struct {
		server string
		want   bool
	}{
		{"http://redmine.example.com", true},
		{"HTTP://redmine.example.com:8080/redmine", true},
		{"http://10.0.0.5", true},
		{"http://user:hunter2@redmine.example.com", true},
		{"https://redmine.example.com", false},
		{"redmine.example.com", false},
		{"http://localhost:3000", false},
		{"http://LOCALHOST", false},
		{"http://redmine.localhost:3000", false},
		{"http://127.0.0.1:3000", false},
		{"http://127.1.2.3", false},
		{"http://[::1]:3000", false},
	}
	for _, tc := range cases {
		t.Run(tc.server, func(t *testing.T) {
			var errOut bytes.Buffer
			f := &cmdutil.Factory{IOStreams: &cmdutil.IOStreams{ErrOut: &errOut}}
			// Called twice, the hint is still printed only once.
			warnCleartextServer(f, tc.server)
			warnCleartextServer(f, tc.server)
			if got := api.IsCleartextServer(tc.server); got != tc.want {
				t.Fatalf("api.IsCleartextServer(%q) = %v, want %v", tc.server, got, tc.want)
			}
			want := ""
			if tc.want {
				want = "warning: " + api.CleartextServerWarning(tc.server) + "\n"
			}
			if errOut.String() != want {
				t.Fatalf("stderr = %q, want %q", errOut.String(), want)
			}
			if strings.Contains(errOut.String(), "hunter2") {
				t.Fatalf("warning leaks the URL password: %q", errOut.String())
			}
		})
	}
}
