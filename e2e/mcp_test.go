//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpServerArgs returns the standard `mcp serve` argv with the active
// runner's config wired in. Extra args are appended verbatim.
func (r *cliRunner) mcpServerArgs(extra ...string) []string {
	args := []string{"--config", r.configPath, "mcp", "serve"}
	return append(args, extra...)
}

// startMCPStdio spawns the CLI as a subprocess on the stdio transport and
// returns a connected MCP client session. Cleanup terminates the subprocess
// and surfaces its stderr on test failure.
func (r *cliRunner) startMCPStdio(t *testing.T, ctx context.Context, extraArgs ...string) *mcp.ClientSession {
	t.Helper()

	cmd := exec.Command(builtCLIPath, r.mcpServerArgs(extraArgs...)...)
	cmd.Dir = r.repoRoot
	cmd.Env = append(os.Environ(), "REDMINE_NO_UPDATE_CHECK=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	transport := &mcp.CommandTransport{Command: cmd}
	client := mcp.NewClient(&mcp.Implementation{Name: "redmine-cli-e2e", Version: "v0"}, nil)

	session, err := client.Connect(ctx, transport, nil)
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

// startMCPHTTP spawns the CLI on a free localhost port over HTTP and returns a
// connected MCP client session plus the chosen base URL. authToken, when
// non-empty, is passed via --auth-token and the client is configured to send
// it on every request.
func (r *cliRunner) startMCPHTTP(t *testing.T, ctx context.Context, authToken string, extraArgs ...string) (*mcp.ClientSession, string) {
	t.Helper()

	port := freeLoopbackPort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	args := []string{"--http", addr}
	if authToken != "" {
		args = append(args, "--auth-token", authToken)
	}
	args = append(args, extraArgs...)

	cmd := exec.Command(builtCLIPath, r.mcpServerArgs(args...)...)
	cmd.Dir = r.repoRoot
	cmd.Env = append(os.Environ(), "REDMINE_NO_UPDATE_CHECK=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start MCP HTTP: %v", err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
		if t.Failed() && stderr.Len() > 0 {
			t.Logf("MCP server stderr:\n%s", stderr.String())
		}
	})

	endpoint := "http://" + addr
	if err := waitForHTTP(ctx, endpoint, 10*time.Second); err != nil {
		t.Fatalf("MCP HTTP server never became ready at %s: %v\nstderr:\n%s", addr, err, stderr.String())
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	if authToken != "" {
		httpClient.Transport = &authedTransport{base: http.DefaultTransport, token: authToken}
	}
	transport := &mcp.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           httpClient,
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "redmine-cli-e2e", Version: "v0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("connect MCP HTTP: %v\nstderr:\n%s", err, stderr.String())
	}
	t.Cleanup(func() { _ = session.Close() })

	return session, endpoint
}

// authedTransport adds an Authorization: Bearer header to every request.
type authedTransport struct {
	base  http.RoundTripper
	token string
}

func (a *authedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+a.token)
	return a.base.RoundTrip(clone)
}

// freeLoopbackPort reserves an OS-assigned port on 127.0.0.1, releases it,
// and returns the port number. There is a small race between release and the
// child process binding it; in practice this is fine for sequential e2e runs.
func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("release port: %v", err)
	}
	return port
}

// waitForHTTP polls endpoint until a TCP connection succeeds or the timeout
// elapses. The MCP server returns 4xx for unrelated requests, which still
// proves the listener is up; we just need the connect to succeed.
func waitForHTTP(ctx context.Context, endpoint string, timeout time.Duration) error {
	host := strings.TrimPrefix(endpoint, "http://")
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		conn, err := net.DialTimeout("tcp", host, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("listener at %s did not come up within %s", endpoint, timeout)
}

// toolNames extracts the tool names from a tools/list response, sorted by
// the order returned. Tests should compare via maps for stable assertions.
func toolNames(tools []*mcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

func TestMCPStdio_HandshakeAndReadTool(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := r.startMCPStdio(t, ctx)

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := toolNames(tools.Tools)
	if len(names) == 0 {
		t.Fatal("ListTools returned no tools")
	}
	mustContain(t, names, "list_projects", "get_issue", "list_issues")

	// list_projects against the live Redmine proves the tool actually
	// reaches the API, not just that it was registered.
	out, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_projects"})
	if err != nil {
		t.Fatalf("CallTool list_projects: %v", err)
	}
	if out.IsError {
		t.Fatalf("list_projects returned IsError: %s", structuredJSON(out))
	}
	if len(out.Content) == 0 {
		t.Fatal("list_projects returned empty content")
	}
}

func TestMCPStdio_WritesGatedByFlag(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := r.startMCPStdio(t, ctx)

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, name := range toolNames(tools.Tools) {
		switch name {
		case "create_issue", "update_issue", "delete_issue", "create_project", "delete_project":
			t.Errorf("write tool %q exposed without --enable-writes", name)
		}
	}
}

func TestMCPStdio_GroupFilter(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := r.startMCPStdio(t, ctx, "--enable-groups", "issues")

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	names := toolNames(tools.Tools)
	if len(names) == 0 {
		t.Fatal("issues group should expose at least one tool")
	}
	mustContain(t, names, "list_issues", "get_issue")
	for _, name := range names {
		// list_projects belongs to the projects group; should be hidden.
		if name == "list_projects" || name == "get_project" || name == "list_users" {
			t.Errorf("--enable-groups issues leaked non-issues tool %q", name)
		}
	}
}

func TestMCPHTTP_HandshakeAndReadTool(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session, _ := r.startMCPHTTP(t, ctx, "")

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools over HTTP: %v", err)
	}
	if len(tools.Tools) == 0 {
		t.Fatal("ListTools over HTTP returned no tools")
	}

	out, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_projects"})
	if err != nil {
		t.Fatalf("CallTool list_projects over HTTP: %v", err)
	}
	if out.IsError {
		t.Fatalf("list_projects returned IsError over HTTP: %s", structuredJSON(out))
	}
}

func TestMCPHTTP_AuthTokenGate(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const token = "e2e-secret-token-do-not-reuse"
	session, endpoint := r.startMCPHTTP(t, ctx, token)

	if _, err := session.ListTools(ctx, nil); err != nil {
		t.Fatalf("ListTools with valid token: %v", err)
	}

	// Probe the same endpoint with no token: must be 401.
	rejectClient := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		t.Fatalf("build probe request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := rejectClient.Do(req)
	if err != nil {
		t.Fatalf("probe without token: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status without token = %d, want 401", resp.StatusCode)
	}
	if got := resp.Header.Get("WWW-Authenticate"); got == "" {
		t.Error("expected WWW-Authenticate header on 401 from auth-gated server")
	}
}

func mustContain(t *testing.T, haystack []string, needles ...string) {
	t.Helper()
	set := make(map[string]bool, len(haystack))
	for _, h := range haystack {
		set[h] = true
	}
	for _, n := range needles {
		if !set[n] {
			t.Errorf("expected %q in tool list, got %v", n, haystack)
		}
	}
}

func structuredJSON(out *mcp.CallToolResult) string {
	b, err := json.Marshal(out)
	if err != nil {
		return fmt.Sprintf("<unmarshalable: %v>", err)
	}
	return string(b)
}
