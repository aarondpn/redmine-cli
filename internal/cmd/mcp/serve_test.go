package mcp

import (
	"bytes"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/mcpserver"
	"github.com/aarondpn/redmine-cli/v2/internal/secrets"
)

func TestNewCmdMCP_HasServe(t *testing.T) {
	cmd := NewCmdMCP(cmdutil.NewFactory())
	if cmd.Use != "mcp" {
		t.Errorf("Use = %q, want mcp", cmd.Use)
	}
	subcommands := map[string]bool{}
	for _, sub := range cmd.Commands() {
		subcommands[sub.Use] = true
	}
	for _, want := range []string{"serve", "tools"} {
		if !subcommands[want] {
			t.Errorf("mcp %s subcommand not registered", want)
		}
	}
}

func TestServeFlags_DefaultsToReadOnly(t *testing.T) {
	cmd := newCmdServe(cmdutil.NewFactory())

	enable, err := cmd.Flags().GetBool("enable-writes")
	if err != nil {
		t.Fatalf("GetBool: %v", err)
	}
	if enable {
		t.Error("--enable-writes defaulted to true; expected false")
	}

	name, err := cmd.Flags().GetString("name")
	if err != nil {
		t.Fatalf("GetString: %v", err)
	}
	if name != "redmine-cli" {
		t.Errorf("--name default = %q, want redmine-cli", name)
	}

	httpAddr, err := cmd.Flags().GetString("http")
	if err != nil {
		t.Fatalf("GetString(http): %v", err)
	}
	if httpAddr != "" {
		t.Errorf("--http default = %q, want empty string", httpAddr)
	}
}

func TestBuildFilterSpec_FlagsBeatConfig(t *testing.T) {
	cfg := &config.Config{MCP: config.MCPConfig{
		EnableGroups: []string{"issues"},
		DisableTools: []string{"delete_issue"},
	}}

	cmd := &cobra.Command{}
	cmd.Flags().StringSlice("enable-groups", nil, "")
	cmd.Flags().StringSlice("disable-groups", nil, "")
	cmd.Flags().StringSlice("enable-tools", nil, "")
	cmd.Flags().StringSlice("disable-tools", nil, "")

	if err := cmd.Flags().Set("enable-groups", "wiki,projects"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	enableGroups, _ := cmd.Flags().GetStringSlice("enable-groups")
	disableGroups, _ := cmd.Flags().GetStringSlice("disable-groups")
	enableTools, _ := cmd.Flags().GetStringSlice("enable-tools")
	disableTools, _ := cmd.Flags().GetStringSlice("disable-tools")

	spec, err := buildFilterSpec(cfg, cmd, enableGroups, disableGroups, enableTools, disableTools)
	if err != nil {
		t.Fatalf("buildFilterSpec: %v", err)
	}

	gotGroups := make([]string, 0, len(spec.EnableGroups))
	for _, g := range spec.EnableGroups {
		gotGroups = append(gotGroups, string(g))
	}
	sort.Strings(gotGroups)
	wantGroups := []string{"projects", "wiki"}
	if !reflect.DeepEqual(gotGroups, wantGroups) {
		t.Errorf("EnableGroups from flag = %v, want %v (config should be ignored)", gotGroups, wantGroups)
	}

	if !reflect.DeepEqual(spec.DisableTools, []string{"delete_issue"}) {
		t.Errorf("DisableTools should fall back to config when flag unset, got %v", spec.DisableTools)
	}
}

func TestBuildFilterSpec_ConfigUsedWhenFlagsAbsent(t *testing.T) {
	cfg := &config.Config{MCP: config.MCPConfig{
		EnableGroups: []string{"issues", "wiki"},
		EnableTools:  []string{"list_issues"},
	}}

	cmd := &cobra.Command{}
	cmd.Flags().StringSlice("enable-groups", nil, "")
	cmd.Flags().StringSlice("disable-groups", nil, "")
	cmd.Flags().StringSlice("enable-tools", nil, "")
	cmd.Flags().StringSlice("disable-tools", nil, "")

	spec, err := buildFilterSpec(cfg, cmd, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildFilterSpec: %v", err)
	}

	got := make([]mcpserver.Group, 0, len(spec.EnableGroups))
	got = append(got, spec.EnableGroups...)
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	want := []mcpserver.Group{mcpserver.GroupIssues, mcpserver.GroupWiki}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("EnableGroups = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(spec.EnableTools, []string{"list_issues"}) {
		t.Errorf("EnableTools = %v, want [list_issues]", spec.EnableTools)
	}
}

func TestResolveEnableWrites(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("enable-writes", false, "")

	tr := true
	cfg := &config.Config{MCP: config.MCPConfig{EnableWrites: &tr}}

	if !resolveEnableWrites(cmd, false, cfg) {
		t.Error("config should win when flag is unset")
	}

	if err := cmd.Flags().Set("enable-writes", "false"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if resolveEnableWrites(cmd, false, cfg) {
		t.Error("explicit flag should override config")
	}
}

func TestResolveAuthToken(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("auth-token", "", "")

	cfg := &config.Config{MCP: config.MCPConfig{AuthToken: "from-config"}}

	if got := resolveAuthToken(cmd, "", cfg); got != "from-config" {
		t.Errorf("config token should win when flag is unset, got %q", got)
	}

	if err := cmd.Flags().Set("auth-token", "from-flag"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := resolveAuthToken(cmd, "from-flag", cfg); got != "from-flag" {
		t.Errorf("explicit flag should override config, got %q", got)
	}

	cmd2 := &cobra.Command{}
	cmd2.Flags().String("auth-token", "", "")
	if err := cmd2.Flags().Set("auth-token", ""); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := resolveAuthToken(cmd2, "", cfg); got != "" {
		t.Errorf("explicitly empty flag should suppress config, got %q", got)
	}
}

func TestBuildFilterSpec_RejectsUnknownTool(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().StringSlice("enable-groups", nil, "")
	cmd.Flags().StringSlice("disable-groups", nil, "")
	cmd.Flags().StringSlice("enable-tools", nil, "")
	cmd.Flags().StringSlice("disable-tools", nil, "")

	if err := cmd.Flags().Set("enable-tools", "list_issues,not_a_real_tool"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	enableGroups, _ := cmd.Flags().GetStringSlice("enable-groups")
	disableGroups, _ := cmd.Flags().GetStringSlice("disable-groups")
	enableTools, _ := cmd.Flags().GetStringSlice("enable-tools")
	disableTools, _ := cmd.Flags().GetStringSlice("disable-tools")

	_, err := buildFilterSpec(nil, cmd, enableGroups, disableGroups, enableTools, disableTools)
	if err == nil {
		t.Fatal("buildFilterSpec should reject unknown tool names")
	}
	if !strings.Contains(err.Error(), `unknown tool "not_a_real_tool"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServeHelp_EnableGroupsUsageRendersNormally(t *testing.T) {
	cmd := newCmdServe(cmdutil.NewFactory())
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Help(); err != nil {
		t.Fatalf("Help: %v", err)
	}

	help := out.String()
	if strings.Contains(help, "--enable-groups redmine mcp tools") {
		t.Fatalf("help output used command text as value placeholder:\n%s", help)
	}
	if !strings.Contains(help, "--enable-groups strings") {
		t.Fatalf("help output missing normal StringSlice placeholder:\n%s", help)
	}
}

func TestServeHelp_AuthTokenHintsAtSaferChannels(t *testing.T) {
	usage := newCmdServe(cmdutil.NewFactory()).Flags().Lookup("auth-token").Usage
	if !strings.Contains(usage, "prefer --auth-token-file, REDMINE_MCP_AUTH_TOKEN, or mcp.auth_token") {
		t.Fatalf("--auth-token help should point at --auth-token-file, REDMINE_MCP_AUTH_TOKEN and mcp.auth_token: %q", usage)
	}
}

func newHTTPAuthCmd() (*cobra.Command, *bytes.Buffer) {
	cmd := &cobra.Command{}
	cmd.Flags().String("http", "", "")
	cmd.Flags().String("auth-token", "", "")
	cmd.Flags().String("auth-token-file", "", "")
	cmd.Flags().Bool("no-auth", false, "")
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	return cmd, &stderr
}

// newHTTPAuthFactory returns a Factory whose streams are the command's, so
// Warn output lands in the same buffer as cmd.ErrOrStderr().
func newHTTPAuthFactory(cmd *cobra.Command) *cmdutil.Factory {
	return &cmdutil.Factory{IOStreams: &cmdutil.IOStreams{
		In:     cmd.InOrStdin(),
		Out:    cmd.OutOrStdout(),
		ErrOut: cmd.ErrOrStderr(),
	}}
}

func TestResolveHTTPAddr(t *testing.T) {
	cfg := &config.Config{MCP: config.MCPConfig{HTTP: "0.0.0.0:8080"}}

	cmd, _ := newHTTPAuthCmd()
	if got := resolveHTTPAddr(cmd, "", cfg); got != "0.0.0.0:8080" {
		t.Errorf("config address should win when flag is unset, got %q", got)
	}
	if got := resolveHTTPAddr(cmd, "", nil); got != "" {
		t.Errorf("no flag and no config should mean stdio, got %q", got)
	}

	if err := cmd.Flags().Set("http", ":9090"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := resolveHTTPAddr(cmd, ":9090", cfg); got != ":9090" {
		t.Errorf("explicit flag should override config, got %q", got)
	}
}

func TestResolveHTTPAuth_LoopbackWithoutTokenStaysOpen(t *testing.T) {
	cmd, stderr := newHTTPAuthCmd()

	token, generated, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, nil, true, httpAuthFlags{})
	if err != nil {
		t.Fatalf("resolveHTTPAuth: %v", err)
	}
	if token != "" || generated {
		t.Errorf("loopback bind should not get a generated token, got %q", token)
	}
	if !strings.Contains(stderr.String(), "warning: serving MCP over HTTP on loopback without an auth token") {
		t.Errorf("expected a loopback warning on stderr, got %q", stderr.String())
	}
}

func TestResolveHTTPAuth_LoopbackWithTokenOrNoAuthIsSilent(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
	}{
		{"configured token", &config.Config{MCP: config.MCPConfig{AuthToken: "from-config"}}},
		{"explicit no-auth", &config.Config{MCP: config.MCPConfig{NoAuth: true}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, stderr := newHTTPAuthCmd()
			token, generated, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, tt.cfg, true, httpAuthFlags{})
			if err != nil {
				t.Fatalf("resolveHTTPAuth: %v", err)
			}
			if generated || token != tt.cfg.MCP.AuthToken {
				t.Errorf("token = %q (generated=%v), want %q", token, generated, tt.cfg.MCP.AuthToken)
			}
			if stderr.Len() != 0 {
				t.Errorf("unexpected stderr output: %q", stderr.String())
			}
		})
	}
}

// TestResolveHTTPAuth_AuthTokenFlag pins that a token on the command line is
// used as is, without a warning (like --api-key); the flag help points at
// the channels that keep it out of the process list.
func TestResolveHTTPAuth_AuthTokenFlag(t *testing.T) {
	cmd, stderr := newHTTPAuthCmd()
	if err := cmd.Flags().Set("auth-token", "from-flag"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	token, generated, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, nil, false, httpAuthFlags{Token: "from-flag"})
	if err != nil {
		t.Fatalf("resolveHTTPAuth: %v", err)
	}
	if token != "from-flag" || generated {
		t.Errorf("token = %q (generated=%v), want from-flag", token, generated)
	}
	if stderr.Len() != 0 {
		t.Errorf("unexpected stderr output: %q", stderr.String())
	}
}

func TestResolveHTTPAuth_AuthTokenFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	good := write("token", "  file-token\n")

	tests := []struct {
		name      string
		path      string
		stdin     string
		cfg       *config.Config
		want      string
		wantError string
	}{
		{name: "file, trailing newline trimmed", path: good, want: "file-token"},
		{name: "stdin", path: "-", stdin: "stdin-token\n", want: "stdin-token"},
		{name: "file beats config token", path: good, cfg: &config.Config{MCP: config.MCPConfig{AuthToken: "from-config"}}, want: "file-token"},
		{name: "empty file", path: write("empty", "\n\n"), wantError: "is empty"},
		{name: "empty stdin", path: "-", wantError: "standard input is empty"},
		{name: "whitespace inside", path: write("two", "a b\n"), wantError: "must not contain whitespace"},
		{name: "missing file", path: filepath.Join(dir, "missing"), wantError: "--auth-token-file"},
		{name: "empty path", path: "", wantError: "file path is empty"},
		{name: "too large", path: write("big", strings.Repeat("a", 64<<10+1)), wantError: "larger than"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, stderr := newHTTPAuthCmd()
			cmd.SetIn(strings.NewReader(tt.stdin))
			if err := cmd.Flags().Set("auth-token-file", tt.path); err != nil {
				t.Fatalf("Set: %v", err)
			}
			token, generated, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, tt.cfg, false, httpAuthFlags{TokenFile: tt.path})
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("err = %v, want it to contain %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveHTTPAuth: %v", err)
			}
			if token != tt.want || generated {
				t.Errorf("token = %q (generated=%v), want %q", token, generated, tt.want)
			}
			if stderr.Len() != 0 {
				t.Errorf("unexpected stderr output: %q", stderr.String())
			}
		})
	}
}

func TestServeFlags_AuthTokenAndFileAreMutuallyExclusive(t *testing.T) {
	cmd := newCmdServe(cmdutil.NewFactory())
	cmd.SetArgs([]string{"--auth-token", "x", "--auth-token-file", "-"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "auth-token") {
		t.Fatalf("err = %v, want a mutually-exclusive flag error", err)
	}
}

func TestResolveHTTPAuth_NonLoopbackGeneratesToken(t *testing.T) {
	cmd, stderr := newHTTPAuthCmd()

	token, generated, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, nil, false, httpAuthFlags{})
	if err != nil {
		t.Fatalf("resolveHTTPAuth: %v", err)
	}
	if !generated || len(token) != 64 {
		t.Fatalf("generated token = %q (generated=%v), want 64 hex chars", token, generated)
	}
	if stderr.Len() != 0 {
		t.Errorf("token must only be printed after the listener is up, got %q", stderr.String())
	}

	var out bytes.Buffer
	printGeneratedToken(&out, token)
	if !strings.Contains(out.String(), token) {
		t.Errorf("printGeneratedToken output misses the token: %q", out.String())
	}

	again, _, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, nil, false, httpAuthFlags{})
	if err != nil {
		t.Fatalf("resolveHTTPAuth: %v", err)
	}
	if again == token {
		t.Error("generated tokens should differ between runs")
	}
}

func TestResolveHTTPAuth_ConfiguredTokenIsKept(t *testing.T) {
	cmd, stderr := newHTTPAuthCmd()
	cfg := &config.Config{MCP: config.MCPConfig{AuthToken: "from-config"}}

	token, generated, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, cfg, false, httpAuthFlags{})
	if err != nil {
		t.Fatalf("resolveHTTPAuth: %v", err)
	}
	if token != "from-config" || generated {
		t.Errorf("token = %q, want from-config", token)
	}
	if stderr.Len() != 0 {
		t.Errorf("configured token should not be echoed, got %q", stderr.String())
	}
}

// loadKeyringProfile writes a keyring profile whose API key and MCP token
// live in the mocked keyring and loads it like the CLI does.
func loadKeyringProfile(t *testing.T) *config.Config {
	t.Helper()
	keyring.MockInit()
	if err := secrets.Default.Set("serve-id", secrets.FieldAPIKey, "kr-key"); err != nil {
		t.Fatal(err)
	}
	if err := secrets.Default.Set("serve-id", secrets.FieldMCPToken, "keyring-token"); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := "profiles:\n  work:\n    server: https://work.example.com\n    auth_method: apikey\n    credential_store: keyring\n    keyring_id: serve-id\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// TestResolveHTTPAuth_KeyringToken pins the lazy lookup: config loading leaves
// a keyring profile's mcp.auth_token in the keyring, and serve --http reads it
// only when no flag, file, environment variable or YAML value supplies one.
func TestResolveHTTPAuth_KeyringToken(t *testing.T) {
	t.Run("used when nothing else is set", func(t *testing.T) {
		cfg := loadKeyringProfile(t)
		if cfg.MCP.AuthToken != "" {
			t.Fatalf("Load resolved MCP.AuthToken = %q, want it left in the keyring", cfg.MCP.AuthToken)
		}
		cmd, stderr := newHTTPAuthCmd()
		token, generated, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, cfg, false, httpAuthFlags{})
		if err != nil {
			t.Fatalf("resolveHTTPAuth: %v", err)
		}
		if token != "keyring-token" || generated {
			t.Errorf("token = %q (generated=%v), want the keyring token", token, generated)
		}
		if stderr.Len() != 0 {
			t.Errorf("unexpected stderr output: %q", stderr.String())
		}
	})

	t.Run("also on loopback", func(t *testing.T) {
		cfg := loadKeyringProfile(t)
		cmd, stderr := newHTTPAuthCmd()
		token, generated, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, cfg, true, httpAuthFlags{})
		if err != nil {
			t.Fatalf("resolveHTTPAuth: %v", err)
		}
		if token != "keyring-token" || generated {
			t.Errorf("token = %q (generated=%v), want the keyring token", token, generated)
		}
		if stderr.Len() != 0 {
			t.Errorf("unexpected stderr output: %q", stderr.String())
		}
	})

	t.Run("keyring failure refuses to serve without the token", func(t *testing.T) {
		for _, loopback := range []bool{true, false} {
			cfg := loadKeyringProfile(t)
			keyring.MockInitWithError(errors.New("keychain locked"))
			cmd, stderr := newHTTPAuthCmd()
			token, generated, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, cfg, loopback, httpAuthFlags{})
			if err == nil || !strings.Contains(err.Error(), "keychain locked") || !strings.Contains(err.Error(), "--no-auth") {
				t.Fatalf("loopback=%v: err = %v, want the keyring error with the --no-auth hint", loopback, err)
			}
			if token != "" || generated {
				t.Fatalf("loopback=%v: token = %q (generated=%v), want none on error", loopback, token, generated)
			}
			if stderr.Len() != 0 {
				t.Fatalf("loopback=%v: unexpected stderr output: %q", loopback, stderr.String())
			}
		}
	})

	t.Run("REDMINE_NO_KEYRING refuses to serve without the token", func(t *testing.T) {
		cfg := loadKeyringProfile(t)
		t.Setenv("REDMINE_NO_KEYRING", "1")
		cmd, _ := newHTTPAuthCmd()
		token, generated, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, cfg, true, httpAuthFlags{})
		if err == nil || !strings.Contains(err.Error(), "REDMINE_NO_KEYRING") || !strings.Contains(err.Error(), "--no-auth") {
			t.Fatalf("err = %v, want the disabled-keyring error with the --no-auth hint", err)
		}
		if token != "" || generated {
			t.Fatalf("token = %q (generated=%v), want none on error", token, generated)
		}
	})

	t.Run("keyring failure with --no-auth serves without a token", func(t *testing.T) {
		cfg := loadKeyringProfile(t)
		keyring.MockInitWithError(errors.New("keychain locked"))
		cmd, _ := newHTTPAuthCmd()
		if err := cmd.Flags().Set("no-auth", "true"); err != nil {
			t.Fatal(err)
		}
		token, generated, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, cfg, true, httpAuthFlags{NoAuth: true})
		if err != nil || token != "" || generated {
			t.Fatalf("token = %q (generated=%v), err = %v; want an explicit --no-auth to serve", token, generated, err)
		}
	})

	t.Run("token file wins", func(t *testing.T) {
		cfg := loadKeyringProfile(t)
		cmd, _ := newHTTPAuthCmd()
		cmd.SetIn(strings.NewReader("file-token\n"))
		if err := cmd.Flags().Set("auth-token-file", "-"); err != nil {
			t.Fatal(err)
		}
		token, _, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, cfg, false, httpAuthFlags{TokenFile: "-"})
		if err != nil {
			t.Fatalf("resolveHTTPAuth: %v", err)
		}
		if token != "file-token" {
			t.Errorf("token = %q, want file-token", token)
		}
	})

	t.Run("explicitly empty flag skips it", func(t *testing.T) {
		cfg := loadKeyringProfile(t)
		cmd, _ := newHTTPAuthCmd()
		if err := cmd.Flags().Set("auth-token", ""); err != nil {
			t.Fatal(err)
		}
		token, generated, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, cfg, false, httpAuthFlags{})
		if err != nil {
			t.Fatalf("resolveHTTPAuth: %v", err)
		}
		if token == "keyring-token" || !generated {
			t.Errorf("token = %q (generated=%v), want a generated token", token, generated)
		}
	})
}

func TestResolveHTTPAuth_NoAuthOptOut(t *testing.T) {
	cmd, stderr := newHTTPAuthCmd()
	cfg := &config.Config{MCP: config.MCPConfig{NoAuth: true}}

	token, _, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, cfg, false, httpAuthFlags{})
	if err != nil {
		t.Fatalf("resolveHTTPAuth: %v", err)
	}
	if token != "" {
		t.Errorf("--no-auth should leave the server unauthenticated, got token %q", token)
	}
	if !strings.Contains(stderr.String(), "without authentication") {
		t.Errorf("expected a warning on stderr, got %q", stderr.String())
	}
}

// A non-loopback --no-auth flag serves unauthenticated, with a warning.
func TestResolveHTTPAuth_NoAuthFlagNonLoopback(t *testing.T) {
	cmd, stderr := newHTTPAuthCmd()
	if err := cmd.Flags().Set("no-auth", "true"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	token, generated, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, nil, false, httpAuthFlags{NoAuth: true})
	if err != nil {
		t.Fatalf("resolveHTTPAuth: %v", err)
	}
	if token != "" || generated {
		t.Errorf("token = %q (generated=%v), want none", token, generated)
	}
	if want := "warning: " + publicNoAuthWarning + "\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestResolveHTTPAuth_NoAuthWarnsOnce(t *testing.T) {
	cmd, stderr := newHTTPAuthCmd()
	f := newHTTPAuthFactory(cmd)
	cfg := &config.Config{MCP: config.MCPConfig{NoAuth: true}}
	for range 2 {
		if _, _, err := resolveHTTPAuth(f, cmd, cfg, false, httpAuthFlags{}); err != nil {
			t.Fatalf("resolveHTTPAuth: %v", err)
		}
	}
	want := "warning: " + publicNoAuthWarning + "\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want the warning exactly once: %q", stderr.String(), want)
	}
}

func TestResolveHTTPAuth_NoAuthConflictsWithToken(t *testing.T) {
	cmd, _ := newHTTPAuthCmd()
	if err := cmd.Flags().Set("no-auth", "true"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	cfg := &config.Config{MCP: config.MCPConfig{AuthToken: "from-config"}}

	if _, _, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, cfg, false, httpAuthFlags{NoAuth: true}); err == nil {
		t.Fatal("--no-auth combined with a token should be rejected")
	}
}

func TestResolveHTTPAuth_ExplicitNoAuthFalseBeatsConfig(t *testing.T) {
	cmd, _ := newHTTPAuthCmd()
	if err := cmd.Flags().Set("no-auth", "false"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	cfg := &config.Config{MCP: config.MCPConfig{NoAuth: true}}

	token, generated, err := resolveHTTPAuth(newHTTPAuthFactory(cmd), cmd, cfg, false, httpAuthFlags{})
	if err != nil {
		t.Fatalf("resolveHTTPAuth: %v", err)
	}
	if !generated || token == "" {
		t.Error("--no-auth=false should override mcp.no_auth and generate a token")
	}
}

func TestResolveHTTPAddr_ExplicitEmptyFlagMeansStdio(t *testing.T) {
	cmd, _ := newHTTPAuthCmd()
	if err := cmd.Flags().Set("http", ""); err != nil {
		t.Fatalf("Set: %v", err)
	}
	cfg := &config.Config{MCP: config.MCPConfig{HTTP: "0.0.0.0:8080"}}

	if got := resolveHTTPAddr(cmd, "", cfg); got != "" {
		t.Errorf("explicit --http \"\" should force stdio, got %q", got)
	}
}

func TestListenAddr(t *testing.T) {
	bound := &net.TCPAddr{IP: net.IPv6unspecified, Port: 41234}
	if got := listenAddr("0.0.0.0:0", bound); got != "0.0.0.0:41234" {
		t.Errorf("listenAddr = %q, want 0.0.0.0:41234", got)
	}
	if got := listenAddr("127.0.0.1:8080", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}); got != "127.0.0.1:8080" {
		t.Errorf("listenAddr = %q, want 127.0.0.1:8080", got)
	}
}
