package cmdutil

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
)

// globalFlagsRoot returns a root command with the persistent flags the real
// root defines that ApplyGlobalFlags reads, and a child to run.
func globalFlagsRoot() (*cobra.Command, *cobra.Command) {
	root := &cobra.Command{Use: "redmine"}
	pf := root.PersistentFlags()
	pf.String("config", "", "")
	pf.String("profile", "", "")
	pf.String("server", "", "")
	pf.String("api-key", "", "")
	pf.String("api-key-file", "", "")
	pf.String("output", "", "")
	pf.Bool("no-color", false, "")
	pf.Bool("verbose", false, "")
	pf.Bool("read-only", false, "")
	pf.StringArray("header", nil, "")
	child := &cobra.Command{Use: "child", RunE: func(*cobra.Command, []string) error { return nil }}
	root.AddCommand(child)
	return root, child
}

// parseGlobalFlags parses args on a fresh root and applies them to a fresh
// factory.
func parseGlobalFlags(t *testing.T, args ...string) (*Factory, error) {
	t.Helper()
	root, child := globalFlagsRoot()
	if err := root.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	f := &Factory{IOStreams: &IOStreams{In: strings.NewReader(""), Out: io.Discard, ErrOut: &bytes.Buffer{}}}
	return f, ApplyGlobalFlags(f, child)
}

func TestApplyGlobalFlagsConfigAndProfileEnv(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		envConfig   string
		envProfile  string
		wantConfig  string
		wantProfile string
	}{
		{name: "nothing set"},
		{name: "env only", envConfig: "/env/cfg.yaml", envProfile: "env-profile", wantConfig: "/env/cfg.yaml", wantProfile: "env-profile"},
		{name: "flags only", args: []string{"--config", "/flag/cfg.yaml", "--profile", "flag-profile"}, wantConfig: "/flag/cfg.yaml", wantProfile: "flag-profile"},
		{
			name:       "flags win over env",
			args:       []string{"--config", "/flag/cfg.yaml", "--profile", "flag-profile"},
			envConfig:  "/env/cfg.yaml",
			envProfile: "env-profile",
			wantConfig: "/flag/cfg.yaml", wantProfile: "flag-profile",
		},
		{
			name:       "each flag wins on its own",
			args:       []string{"--profile", "flag-profile"},
			envConfig:  "/env/cfg.yaml",
			envProfile: "env-profile",
			wantConfig: "/env/cfg.yaml", wantProfile: "flag-profile",
		},
		{
			name:       "explicit empty flag wins over env",
			args:       []string{"--profile="},
			envProfile: "env-profile",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(config.ConfigEnv, tt.envConfig)
			t.Setenv(config.ProfileEnv, tt.envProfile)
			t.Setenv(config.HeadersEnv, "")
			f, err := parseGlobalFlags(t, tt.args...)
			if err != nil {
				t.Fatal(err)
			}
			if f.ConfigPath != tt.wantConfig || f.ProfileOverride != tt.wantProfile {
				t.Fatalf("ConfigPath = %q, ProfileOverride = %q; want %q, %q", f.ConfigPath, f.ProfileOverride, tt.wantConfig, tt.wantProfile)
			}
		})
	}
}

func TestApplyConfigSelectionKeepsPresetWithoutFlagOrEnv(t *testing.T) {
	t.Setenv(config.ConfigEnv, "")
	t.Setenv(config.ProfileEnv, "")
	root, _ := globalFlagsRoot()
	f := &Factory{ConfigPath: "/preset.yaml", ProfileOverride: "preset"}
	ApplyConfigSelection(f, root.PersistentFlags())
	if f.ConfigPath != "/preset.yaml" || f.ProfileOverride != "preset" {
		t.Fatalf("ConfigPath = %q, ProfileOverride = %q; want the preset values", f.ConfigPath, f.ProfileOverride)
	}

	// Without the flags defined, nothing is resolved, not even from env.
	t.Setenv(config.ProfileEnv, "env")
	f = &Factory{}
	ApplyConfigSelection(f, (&cobra.Command{}).PersistentFlags())
	if f.ProfileOverride != "" {
		t.Fatalf("ProfileOverride = %q for a flag set without --profile", f.ProfileOverride)
	}
}

func TestApplyGlobalFlagsHeadersEnv(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		env         string
		wantErr     string
		wantHeaders []string
	}{
		{name: "valid env, no flags", env: "X-A: 1\nX-B: 2"},
		{name: "invalid env fails early", env: "broken", wantErr: config.HeadersEnv},
		{name: "invalid env fails even with flags", args: []string{"--header", "X-A: 1"}, env: "Host: x", wantErr: config.HeadersEnv},
		// Env headers are merged by Config, so Headers keeps only the flags.
		{name: "flags kept separate", args: []string{"--header", "X-A: 1"}, env: "X-B: 2", wantHeaders: []string{"X-A: 1"}},
		{name: "invalid flag", args: []string{"--header", "nope"}, wantErr: "invalid header"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(config.HeadersEnv, tt.env)
			f, err := parseGlobalFlags(t, tt.args...)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ApplyGlobalFlags() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(f.Headers, tt.wantHeaders) {
				t.Fatalf("Headers = %q, want %q", f.Headers, tt.wantHeaders)
			}
		})
	}
}

func TestFactoryConfigHeaderPrecedence(t *testing.T) {
	cfgPath := writeConfigFile(t, "active_profile: default\nprofiles:\n  default:\n    server: https://x\n    api_key: k\n    headers:\n      X-Profile: profile\n      X-Env: profile\n      X-Flag: profile\n")
	tests := []struct {
		name  string
		env   string
		flags []string
		want  map[string]string
	}{
		{
			name: "profile only",
			want: map[string]string{"X-Profile": "profile", "X-Env": "profile", "X-Flag": "profile"},
		},
		{
			name: "env over profile",
			env:  "x-env: env\nX-New: env",
			want: map[string]string{"X-Profile": "profile", "X-Env": "env", "X-Flag": "profile", "X-New": "env"},
		},
		{
			name:  "flag over env over profile",
			env:   "X-Env: env\nX-Flag: env",
			flags: []string{"x-flag: flag"},
			want:  map[string]string{"X-Profile": "profile", "X-Env": "env", "X-Flag": "flag"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(config.HeadersEnv, tt.env)
			f := NewFactory()
			f.ConfigPath = cfgPath
			f.Headers = tt.flags
			cfg, err := f.Config()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(cfg.Headers, tt.want) {
				t.Fatalf("Headers = %v, want %v", cfg.Headers, tt.want)
			}
		})
	}
}

func TestFactoryConfigRejectsInvalidEnvHeaders(t *testing.T) {
	cfgPath := writeConfigFile(t, "server: https://x\napi_key: k\n")
	t.Setenv(config.HeadersEnv, "X-A: 1\nno colon here")
	f := NewFactory()
	f.ConfigPath = cfgPath
	if _, err := f.Config(); err == nil || !strings.Contains(err.Error(), config.HeadersEnv) {
		t.Fatalf("Config() error = %v, want it to name %s", err, config.HeadersEnv)
	}
}

func TestFactoryConfigEnvHeaderSilencesProfileHeaderWarning(t *testing.T) {
	cfgPath := writeConfigFile(t, "active_profile: work\nprofiles:\n  work:\n    server: https://redmine.work.com\n    api_key: k\n    headers:\n      X-Proxy-Token: t\n")
	tests := []struct {
		name     string
		env      string
		wantWarn bool
	}{
		{name: "profile header sent elsewhere warns", wantWarn: true},
		{name: "env header replaces it", env: "X-Proxy-Token: other", wantWarn: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(config.HeadersEnv, tt.env)
			errOut := &bytes.Buffer{}
			f := &Factory{
				ConfigPath:     cfgPath,
				ServerOverride: "https://other.example.com",
				APIKeyOverride: "explicit",
				IOStreams:      &IOStreams{In: strings.NewReader(""), Out: io.Discard, ErrOut: errOut},
			}
			if _, err := f.Config(); err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(errOut.String(), "X-Proxy-Token"); got != tt.wantWarn {
				t.Fatalf("warning printed = %v, want %v; stderr:\n%s", got, tt.wantWarn, errOut.String())
			}
		})
	}
}

func TestFactoryConfigProfileFromEnvViaGlobalFlags(t *testing.T) {
	cfgPath := writeConfigFile(t, "active_profile: a\nprofiles:\n  a:\n    server: https://a.example.com\n    api_key: ka\n  b:\n    server: https://b.example.com\n    api_key: kb\n")
	t.Setenv(config.ConfigEnv, cfgPath)
	t.Setenv(config.ProfileEnv, "b")
	t.Setenv(config.HeadersEnv, "")
	t.Setenv("REDMINE_SERVER", "")
	t.Setenv("REDMINE_API_KEY", "")

	f, err := parseGlobalFlags(t)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := f.Config()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server != "https://b.example.com" {
		t.Fatalf("Server = %q, want the REDMINE_PROFILE profile", cfg.Server)
	}

	f, err = parseGlobalFlags(t, "--profile", "a")
	if err != nil {
		t.Fatal(err)
	}
	if cfg, err = f.Config(); err != nil || cfg.Server != "https://a.example.com" {
		t.Fatalf("--profile a: Server = %q, %v; want the flag to win", cfg.Server, err)
	}

	t.Setenv(config.ProfileEnv, "missing")
	f, err = parseGlobalFlags(t)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Config(); err == nil || !strings.Contains(err.Error(), `profile "missing" not found`) {
		t.Fatalf("unknown REDMINE_PROFILE: error = %v", err)
	}
}
