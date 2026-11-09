//go:build unix

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/runner"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
	"github.com/aarondpn/redmine-cli/v2/internal/mcpserver"
)

// echoScript is the executable of the test extensions: it prints its stdin
// and protocol environment as one JSON object.
const echoScript = `#!/bin/sh
set -u
esc() {
	printf '%s' "$1" | awk 'BEGIN { ORS = "" } { gsub(/\\/, "\\\\"); gsub(/"/, "\\\""); if (NR > 1) print "\\n"; print }'
}
input=$(cat)
printf '{"stdin":%s,"env":{' "${input:-null}"
sep=""
for k in REDMINE_EXT_NAME REDMINE_EXT_COMMAND REDMINE_EXT_SURFACE REDMINE_MCP REDMINE_MCP_TOOL \
	REDMINE_PROFILE REDMINE_CONFIG REDMINE_SERVER REDMINE_HEADERS REDMINE_READ_ONLY REDMINE_NO_UPDATE_CHECK; do
	eval "isset=\${$k+x}"
	if [ -n "$isset" ]; then
		eval "v=\$$k"
		printf '%s"%s":"%s"' "$sep" "$k" "$(esc "$v")"
		sep=","
	fi
done
printf '}}\n'
`

const echoManifest = `schema_version: 1
name: echo
description: Echo test extension
commands:
  - name: show
    description: Echo the input
    params:
      - { name: issue_id, type: integer, required: true, positional: true }
  - name: write
    description: Write something
    writes: true
  - name: cli-only
    description: Not a tool
    mcp: false
`

// clashManifest gives extension "list" a command whose tool name is the
// core tool list_issues.
const clashManifest = `schema_version: 1
name: list
description: Clashing test extension
commands:
  - name: issues
    description: Clashes with a core tool
  - name: things
    description: Does not clash
`

// writeExtension creates the extension name below root, with manifest
// (none when empty) and, when tag is set, the install metadata of a binary
// release.
func writeExtension(t *testing.T, root, name, manifest, tag string) {
	t.Helper()
	dir := filepath.Join(root, extmgr.Prefix+name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, extmgr.Prefix+name), []byte(echoScript), 0o755); err != nil {
		t.Fatal(err)
	}
	if manifest != "" {
		if err := os.WriteFile(filepath.Join(dir, "redmine-extension.yml"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if tag != "" {
		info := "owner: acme\nrepo: redmine-cli-" + name + "\ntag: " + tag + "\npinned: false\ninstalled_at: 2026-01-01T00:00:00Z\nkind: binary\n"
		if err := os.WriteFile(filepath.Join(dir, extmgr.InstallFile), []byte(info), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// testExtensions installs the echo extension (v1.2.0) and returns Options
// listing the extensions below a fresh root. extra adds more extensions as
// name -> manifest.
func testExtensions(t *testing.T, extra map[string]string) Options {
	t.Helper()
	root := t.TempDir()
	writeExtension(t, root, "echo", echoManifest, "v1.2.0")
	for name, manifest := range extra {
		writeExtension(t, root, name, manifest, "")
	}
	m := &extmgr.Manager{Root: root, CLIVersion: "dev"}
	return Options{Extensions: m.List}
}

// serveConfig is a profile pointing at a closed port: the server starts,
// and nothing in these tests needs Redmine itself.
const serveConfig = "active_profile: work\nprofiles:\n  work:\n    server: http://127.0.0.1:1\n    api_key: test\n    auth_method: apikey\n"

type serveRun struct {
	stderr *lockedBuffer
	cfg    string
}

// startServe runs `mcp serve args...` with opts in the background, over an
// in-memory transport unless args ask for HTTP, and returns a connected
// client session (nil for HTTP). The server stops when the test ends.
func startServe(t *testing.T, opts Options, content string, env map[string]string, setup func(*cmdutil.Factory), args ...string) (*sdk.ClientSession, *serveRun) {
	t.Helper()
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, "REDMINE_") {
			t.Setenv(k, "")
			_ = os.Unsetenv(k)
		}
	}
	t.Setenv("REDMINE_NO_KEYRING", "1")
	for k, v := range env {
		t.Setenv(k, v)
	}
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	ct, st := sdk.NewInMemoryTransports()
	orig := newStdioTransport
	t.Cleanup(func() { newStdioTransport = orig })
	newStdioTransport = func() sdk.Transport { return st }

	run := &serveRun{stderr: &lockedBuffer{}, cfg: cfgPath}
	f := &cmdutil.Factory{
		ConfigPath: cfgPath,
		IOStreams:  &cmdutil.IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: run.stderr},
	}
	if setup != nil {
		setup(f)
	}
	cmd := newCmdServe(f, opts)
	cmd.SetArgs(args)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(run.stderr)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- cmd.ExecuteContext(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("serve did not stop")
		}
	})

	if httpMode(args) {
		return nil, run
	}
	client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "v0"}, nil)
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer connectCancel()
	cs, err := client.Connect(connectCtx, ct, nil)
	if err != nil {
		select {
		case serveErr := <-done:
			t.Fatalf("serve failed: %v; stderr:\n%s", serveErr, run.stderr.String())
		default:
		}
		t.Fatalf("client Connect: %v; stderr:\n%s", err, run.stderr.String())
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs, run
}

func httpMode(args []string) bool {
	for _, a := range args {
		if a == "--http" || strings.HasPrefix(a, "--http=") {
			return true
		}
	}
	return false
}

func toolNames(t *testing.T, cs *sdk.ClientSession) []string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

func hasName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

func TestServe_ExtensionToolsEnablement(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		config  string
		want    []string
		missing []string
	}{
		{name: "stdio default on", want: []string{"echo_show", "list_issues"}, missing: []string{"echo_write", "echo_cli_only"}},
		{name: "writes need --enable-writes", args: []string{"--enable-writes"}, want: []string{"echo_show", "echo_write"}},
		{name: "explicit flag false", args: []string{"--enable-extensions=false"}, want: []string{"list_issues"}, missing: []string{"echo_show"}},
		{name: "env false", env: map[string]string{config.MCPEnableExtensionsEnv: "0"}, missing: []string{"echo_show"}},
		{name: "config false", config: "    mcp:\n      enable_extensions: false\n", missing: []string{"echo_show"}},
		{name: "flag beats config", args: []string{"--enable-extensions"}, config: "    mcp:\n      enable_extensions: false\n", want: []string{"echo_show"}},
		{name: "core groups only", args: []string{"--enable-groups", "issues"}, want: []string{"list_issues"}, missing: []string{"echo_show"}},
		{name: "extension group only", args: []string{"--enable-groups", "ext-echo"}, want: []string{"echo_show"}, missing: []string{"list_issues"}},
		{name: "umbrella group disabled", args: []string{"--disable-groups", "extensions"}, want: []string{"list_issues"}, missing: []string{"echo_show"}},
		{name: "tool deny-list", args: []string{"--disable-tools", "echo_show"}, missing: []string{"echo_show"}},
		{name: "tool allow-list", args: []string{"--enable-tools", "echo_show"}, want: []string{"echo_show"}, missing: []string{"list_issues"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs, run := startServe(t, testExtensions(t, nil), serveConfig+tt.config, tt.env, nil, tt.args...)
			names := toolNames(t, cs)
			for _, n := range tt.want {
				if !hasName(names, n) {
					t.Errorf("tool %q missing; stderr:\n%s", n, run.stderr.String())
				}
			}
			for _, n := range tt.missing {
				if hasName(names, n) {
					t.Errorf("tool %q must not be registered", n)
				}
			}
		})
	}
}

func TestServe_ExtensionNameClashAndBrokenExtension(t *testing.T) {
	opts := testExtensions(t, map[string]string{
		"list":   clashManifest,
		"broken": "schema_version: 1\nname: other\n",
	})
	cs, run := startServe(t, opts, serveConfig, nil, nil)
	names := toolNames(t, cs)
	if !hasName(names, "list_things") || !hasName(names, "echo_show") {
		t.Errorf("tools = %v, want the non-clashing extension tools", names)
	}
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name == "list_issues" && strings.Contains(tool.Description, "Clashes") {
			t.Error("the extension replaced the core list_issues tool")
		}
	}
	stderr := run.stderr.String()
	for _, want := range []string{
		"warning: extension list: skipping MCP tool list_issues (command issues): a core tool has the same name",
		"warning: extension broken is unavailable and provides no MCP tools",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr misses %q:\n%s", want, stderr)
		}
	}
}

func TestServe_ExtensionEnvForwarding(t *testing.T) {
	var cfgPath string
	cs, run := startServe(t, testExtensions(t, nil), serveConfig,
		map[string]string{config.HeadersEnv: "X-From-Env: 1"},
		func(f *cmdutil.Factory) {
			cfgPath = f.ConfigPath
			f.ProfileOverride = "work"
			f.ServerOverride = "http://127.0.0.1:2"
			f.Headers = []string{"X-From-Flag: 2"}
			f.APIKeyOverride = "never-forwarded"
		}, "--enable-writes")

	call := func(tool string, args map[string]any) map[string]string {
		t.Helper()
		res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: tool, Arguments: args})
		if err != nil {
			t.Fatalf("CallTool: %v", err)
		}
		if res.IsError {
			t.Fatalf("tool error: %+v", res.Content)
		}
		var out struct {
			Env map[string]string `json:"env"`
		}
		text := res.Content[0].(*sdk.TextContent).Text
		if err := json.Unmarshal([]byte(text), &out); err != nil {
			t.Fatalf("decoding %q: %v", text, err)
		}
		if strings.Contains(text, "never-forwarded") {
			t.Errorf("the --api-key value reached the extension: %s", text)
		}
		return out.Env
	}

	env := call("echo_show", map[string]any{"issue_id": 7})
	absCfg, _ := filepath.Abs(cfgPath)
	want := map[string]string{
		runner.EnvExtName:       "echo",
		runner.EnvExtCommand:    "show",
		runner.EnvExtSurface:    "mcp",
		runner.EnvMCP:           "1",
		runner.EnvMCPTool:       "echo_show",
		runner.EnvProfile:       "work",
		runner.EnvConfig:        absCfg,
		runner.EnvServer:        "http://127.0.0.1:2",
		runner.EnvHeaders:       "X-From-Env: 1\nX-From-Flag: 2",
		runner.EnvReadOnly:      "1",
		runner.EnvNoUpdateCheck: "1",
	}
	for k, v := range want {
		if env[k] != v {
			t.Errorf("%s = %q, want %q", k, env[k], v)
		}
	}

	// A writing tool is not forced read-only.
	if ro, set := call("echo_write", nil)[runner.EnvReadOnly]; set {
		t.Errorf("echo_write got REDMINE_READ_ONLY=%q, want it unset", ro)
	}
	if got := run.stderr.String(); strings.Count(got, apiKeyNotForwardedWarning) != 1 {
		t.Errorf("want the --api-key warning exactly once, stderr:\n%s", got)
	}
}

// TestServe_ReadOnlyServer pins that a server in read-only mode runs even
// writing extension tools read-only.
func TestServe_ReadOnlyServer(t *testing.T) {
	cs, _ := startServe(t, testExtensions(t, nil), serveConfig, map[string]string{"REDMINE_READ_ONLY": "1"}, nil, "--enable-writes")
	res, err := cs.CallTool(context.Background(), &sdk.CallToolParams{Name: "echo_write"})
	if err != nil {
		t.Fatal(err)
	}
	if text := res.Content[0].(*sdk.TextContent).Text; !strings.Contains(text, `"REDMINE_READ_ONLY":"1"`) {
		t.Errorf("echo_write output = %s, want REDMINE_READ_ONLY=1", text)
	}
}

func TestServe_UnconfiguredHasNoExtensionTools(t *testing.T) {
	cs, _ := startServe(t, testExtensions(t, nil), "profiles: {}\n", nil, nil)
	if names := toolNames(t, cs); len(names) != 1 || names[0] != "setup_status" {
		t.Errorf("setup server tools = %v, want only setup_status", names)
	}
}

func TestServe_FilterValidationKnowsExtensions(t *testing.T) {
	tests := []struct {
		args    []string
		wantErr string
	}{
		{args: []string{"--enable-groups", "ext-nope"}, wantErr: `no installed extension "nope"`},
		{args: []string{"--enable-tools", "echo_nope"}, wantErr: `unknown tool "echo_nope"`},
		{args: []string{"--enable-tools", "echo_cli_only"}, wantErr: `unknown tool "echo_cli_only"`},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			t.Setenv("REDMINE_NO_KEYRING", "1")
			cfgPath := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(cfgPath, []byte(serveConfig), 0o600); err != nil {
				t.Fatal(err)
			}
			f := &cmdutil.Factory{ConfigPath: cfgPath, IOStreams: &cmdutil.IOStreams{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}}
			cmd := newCmdServe(f, testExtensions(t, nil))
			cmd.SetArgs(tt.args)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			err := cmd.ExecuteContext(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
	// A filter naming an installed extension is valid even where its
	// tools are not served (here: extensions explicitly off).
	cs, _ := startServe(t, testExtensions(t, nil), serveConfig, nil, nil, "--enable-extensions=false", "--disable-groups", "ext-echo", "--disable-tools", "echo_show")
	if names := toolNames(t, cs); hasName(names, "echo_show") || !hasName(names, "list_issues") {
		t.Errorf("tools = %v", names)
	}
}

var listeningRE = regexp.MustCompile(`MCP server listening on (http://\S+)`)

func TestServe_HTTPExtensionDefaultOff(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want bool
	}{
		{name: "default off", want: false},
		{name: "flag on", args: []string{"--enable-extensions"}, want: true},
		{name: "env on", env: map[string]string{config.MCPEnableExtensionsEnv: "1"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"--http", "127.0.0.1:0", "--no-auth"}, tt.args...)
			_, run := startServe(t, testExtensions(t, nil), serveConfig, tt.env, nil, args...)
			deadline := time.Now().Add(10 * time.Second)
			var url string
			for url == "" {
				if m := listeningRE.FindStringSubmatch(run.stderr.String()); m != nil {
					url = m[1]
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("server never listened; stderr:\n%s", run.stderr.String())
				}
				time.Sleep(10 * time.Millisecond)
			}
			client := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "v0"}, nil)
			cs, err := client.Connect(context.Background(), &sdk.StreamableClientTransport{Endpoint: url}, nil)
			if err != nil {
				t.Fatalf("Connect: %v", err)
			}
			defer func() { _ = cs.Close() }()
			names := toolNames(t, cs)
			if got := hasName(names, "echo_show"); got != tt.want {
				t.Errorf("echo_show registered = %v, want %v", got, tt.want)
			}
			if !hasName(names, "list_issues") {
				t.Error("core tools missing")
			}
		})
	}
}

func TestResolveEnableExtensions(t *testing.T) {
	on, off := true, false
	tests := []struct {
		name string
		flag *bool
		cfg  *config.Config
		http bool
		want bool
	}{
		{name: "stdio default", want: true},
		{name: "http default", http: true, want: false},
		{name: "nil config", cfg: nil, want: true},
		{name: "config true over http", cfg: &config.Config{MCP: config.MCPConfig{EnableExtensions: &on}}, http: true, want: true},
		{name: "config false on stdio", cfg: &config.Config{MCP: config.MCPConfig{EnableExtensions: &off}}, want: false},
		{name: "flag true over http", flag: &on, http: true, want: true},
		{name: "flag false on stdio", flag: &off, want: false},
		{name: "flag beats config", flag: &on, cfg: &config.Config{MCP: config.MCPConfig{EnableExtensions: &off}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().Bool(enableExtensionsFlag, false, "")
			flagVal := false
			if tt.flag != nil {
				flagVal = *tt.flag
				if err := cmd.Flags().Set(enableExtensionsFlag, map[bool]string{true: "true", false: "false"}[flagVal]); err != nil {
					t.Fatal(err)
				}
			}
			cfg := tt.cfg
			if cfg == nil {
				cfg = &config.Config{}
			}
			if got := resolveEnableExtensions(cmd, flagVal, cfg, tt.http); got != tt.want {
				t.Errorf("resolveEnableExtensions = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMCPTools_ListsExtensionTools(t *testing.T) {
	opts := testExtensions(t, map[string]string{"list": clashManifest, "plain": ""})
	var out, errOut bytes.Buffer
	f := &cmdutil.Factory{IOStreams: &cmdutil.IOStreams{In: strings.NewReader(""), Out: &out, ErrOut: &errOut}}
	cmd := newCmdTools(f, opts)
	cmd.SetOut(&out)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"ext-echo\n",
		"  echo_show   (read, extension echo@v1.2.0)   Echo the input.\n",
		"  echo_write  (write, extension echo@v1.2.0)  Write something. Requires --enable-writes.\n",
		"ext-list\n  list_things  (read, extension list)  Does not clash.\n",
		"extensions\n  Selects every ext-<name> group above.\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output misses %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "echo_cli_only") || strings.Contains(got, "ext-plain") {
		t.Errorf("output lists commands that are not tools:\n%s", got)
	}
	if n := len(regexp.MustCompile(`(?m)^  list_issues `).FindAllString(got, -1)); n != 1 {
		t.Errorf("list_issues should be listed once, as the core tool:\n%s", got)
	}
	if !strings.Contains(errOut.String(), "skipping MCP tool list_issues") {
		t.Errorf("stderr misses the clash warning: %q", errOut.String())
	}
	// Extension groups come after the canonical ones.
	if strings.Index(got, "ext-echo") < strings.Index(got, "news\n") {
		t.Errorf("extension groups must follow the core groups:\n%s", got)
	}
}

func TestMCPTools_WithoutExtensions(t *testing.T) {
	var out bytes.Buffer
	cmd := newCmdTools(&cmdutil.Factory{IOStreams: &cmdutil.IOStreams{Out: &out, ErrOut: &bytes.Buffer{}}}, Options{})
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "ext-") || strings.Contains(out.String(), "extensions\n") {
		t.Errorf("no extension sections expected without extensions:\n%s", out.String())
	}
	if !regexp.MustCompile(`(?m)^  list_issues +\(read\) `).MatchString(out.String()) {
		t.Errorf("core catalog missing:\n%s", out.String())
	}
}

// TestPrintToolCatalog_SanitizesExtensionText lists a tool whose manifest
// text carries escape sequences (validation rejects them; this is the
// second line of defense): `mcp tools` must not pass them to the terminal.
func TestPrintToolCatalog_SanitizesExtensionText(t *testing.T) {
	tools := []mcpserver.ExtensionTool{{
		Extension: "ro",
		Version:   "v1\x1b]0;title\x07",
		Path:      "/nonexistent",
		Command:   &spec.Command{Name: "go", Description: "go \x1b[31mRED\x1b[0m"},
	}}
	var out bytes.Buffer
	if err := printToolCatalog(&out, mcpserver.NewCatalog(tools)); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(out.String(), 0x1b) || strings.ContainsRune(out.String(), 0x07) {
		t.Fatalf("raw control characters in the catalog: %q", out.String())
	}
	if !strings.Contains(out.String(), "go ^[[31mRED^[[0m") {
		t.Errorf("description not shown neutralized:\n%s", out.String())
	}
}
