package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	extcmd "github.com/aarondpn/redmine-cli/v2/internal/cmd/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// fixtureDir holds the fixture extension: a shell script that prints the
// arguments, stdin and protocol environment it received as JSON.
const fixtureDir = "extension/testdata/redmine-cli-fixture"

// dispatchEnv is the protocol environment the fixture reports.
type fixtureOutput struct {
	Args  []string          `json:"args"`
	Stdin map[string]any    `json:"stdin"`
	Env   map[string]string `json:"env"`
}

// unsetEnv removes k for the test and restores it afterwards.
func unsetEnv(t *testing.T, k string) {
	t.Helper()
	t.Setenv(k, "")
	_ = os.Unsetenv(k)
}

// setupExtensions creates an extensions directory with the fixture
// installed as "fixture" (with manifest), "plain" (without), "broken"
// (manifest for another name) and "issues" (shadowed by the core command),
// plus a config with the profiles "a" (active) and "ro" (read-only).
func setupExtensions(t *testing.T) (cfgPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fixture extension is a shell script")
	}
	dir := t.TempDir()
	extRoot := filepath.Join(dir, "extensions")
	t.Setenv(extmgr.DirEnv, extRoot)
	t.Setenv("REDMINE_NO_UPDATE_CHECK", "1")
	for _, k := range []string{
		"REDMINE_PROFILE", "REDMINE_CONFIG", "REDMINE_SERVER", "REDMINE_HEADERS", "REDMINE_READ_ONLY",
		"REDMINE_NO_COLOR", "REDMINE_API_KEY", "REDMINE_EXT_COMMAND", "REDMINE_MCP", "REDMINE_MCP_TOOL",
		"REDMINE_OUTPUT_FORMAT", "NO_COLOR",
		"FIXTURE_EXIT", "FIXTURE_STDERR", "FIXTURE_STDOUT", "FIXTURE_SLEEP",
	} {
		unsetEnv(t, k)
	}

	script, err := os.ReadFile(filepath.Join(fixtureDir, "redmine-cli-fixture"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join(fixtureDir, "redmine-extension.yml"))
	if err != nil {
		t.Fatal(err)
	}
	install := func(name, manifestName string) {
		extDir := filepath.Join(extRoot, extmgr.Prefix+name)
		if err := os.MkdirAll(extDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(extDir, extmgr.Prefix+name), script, 0o755); err != nil {
			t.Fatal(err)
		}
		if manifestName != "" {
			m := strings.Replace(string(manifest), "name: fixture", "name: "+manifestName, 1)
			if err := os.WriteFile(filepath.Join(extDir, "redmine-extension.yml"), []byte(m), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	install("fixture", "fixture")
	install("plain", "")
	install("broken", "someone-else")
	install("issues", "issues")

	cfgPath = filepath.Join(dir, "config.yaml")
	cfg := `active_profile: a
profiles:
  a:
    server: https://a.example.com
    api_key: ka
  ro:
    server: https://ro.example.com
    api_key: kr
    read_only: true
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

type rootRun struct {
	stdout string
	stderr string
	err    error
}

// runWithExtensions runs the real root command with extension discovery
// enabled, as main does.
func runWithExtensions(t *testing.T, stdin string, args ...string) rootRun {
	t.Helper()
	var out, errOut bytes.Buffer
	root, f := NewRootCmdWithOptions("test", RootOptions{Extensions: DiscoverExtensions("test", &errOut)})
	f.IOStreams.In = strings.NewReader(stdin)
	f.IOStreams.Out, f.IOStreams.ErrOut, f.IOStreams.IsTTY = &out, &errOut, false
	root.SetIn(f.IOStreams.In)
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)
	// main executes with os.Args, which passthrough commands read; SetArgs
	// needs them recorded.
	err := root.ExecuteContext(extcmd.WithInvocationArgs(t.Context(), args))
	return rootRun{stdout: out.String(), stderr: errOut.String(), err: err}
}

func decodeFixture(t *testing.T, r rootRun) fixtureOutput {
	t.Helper()
	if r.err != nil {
		t.Fatalf("Execute: %v\nstderr: %s", r.err, r.stderr)
	}
	var got fixtureOutput
	if err := json.Unmarshal([]byte(r.stdout), &got); err != nil {
		t.Fatalf("stdout is not the fixture JSON: %v\n%s", err, r.stdout)
	}
	return got
}

func TestExtensionDispatchEnvContract(t *testing.T) {
	cfgPath := setupExtensions(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	exe, _ = filepath.Abs(exe)

	tests := []struct {
		name      string
		env       map[string]string
		args      []string
		wantArgs  []string
		wantStdin map[string]any
		wantEnv   map[string]string
		wantUnset []string
		wantWarn  string
	}{
		{
			name:      "read command is forced read-only",
			args:      []string{"--config", cfgPath, "fixture", "show", "12", "--label", "a", "--label", "b", "-o", "json"},
			wantArgs:  []string{},
			wantStdin: map[string]any{"issue_id": float64(12), "state": "open", "label": []any{"a", "b"}},
			wantEnv: map[string]string{
				"REDMINE_CLI_BIN": exe, "REDMINE_EXT_NAME": "fixture", "REDMINE_EXT_COMMAND": "show",
				"REDMINE_EXT_SURFACE": "cli", "REDMINE_PROFILE": "a", "REDMINE_CONFIG": cfgPath,
				"REDMINE_READ_ONLY": "1", "REDMINE_NO_UPDATE_CHECK": "1",
			},
			wantUnset: []string{"REDMINE_MCP", "REDMINE_MCP_TOOL", "REDMINE_SERVER", "REDMINE_HEADERS", "REDMINE_NO_COLOR", "REDMINE_API_KEY"},
		},
		{
			name:      "write command is not forced read-only",
			env:       map[string]string{"REDMINE_CONFIG": cfgPath},
			args:      []string{"fixture", "write", "-o", "json"},
			wantStdin: map[string]any{},
			wantEnv:   map[string]string{"REDMINE_EXT_COMMAND": "write", "REDMINE_PROFILE": "a", "REDMINE_CONFIG": cfgPath},
			wantUnset: []string{"REDMINE_READ_ONLY"},
		},
		{
			name: "global flags are forwarded",
			args: []string{
				"--config", cfgPath, "--profile", "ro", "--server", "https://s.example.com",
				"--header", "X-A: 1", "--header", "X-B: two", "--no-color", "fixture", "write", "-o", "json",
			},
			wantEnv: map[string]string{
				"REDMINE_PROFILE": "ro", "REDMINE_SERVER": "https://s.example.com",
				"REDMINE_HEADERS": "X-A: 1\nX-B: two", "REDMINE_NO_COLOR": "1",
				// The ro profile is read-only, so even a write command is.
				"REDMINE_READ_ONLY": "1",
			},
		},
		{
			name:    "env headers are merged before flag headers",
			env:     map[string]string{"REDMINE_HEADERS": "X-Env: e"},
			args:    []string{"--config", cfgPath, "--header", "X-Flag: f", "fixture", "write", "-o", "json"},
			wantEnv: map[string]string{"REDMINE_HEADERS": "X-Env: e\nX-Flag: f"},
		},
		{
			name:    "env headers alone are inherited",
			env:     map[string]string{"REDMINE_HEADERS": "X-Env: e"},
			args:    []string{"--config", cfgPath, "fixture", "write", "-o", "json"},
			wantEnv: map[string]string{"REDMINE_HEADERS": "X-Env: e"},
		},
		{
			name:    "--read-only forces a write command read-only",
			args:    []string{"--config", cfgPath, "--read-only", "fixture", "write", "-o", "json"},
			wantEnv: map[string]string{"REDMINE_READ_ONLY": "1"},
		},
		{
			name:    "--read-only=false clears an inherited REDMINE_READ_ONLY",
			env:     map[string]string{"REDMINE_READ_ONLY": "1"},
			args:    []string{"--config", cfgPath, "--read-only=false", "fixture", "write", "-o", "json"},
			wantEnv: map[string]string{"REDMINE_READ_ONLY": "0"},
		},
		{
			name:      "--api-key is not forwarded and warns",
			args:      []string{"--config", cfgPath, "--api-key", "secret", "fixture", "write", "-o", "json"},
			wantUnset: []string{"REDMINE_API_KEY"},
			wantWarn:  "--api-key and --api-key-file are not passed to extensions",
		},
		{
			name:    "inherited REDMINE_API_KEY is left alone",
			env:     map[string]string{"REDMINE_API_KEY": "from-env"},
			args:    []string{"--config", cfgPath, "fixture", "write", "-o", "json"},
			wantEnv: map[string]string{"REDMINE_API_KEY": "from-env"},
		},
		{
			name:    "REDMINE_PROFILE selects the forwarded profile",
			env:     map[string]string{"REDMINE_PROFILE": "ro", "REDMINE_CONFIG": cfgPath},
			args:    []string{"fixture", "write", "-o", "json"},
			wantEnv: map[string]string{"REDMINE_PROFILE": "ro", "REDMINE_READ_ONLY": "1"},
		},
		{
			name:     "raw command passthrough",
			args:     []string{"--config", cfgPath, "--profile", "ro", "fixture", "shell", "--x", "y", "-h"},
			wantArgs: []string{"shell", "--x", "y", "-h"},
			wantEnv:  map[string]string{"REDMINE_EXT_COMMAND": "shell", "REDMINE_PROFILE": "ro", "REDMINE_READ_ONLY": "1"},
		},
		{
			name:      "manifest-less passthrough with leading global flags",
			args:      []string{"--config", cfgPath, "--profile", "ro", "plain", "sub", "--profile", "x", "--help"},
			wantArgs:  []string{"sub", "--profile", "x", "--help"},
			wantEnv:   map[string]string{"REDMINE_EXT_NAME": "plain", "REDMINE_PROFILE": "ro", "REDMINE_READ_ONLY": "1"},
			wantUnset: []string{"REDMINE_EXT_COMMAND"},
		},
		{
			// Everything after the extension name is the extension's,
			// global flags included.
			name:      "manifest-less passthrough keeps flags after the name",
			args:      []string{"--config", cfgPath, "plain", "--profile", "ro", "-v", "sub"},
			wantArgs:  []string{"--profile", "ro", "-v", "sub"},
			wantEnv:   map[string]string{"REDMINE_PROFILE": "a"},
			wantUnset: []string{"REDMINE_READ_ONLY"},
		},
		{
			name:     "raw command keeps its own global-looking flags",
			args:     []string{"--config", cfgPath, "fixture", "shell", "-v", "--output", "x", "--profile", "ro", "a"},
			wantArgs: []string{"shell", "-v", "--output", "x", "--profile", "ro", "a"},
			wantEnv:  map[string]string{"REDMINE_PROFILE": "a"},
		},
		{
			name:     "raw command with the same flag before and after",
			args:     []string{"--config", cfgPath, "--profile", "ro", "fixture", "shell", "--profile", "x", "a"},
			wantArgs: []string{"shell", "--profile", "x", "a"},
			wantEnv:  map[string]string{"REDMINE_PROFILE": "ro"},
		},
		{
			name:     "global flags between the extension and its raw command",
			args:     []string{"--config", cfgPath, "fixture", "--profile", "ro", "shell", "--profile", "x"},
			wantArgs: []string{"shell", "--profile", "x"},
			wantEnv:  map[string]string{"REDMINE_PROFILE": "ro"},
		},
		{
			name:     "exec of a raw command keeps its flags",
			args:     []string{"--config", cfgPath, "extension", "exec", "--profile", "ro", "fixture", "shell", "-v", "--profile", "x"},
			wantArgs: []string{"shell", "-v", "--profile", "x"},
			wantEnv:  map[string]string{"REDMINE_PROFILE": "ro"},
		},
		{
			name:      "exec of a passthrough extension keeps its flags",
			args:      []string{"--config", cfgPath, "-v", "ext", "exec", "plain", "-v", "--server", "s"},
			wantArgs:  []string{"-v", "--server", "s"},
			wantEnv:   map[string]string{"REDMINE_EXT_NAME": "plain"},
			wantUnset: []string{"REDMINE_SERVER"},
		},
		{
			// A read_only profile with --read-only=false: the nested
			// "redmine api" loads the same profile and must not become
			// read-only again.
			name:    "--read-only=false overrides a read-only profile",
			args:    []string{"--config", cfgPath, "--profile", "ro", "--read-only=false", "fixture", "write", "-o", "json"},
			wantEnv: map[string]string{"REDMINE_PROFILE": "ro", "REDMINE_READ_ONLY": "0"},
		},
		{
			name:      "manifest-less passthrough is not forced read-only",
			args:      []string{"--config", cfgPath, "plain", "a", "b"},
			wantArgs:  []string{"a", "b"},
			wantUnset: []string{"REDMINE_EXT_COMMAND", "REDMINE_READ_ONLY"},
		},
		{
			name:      "exec runs an extension shadowed by a core command",
			args:      []string{"--config", cfgPath, "extension", "exec", "--profile", "ro", "issues", "show", "5", "-o", "json"},
			wantStdin: map[string]any{"issue_id": float64(5), "state": "open"},
			wantEnv:   map[string]string{"REDMINE_EXT_NAME": "issues", "REDMINE_EXT_COMMAND": "show", "REDMINE_PROFILE": "ro"},
		},
		{
			name:     "exec of a passthrough extension",
			args:     []string{"--config", cfgPath, "ext", "exec", "plain", "x", "--y"},
			wantArgs: []string{"x", "--y"},
			wantEnv:  map[string]string{"REDMINE_EXT_NAME": "plain"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			r := runWithExtensions(t, "", tt.args...)
			got := decodeFixture(t, r)
			if tt.wantArgs != nil && !slices.Equal(got.Args, tt.wantArgs) {
				t.Errorf("args = %q, want %q", got.Args, tt.wantArgs)
			}
			if tt.wantStdin != nil {
				gotJSON, _ := json.Marshal(got.Stdin)
				wantJSON, _ := json.Marshal(tt.wantStdin)
				if string(gotJSON) != string(wantJSON) {
					t.Errorf("stdin = %s, want %s", gotJSON, wantJSON)
				}
			}
			for k, v := range tt.wantEnv {
				if got.Env[k] != v {
					t.Errorf("%s = %q, want %q (env %v)", k, got.Env[k], v, got.Env)
				}
			}
			for _, k := range tt.wantUnset {
				if v, ok := got.Env[k]; ok {
					t.Errorf("%s = %q, want unset", k, v)
				}
			}
			if tt.wantWarn != "" {
				if n := strings.Count(r.stderr, tt.wantWarn); n != 1 {
					t.Errorf("warning printed %d times, want once; stderr: %s", n, r.stderr)
				}
			}
		})
	}
}

func TestExtensionDispatchRendering(t *testing.T) {
	cfgPath := setupExtensions(t)
	t.Setenv("REDMINE_CONFIG", cfgPath)
	list := `[{"id":1,"name":"alpha"},{"id":2,"name":"beta"}]`

	tests := []struct {
		name   string
		stdout string
		args   []string
		want   []string
		exact  string
	}{
		{name: "detail spec for objects", args: []string{"fixture", "show", "7"}, want: []string{"Issue:", "7", "State:", "open", "Command:", "show"}},
		{name: "table spec for arrays", stdout: list, args: []string{"fixture", "show", "1"}, want: []string{"ID", "Name", "alpha", "beta"}},
		{name: "json output", stdout: list, args: []string{"fixture", "show", "1", "-o", "json"}, want: []string{`"name": "alpha"`}},
		{name: "jq", stdout: list, args: []string{"fixture", "show", "1", "--jq", ".[].id"}, exact: "1\n2\n"},
		{name: "large ids keep their digits", stdout: `{"id": 12345678901234567890}`, args: []string{"fixture", "show", "1", "--jq", ".id"}, exact: "12345678901234567890\n"},
		{name: "no spec match prints json", stdout: `"done"`, args: []string{"fixture", "show", "1"}, exact: "\"done\"\n"},
		{name: "text output", stdout: "plain text", args: []string{"fixture", "show", "1"}, exact: "plain text\n"},
		{name: "text output in json mode", stdout: "plain text", args: []string{"fixture", "show", "1", "-o", "json"}, exact: "\"plain text\"\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.stdout != "" {
				t.Setenv("FIXTURE_STDOUT", tt.stdout)
			}
			r := runWithExtensions(t, "", tt.args...)
			if r.err != nil {
				t.Fatalf("Execute: %v\nstderr: %s", r.err, r.stderr)
			}
			if tt.exact != "" && r.stdout != tt.exact {
				t.Fatalf("stdout = %q, want %q", r.stdout, tt.exact)
			}
			for _, w := range tt.want {
				if !strings.Contains(r.stdout, w) {
					t.Errorf("stdout lacks %q:\n%s", w, r.stdout)
				}
			}
		})
	}
}

func TestExtensionDispatchFailures(t *testing.T) {
	cfgPath := setupExtensions(t)
	t.Setenv("REDMINE_CONFIG", cfgPath)

	tests := []struct {
		name       string
		env        map[string]string
		args       []string
		wantCode   int
		wantCoded  string
		wantErr    string
		wantStderr string
	}{
		{name: "exit code is propagated", env: map[string]string{"FIXTURE_EXIT": "3", "FIXTURE_STDERR": "boom"}, args: []string{"fixture", "show", "1"}, wantCode: 3, wantStderr: "boom\n"},
		{name: "exit without stderr explains itself", env: map[string]string{"FIXTURE_EXIT": "5"}, args: []string{"fixture", "write"}, wantCode: 5, wantStderr: "redmine fixture write exited with status 5"},
		{name: "json mode ends stderr with an envelope", env: map[string]string{"FIXTURE_EXIT": "2", "FIXTURE_STDERR": "bad thing"}, args: []string{"fixture", "show", "1", "-o", "json"}, wantCode: 2, wantStderr: `"code":"extension_failed"`},
		{name: "raw exit code", env: map[string]string{"FIXTURE_EXIT": "4"}, args: []string{"fixture", "shell"}, wantCode: 4},
		{name: "passthrough exit code", env: map[string]string{"FIXTURE_EXIT": "6"}, args: []string{"plain"}, wantCode: 6},
		{name: "timeout", env: map[string]string{"FIXTURE_SLEEP": "10"}, args: []string{"fixture", "slow"}, wantCoded: output.ErrCodeTimeout, wantErr: "timed out after 300ms"},
		{name: "invalid positional", args: []string{"fixture", "show", "abc"}, wantCoded: output.ErrCodeValidationFailed},
		{name: "invalid enum", args: []string{"fixture", "show", "1", "--state", "bogus"}, wantCoded: output.ErrCodeValidationFailed},
		{name: "missing positional", args: []string{"fixture", "show"}, wantErr: "missing required argument <issue-id>"},
		{name: "destructive needs --yes without a terminal", args: []string{"fixture", "wipe"}, wantCoded: output.ErrCodeValidationFailed, wantErr: "--yes"},
		{name: "broken manifest stub", args: []string{"broken", "anything"}, wantCoded: output.ErrCodeValidationFailed, wantErr: "invalid redmine-extension.yml"},
		{name: "jq is rejected for passthrough", args: []string{"--jq", ".", "plain"}, wantErr: "--jq is not supported"},
		{name: "exec of an unknown extension", args: []string{"extension", "exec", "nope"}, wantCoded: output.ErrCodeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			start := time.Now()
			r := runWithExtensions(t, "", tt.args...)
			if time.Since(start) > 8*time.Second {
				t.Errorf("took %s", time.Since(start))
			}
			if r.err == nil {
				t.Fatalf("expected an error; stdout: %s", r.stdout)
			}
			if tt.wantCode != 0 {
				var silent *cmdutil.SilentError
				if !errors.As(r.err, &silent) || silent.Code != tt.wantCode {
					t.Fatalf("err = %#v, want SilentError{%d}", r.err, tt.wantCode)
				}
			}
			if tt.wantCoded != "" {
				var coded *cmdutil.CodedError
				if !errors.As(r.err, &coded) || coded.Code != tt.wantCoded {
					t.Fatalf("err = %v (%T), want code %s", r.err, r.err, tt.wantCoded)
				}
			}
			if tt.wantErr != "" && !strings.Contains(r.err.Error(), tt.wantErr) {
				t.Errorf("err = %q, want %q", r.err, tt.wantErr)
			}
			if tt.wantStderr != "" && !strings.Contains(r.stderr, tt.wantStderr) {
				t.Errorf("stderr = %q, want %q", r.stderr, tt.wantStderr)
			}
		})
	}
}

func TestExtensionDispatchJSONFailureEnvelopeIsLastLine(t *testing.T) {
	setupExtensions(t)
	t.Setenv("FIXTURE_EXIT", "2")
	t.Setenv("FIXTURE_STDERR", "first\nsecond")
	r := runWithExtensions(t, "", "fixture", "show", "1", "-o", "json")
	lines := strings.Split(strings.TrimSpace(r.stderr), "\n")
	var env output.ErrorEnvelope
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &env); err != nil {
		t.Fatalf("last stderr line is not an envelope: %q", r.stderr)
	}
	if env.Error.Code != "extension_failed" || !strings.Contains(env.Error.Message, "second") {
		t.Fatalf("envelope = %+v", env)
	}
	if r.stdout != "" {
		t.Fatalf("stdout = %q, want empty", r.stdout)
	}
}

func TestExtensionDispatchDestructiveWithYes(t *testing.T) {
	setupExtensions(t)
	got := decodeFixture(t, runWithExtensions(t, "", "fixture", "wipe", "--yes", "-o", "json"))
	if got.Env["REDMINE_EXT_COMMAND"] != "wipe" {
		t.Fatalf("env = %v", got.Env)
	}
	if _, ok := got.Env["REDMINE_READ_ONLY"]; ok {
		t.Fatal("destructive write command ran read-only")
	}
}

func TestExtensionDispatchRawStdin(t *testing.T) {
	setupExtensions(t)
	got := decodeFixture(t, runWithExtensions(t, `{"piped": true}`, "fixture", "shell"))
	if got.Stdin["piped"] != true {
		t.Fatalf("stdin = %v", got.Stdin)
	}
}

func TestExtensionRegistration(t *testing.T) {
	setupExtensions(t)
	root, _ := NewRootCmdWithOptions("test", RootOptions{Extensions: DiscoverExtensions("test", io.Discard)})

	byName := map[string]bool{}
	for _, c := range root.Commands() {
		byName[c.Name()] = true
		switch c.Name() {
		case "fixture", "plain", "broken":
			if c.GroupID != "extensions" {
				t.Errorf("%s group = %q", c.Name(), c.GroupID)
			}
		case "issues":
			if c.GroupID != "core" || c.Annotations["redmine-cli/extension"] != "" {
				t.Errorf("issues is not the core command (group %q)", c.GroupID)
			}
		}
	}
	for _, name := range []string{"fixture", "plain", "broken"} {
		if !byName[name] {
			t.Errorf("extension %s not registered", name)
		}
	}
	if !strings.Contains(root.UsageString(), "Extension Commands:") {
		t.Errorf("help lacks the extension group:\n%s", root.UsageString())
	}

	fixture, _, err := root.Find([]string{"fixture", "show"})
	if err != nil || fixture.Name() != "show" {
		t.Fatalf("find fixture show: %v", err)
	}
	if fixture.Flags().Lookup("state") == nil || fixture.Flags().Lookup("label") == nil {
		t.Error("derived flags missing")
	}
	if wipe, _, _ := root.Find([]string{"fixture", "wipe"}); wipe.Flags().Lookup("yes") == nil {
		t.Error("destructive command lacks --yes")
	}

	// Tests build the root through NewRootCmd, which must not pick up the
	// installed extensions.
	plain := NewRootCmd("test")
	for _, c := range plain.Commands() {
		if c.Name() == "fixture" || c.Name() == "plain" {
			t.Fatalf("NewRootCmd registered extension %s", c.Name())
		}
	}
}

func TestExtensionCompletion(t *testing.T) {
	setupExtensions(t)
	tests := []struct {
		args []string
		want []string
	}{
		{args: []string{"__complete", "fix"}, want: []string{"fixture"}},
		{args: []string{"__complete", "fixture", ""}, want: []string{"show", "write", "wipe", "slow", "shell"}},
		{args: []string{"__complete", "fixture", "show", "1", "--state", ""}, want: []string{"open", "closed"}},
		{args: []string{"__complete", "extension", "exec", ""}, want: []string{"fixture", "plain", "issues"}},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			r := runWithExtensions(t, "", tt.args...)
			if r.err != nil {
				t.Fatal(r.err)
			}
			for _, w := range tt.want {
				if !strings.Contains(r.stdout, w) {
					t.Errorf("completion lacks %q:\n%s", w, r.stdout)
				}
			}
		})
	}
}

func TestAttachesTerminal(t *testing.T) {
	setupExtensions(t)
	root, _ := NewRootCmdWithOptions("test", RootOptions{Extensions: DiscoverExtensions("test", io.Discard)})
	tests := []struct {
		args []string
		want bool
	}{
		{args: []string{"fixture", "shell", "-x"}, want: true},
		{args: []string{"--profile", "a", "plain", "anything"}, want: true},
		{args: []string{"fixture", "show", "1"}},
		{args: []string{"issues", "list"}},
		{args: []string{"extension", "exec", "plain"}, want: true},
		{args: []string{"ext", "exec", "--profile", "a", "fixture", "shell"}, want: true},
		{args: []string{"extension", "exec", "fixture", "show", "1"}},
		{args: []string{"extension", "exec"}},
		{args: []string{"extension", "list"}},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			if got := AttachesTerminal(root, tt.args, "test"); got != tt.want {
				t.Fatalf("AttachesTerminal = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtensionListThroughRoot(t *testing.T) {
	setupExtensions(t)
	r := runWithExtensions(t, "", "extension", "list", "-o", "json")
	if r.err != nil {
		t.Fatalf("%v\n%s", r.err, r.stderr)
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(r.stdout), &items); err != nil {
		t.Fatalf("%v\n%s", err, r.stdout)
	}
	got := map[string]map[string]any{}
	for _, it := range items {
		got[it["name"].(string)] = it
	}
	if got["fixture"]["commands"] != float64(5) || got["fixture"]["mcp_tools"] != float64(3) {
		t.Errorf("fixture = %v", got["fixture"])
	}
	if got["broken"]["error"] == nil {
		t.Errorf("broken has no error: %v", got["broken"])
	}
	if got["issues"]["shadowed"] != true {
		t.Errorf("issues not shadowed: %v", got["issues"])
	}
	if got["plain"]["commands"] != float64(0) {
		t.Errorf("plain = %v", got["plain"])
	}
}

// TestBrokenStubParsesOutputFlags checks that -o json and --jq after the
// name of a broken extension select JSON output (main renders the error as
// a JSON envelope then), while the extension's own flags are ignored.
func TestBrokenStubParsesOutputFlags(t *testing.T) {
	setupExtensions(t)
	tests := []struct {
		name string
		args []string
		jq   bool
	}{
		{name: "-o json after the name", args: []string{"broken", "x", "-o", "json"}},
		{name: "-o json among unknown flags", args: []string{"broken", "x", "--state", "open", "--bogus", "-o", "json"}},
		{name: "--jq after the name", args: []string{"broken", "x", "--jq", ".error"}, jq: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, f := NewRootCmdWithOptions("test", RootOptions{Extensions: DiscoverExtensions("test", io.Discard)})
			var out, errOut bytes.Buffer
			f.IOStreams.In = strings.NewReader("")
			f.IOStreams.Out, f.IOStreams.ErrOut, f.IOStreams.IsTTY = &out, &errOut, false
			root.SetOut(&out)
			root.SetErr(&errOut)
			root.SetArgs(tt.args)
			err := root.ExecuteContext(extcmd.WithInvocationArgs(t.Context(), tt.args))
			var coded *cmdutil.CodedError
			if !errors.As(err, &coded) || coded.Code != output.ErrCodeValidationFailed {
				t.Fatalf("err = %v (%T), want the invalid manifest error", err, err)
			}
			if tt.jq {
				if fl := root.PersistentFlags().Lookup("jq"); fl == nil || !fl.Changed {
					t.Fatal("--jq was not parsed")
				}
				return
			}
			if f.OutputFormat != output.FormatJSON {
				t.Fatalf("OutputFormat = %q, want json", f.OutputFormat)
			}
		})
	}
}
