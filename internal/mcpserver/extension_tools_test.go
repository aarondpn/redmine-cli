package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/runner"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
)

// fixtureDir holds the script extension "fix" used by these tests.
var fixtureDir = filepath.Join("testdata", "redmine-cli-fix")

// protocolEnv lists the variables the fixture reports; they are cleared for
// every test so the developer's own environment cannot leak into the
// assertions.
var protocolEnv = []string{
	runner.EnvCLIBin, runner.EnvExtName, runner.EnvExtCommand, runner.EnvExtSurface,
	runner.EnvMCP, runner.EnvMCPTool, runner.EnvProfile, runner.EnvConfig, runner.EnvServer,
	runner.EnvHeaders, runner.EnvReadOnly, runner.EnvNoColor, runner.EnvNoUpdateCheck,
}

func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		t.Setenv(k, "")
		if err := os.Unsetenv(k); err != nil {
			t.Fatal(err)
		}
	}
}

// fixtureTools returns the MCP tools of the fixture extension with env as
// the invocation-level environment.
func fixtureTools(t *testing.T, env runner.Env) []ExtensionTool {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fixture extension is a sh script")
	}
	unsetEnv(t, protocolEnv...)
	m, err := spec.ParseFile(filepath.Join(fixtureDir, spec.FileName), spec.Options{ExtensionName: "fix"})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	path, err := filepath.Abs(filepath.Join(fixtureDir, "redmine-cli-fix"))
	if err != nil {
		t.Fatal(err)
	}
	var tools []ExtensionTool
	for _, c := range m.MCPCommands() {
		tools = append(tools, ExtensionTool{Extension: "fix", Version: "v1.2.0", Path: path, Command: c, Env: env})
	}
	return tools
}

// newExtensionSession connects a client to a server with opts.
func newExtensionSession(t *testing.T, opts Options) *mcp.ClientSession {
	t.Helper()
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	t.Cleanup(closeTS)
	if opts.Version == "" {
		opts.Version = "v0"
	}
	cs, cleanup := newConnectedSession(t, apiClient, opts)
	t.Cleanup(cleanup)
	return cs
}

func callTool(t *testing.T, cs *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return res
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("content has %d blocks, want 1: %+v", len(res.Content), res.Content)
	}
	return asText(t, res.Content[0])
}

// fixtureOutput is what the fixture's default commands print.
type fixtureOutput struct {
	APIKey string            `json:"api_key"`
	User   map[string]any    `json:"user"`
	Stdin  map[string]any    `json:"stdin"`
	Env    map[string]string `json:"env"`
}

func decodeFixture(t *testing.T, text string) fixtureOutput {
	t.Helper()
	var out fixtureOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("decoding %q: %v", text, err)
	}
	return out
}

func TestExtensionTools_ToolsList(t *testing.T) {
	tools := fixtureTools(t, runner.Env{})
	tests := []struct {
		name         string
		enableWrites bool
		want         []string
		missing      []string
	}{
		{
			name:    "read-only server",
			want:    []string{"fix_show", "fix_list", "fix_text", "fix_empty", "fix_fail", "fix_slow", "list_issues"},
			missing: []string{"fix_write", "fix_wipe", "fix_cli_only", "fix_shell"},
		},
		{
			name:         "writes enabled",
			enableWrites: true,
			want:         []string{"fix_show", "fix_write", "fix_wipe", "create_issue"},
			missing:      []string{"fix_cli_only", "fix_shell"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := newExtensionSession(t, Options{EnableWrites: tt.enableWrites, ExtensionTools: tools})
			names := listToolNames(t, cs)
			for _, n := range tt.want {
				if !contains(names, n) {
					t.Errorf("tool %q missing from tools/list", n)
				}
			}
			for _, n := range tt.missing {
				if contains(names, n) {
					t.Errorf("tool %q must not be registered", n)
				}
			}
		})
	}
}

func TestExtensionTools_ToolDefinition(t *testing.T) {
	cs := newExtensionSession(t, Options{EnableWrites: true, ExtensionTools: fixtureTools(t, runner.Env{})})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		byName[tool.Name] = tool
	}

	show := byName["fix_show"]
	if show == nil {
		t.Fatal("fix_show missing")
	}
	if show.Description != "Echo the input and protocol environment as an object." {
		t.Errorf("fix_show description = %q", show.Description)
	}
	if show.Annotations == nil || !show.Annotations.ReadOnlyHint {
		t.Errorf("fix_show annotations = %+v, want read-only", show.Annotations)
	}
	schema, err := json.Marshal(show.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(schema, &s); err != nil {
		t.Fatal(err)
	}
	if s.Type != "object" || len(s.Properties) != 4 || len(s.Required) != 1 || s.Required[0] != "issue_id" {
		t.Errorf("fix_show input schema = %s", schema)
	}
	if !strings.Contains(string(s.Properties["state"]), `"enum":["open","closed"]`) || !strings.Contains(string(s.Properties["state"]), `"default":"open"`) {
		t.Errorf("state schema = %s, want enum and default", s.Properties["state"])
	}

	wipe := byName["fix_wipe"]
	if wipe == nil {
		t.Fatal("fix_wipe missing")
	}
	if wipe.Description != "Destructive command. Destructive. Requires --enable-writes." {
		t.Errorf("fix_wipe description = %q", wipe.Description)
	}
	if wipe.Annotations == nil || wipe.Annotations.ReadOnlyHint || wipe.Annotations.DestructiveHint == nil || !*wipe.Annotations.DestructiveHint {
		t.Errorf("fix_wipe annotations = %+v, want destructive", wipe.Annotations)
	}
	write := byName["fix_write"]
	if write == nil || write.Annotations == nil || write.Annotations.DestructiveHint == nil || *write.Annotations.DestructiveHint {
		t.Errorf("fix_write annotations = %+v, want non-destructive write", write)
	}
}

func TestExtensionTools_SchemaRejection(t *testing.T) {
	cs := newExtensionSession(t, Options{ExtensionTools: fixtureTools(t, runner.Env{})})
	tests := []struct {
		name string
		args any
		want []string
	}{
		{"missing required", map[string]any{}, []string{`validating "arguments"`, "issue_id"}},
		{"wrong type", map[string]any{"issue_id": "twelve"}, []string{"issue_id", "integer"}},
		{"fraction for integer", map[string]any{"issue_id": 1.5}, []string{"issue_id"}},
		{"enum", map[string]any{"issue_id": 1, "state": "merged"}, []string{"state"}},
		{"unknown property", map[string]any{"issue_id": 1, "bogus": true}, []string{"bogus"}},
		{"array item type", map[string]any{"issue_id": 1, "label": []any{"a", 2}}, []string{"label"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := callTool(t, cs, "fix_show", tt.args)
			if !res.IsError {
				t.Fatalf("expected a tool error, got %+v", res.Content)
			}
			text := resultText(t, res)
			for _, w := range tt.want {
				if !strings.Contains(text, w) {
					t.Errorf("error %q does not mention %q", text, w)
				}
			}
			if res.StructuredContent != nil {
				t.Errorf("error result has structured content %v", res.StructuredContent)
			}
		})
	}
}

func TestExtensionTools_StructuredResult(t *testing.T) {
	env := runner.Env{
		CLIBin:     "/usr/local/bin/redmine",
		Profile:    "work",
		ConfigPath: "/tmp/redmine.yaml",
		Server:     "https://redmine.example.com",
		Headers:    []string{"X-One: 1", "X-Two: 2"},
		NoColor:    true,
	}
	cs := newExtensionSession(t, Options{ExtensionTools: fixtureTools(t, env)})

	res := callTool(t, cs, "fix_show", map[string]any{
		"issue_id": json.Number("12345678901234567"),
		"label":    []any{"a", "b"},
		"extra":    map[string]any{"nested": []any{1, true, nil}},
		// Clients send null for optionals they leave unset.
		"state": nil,
	})
	if res.IsError {
		t.Fatalf("unexpected tool error: %s", resultText(t, res))
	}
	text := resultText(t, res)
	if strings.Contains(text, "secret") {
		t.Errorf("text result leaks a redacted secret: %s", text)
	}
	if !strings.Contains(text, `"issue_id":12345678901234567`) {
		t.Errorf("large integer did not reach the extension unchanged: %s", text)
	}
	out := decodeFixture(t, text)
	if out.APIKey != "" || out.User["api_key"] != nil || out.User["login"] != "jdoe" {
		t.Errorf("redaction: api_key=%q user=%v", out.APIKey, out.User)
	}
	if out.Stdin["state"] != "open" {
		t.Errorf("default not applied: stdin = %v", out.Stdin)
	}
	if got, _ := json.Marshal(out.Stdin["extra"]); string(got) != `{"nested":[1,true,null]}` {
		t.Errorf("json param = %s", got)
	}

	wantEnv := map[string]string{
		runner.EnvCLIBin:        "/usr/local/bin/redmine",
		runner.EnvExtName:       "fix",
		runner.EnvExtCommand:    "show",
		runner.EnvExtSurface:    "mcp",
		runner.EnvMCP:           "1",
		runner.EnvMCPTool:       "fix_show",
		runner.EnvProfile:       "work",
		runner.EnvConfig:        "/tmp/redmine.yaml",
		runner.EnvServer:        "https://redmine.example.com",
		runner.EnvHeaders:       "X-One: 1\nX-Two: 2",
		runner.EnvReadOnly:      "1",
		runner.EnvNoColor:       "1",
		runner.EnvNoUpdateCheck: "1",
	}
	for k, want := range wantEnv {
		if got := out.Env[k]; got != want {
			t.Errorf("env %s = %q, want %q", k, got, want)
		}
	}

	structured, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content = %T, want an object", res.StructuredContent)
	}
	if _, leaked := structured["api_key"]; leaked {
		t.Error("structured content leaks api_key")
	}
	if user, _ := structured["user"].(map[string]any); user == nil || user["api_key"] != nil {
		t.Errorf("structured user = %v, want api_key redacted", structured["user"])
	}
}

func TestExtensionTools_TextResults(t *testing.T) {
	cs := newExtensionSession(t, Options{ExtensionTools: fixtureTools(t, runner.Env{})})
	tests := []struct {
		tool string
		want string
	}{
		// Arrays are JSON but not objects: text only, still redacted.
		{"fix_list", `[{"id":1},{"id":2,"user":{"login":"jdoe"}}]`},
		{"fix_text", "plain text\n"},
		{"fix_empty", ""},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			res := callTool(t, cs, tt.tool, nil)
			if res.IsError {
				t.Fatalf("unexpected tool error: %s", resultText(t, res))
			}
			if got := resultText(t, res); got != tt.want {
				t.Errorf("text = %q, want %q", got, tt.want)
			}
			if res.StructuredContent != nil {
				t.Errorf("structured content = %v, want none", res.StructuredContent)
			}
		})
	}
}

func TestExtensionTools_Failures(t *testing.T) {
	cs := newExtensionSession(t, Options{ExtensionTools: fixtureTools(t, runner.Env{})})
	tests := []struct {
		tool string
		want []string
	}{
		{"fix_fail", []string{"fix_fail failed with exit status 3", "boom"}},
		{"fix_slow", []string{"fix_slow timed out after 300ms"}},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			res := callTool(t, cs, tt.tool, map[string]any{})
			if !res.IsError {
				t.Fatalf("expected isError, got %+v", res.Content)
			}
			text := resultText(t, res)
			for _, w := range tt.want {
				if !strings.Contains(text, w) {
					t.Errorf("error %q does not contain %q", text, w)
				}
			}
		})
	}
}

func TestExtensionTools_ReadOnlyEnv(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		envRO    bool
		inherit  string
		wantRO   string
		wantSet  bool
		inherits bool
	}{
		{name: "read tool is forced read-only", tool: "fix_show", wantRO: "1", wantSet: true},
		{name: "write tool is not", tool: "fix_write", wantSet: false},
		{name: "write tool in a read-only server", tool: "fix_write", envRO: true, wantRO: "1", wantSet: true},
		{name: "explicit read-only off beats the inherited value", tool: "fix_write", inherit: "1", wantRO: "0", wantSet: true, inherits: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools := fixtureTools(t, runner.Env{ReadOnly: tt.envRO})
			if tt.inherits {
				t.Setenv(runner.EnvReadOnly, tt.inherit)
			}
			cs := newExtensionSession(t, Options{EnableWrites: true, ExtensionTools: tools})
			args := map[string]any{}
			if tt.tool == "fix_show" {
				args["issue_id"] = 1
			}
			res := callTool(t, cs, tt.tool, args)
			if res.IsError {
				t.Fatalf("unexpected tool error: %s", resultText(t, res))
			}
			out := decodeFixture(t, resultText(t, res))
			got, set := out.Env[runner.EnvReadOnly]
			if set != tt.wantSet || got != tt.wantRO {
				t.Errorf("REDMINE_READ_ONLY = %q (set=%v), want %q (set=%v)", got, set, tt.wantRO, tt.wantSet)
			}
		})
	}
}

func TestExtensionTools_GroupFiltering(t *testing.T) {
	tools := fixtureTools(t, runner.Env{})
	tests := []struct {
		name    string
		spec    FilterSpec
		want    []string
		missing []string
	}{
		{name: "no filter", want: []string{"fix_show", "list_issues"}},
		{name: "only the extension group", spec: FilterSpec{EnableGroups: []Group{"ext-fix"}}, want: []string{"fix_show", "fix_list"}, missing: []string{"list_issues", "search"}},
		{name: "umbrella group", spec: FilterSpec{EnableGroups: []Group{GroupExtensions}}, want: []string{"fix_show"}, missing: []string{"list_issues"}},
		{name: "core group only", spec: FilterSpec{EnableGroups: []Group{GroupIssues}}, want: []string{"list_issues"}, missing: []string{"fix_show"}},
		{name: "disable umbrella", spec: FilterSpec{DisableGroups: []Group{GroupExtensions}}, want: []string{"list_issues", "search"}, missing: []string{"fix_show", "fix_list"}},
		{name: "disable extension group", spec: FilterSpec{DisableGroups: []Group{"ext-fix"}}, want: []string{"list_issues"}, missing: []string{"fix_show"}},
		{name: "enable umbrella minus one group", spec: FilterSpec{EnableGroups: []Group{GroupExtensions, GroupIssues}, DisableGroups: []Group{"ext-fix"}}, want: []string{"list_issues"}, missing: []string{"fix_show", "search"}},
		{name: "deny one tool", spec: FilterSpec{DisableTools: []string{"fix_show"}}, want: []string{"fix_list"}, missing: []string{"fix_show"}},
		{name: "allow-list", spec: FilterSpec{EnableTools: []string{"fix_show", "list_issues"}}, want: []string{"fix_show", "list_issues"}, missing: []string{"fix_list", "search"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ResolveOptions(Options{ExtensionTools: tools}, tt.spec)
			names := listToolNames(t, newExtensionSession(t, opts))
			for _, n := range tt.want {
				if !contains(names, n) {
					t.Errorf("tool %q missing", n)
				}
			}
			for _, n := range tt.missing {
				if contains(names, n) {
					t.Errorf("tool %q must be filtered out", n)
				}
			}
		})
	}
}

// clashTool is an extension tool that is never run.
func clashTool(ext, command string) ExtensionTool {
	return ExtensionTool{Extension: ext, Path: "/nonexistent", Command: &spec.Command{Name: command, Description: "Tool " + ext + "/" + command}}
}

func TestResolveExtensionTools_NameClash(t *testing.T) {
	tools := []ExtensionTool{
		clashTool("list", "issues"), // list_issues is a core tool
		clashTool("a-b", "c"),       // a_b_c, but extension a comes first
		clashTool("a", "b-c"),       // a_b_c
		clashTool("pr", "list"),
		{Extension: "raw", Command: &spec.Command{Name: "shell", Raw: true}},
		{Extension: "nil"},
	}
	kept, warnings := ResolveExtensionTools(tools)

	var keptNames []string
	for _, k := range kept {
		keptNames = append(keptNames, k.Extension+":"+k.Name())
	}
	want := []string{"a:a_b_c", "pr:pr_list"}
	if strings.Join(keptNames, ",") != strings.Join(want, ",") {
		t.Errorf("kept = %v, want %v", keptNames, want)
	}
	wantWarnings := []string{
		"extension a-b: skipping MCP tool a_b_c (command c): extension a already provides a tool of that name",
		"extension list: skipping MCP tool list_issues (command issues): a core tool has the same name",
	}
	if strings.Join(warnings, "\n") != strings.Join(wantWarnings, "\n") {
		t.Errorf("warnings =\n%s\nwant\n%s", strings.Join(warnings, "\n"), strings.Join(wantWarnings, "\n"))
	}
}

// A clashing extension tool must not replace the core tool even when the
// caller passes it unresolved.
func TestExtensionTools_NameClashKeepsCoreTool(t *testing.T) {
	cs := newExtensionSession(t, Options{ExtensionTools: []ExtensionTool{clashTool("list", "issues"), clashTool("pr", "list")}})
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, tool := range res.Tools {
		if tool.Name == "list_issues" {
			count++
			if strings.Contains(tool.Description, "Tool list/issues") {
				t.Errorf("list_issues was replaced by the extension tool")
			}
		}
	}
	if count != 1 {
		t.Errorf("list_issues registered %d times", count)
	}
	if !contains(listToolNames(t, cs), "pr_list") {
		t.Error("pr_list should still be registered")
	}
}

func TestBuildSetupServer_HasNoExtensionTools(t *testing.T) {
	srv := BuildSetupServer("no server", Options{ExtensionTools: []ExtensionTool{clashTool("pr", "list")}})
	ct, st := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ss.Close() }()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil).Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cs.Close() }()
	names := listToolNames(t, cs)
	if len(names) != 1 || names[0] != SetupToolName {
		t.Errorf("setup server tools = %v, want only %s", names, SetupToolName)
	}
}

func TestExtensionToolDescription(t *testing.T) {
	tests := []struct {
		name string
		cmd  spec.Command
		want string
	}{
		{"read", spec.Command{Name: "a", Description: "List PRs"}, "List PRs."},
		{"keeps final period", spec.Command{Name: "a", Description: "List PRs. "}, "List PRs."},
		{"keeps question mark", spec.Command{Name: "a", Description: "Who?"}, "Who?"},
		{"write", spec.Command{Name: "a", Description: "Link a PR", Writes: true}, "Link a PR. Requires --enable-writes."},
		{"destructive", spec.Command{Name: "a", Description: "Unlink all", Writes: true, Destructive: true}, "Unlink all. Destructive. Requires --enable-writes."},
		{"empty description", spec.Command{Name: "a", Writes: true}, "Requires --enable-writes."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := ExtensionTool{Extension: "pr", Command: &tt.cmd}
			if got := tool.Description(); got != tt.want {
				t.Errorf("Description() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecodeArguments(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr string
	}{
		{name: "absent", raw: "", want: "{}"},
		{name: "null", raw: " null ", want: "{}"},
		{name: "object keeps big numbers", raw: `{"id":12345678901234567890}`, want: `{"id":12345678901234567890}`},
		{name: "array", raw: `[1]`, wantErr: "must be a JSON object"},
		{name: "scalar", raw: `"x"`, wantErr: "must be a JSON object"},
		{name: "invalid", raw: `{`, wantErr: "invalid JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeArguments(json.RawMessage(tt.raw))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			body, _ := json.Marshal(got)
			if string(body) != tt.want {
				t.Errorf("decoded = %s, want %s", body, tt.want)
			}
		})
	}
}

func TestDescribeRunError(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{&runner.ExitError{Code: 2}, "pr_list failed with exit status 2"},
		{&runner.ExitError{Code: 1, Message: "no token"}, "pr_list failed with exit status 1: no token"},
		{&runner.TimeoutError{Timeout: 30_000_000_000}, "pr_list timed out after 30s and was stopped"},
		{&runner.OutputTooLargeError{Limit: 10}, "pr_list was stopped: its output exceeded 10 bytes"},
		{context.Canceled, "pr_list: context canceled"},
	}
	for _, tt := range tests {
		if got := describeRunError("pr_list", tt.err); got != tt.want {
			t.Errorf("describeRunError(%v) = %q, want %q", tt.err, got, tt.want)
		}
	}
}

func TestNormalizeNumbers(t *testing.T) {
	c := &spec.Command{Name: "x", Params: []spec.Param{
		{Name: "id", Type: spec.TypeInteger},
		{Name: "ids", Type: spec.TypeInteger, Array: true},
		{Name: "ratio", Type: spec.TypeNumber},
		{Name: "extra", Type: spec.TypeJSON},
	}}
	got := normalizeNumbers(c, map[string]any{
		"id":      json.Number("12345678901234567"),
		"ids":     []any{json.Number("1"), json.Number("2")},
		"ratio":   json.Number("0.5"),
		"extra":   map[string]any{"n": json.Number("123456789012345678901234567890")},
		"unknown": json.Number("1e400"),
	})
	if got["id"] != int64(12345678901234567) {
		t.Errorf("id = %#v", got["id"])
	}
	if ids, _ := got["ids"].([]any); len(ids) != 2 || ids[0] != int64(1) || ids[1] != int64(2) {
		t.Errorf("ids = %#v", got["ids"])
	}
	if got["ratio"] != 0.5 {
		t.Errorf("ratio = %#v", got["ratio"])
	}
	if extra, _ := got["extra"].(map[string]any); extra["n"] != json.Number("123456789012345678901234567890") {
		t.Errorf("json param changed: %#v", got["extra"])
	}
	if got["unknown"] != json.Number("1e400") {
		t.Errorf("unrepresentable number = %#v, want it unchanged", got["unknown"])
	}
}
