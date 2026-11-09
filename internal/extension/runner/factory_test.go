package runner

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

func TestFromFactory(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `active_profile: a
profiles:
  a:
    server: https://a.example.com
    api_key: ka
  ro:
    server: https://ro.example.com
    api_key: kr
    read_only: true
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	orig := Executable
	Executable = func() (string, error) { return "/usr/local/bin/redmine", nil }
	t.Cleanup(func() { Executable = orig })

	boolPtr := func(b bool) *bool { return &b }
	tests := []struct {
		name         string
		f            *cmdutil.Factory
		envReadOnly  string
		envHeaders   string
		wantProfile  string
		wantReadOnly bool
		wantDisabled bool
		wantHeaders  []string
		wantServer   string
		wantNoColor  bool
		wantErr      bool
	}{
		{name: "active profile", f: &cmdutil.Factory{ConfigPath: cfgPath}, wantProfile: "a"},
		{name: "override profile read-only from config", f: &cmdutil.Factory{ConfigPath: cfgPath, ProfileOverride: "ro"}, wantProfile: "ro", wantReadOnly: true},
		{name: "flag beats profile", f: &cmdutil.Factory{ConfigPath: cfgPath, ProfileOverride: "ro", ReadOnly: boolPtr(false)}, wantProfile: "ro", wantDisabled: true},
		{name: "flag beats env", f: &cmdutil.Factory{ConfigPath: cfgPath, ReadOnly: boolPtr(false)}, envReadOnly: "1", wantProfile: "a", wantDisabled: true},
		{name: "env beats profile", f: &cmdutil.Factory{ConfigPath: cfgPath, ProfileOverride: "ro"}, envReadOnly: "0", wantProfile: "ro"},
		{name: "env read-only", f: &cmdutil.Factory{ConfigPath: cfgPath}, envReadOnly: "true", wantProfile: "a", wantReadOnly: true},
		{name: "flag read-only", f: &cmdutil.Factory{ConfigPath: cfgPath, ReadOnly: boolPtr(true)}, wantProfile: "a", wantReadOnly: true},
		{
			name:        "flag headers merged after env headers",
			f:           &cmdutil.Factory{ConfigPath: cfgPath, Headers: []string{"X-Flag: f"}},
			envHeaders:  "X-Env: e",
			wantProfile: "a",
			wantHeaders: []string{"X-Env: e", "X-Flag: f"},
		},
		{name: "env headers alone are inherited", f: &cmdutil.Factory{ConfigPath: cfgPath}, envHeaders: "X-Env: e", wantProfile: "a"},
		{name: "invalid env headers", f: &cmdutil.Factory{ConfigPath: cfgPath, Headers: []string{"X-Flag: f"}}, envHeaders: "garbage", wantErr: true},
		{name: "server and color", f: &cmdutil.Factory{ConfigPath: cfgPath, ServerOverride: "https://s.example.com", NoColorOverride: true}, wantProfile: "a", wantServer: "https://s.example.com", wantNoColor: true},
		{name: "missing config file", f: &cmdutil.Factory{ConfigPath: filepath.Join(dir, "missing.yaml")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvReadOnly, tt.envReadOnly)
			t.Setenv(EnvHeaders, tt.envHeaders)
			env, err := FromFactory(tt.f)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if env.CLIBin != "/usr/local/bin/redmine" {
				t.Errorf("CLIBin = %q", env.CLIBin)
			}
			if env.Profile != tt.wantProfile {
				t.Errorf("Profile = %q, want %q", env.Profile, tt.wantProfile)
			}
			if env.ReadOnly != tt.wantReadOnly {
				t.Errorf("ReadOnly = %v, want %v", env.ReadOnly, tt.wantReadOnly)
			}
			if env.ReadOnlyDisabled != tt.wantDisabled {
				t.Errorf("ReadOnlyDisabled = %v, want %v", env.ReadOnlyDisabled, tt.wantDisabled)
			}
			if !slices.Equal(env.Headers, tt.wantHeaders) {
				t.Errorf("Headers = %q, want %q", env.Headers, tt.wantHeaders)
			}
			if env.Server != tt.wantServer || env.NoColor != tt.wantNoColor {
				t.Errorf("Server = %q NoColor = %v", env.Server, env.NoColor)
			}
			if !filepath.IsAbs(env.ConfigPath) {
				t.Errorf("ConfigPath %q is not absolute", env.ConfigPath)
			}
		})
	}
}

func TestFromFactoryRelativeConfigBecomesAbsolute(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("c.yaml", []byte("server: https://x\napi_key: k\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env, err := FromFactory(&cmdutil.Factory{ConfigPath: "c.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if env.ConfigPath != filepath.Join(wd, "c.yaml") {
		t.Fatalf("ConfigPath = %q", env.ConfigPath)
	}
}
