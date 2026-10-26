package auth

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// TestCheckCleartextServerMatchesCommands pins that auth login judges a
// server and words the warning exactly like every other command
// (api.IsCleartextServer / api.CleartextServerWarning), including the
// userinfo scrubbing and the *.localhost exemption.
func TestCheckCleartextServerMatchesCommands(t *testing.T) {
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
			t.Setenv("REDMINE_STRICT", "")
			var errOut bytes.Buffer
			// A missing config file keeps the lookup away from the real ~/.redmine-cli.yaml.
			f := &cmdutil.Factory{ConfigPath: filepath.Join(t.TempDir(), "config.yaml"), IOStreams: &cmdutil.IOStreams{ErrOut: &errOut}}
			if err := checkCleartextServer(f, tc.server, "work"); err != nil {
				t.Fatalf("checkCleartextServer error = %v, want nil outside strict mode", err)
			}
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

func TestCheckCleartextServer(t *testing.T) {
	const strictProfile = "profiles:\n  work:\n    server: http://redmine.example.com\n    strict: true\n"
	cases := []struct {
		name       string
		server     string
		profile    string // the profile login writes; default "work"
		configYAML string
		strictEnv  string
		wantStrict bool
		wantWarn   bool
	}{
		{name: "http warns", server: "http://redmine.example.com", wantWarn: true},
		{name: "https is silent", server: "https://redmine.example.com"},
		{name: "loopback http is silent", server: "http://127.0.0.1:3000"},
		{name: "strict profile refuses", server: "http://redmine.example.com", configYAML: strictProfile, wantStrict: true},
		{name: "strict setting of other profiles is ignored", server: "http://redmine.example.com", profile: "home", configYAML: "active_profile: work\n" + strictProfile + "  home:\n    server: http://home.example.com\n", wantWarn: true},
		{name: "REDMINE_STRICT refuses", server: "http://redmine.example.com", strictEnv: "1", wantStrict: true},
		{name: "REDMINE_STRICT=0 cannot lower a strict profile", server: "http://redmine.example.com", configYAML: strictProfile, strictEnv: "0", wantStrict: true},
		{name: "unparsable config fails closed", server: "http://redmine.example.com", configYAML: "profiles: [", wantStrict: true},
		{name: "strict allows loopback", server: "http://localhost:3000", strictEnv: "1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.strictEnv != "" {
				t.Setenv("REDMINE_STRICT", tc.strictEnv)
			}
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			if tc.configYAML != "" {
				if err := os.WriteFile(configPath, []byte(tc.configYAML), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var errOut bytes.Buffer
			f := &cmdutil.Factory{ConfigPath: configPath, IOStreams: &cmdutil.IOStreams{ErrOut: &errOut}}

			profile := tc.profile
			if profile == "" {
				profile = "work"
			}
			err := checkCleartextServer(f, tc.server, profile)
			var strictErr *cmdutil.StrictModeError
			if tc.wantStrict {
				if !errors.As(err, &strictErr) {
					t.Fatalf("checkCleartextServer error = %v, want *StrictModeError", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("checkCleartextServer error = %v, want nil", err)
			}
			warned := strings.Contains(errOut.String(), "sent in cleartext")
			if warned != tc.wantWarn {
				t.Fatalf("warned = %v, want %v (stderr %q)", warned, tc.wantWarn, errOut.String())
			}
		})
	}
}
