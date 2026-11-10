//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// readOnlyRefusal is how `redmine api` refuses a write in read-only mode,
// before anything is sent (see api.ErrReadOnly).
const readOnlyRefusal = "read-only mode enabled: refusing POST /projects.json"

// installE2EExtension installs the fixture extension of
// testdata/redmine-cli-e2eext as a local extension in a fresh extensions
// directory and returns the environment that selects that directory. The
// fixture calls `$REDMINE_CLI_BIN api` like a real extension.
func installE2EExtension(t *testing.T, r *cliRunner) []string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fixture extension is a shell script")
	}
	src := filepath.Join(r.repoRoot, "e2e", "testdata", "redmine-cli-e2eext")
	dir := filepath.Join(t.TempDir(), "redmine-cli-e2eext")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, mode := range map[string]os.FileMode{"redmine-cli-e2eext": 0o755, "redmine-extension.yml": 0o644} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, mode); err != nil {
			t.Fatal(err)
		}
	}
	env := []string{"REDMINE_EXTENSIONS_DIR=" + filepath.Join(t.TempDir(), "extensions")}
	r.runWithEnv(t, env, "extension", "install", dir)
	return env
}

// TestExtensions_CLI runs the fixture through `redmine <ext> <command>`:
// the nested `redmine api` call must use the forwarded config and profile,
// and a read command must not be able to write.
func TestExtensions_CLI(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	env := installE2EExtension(t, r)

	t.Run("nested api call uses the forwarded profile", func(t *testing.T) {
		var resp struct {
			User struct {
				Login string `json:"login"`
			} `json:"user"`
		}
		r.runJSONWithEnv(t, env, &resp, "e2eext", "whoami")
		if resp.User.Login != e2eUsername() {
			t.Fatalf("login = %q, want %q", resp.User.Login, e2eUsername())
		}
	})

	t.Run("extension exec runs it too", func(t *testing.T) {
		var resp struct {
			User struct {
				Login string `json:"login"`
			} `json:"user"`
		}
		r.runJSONWithEnv(t, env, &resp, "extension", "exec", "e2eext", "whoami")
		if resp.User.Login != e2eUsername() {
			t.Fatalf("login = %q, want %q", resp.User.Login, e2eUsername())
		}
	})

	writeTests := []struct {
		name        string
		args        []string
		wantRefusal bool
	}{
		{name: "read command cannot write", args: []string{"e2eext", "try-write"}, wantRefusal: true},
		{name: "--read-only blocks a write command", args: []string{"--read-only", "e2eext", "try-write-allowed"}, wantRefusal: true},
		// The control: without read-only mode the POST reaches Redmine,
		// which rejects the empty body, so the refusal above is real.
		{name: "write command may write", args: []string{"e2eext", "try-write-allowed"}},
	}
	for _, tt := range writeTests {
		t.Run(tt.name, func(t *testing.T) {
			_, stderr, err := r.runRawWithEnv(env, tt.args...)
			if err == nil {
				t.Fatalf("the empty POST succeeded; stderr:\n%s", stderr)
			}
			if got := strings.Contains(string(stderr), readOnlyRefusal); got != tt.wantRefusal {
				t.Fatalf("read-only refusal = %v, want %v; stderr:\n%s", got, tt.wantRefusal, stderr)
			}
			if !tt.wantRefusal && !bytes.Contains(stderr, []byte("extension_failed")) {
				t.Errorf("want the extension failure envelope; stderr:\n%s", stderr)
			}
		})
	}
}

// TestExtensions_ReadOnlyProfile uses a profile with read_only: true. The
// nested `redmine api` call loads the same profile, so it is read-only too,
// unless the invocation turned read-only off with --read-only=false.
func TestExtensions_ReadOnlyProfile(t *testing.T) {
	requireE2E(t)
	r := newRunnerWithConfig(t, fmt.Sprintf(`active_profile: ro
profiles:
  ro:
    server: %s
    api_key: %s
    auth_method: apikey
    output_format: json
    read_only: true
`, e2eBaseURL(), e2eAPIKey()))
	env := installE2EExtension(t, r)
	tests := []struct {
		name        string
		args        []string
		wantRefusal bool
	}{
		{name: "the profile's read-only mode reaches the nested call", args: []string{"e2eext", "try-write-allowed"}, wantRefusal: true},
		{name: "--read-only=false reaches the nested call", args: []string{"--read-only=false", "e2eext", "try-write-allowed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, stderr, err := r.runRawWithEnv(env, tt.args...)
			if err == nil {
				t.Fatalf("the empty POST succeeded; stderr:\n%s", stderr)
			}
			if got := strings.Contains(string(stderr), readOnlyRefusal); got != tt.wantRefusal {
				t.Fatalf("read-only refusal = %v, want %v; stderr:\n%s", got, tt.wantRefusal, stderr)
			}
		})
	}
}

// startMCPStdioWithEnv is startMCPStdio with extra environment variables.
func (r *cliRunner) startMCPStdioWithEnv(t *testing.T, ctx context.Context, extraEnv []string, extraArgs ...string) *mcp.ClientSession {
	t.Helper()
	cmd := exec.Command(builtCLIPath, r.mcpServerArgs(extraArgs...)...)
	cmd.Dir = r.repoRoot
	cmd.Env = append(append(os.Environ(), "REDMINE_NO_UPDATE_CHECK=1"), extraEnv...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	client := mcp.NewClient(&mcp.Implementation{Name: "redmine-cli-e2e", Version: "v0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect MCP stdio: %v\nserver stderr:\n%s", err, stderr.String())
	}
	t.Cleanup(func() {
		_ = session.Close()
		if t.Failed() && stderr.Len() > 0 {
			t.Logf("MCP server stderr:\n%s", stderr.String())
		}
	})
	return session
}

// TestExtensions_MCP runs the fixture's tools over MCP stdio, where
// extensions are on by default: the same nested call, its result as
// structured content without the API key, and read-only enforcement.
func TestExtensions_MCP(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	env := installE2EExtension(t, r)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	session := r.startMCPStdioWithEnv(t, ctx, env)

	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range list.Tools {
		names = append(names, tool.Name)
	}
	for _, want := range []string{"e2eext_whoami", "e2eext_try_write"} {
		if !slices.Contains(names, want) {
			t.Errorf("tools/list lacks %s", want)
		}
	}
	if slices.Contains(names, "e2eext_try_write_allowed") {
		t.Error("a writing extension tool is listed without --enable-writes")
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "e2eext_whoami", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("e2eext_whoami failed: %+v", res.Content)
	}
	body, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var whoami struct {
		User map[string]any `json:"user"`
	}
	if err := json.Unmarshal(body, &whoami); err != nil {
		t.Fatalf("structured content %s: %v", body, err)
	}
	if whoami.User["login"] != e2eUsername() {
		t.Fatalf("login = %v, want %q (structured content %s)", whoami.User["login"], e2eUsername(), body)
	}
	if _, leaked := whoami.User["api_key"]; leaked {
		t.Errorf("structured content leaks api_key: %s", body)
	}
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok && strings.Contains(tc.Text, e2eAPIKey()) {
			t.Errorf("text content leaks the API key")
		}
	}

	res, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "e2eext_try_write", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !res.IsError || !strings.Contains(text, readOnlyRefusal) {
		t.Fatalf("e2eext_try_write: isError=%v text=%q, want the read-only refusal", res.IsError, text)
	}
}
