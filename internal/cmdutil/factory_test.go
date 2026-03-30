package cmdutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/config"
)

// writeConfigFile creates a temporary YAML config file and returns its path.
func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFactory_Config_CLIFlagOverridesServer(t *testing.T) {
	cfgPath := writeConfigFile(t, "server: https://file.example.com\napi_key: file-key\n")

	f := &Factory{
		ConfigPath:     cfgPath,
		ServerOverride: "https://flag.example.com",
		IOStreams:      &IOStreams{Out: os.Stdout, ErrOut: os.Stderr},
	}

	cfg, err := f.Config()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server != "https://flag.example.com" {
		t.Errorf("expected server from CLI flag, got %q", cfg.Server)
	}
	// API key should come from file when no override is set.
	if cfg.APIKey != "file-key" {
		t.Errorf("expected api_key from file, got %q", cfg.APIKey)
	}
}

func TestFactory_Config_CLIFlagOverridesAPIKey(t *testing.T) {
	cfgPath := writeConfigFile(t, "server: https://file.example.com\napi_key: file-key\n")

	f := &Factory{
		ConfigPath:     cfgPath,
		APIKeyOverride: "flag-key",
		IOStreams:      &IOStreams{Out: os.Stdout, ErrOut: os.Stderr},
	}

	cfg, err := f.Config()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.APIKey != "flag-key" {
		t.Errorf("expected api_key from CLI flag, got %q", cfg.APIKey)
	}
	// Server should come from file when no override is set.
	if cfg.Server != "https://file.example.com" {
		t.Errorf("expected server from file, got %q", cfg.Server)
	}
}

func TestFactory_Config_CLIFlagOverridesNoColor(t *testing.T) {
	cfgPath := writeConfigFile(t, "server: https://file.example.com\nno_color: false\n")

	f := &Factory{
		ConfigPath:      cfgPath,
		NoColorOverride: true,
		IOStreams:       &IOStreams{Out: os.Stdout, ErrOut: os.Stderr},
	}

	cfg, err := f.Config()
	if err != nil {
		t.Fatal(err)
	}

	if !cfg.NoColor {
		t.Error("expected no_color=true from CLI flag override")
	}
}

func TestFactory_Config_FileValuesUsedWithoutOverrides(t *testing.T) {
	cfgPath := writeConfigFile(t, "server: https://file.example.com\napi_key: file-key\nno_color: true\n")

	f := &Factory{
		ConfigPath: cfgPath,
		IOStreams:  &IOStreams{Out: os.Stdout, ErrOut: os.Stderr},
	}

	cfg, err := f.Config()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server != "https://file.example.com" {
		t.Errorf("expected server from file, got %q", cfg.Server)
	}
	if cfg.APIKey != "file-key" {
		t.Errorf("expected api_key from file, got %q", cfg.APIKey)
	}
	if !cfg.NoColor {
		t.Error("expected no_color=true from file")
	}
}

func TestFactory_Config_EnvOverridesFile(t *testing.T) {
	cfgPath := writeConfigFile(t, "server: https://file.example.com\napi_key: file-key\n")

	t.Setenv("REDMINE_SERVER", "https://env.example.com")
	t.Setenv("REDMINE_API_KEY", "env-key")

	f := &Factory{
		ConfigPath: cfgPath,
		IOStreams:  &IOStreams{Out: os.Stdout, ErrOut: os.Stderr},
	}

	cfg, err := f.Config()
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

func TestFactory_Config_CLIFlagOverridesEnv(t *testing.T) {
	cfgPath := writeConfigFile(t, "server: https://file.example.com\napi_key: file-key\n")

	t.Setenv("REDMINE_SERVER", "https://env.example.com")
	t.Setenv("REDMINE_API_KEY", "env-key")

	f := &Factory{
		ConfigPath:     cfgPath,
		ServerOverride: "https://flag.example.com",
		APIKeyOverride: "flag-key",
		IOStreams:      &IOStreams{Out: os.Stdout, ErrOut: os.Stderr},
	}

	cfg, err := f.Config()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Server != "https://flag.example.com" {
		t.Errorf("expected server from CLI flag (over env), got %q", cfg.Server)
	}
	if cfg.APIKey != "flag-key" {
		t.Errorf("expected api_key from CLI flag (over env), got %q", cfg.APIKey)
	}
}

func TestFactory_Config_NoGlobalViperMutation(t *testing.T) {
	// Verify that the factory does not depend on hidden global viper state.
	// Two independent factories with different overrides must resolve
	// independently without interfering with each other.
	cfgPath := writeConfigFile(t, "server: https://file.example.com\napi_key: file-key\n")

	f1 := &Factory{
		ConfigPath:     cfgPath,
		ServerOverride: "https://f1.example.com",
		APIKeyOverride: "f1-key",
		IOStreams:      &IOStreams{Out: os.Stdout, ErrOut: os.Stderr},
	}

	f2 := &Factory{
		ConfigPath:     cfgPath,
		ServerOverride: "https://f2.example.com",
		APIKeyOverride: "f2-key",
		IOStreams:      &IOStreams{Out: os.Stdout, ErrOut: os.Stderr},
	}

	cfg1, err := f1.Config()
	if err != nil {
		t.Fatal(err)
	}
	cfg2, err := f2.Config()
	if err != nil {
		t.Fatal(err)
	}

	if cfg1.Server != "https://f1.example.com" {
		t.Errorf("factory 1: expected server https://f1.example.com, got %q", cfg1.Server)
	}
	if cfg2.Server != "https://f2.example.com" {
		t.Errorf("factory 2: expected server https://f2.example.com, got %q", cfg2.Server)
	}
}

func TestFactory_Config_PrecedenceOrder(t *testing.T) {
	// Full precedence test: CLI flags > env vars > config file > defaults.
	// The config file sets all three sources for server and api_key.
	// The env sets a different value. The CLI flag sets yet another.
	// Expected: CLI flag wins.
	cfgPath := writeConfigFile(t, `
server: https://file.example.com
api_key: file-key
auth_method: basic
output_format: csv
page_size: 50
`)
	t.Setenv("REDMINE_SERVER", "https://env.example.com")
	t.Setenv("REDMINE_API_KEY", "env-key")

	f := &Factory{
		ConfigPath:     cfgPath,
		ServerOverride: "https://flag.example.com",
		APIKeyOverride: "flag-key",
		IOStreams:      &IOStreams{Out: os.Stdout, ErrOut: os.Stderr},
	}

	cfg, err := f.Config()
	if err != nil {
		t.Fatal(err)
	}

	// CLI flag overrides should win.
	if cfg.Server != "https://flag.example.com" {
		t.Errorf("server: expected CLI flag value, got %q", cfg.Server)
	}
	if cfg.APIKey != "flag-key" {
		t.Errorf("api_key: expected CLI flag value, got %q", cfg.APIKey)
	}
	// Non-overridden fields should retain file values.
	if cfg.AuthMethod != "basic" {
		t.Errorf("auth_method: expected file value 'basic', got %q", cfg.AuthMethod)
	}
	if cfg.OutputFormat != "csv" {
		t.Errorf("output_format: expected file value 'csv', got %q", cfg.OutputFormat)
	}
	if cfg.PageSize != 50 {
		t.Errorf("page_size: expected file value 50, got %d", cfg.PageSize)
	}
}

func TestFactory_Config_ConfigCached(t *testing.T) {
	cfgPath := writeConfigFile(t, "server: https://file.example.com\napi_key: file-key\n")

	f := &Factory{
		ConfigPath:     cfgPath,
		ServerOverride: "https://flag.example.com",
		IOStreams:      &IOStreams{Out: os.Stdout, ErrOut: os.Stderr},
	}

	cfg1, err := f.Config()
	if err != nil {
		t.Fatal(err)
	}
	cfg2, err := f.Config()
	if err != nil {
		t.Fatal(err)
	}

	if cfg1 != cfg2 {
		t.Error("expected Config() to return the same cached pointer on second call")
	}
}

// Ensure unused import doesn't cause issues. This is a compile-time check that
// config package is accessible for test helpers.
var _ = config.Config{}
