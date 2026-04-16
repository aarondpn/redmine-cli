package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
)

func TestStatus_HonorsProfileOverride(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    auth_method: apikey
    api_key: work-key
  personal:
    server: https://personal.example.com
    auth_method: apikey
    api_key: personal-key
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	f := &cmdutil.Factory{
		ConfigPath:      cfgPath,
		ProfileOverride: "personal",
		IOStreams: &cmdutil.IOStreams{
			In:     strings.NewReader(""),
			Out:    &strings.Builder{},
			ErrOut: &strings.Builder{},
			IsTTY:  false,
		},
	}

	cmd := NewCmdStatus(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := f.IOStreams.Out.(*strings.Builder).String()
	// Should show personal profile's server, not work's
	if !strings.Contains(output, "https://personal.example.com") {
		t.Errorf("output should contain personal server URL, got:\n%s", output)
	}
	if strings.Contains(output, "https://work.example.com") {
		t.Errorf("output should NOT contain work server URL, got:\n%s", output)
	}
	// Should show "personal" as the profile name
	if !strings.Contains(output, "personal") {
		t.Errorf("output should contain 'personal' profile name, got:\n%s", output)
	}
}

func TestStatus_NoActiveProfile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	// Empty profiles config (new format, no profiles defined)
	if err := os.WriteFile(cfgPath, []byte("profiles: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f := &cmdutil.Factory{
		ConfigPath: cfgPath,
		IOStreams: &cmdutil.IOStreams{
			In:     strings.NewReader(""),
			Out:    &strings.Builder{},
			ErrOut: &strings.Builder{},
			IsTTY:  false,
		},
	}

	cmd := NewCmdStatus(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error for empty profiles, got: %v", err)
	}

	output := f.IOStreams.ErrOut.(*strings.Builder).String()
	if !strings.Contains(output, noProfilesConfiguredMessage) {
		t.Errorf("expected %q warning in stderr, got:\n%s", noProfilesConfiguredMessage, output)
	}
}

func TestStatus_NonExistentConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "nonexistent.yaml")

	f := &cmdutil.Factory{
		ConfigPath: cfgPath,
		IOStreams: &cmdutil.IOStreams{
			In:     strings.NewReader(""),
			Out:    &strings.Builder{},
			ErrOut: &strings.Builder{},
			IsTTY:  false,
		},
	}

	cmd := NewCmdStatus(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)

	// Should not error, should show "no active profile" warning
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error for nonexistent config, got: %v", err)
	}

	output := f.IOStreams.ErrOut.(*strings.Builder).String()
	if !strings.Contains(output, noProfilesConfiguredMessage) {
		t.Errorf("expected %q warning in stderr, got:\n%s", noProfilesConfiguredMessage, output)
	}
}

func TestStatus_ProfileOverrideWithNonexistentProfile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	f := &cmdutil.Factory{
		ConfigPath:      cfgPath,
		ProfileOverride: "nonexistent",
		IOStreams: &cmdutil.IOStreams{
			In:     strings.NewReader(""),
			Out:    &strings.Builder{},
			ErrOut: &strings.Builder{},
			IsTTY:  false,
		},
	}

	cmd := NewCmdStatus(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent profile")
	}
	if got := err.Error(); got != profileNotFoundError("nonexistent").Error() {
		t.Errorf("expected %q, got %q", profileNotFoundError("nonexistent").Error(), got)
	}
}

func TestStatus_FallsBackToSoleProfile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	// Single profile but no active_profile set
	content := `profiles:
  only:
    server: https://only.example.com
    auth_method: apikey
    api_key: only-key
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	f := &cmdutil.Factory{
		ConfigPath: cfgPath,
		IOStreams: &cmdutil.IOStreams{
			In:     strings.NewReader(""),
			Out:    &strings.Builder{},
			ErrOut: &strings.Builder{},
			IsTTY:  false,
		},
	}

	cmd := NewCmdStatus(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := f.IOStreams.Out.(*strings.Builder).String()
	// Should show the sole profile's server
	if !strings.Contains(output, "https://only.example.com") {
		t.Errorf("output should contain sole profile's server URL, got:\n%s", output)
	}
	// Should show the profile name
	if !strings.Contains(output, "only") {
		t.Errorf("output should contain profile name 'only', got:\n%s", output)
	}
}

func TestStatus_UsesCLIOverridesWithoutActiveProfile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `profiles:
  work:
    server: https://work.example.com
    auth_method: apikey
    api_key: work-key
  personal:
    server: https://personal.example.com
    auth_method: apikey
    api_key: personal-key
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	f := &cmdutil.Factory{
		ConfigPath:     cfgPath,
		ServerOverride: "http://127.0.0.1:1",
		APIKeyOverride: "override-key",
		IOStreams: &cmdutil.IOStreams{
			In:     strings.NewReader(""),
			Out:    &strings.Builder{},
			ErrOut: &strings.Builder{},
			IsTTY:  false,
		},
	}

	cmd := NewCmdStatus(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := f.IOStreams.Out.(*strings.Builder).String()
	if !strings.Contains(output, "http://127.0.0.1:1") {
		t.Errorf("output should contain overridden server URL, got:\n%s", output)
	}
	if !strings.Contains(output, "(override)") {
		t.Errorf("output should identify override-based config, got:\n%s", output)
	}

	errOutput := f.IOStreams.ErrOut.(*strings.Builder).String()
	if strings.Contains(errOutput, noActiveProfileMessage) {
		t.Errorf("status should not warn about missing active profile when CLI overrides are provided, got:\n%s", errOutput)
	}
}

func TestStatus_ShowsEffectiveServerAfterEnvOverride(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    auth_method: apikey
    api_key: work-key
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("REDMINE_SERVER", "http://127.0.0.1:1")
	t.Setenv("REDMINE_API_KEY", "env-key")

	f := &cmdutil.Factory{
		ConfigPath: cfgPath,
		IOStreams: &cmdutil.IOStreams{
			In:     strings.NewReader(""),
			Out:    &strings.Builder{},
			ErrOut: &strings.Builder{},
			IsTTY:  false,
		},
	}

	cmd := NewCmdStatus(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := f.IOStreams.Out.(*strings.Builder).String()
	if !strings.Contains(output, "http://127.0.0.1:1") {
		t.Errorf("output should contain env-overridden server URL, got:\n%s", output)
	}
	if strings.Contains(output, "https://work.example.com") {
		t.Errorf("output should not contain stored profile server after env override, got:\n%s", output)
	}
}

func TestStatus_JSONMarksInactiveWhenUserProbeFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/current.json" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors":["invalid credentials"]}`))
	}))
	defer srv.Close()

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := "server: " + srv.URL + "\nauth_method: apikey\napi_key: bad-key\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	f := &cmdutil.Factory{
		ConfigPath:   cfgPath,
		OutputFormat: "json",
		IOStreams: &cmdutil.IOStreams{
			In:     strings.NewReader(""),
			Out:    &strings.Builder{},
			ErrOut: &strings.Builder{},
			IsTTY:  false,
		},
	}

	cmd := NewCmdStatus(f)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var payload struct {
		Active bool   `json:"active"`
		User   string `json:"user"`
		Server string `json:"server"`
	}
	if err := json.Unmarshal([]byte(f.IOStreams.Out.(*strings.Builder).String()), &payload); err != nil {
		t.Fatalf("expected JSON output, got: %v", err)
	}
	if payload.Active {
		t.Fatalf("expected active=false when current-user lookup fails, got %+v", payload)
	}
	if payload.User != "authentication failed" {
		t.Fatalf("user = %q, want %q", payload.User, "authentication failed")
	}
	if payload.Server != srv.URL {
		t.Fatalf("server = %q, want %q", payload.Server, srv.URL)
	}
}
