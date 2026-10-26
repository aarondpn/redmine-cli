//go:build unix

package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// serveHelperEnv makes TestServeHelperProcess run `mcp serve` instead of
// returning. Its value selects who handles signals: "caller" cancels the
// command context on the first SIGINT and then restores default handling,
// like main does; "none" installs no handler at all.
const serveHelperEnv = "REDMINE_MCP_SERVE_HELPER"

// TestServeHelperProcess is not a real test: the signal tests re-execute the
// test binary with serveHelperEnv set and the serve arguments after "--".
func TestServeHelperProcess(t *testing.T) {
	mode := os.Getenv(serveHelperEnv)
	if mode == "" {
		return
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}

	ctx := context.Background()
	if mode == "caller" {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		go func() {
			<-sigs
			signal.Stop(sigs)
			cancel()
		}()
	}

	f := &cmdutil.Factory{
		ConfigPath: os.Getenv("REDMINE_MCP_SERVE_HELPER_CONFIG"),
		IOStreams:  &cmdutil.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
	}
	cmd := NewCmdMCP(f)
	cmd.SetArgs(append([]string{"serve"}, args...))
	fmt.Fprintln(os.Stderr, "helper ready")
	err := cmd.ExecuteContext(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "serve error:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// lockedBuffer is a bytes.Buffer the test can read while the helper writes.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// startServeHelper runs `mcp serve args...` in a child process and waits
// until its stderr contains ready. Its stdin stays open until the test ends.
func startServeHelper(t *testing.T, mode, ready string, args ...string) (*exec.Cmd, *lockedBuffer) {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := "server: http://127.0.0.1:1\napi_key: test\nauth_method: apikey\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestServeHelperProcess$", "--"}, args...)...)
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "REDMINE_") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, serveHelperEnv+"="+mode, "REDMINE_MCP_SERVE_HELPER_CONFIG="+cfgPath, "REDMINE_NO_KEYRING=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close() })
	var stderr lockedBuffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(stderr.String(), ready) {
		if time.Now().After(deadline) {
			t.Fatalf("serve never reported %q; stderr:\n%s", ready, stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cmd, &stderr
}

// waitServeHelper waits for the helper to exit and returns its wait status.
func waitServeHelper(t *testing.T, cmd *exec.Cmd, stderr *lockedBuffer) syscall.WaitStatus {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		var exitErr *exec.ExitError
		if err != nil && !errors.As(err, &exitErr) {
			t.Fatalf("wait: %v", err)
		}
		return cmd.ProcessState.Sys().(syscall.WaitStatus)
	case <-time.After(10 * time.Second):
		t.Fatalf("serve did not exit after SIGINT; stderr:\n%s", stderr.String())
		return 0
	}
}

// TestServeStopsOnFirstSignal checks that both transports shut down cleanly
// when the caller cancels the command context on the first Ctrl-C.
func TestServeStopsOnFirstSignal(t *testing.T) {
	cases := []struct {
		name  string
		ready string
		args  []string
	}{
		{name: "stdio", ready: "helper ready"},
		{name: "http", ready: "MCP server listening on", args: []string{"--http", "127.0.0.1:0", "--no-auth"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, stderr := startServeHelper(t, "caller", tc.ready, tc.args...)
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				t.Fatal(err)
			}
			ws := waitServeHelper(t, cmd, stderr)
			if ws.Signaled() || ws.ExitStatus() != 0 {
				t.Fatalf("wait status = %v, want a clean exit; stderr:\n%s", ws, stderr.String())
			}
		})
	}
}

// TestServeLeavesSignalsToCaller pins that serve installs no signal handler
// of its own. main restores default handling after the first Ctrl-C so a
// second one kills the process; a handler in serve would swallow it.
func TestServeLeavesSignalsToCaller(t *testing.T) {
	cmd, stderr := startServeHelper(t, "none", "MCP server listening on", "--http", "127.0.0.1:0", "--no-auth")
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	ws := waitServeHelper(t, cmd, stderr)
	if !ws.Signaled() || ws.Signal() != syscall.SIGINT {
		t.Fatalf("wait status = %v, want the process killed by SIGINT (serve caught the signal itself); stderr:\n%s", ws, stderr.String())
	}
}
