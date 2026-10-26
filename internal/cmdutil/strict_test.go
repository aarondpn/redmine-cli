package cmdutil

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStrictMode(t *testing.T) {
	const profiles = "active_profile: work\n" +
		"profiles:\n" +
		"  work:\n" +
		"    server: https://work.example.com\n" +
		"    credential_store: keyring\n" +
		"    strict: true\n" +
		"  home:\n" +
		"    server: https://home.example.com\n"
	tests := []struct {
		name       string
		configYAML string // empty: no config file
		profile    string // --profile
		strictEnv  string
		want       bool
	}{
		{name: "no config file", want: false},
		{name: "no config file, REDMINE_STRICT=1", strictEnv: "1", want: true},
		{name: "active strict profile", configYAML: profiles, want: true},
		{name: "--profile selects a non-strict profile", configYAML: profiles, profile: "home", want: false},
		{name: "REDMINE_STRICT raises a non-strict profile", configYAML: profiles, profile: "home", strictEnv: "true", want: true},
		{name: "REDMINE_STRICT=0 cannot lower a strict profile", configYAML: profiles, strictEnv: "0", want: true},
		{name: "REDMINE_STRICT=false cannot lower a strict profile", configYAML: profiles, strictEnv: "false", want: true},
		{name: "only profile without active_profile", configYAML: "profiles:\n  solo:\n    server: https://x.example.com\n    strict: true\n", want: true},
		{name: "unknown --profile", configYAML: profiles, profile: "missing", want: false},
		{name: "unparsable config fails closed", configYAML: "profiles: [", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("REDMINE_STRICT", tt.strictEnv)
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			if tt.configYAML != "" {
				if err := os.WriteFile(configPath, []byte(tt.configYAML), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			f := &Factory{
				ConfigPath:      configPath,
				ProfileOverride: tt.profile,
				IOStreams:       &IOStreams{ErrOut: &bytes.Buffer{}},
			}
			if got := f.StrictMode(); got != tt.want {
				t.Errorf("StrictMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProfileStrictMode_NamedProfile(t *testing.T) {
	t.Setenv("REDMINE_STRICT", "")
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	data := "active_profile: home\n" +
		"profiles:\n" +
		"  work:\n" +
		"    server: https://work.example.com\n" +
		"    strict: true\n" +
		"  home:\n" +
		"    server: https://home.example.com\n"
	if err := os.WriteFile(configPath, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &Factory{ConfigPath: configPath, IOStreams: &IOStreams{ErrOut: &bytes.Buffer{}}}

	if !f.ProfileStrictMode("work") {
		t.Error(`ProfileStrictMode("work") = false, want true`)
	}
	if f.ProfileStrictMode("home") {
		t.Error(`ProfileStrictMode("home") = true, want false`)
	}
	if f.ProfileStrictMode("new") {
		t.Error(`ProfileStrictMode("new") = true for a profile that does not exist yet`)
	}
}

// TestStrictMode_WhenConfigFails pins the reason StrictMode exists: on a
// strict profile Config fails because strict mode refused something, and
// strictness must still be known afterwards.
func TestStrictMode_WhenConfigFails(t *testing.T) {
	t.Setenv("REDMINE_STRICT", "")
	t.Setenv("REDMINE_API_KEY", "")
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	data := "profiles:\n  work:\n    server: https://work.example.com\n    api_key: stored\n    strict: true\n"
	if err := os.WriteFile(configPath, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	f := &Factory{
		ConfigPath:     configPath,
		APIKeyOverride: "from-argv",
		IOStreams:      &IOStreams{ErrOut: &bytes.Buffer{}},
	}

	_, err := f.Config()
	var strictErr *StrictModeError
	if !errors.As(err, &strictErr) {
		t.Fatalf("Config() error = %v, want *StrictModeError", err)
	}
	if !f.StrictMode() {
		t.Error("StrictMode() = false after Config() refused --api-key on a strict profile")
	}
}
