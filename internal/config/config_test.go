package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

func TestLoadLegacyFlatFormat(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("server: https://redmine.example.com\nauth_method: apikey\napi_key: test-key\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server != "https://redmine.example.com" {
		t.Fatalf("Server = %q, want %q", cfg.Server, "https://redmine.example.com")
	}
	if cfg.OutputFormat != "table" {
		t.Fatalf("OutputFormat = %q, want %q", cfg.OutputFormat, "table")
	}

	// Verify it was migrated to profile format
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "profiles:") {
		t.Fatal("expected legacy config to be migrated to profile format")
	}
}

func TestLoadProfileFormat(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    api_key: work-key
    auth_method: apikey
  personal:
    server: https://personal.example.com
    api_key: personal-key
    auth_method: apikey
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server != "https://work.example.com" {
		t.Fatalf("Server = %q, want %q", cfg.Server, "https://work.example.com")
	}
	if cfg.APIKey != "work-key" {
		t.Fatalf("APIKey = %q, want %q", cfg.APIKey, "work-key")
	}
}

func TestLoadProfileOverride(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
    api_key: work-key
  personal:
    server: https://personal.example.com
    api_key: personal-key
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath, "personal", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server != "https://personal.example.com" {
		t.Fatalf("Server = %q, want %q", cfg.Server, "https://personal.example.com")
	}
}

func TestLoadProfileNotFound(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: work
profiles:
  work:
    server: https://work.example.com
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath, "nonexistent", debug.New(nil))
	if err == nil {
		t.Fatal("expected error for nonexistent profile")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' in error, got %q", err.Error())
	}
}

func TestLoadDefaultsOutputFormat(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: test
profiles:
  test:
    server: https://redmine.example.com
    auth_method: apikey
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.OutputFormat != "table" {
		t.Fatalf("OutputFormat = %q, want %q", cfg.OutputFormat, "table")
	}
}

func TestSaveProfile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := &Config{
		Server:         "https://redmine.example.com",
		AuthMethod:     "apikey",
		APIKey:         "secret",
		DefaultProject: "demo",
		OutputFormat:   "json",
		NoColor:        true,
	}

	if err := SaveProfile("test", cfg, cfgPath); err != nil {
		t.Fatal(err)
	}

	// Verify the profile was saved
	pc, err := LoadProfiles(cfgPath, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := pc.Profiles["test"]; !ok {
		t.Fatal("expected profile 'test' to exist")
	}
	if pc.ActiveProfile != "test" {
		t.Fatalf("ActiveProfile = %q, want %q", pc.ActiveProfile, "test")
	}
	if pc.Profiles["test"].Server != "https://redmine.example.com" {
		t.Fatalf("Server = %q, want %q", pc.Profiles["test"].Server, "https://redmine.example.com")
	}
}

func TestDeleteProfile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: a
profiles:
  a:
    server: https://a.example.com
  b:
    server: https://b.example.com
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := DeleteProfile("a", cfgPath); err != nil {
		t.Fatal(err)
	}

	pc, err := LoadProfiles(cfgPath, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := pc.Profiles["a"]; ok {
		t.Fatal("expected profile 'a' to be deleted")
	}
	if pc.ActiveProfile != "b" {
		t.Fatalf("ActiveProfile = %q, want %q after deleting active", pc.ActiveProfile, "b")
	}
}

func TestSetActiveProfile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: a
profiles:
  a:
    server: https://a.example.com
  b:
    server: https://b.example.com
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SetActiveProfile("b", cfgPath); err != nil {
		t.Fatal(err)
	}

	pc, err := LoadProfiles(cfgPath, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}

	if pc.ActiveProfile != "b" {
		t.Fatalf("ActiveProfile = %q, want %q", pc.ActiveProfile, "b")
	}
}

func TestProfileNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://redmine.example.com", "redmine-example-com"},
		{"https://www.redmine.io", "redmine-io"},
		{"https://redmine.work.com:8080", "redmine-work-com"},
		{"", ""},
	}

	for _, tt := range tests {
		got := ProfileNameFromURL(tt.url)
		if got != tt.want {
			t.Errorf("ProfileNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: test
profiles:
  test:
    server: https://file.example.com
    api_key: file-key
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("REDMINE_SERVER", "https://env.example.com")
	t.Setenv("REDMINE_API_KEY", "env-key")

	cfg, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server != "https://env.example.com" {
		t.Errorf("expected server from env, got %q", cfg.Server)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("expected api_key from env, got %q", cfg.APIKey)
	}
}

func TestLoadNoActiveProfileWithEnvOverrides(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `profiles:
  a:
    server: https://a.example.com
  b:
    server: https://b.example.com
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("REDMINE_SERVER", "https://env.example.com")
	t.Setenv("REDMINE_API_KEY", "env-key")

	cfg, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatalf("Load with no active profile and env overrides failed: %v", err)
	}

	if cfg.Server != "https://env.example.com" {
		t.Errorf("expected server from env, got %q", cfg.Server)
	}
	if cfg.APIKey != "env-key" {
		t.Errorf("expected api_key from env, got %q", cfg.APIKey)
	}
}

func TestLoadNoActiveProfileNoEnvOverridesFails(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `profiles:
  a:
    server: https://a.example.com
  b:
    server: https://b.example.com
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Ensure no env overrides
	t.Setenv("REDMINE_SERVER", "")
	t.Setenv("REDMINE_API_KEY", "")

	_, err := Load(cfgPath, "", debug.New(nil))
	if err == nil {
		t.Fatal("expected error for no active profile without env overrides")
	}
	if !strings.Contains(err.Error(), "multiple profiles exist but no active profile") {
		t.Errorf("expected 'multiple profiles' error, got: %v", err)
	}
}

func TestLoadAllowNoActiveProfile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `profiles:
  a:
    server: https://a.example.com
  b:
    server: https://b.example.com
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAllowNoActiveProfile(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatalf("LoadAllowNoActiveProfile failed: %v", err)
	}

	if cfg.Server != "" {
		t.Fatalf("Server = %q, want empty string when no explicit credentials are provided", cfg.Server)
	}
	if cfg.AuthMethod != "apikey" {
		t.Fatalf("AuthMethod = %q, want %q", cfg.AuthMethod, "apikey")
	}
	if cfg.OutputFormat != "table" {
		t.Fatalf("OutputFormat = %q, want %q", cfg.OutputFormat, "table")
	}
}

func TestLoadSingleProfileNoActive(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `profiles:
  only:
    server: https://only.example.com
    api_key: only-key
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server != "https://only.example.com" {
		t.Fatalf("Server = %q, want %q", cfg.Server, "https://only.example.com")
	}
}

func TestLoadProfilesNonExistent(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "nonexistent.yaml")

	pc, err := LoadProfiles(cfgPath, debug.New(nil))
	if err != nil {
		t.Fatalf("LoadProfiles on nonexistent file returned error: %v", err)
	}

	if len(pc.Profiles) != 0 {
		t.Fatalf("Profiles count = %d, want 0", len(pc.Profiles))
	}
	if pc.ActiveProfile != "" {
		t.Fatalf("ActiveProfile = %q, want empty string", pc.ActiveProfile)
	}
}

func TestEffectiveProfileName(t *testing.T) {
	pc := &ProfileConfig{
		Profiles: map[string]Config{
			"only": {Server: "https://only.example.com"},
		},
	}

	if got := EffectiveProfileName(pc, ""); got != "only" {
		t.Fatalf("EffectiveProfileName(single profile) = %q, want %q", got, "only")
	}
	if got := EffectiveProfileName(pc, "override"); got != "override" {
		t.Fatalf("EffectiveProfileName(override) = %q, want %q", got, "override")
	}
}

func TestDeleteProfileActiveWithMultipleRemaining(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: a
profiles:
  a:
    server: https://a.example.com
  b:
    server: https://b.example.com
  c:
    server: https://c.example.com
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := DeleteProfile("a", cfgPath); err != nil {
		t.Fatal(err)
	}

	pc, err := LoadProfiles(cfgPath, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := pc.Profiles["a"]; ok {
		t.Fatal("profile 'a' should be deleted")
	}
	// ActiveProfile should be cleared, not set to a random remaining profile
	if pc.ActiveProfile != "" {
		t.Fatalf("ActiveProfile = %q, want empty string when multiple profiles remain after deleting active", pc.ActiveProfile)
	}
}

func TestDeleteLastProfileRemovesConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: only
profiles:
  only:
    server: https://only.example.com
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := DeleteProfile("only", cfgPath); err != nil {
		t.Fatal(err)
	}

	// Config file should be removed
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatal("expected config file to be removed after deleting last profile")
	}

	// LoadProfiles should return empty config, not error
	pc, err := LoadProfiles(cfgPath, debug.New(nil))
	if err != nil {
		t.Fatalf("LoadProfiles after last profile deleted returned error: %v", err)
	}
	if len(pc.Profiles) != 0 {
		t.Fatalf("Profiles count = %d, want 0", len(pc.Profiles))
	}
}

func TestDeleteProfileNonActivePreservesOthers(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := `active_profile: a
profiles:
  a:
    server: https://a.example.com
  b:
    server: https://b.example.com
  c:
    server: https://c.example.com
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := DeleteProfile("b", cfgPath); err != nil {
		t.Fatal(err)
	}

	pc, err := LoadProfiles(cfgPath, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := pc.Profiles["a"]; !ok {
		t.Fatal("profile 'a' should still exist")
	}
	if _, ok := pc.Profiles["c"]; !ok {
		t.Fatal("profile 'c' should still exist")
	}
	if pc.ActiveProfile != "a" {
		t.Fatalf("ActiveProfile = %q, want %q", pc.ActiveProfile, "a")
	}
}

func TestApplyEnvOverrides_MCPListsAndBool(t *testing.T) {
	t.Setenv("REDMINE_MCP_ENABLE_GROUPS", "issues, wiki ,projects")
	t.Setenv("REDMINE_MCP_DISABLE_TOOLS", "delete_issue,delete_project")
	t.Setenv("REDMINE_MCP_ENABLE_WRITES", "true")

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("server: https://example.com\napi_key: k\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	want := []string{"issues", "wiki", "projects"}
	if got := cfg.MCP.EnableGroups; len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("EnableGroups = %v, want %v", got, want)
	}
	if got := cfg.MCP.DisableTools; len(got) != 2 || got[0] != "delete_issue" || got[1] != "delete_project" {
		t.Errorf("DisableTools = %v", got)
	}
	if cfg.MCP.EnableWrites == nil || !*cfg.MCP.EnableWrites {
		t.Errorf("EnableWrites should be true (got %v)", cfg.MCP.EnableWrites)
	}
}

func TestApplyEnvOverrides_MCPAuthToken(t *testing.T) {
	t.Setenv("REDMINE_MCP_AUTH_TOKEN", "from-env")

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("server: https://example.com\napi_key: k\nmcp:\n  auth_token: from-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath, "", debug.New(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MCP.AuthToken != "from-env" {
		t.Errorf("AuthToken = %q, want from-env (env should override file)", cfg.MCP.AuthToken)
	}
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}

func TestSaveProfilesWritesOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not meaningful on Windows")
	}
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	pc := &ProfileConfig{
		ActiveProfile: "example-com",
		Profiles: map[string]Config{
			"example-com": {Server: "https://example.com", APIKey: "secret"},
		},
	}
	if err := SaveProfiles(pc, cfgPath); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}

	if got := fileMode(t, cfgPath); got != 0o600 {
		t.Errorf("new config mode = %o, want 600", got)
	}
}

func TestSaveProfilesTightensLegacyWorldReadableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not meaningful on Windows")
	}
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	if err := os.WriteFile(cfgPath, []byte("profiles:\n  example-com:\n    server: https://example.com\n    api_key: old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := fileMode(t, cfgPath); got != 0o644 {
		t.Fatalf("precondition mode = %o, want 644", got)
	}

	pc := &ProfileConfig{
		ActiveProfile: "example-com",
		Profiles: map[string]Config{
			"example-com": {Server: "https://example.com", APIKey: "new"},
		},
	}
	if err := SaveProfiles(pc, cfgPath); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}

	if got := fileMode(t, cfgPath); got != 0o600 {
		t.Errorf("overwritten legacy config mode = %o, want 600 (should be tightened)", got)
	}
}

func TestLoadProfilesTightensLegacyWorldReadableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not meaningful on Windows")
	}
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	if err := os.WriteFile(cfgPath, []byte("profiles:\n  example-com:\n    server: https://example.com\n    api_key: k\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadProfiles(cfgPath, debug.New(nil)); err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}

	if got := fileMode(t, cfgPath); got != 0o600 {
		t.Errorf("config mode after load = %o, want 600 (should be tightened on read)", got)
	}
}

func TestReadOnlyFromConfigFile(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	body := "profiles:\n  default:\n    server: https://x\n    api_key: k\n    read_only: true\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgPath, "default", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ReadOnly {
		t.Fatal("expected ReadOnly=true from profile read_only: true")
	}
}

func TestReadOnlyEnvDisablesConfig(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	body := "profiles:\n  default:\n    server: https://x\n    api_key: k\n    read_only: true\n"
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REDMINE_READ_ONLY", "false")
	cfg, err := Load(cfgPath, "default", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReadOnly {
		t.Fatal("REDMINE_READ_ONLY=false must override profile read_only: true")
	}
}

func TestReadOnlyEnvOverride(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("profiles:\n  default:\n    server: https://x\n    api_key: k\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REDMINE_READ_ONLY", "1")
	cfg, err := Load(cfgPath, "default", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ReadOnly {
		t.Fatal("expected ReadOnly=true from REDMINE_READ_ONLY=1")
	}
}
