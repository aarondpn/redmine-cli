package extension

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var coded *cmdutil.CodedError
	if !errors.As(err, &coded) || coded.Code != code {
		t.Fatalf("err = %v (%T), want code %s", err, err, code)
	}
}

func TestSplitGlobalFlags(t *testing.T) {
	fs := pflag.NewFlagSet("root", pflag.ContinueOnError)
	fs.String("profile", "", "")
	fs.StringP("output", "o", "", "")
	fs.BoolP("verbose", "v", false, "")
	fs.Bool("no-color", false, "")
	tests := []struct {
		args       []string
		wantGlobal []string
		wantRest   []string
	}{
		{args: []string{"--profile", "a", "list"}, wantGlobal: []string{"--profile", "a"}, wantRest: []string{"list"}},
		{args: []string{"--profile=a", "-v", "--no-color", "x", "--profile", "b"}, wantGlobal: []string{"--profile=a", "-v", "--no-color"}, wantRest: []string{"x", "--profile", "b"}},
		{args: []string{"-ojson", "-o", "table", "x"}, wantGlobal: []string{"-ojson", "-o", "table"}, wantRest: []string{"x"}},
		{args: []string{"-o=json", "x"}, wantGlobal: []string{"-o=json"}, wantRest: []string{"x"}},
		{args: []string{"--unknown", "--profile", "a"}, wantRest: []string{"--unknown", "--profile", "a"}},
		{args: []string{"-vx"}, wantRest: []string{"-vx"}},
		{args: []string{"--", "--profile", "a"}, wantRest: []string{"--", "--profile", "a"}},
		{args: []string{"-", "x"}, wantRest: []string{"-", "x"}},
		{args: []string{"--help"}, wantRest: []string{"--help"}},
		{args: []string{"--profile"}, wantRest: []string{"--profile"}},
		{args: nil},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			global, rest := splitGlobalFlags(fs, tt.args)
			if !slices.Equal(global, tt.wantGlobal) || !slices.Equal(rest, tt.wantRest) {
				t.Fatalf("got %q / %q, want %q / %q", global, rest, tt.wantGlobal, tt.wantRest)
			}
		})
	}
}

func TestInstallCommand(t *testing.T) {
	asset := extmgr.BinaryAssetName("pr", "linux", "amd64")
	tests := []struct {
		name      string
		args      []string
		tty       bool
		answer    string
		manifest  string
		wantCode  string
		wantErr   string
		wantStore bool
		wantOut   string
	}{
		{name: "non-tty needs --yes", args: []string{"extension", "install", "octo/redmine-cli-pr"}, wantCode: output.ErrCodeValidationFailed, wantErr: "pass --yes"},
		{name: "tty declined", args: []string{"extension", "install", "octo/redmine-cli-pr"}, tty: true, answer: "n\n", wantOut: "Install cancelled"},
		{name: "tty accepted", args: []string{"extension", "install", "octo/redmine-cli-pr"}, tty: true, answer: "y\n", wantStore: true},
		{name: "yes", args: []string{"ext", "install", "octo/redmine-cli-pr", "--yes"}, wantStore: true},
		{name: "invalid source", args: []string{"extension", "install", "octo/pr", "--yes"}, wantCode: output.ErrCodeValidationFailed},
		{name: "core name clash", args: []string{"extension", "install", "octo/redmine-cli-issue", "--yes"}, wantCode: output.ErrCodeValidationFailed, wantErr: "core command"},
		{name: "invalid manifest", args: []string{"extension", "install", "octo/redmine-cli-pr", "--yes"}, manifest: "schema_version: 1\nname: pr\n", wantCode: output.ErrCodeValidationFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			te := newTestEnv(t)
			setTerminal(t, tt.tty)
			assets := map[string][]byte{asset: []byte("#!/bin/sh\n")}
			if tt.manifest != "" {
				assets[spec.FileName] = []byte(tt.manifest)
			}
			te.gh.set("octo/redmine-cli-pr", "v1.0.0", assets)
			te.stdin.WriteString(tt.answer)
			err := te.run(tt.args...)
			if tt.wantCode != "" {
				wantCode(t, err, tt.wantCode)
			} else if err != nil {
				t.Fatalf("%v\nstderr: %s", err, te.stderr)
			}
			if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %q, want %q", err, tt.wantErr)
			}
			if tt.tty && !strings.Contains(te.stderr.String(), "Extensions are not verified by redmine-cli and run with your user permissions (also as MCP tools). Install octo/redmine-cli-pr? [y/N]") {
				t.Errorf("trust prompt missing: %q", te.stderr)
			}
			if tt.wantOut != "" && !strings.Contains(te.stderr.String(), tt.wantOut) {
				t.Errorf("stderr = %q, want %q", te.stderr, tt.wantOut)
			}
			_, getErr := te.manager().Get("pr")
			if installed := getErr == nil; installed != tt.wantStore {
				t.Errorf("installed = %v, want %v (%v)", installed, tt.wantStore, getErr)
			}
		})
	}
}

func TestInstallCommandOutput(t *testing.T) {
	te := newTestEnv(t)
	asset := extmgr.BinaryAssetName("pr", "linux", "amd64")
	te.gh.set("octo/redmine-cli-pr", "v1.0.0", map[string][]byte{asset: []byte("#!/bin/sh\n"), spec.FileName: []byte(manifestFor("pr"))})
	if err := te.run("extension", "install", "octo/redmine-cli-pr", "--yes", "-o", "json"); err != nil {
		t.Fatal(err)
	}
	var got extensionSummary
	if err := json.Unmarshal(te.stdout.Bytes(), &got); err != nil {
		t.Fatalf("%v\n%s", err, te.stdout)
	}
	if got.Name != "pr" || got.Repo != "octo/redmine-cli-pr" || got.Version != "v1.0.0" || got.Kind != "binary" {
		t.Errorf("summary = %+v", got)
	}
	if !slices.Equal(got.Commands, []string{"redmine pr list <issue-id>", "redmine pr push"}) || !slices.Equal(got.MCPTools, []string{"pr_list"}) {
		t.Errorf("commands = %q, tools = %q", got.Commands, got.MCPTools)
	}

	// A second install without --force reports already_exists.
	wantCode(t, te.run("extension", "install", "octo/redmine-cli-pr", "--yes"), output.ErrCodeAlreadyExists)

	// Table output lists the commands and tools.
	if err := te.run("extension", "install", "octo/redmine-cli-pr", "--yes", "--force"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"redmine pr list <issue-id>", "pr_list", "v1.0.0"} {
		if !strings.Contains(te.stdout.String(), want) {
			t.Errorf("table output lacks %q:\n%s", want, te.stdout)
		}
	}
	if !strings.Contains(te.stderr.String(), "Installed extension pr") {
		t.Errorf("stderr = %q", te.stderr)
	}
}

func TestInstallLocalDoesNotPrompt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks")
	}
	te := newTestEnv(t)
	setTerminal(t, false)
	dir := filepath.Join(t.TempDir(), "redmine-cli-dev")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "redmine-cli-dev"), []byte("#!/bin/sh\n"), 0o755)
	t.Chdir(dir)
	if err := te.run("extension", "install", "."); err != nil {
		t.Fatalf("%v\n%s", err, te.stderr)
	}
	ext, err := te.manager().Get("dev")
	if err != nil || ext.Kind != extmgr.KindLocal {
		t.Fatalf("ext = %+v, %v", ext, err)
	}
}

func installBinary(t *testing.T, te *testEnv, name, tag string, manifest string) {
	t.Helper()
	assets := map[string][]byte{extmgr.BinaryAssetName(name, "linux", "amd64"): []byte("#!/bin/sh\n")}
	if manifest != "" {
		assets[spec.FileName] = []byte(manifest)
	}
	te.gh.set("octo/redmine-cli-"+name, tag, assets)
	src := extmgr.Source{Owner: "octo", Repo: "redmine-cli-" + name, Name: name}
	if _, err := te.manager().Install(t.Context(), src, extmgr.InstallOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestListCommand(t *testing.T) {
	te := newTestEnv(t)
	if err := te.run("extension", "list"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(te.stderr.String(), "No extensions found") {
		t.Errorf("empty list: stderr = %q", te.stderr)
	}
	if err := te.run("extension", "list", "-o", "json"); err != nil || strings.TrimSpace(te.stdout.String()) != "[]" {
		t.Fatalf("empty json list = %q, %v", te.stdout, err)
	}

	installBinary(t, te, "pr", "v1.0.0", manifestFor("pr"))
	installBinary(t, te, "plain", "v0.1.0", "")
	// A shadowed one, installed before the core command existed.
	shadowDir := filepath.Join(te.extRoot, "redmine-cli-issues")
	_ = os.MkdirAll(shadowDir, 0o755)
	_ = os.WriteFile(filepath.Join(shadowDir, "redmine-cli-issues"), []byte("#!/bin/sh\n"), 0o755)

	if err := te.run("extension", "ls"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"pr", "octo/redmine-cli-pr", "v1.0.0", "binary", "no manifest (passthrough)", "shadowed by a core command"} {
		if !strings.Contains(te.stdout.String(), want) {
			t.Errorf("list lacks %q:\n%s", want, te.stdout)
		}
	}
	if err := te.run("extension", "list", "-o", "json"); err != nil {
		t.Fatal(err)
	}
	var items []listItem
	if err := json.Unmarshal(te.stdout.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[2].Name != "pr" || items[2].Commands != 2 || items[2].MCPTools != 1 || !items[0].Shadowed {
		t.Fatalf("items = %+v", items)
	}
}

func TestUpgradeCommand(t *testing.T) {
	te := newTestEnv(t)
	installBinary(t, te, "pr", "v1.0.0", "")

	wantCode(t, te.run("extension", "upgrade"), output.ErrCodeValidationFailed)
	wantCode(t, te.run("extension", "upgrade", "pr", "--all"), output.ErrCodeValidationFailed)
	wantCode(t, te.run("extension", "upgrade", "nope"), output.ErrCodeNotFound)

	te.gh.set("octo/redmine-cli-pr", "v1.1.0", map[string][]byte{extmgr.BinaryAssetName("pr", "linux", "amd64"): []byte("new")})
	if err := te.run("extension", "upgrade", "--all", "--dry-run", "-o", "json"); err != nil {
		t.Fatal(err)
	}
	var results []extmgr.UpgradeResult
	if err := json.Unmarshal(te.stdout.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != extmgr.StatusAvailable || results[0].To != "v1.1.0" {
		t.Fatalf("dry run = %+v", results)
	}
	if err := te.run("extension", "upgrade", "pr"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(te.stdout.String(), "upgraded") || !strings.Contains(te.stdout.String(), "v1.1.0") {
		t.Fatalf("upgrade output:\n%s", te.stdout)
	}
	if ext, _ := te.manager().Get("pr"); ext.Version() != "v1.1.0" {
		t.Fatalf("version after upgrade = %s", ext.Version())
	}

	// A failing extension in --all fails the command after the report.
	te.gh.set("octo/redmine-cli-pr", "v1.2.0", map[string][]byte{"unrelated": []byte("x")})
	err := te.run("extension", "upgrade", "--all")
	if err == nil || !strings.Contains(err.Error(), "1 extension(s) failed") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(te.stdout.String(), "failed") {
		t.Fatalf("report lacks the failure:\n%s", te.stdout)
	}
}

func TestRemoveCommand(t *testing.T) {
	te := newTestEnv(t)
	installBinary(t, te, "pr", "v1.0.0", "")
	if err := te.run("extension", "remove", "pr", "-o", "json"); err != nil {
		t.Fatal(err)
	}
	var env output.ActionEnvelope
	if err := json.Unmarshal(te.stdout.Bytes(), &env); err != nil || !env.Ok || env.Action != output.ActionDeleted {
		t.Fatalf("envelope = %+v, %v (%s)", env, err, te.stdout)
	}
	wantCode(t, te.run("extension", "rm", "pr"), output.ErrCodeNotFound)
}

func TestSearchCommand(t *testing.T) {
	te := newTestEnv(t)
	installBinary(t, te, "pr", "v1.0.0", "")
	te.gh.search = []map[string]any{
		{"full_name": "octo/redmine-cli-pr", "name": "redmine-cli-pr", "description": "PRs", "stargazers_count": 3},
		{"full_name": "x/redmine-cli-deploy", "name": "redmine-cli-deploy", "description": "Deploy", "stargazers_count": 1},
	}
	if err := te.run("extension", "search", "-o", "json"); err != nil {
		t.Fatal(err)
	}
	var results []extmgr.SearchResult
	if err := json.Unmarshal(te.stdout.Bytes(), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || !results[0].Installed || results[1].Installed {
		t.Fatalf("results = %+v", results)
	}
	if err := te.run("extension", "search", "deploy"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(te.stdout.String(), "installed") || !strings.Contains(te.stdout.String(), "x/redmine-cli-deploy") {
		t.Fatalf("table:\n%s", te.stdout)
	}
	wantCode(t, te.run("extension", "search", "--limit", "0"), output.ErrCodeValidationFailed)
	te.gh.search = nil
	if err := te.run("extension", "search"); err != nil || !strings.Contains(te.stderr.String(), "No extensions found") {
		t.Fatalf("empty search: %v %q", err, te.stderr)
	}
}

func TestCreateCommand(t *testing.T) {
	for _, lang := range []string{"", "go"} {
		t.Run("lang="+lang, func(t *testing.T) {
			te := newTestEnv(t)
			t.Chdir(t.TempDir())
			args := []string{"extension", "create", "hello"}
			if lang != "" {
				args = append(args, "--precompiled="+lang)
			}
			if err := te.run(args...); err != nil {
				t.Fatalf("%v\n%s", err, te.stderr)
			}
			dir := "redmine-cli-hello"
			if _, err := spec.ParseFile(filepath.Join(dir, spec.FileName), spec.Options{ExtensionName: "hello", CLIVersion: "2.9.0"}); err != nil {
				t.Fatalf("scaffolded manifest is invalid: %v", err)
			}
			data, _ := os.ReadFile(filepath.Join(dir, spec.FileName))
			if !strings.HasPrefix(string(data), "# yaml-language-server: $schema="+spec.SchemaID+"\n") {
				t.Errorf("manifest lacks the modeline:\n%s", data)
			}
			var settings map[string]map[string]string
			data, _ = os.ReadFile(filepath.Join(dir, ".vscode", "settings.json"))
			if err := json.Unmarshal(data, &settings); err != nil || settings["yaml.schemas"][spec.SchemaID] != spec.FileName {
				t.Errorf("settings.json = %s (%v)", data, err)
			}
			want := []string{"README.md"}
			if lang == "go" {
				want = append(want, "main.go", "go.mod", ".gitignore", ".github/workflows/release.yml")
			} else {
				want = append(want, "redmine-cli-hello")
			}
			for _, name := range want {
				if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
					t.Errorf("missing %s", name)
				}
			}
			if lang == "go" {
				wf, _ := os.ReadFile(filepath.Join(dir, ".github", "workflows", "release.yml"))
				for _, w := range []string{"redmine-cli-hello-$goos-$goarch$suffix", "checksums.txt", "cp redmine-extension.yml dist/", "${{ github.token }}"} {
					if !strings.Contains(string(wf), w) {
						t.Errorf("workflow lacks %q", w)
					}
				}
			} else if runtime.GOOS != "windows" {
				if info, _ := os.Stat(filepath.Join(dir, "redmine-cli-hello")); info.Mode().Perm()&0o100 == 0 {
					t.Errorf("script mode = %v", info.Mode())
				}
			}
			if !strings.Contains(te.stderr.String(), "redmine extension install .") {
				t.Errorf("next steps missing: %q", te.stderr)
			}
			wantCode(t, te.run(args...), output.ErrCodeAlreadyExists)
		})
	}
}

func TestCreateCommandRejects(t *testing.T) {
	te := newTestEnv(t)
	t.Chdir(t.TempDir())
	wantCode(t, te.run("extension", "create", "issues"), output.ErrCodeValidationFailed)
	wantCode(t, te.run("extension", "create", "Bad_Name"), output.ErrCodeValidationFailed)
	wantCode(t, te.run("extension", "create", "ok", "--precompiled=rust"), output.ErrCodeValidationFailed)
	if entries, _ := os.ReadDir("."); len(entries) != 0 {
		t.Fatalf("rejected create left files: %v", entries)
	}
}

// TestCreatedScriptRunsThroughCheck runs the scaffolded script extension
// through the real protocol.
func TestCreatedScriptRunsThroughCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the script scaffold needs sh")
	}
	te := newTestEnv(t)
	t.Chdir(t.TempDir())
	if err := te.run("extension", "create", "hello"); err != nil {
		t.Fatal(err)
	}
	if err := te.run("extension", "check", "redmine-cli-hello", "--command", "echo", "--input", `{"message": "hi", "tag": ["a"]}`, "-o", "json"); err != nil {
		t.Fatalf("%v\nstderr: %s", err, te.stderr)
	}
	var got map[string]any
	if err := json.Unmarshal(te.stdout.Bytes(), &got); err != nil || got["message"] != "hi" {
		t.Fatalf("result = %s (%v)", te.stdout, err)
	}
}

func TestCheckCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture extension is a shell script")
	}
	te := newTestEnv(t)
	dir := filepath.Join(t.TempDir(), "redmine-cli-fixture")
	copyFixture(t, dir)

	// Summary.
	if err := te.run("extension", "check", dir, "-o", "json"); err != nil {
		t.Fatal(err)
	}
	var res checkResult
	if err := json.Unmarshal(te.stdout.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.Name != "fixture" || len(res.Commands) != 5 || res.Commands[0].MCPTool != "fixture_show" || res.Commands[2].MCPTool != "" || !res.Commands[4].Raw {
		t.Fatalf("check = %+v", res)
	}
	if err := te.run("extension", "check", dir); err != nil || !strings.Contains(te.stderr.String(), "is valid") {
		t.Fatalf("table check: %v %q", err, te.stderr)
	}

	// Run a command over the MCP surface.
	if err := te.run("extension", "check", dir, "--command", "show", "--input", `{"issue_id": 3, "label": null}`, "--mcp", "-o", "json"); err != nil {
		t.Fatalf("%v\n%s", err, te.stderr)
	}
	var out struct {
		Stdin map[string]any    `json:"stdin"`
		Env   map[string]string `json:"env"`
	}
	if err := json.Unmarshal(te.stdout.Bytes(), &out); err != nil {
		t.Fatalf("%v\n%s", err, te.stdout)
	}
	if out.Stdin["issue_id"] != float64(3) || out.Stdin["state"] != "open" || out.Env["REDMINE_EXT_SURFACE"] != "mcp" || out.Env["REDMINE_MCP"] != "1" || out.Env["REDMINE_MCP_TOOL"] != "fixture_show" || out.Env["REDMINE_READ_ONLY"] != "1" {
		t.Fatalf("mcp run = %+v", out)
	}

	// @- reads the input from stdin.
	te.stdin.WriteString(`{"issue_id": 9}`)
	if err := te.run("extension", "check", dir, "--command", "show", "--input", "@-", "-o", "json"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(te.stdout.String(), `"issue_id": 9`) {
		t.Fatalf("stdin input: %s", te.stdout)
	}

	tests := []struct {
		name string
		args []string
		code string
		err  string
	}{
		{name: "schema rejection", args: []string{"--command", "show", "--input", `{"issue_id": "x"}`}, code: output.ErrCodeValidationFailed},
		{name: "missing required", args: []string{"--command", "show"}, code: output.ErrCodeValidationFailed},
		{name: "not an object", args: []string{"--command", "show", "--input", `[1]`}, code: output.ErrCodeValidationFailed, err: "JSON object"},
		{name: "unknown command", args: []string{"--command", "nope"}, code: output.ErrCodeValidationFailed, err: "commands: show, write, wipe, slow, shell"},
		{name: "raw command", args: []string{"--command", "shell"}, code: output.ErrCodeValidationFailed, err: "raw command"},
		{name: "cli-only command over mcp", args: []string{"--command", "wipe", "--mcp"}, code: output.ErrCodeValidationFailed, err: "not an MCP tool"},
		{name: "input without command", args: []string{"--input", "{}"}, code: output.ErrCodeValidationFailed},
		{name: "yes without command", args: []string{"--yes"}, code: output.ErrCodeValidationFailed},
		{name: "null input", args: []string{"--command", "show", "--input", "null"}, code: output.ErrCodeValidationFailed, err: "JSON object"},
		{name: "trailing input", args: []string{"--command", "show", "--input", `{"issue_id":1} x`}, code: output.ErrCodeValidationFailed, err: "JSON object"},
		// check runs the command for real, so a destructive one needs the
		// same confirmation as on the CLI; there is no terminal here.
		{name: "destructive needs --yes", args: []string{"--command", "wipe"}, code: output.ErrCodeValidationFailed, err: "--yes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := te.run(append([]string{"extension", "check", dir}, tt.args...)...)
			wantCode(t, err, tt.code)
			if tt.err != "" && !strings.Contains(err.Error(), tt.err) {
				t.Errorf("err = %q, want %q", err, tt.err)
			}
		})
	}
}

func TestCheckCommandInvalidAndMissingManifest(t *testing.T) {
	te := newTestEnv(t)
	dir := filepath.Join(t.TempDir(), "redmine-cli-bad")
	_ = os.MkdirAll(dir, 0o755)
	wantCode(t, te.run("extension", "check", dir), output.ErrCodeNotFound)

	_ = os.WriteFile(filepath.Join(dir, spec.FileName), []byte("schema_version: 1\nname: other\ndescription: x\ncommands:\n  - name: a\n    description: b\n"), 0o644)
	err := te.run("extension", "check", dir)
	var coded *cmdutil.CodedError
	if !errors.As(err, &coded) || coded.Code != output.ErrCodeValidationFailed || len(coded.Details) == 0 || !strings.Contains(coded.Details[0], "redmine-extension.yml:2") {
		t.Fatalf("err = %#v", err)
	}
	wantCode(t, te.run("extension", "check", "notinstalled"), output.ErrCodeNotFound)
}

func copyFixture(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, mode := range map[string]os.FileMode{"redmine-cli-fixture": 0o755, spec.FileName: 0o644} {
		data, err := os.ReadFile(filepath.Join("testdata", "redmine-cli-fixture", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, mode); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCheckCommandDestructiveAndNumbers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture extension is a shell script")
	}
	dir := filepath.Join(t.TempDir(), "redmine-cli-fixture")
	copyFixture(t, dir)
	ran := func(te *testEnv) bool { return strings.Contains(te.stdout.String(), `"REDMINE_EXT_COMMAND"`) }

	t.Run("--yes runs a destructive command", func(t *testing.T) {
		te := newTestEnv(t)
		if err := te.run("extension", "check", dir, "--command", "wipe", "--yes", "-o", "json"); err != nil {
			t.Fatalf("%v\n%s", err, te.stderr)
		}
		if !ran(te) {
			t.Fatalf("wipe did not run: %s", te.stdout)
		}
	})
	t.Run("declined at a terminal", func(t *testing.T) {
		te := newTestEnv(t)
		setTerminal(t, true)
		te.stdin.WriteString("n\n")
		if err := te.run("extension", "check", dir, "--command", "wipe"); err != nil {
			t.Fatalf("%v\n%s", err, te.stderr)
		}
		if ran(te) || !strings.Contains(te.stderr.String(), "is destructive. Continue? [y/N]") {
			t.Fatalf("stdout %q stderr %q", te.stdout, te.stderr)
		}
	})
	t.Run("confirmed at a terminal", func(t *testing.T) {
		te := newTestEnv(t)
		setTerminal(t, true)
		te.stdin.WriteString("y\n")
		if err := te.run("extension", "check", dir, "--command", "wipe", "-o", "json"); err != nil {
			t.Fatalf("%v\n%s", err, te.stderr)
		}
		if !ran(te) {
			t.Fatalf("wipe did not run: %s", te.stdout)
		}
	})
	t.Run("integers beyond 2^53 are accepted like over MCP", func(t *testing.T) {
		te := newTestEnv(t)
		if err := te.run("extension", "check", dir, "--command", "show", "--input", `{"issue_id": 9007199254740993}`, "--mcp", "-o", "json"); err != nil {
			t.Fatalf("%v\n%s", err, te.stderr)
		}
		if !strings.Contains(te.stdout.String(), `"issue_id": 9007199254740993`) {
			t.Fatalf("issue_id lost its digits: %s", te.stdout)
		}
	})
}
