//go:build unix

package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	extmgr "github.com/aarondpn/redmine-cli/v2/internal/extension"
)

// serveHelperEnv makes TestServeHelperProcess run `mcp serve` instead of
// returning. Its value selects who handles signals: "caller" cancels the
// command context on the first SIGINT and then restores default handling,
// like main does; "none" installs no handler at all.
const serveHelperEnv = "REDMINE_MCP_SERVE_HELPER"

// serveHelperExtensionsEnv, when set, is the extensions directory the
// helper's serve lists.
const serveHelperExtensionsEnv = "REDMINE_MCP_SERVE_HELPER_EXTENSIONS"

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
	var opts Options
	if root := os.Getenv(serveHelperExtensionsEnv); root != "" {
		m := &extmgr.Manager{Root: root, CLIVersion: "dev"}
		opts.Extensions = m.List
	}
	cmd := NewCmdMCP(f, opts)
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
	cmd, _, stderr := startServeHelperWith(t, mode, ready, nil, args...)
	return cmd, stderr
}

// startServeHelperWith is startServeHelper with extra environment
// variables for the child, and returns its stdin.
func startServeHelperWith(t *testing.T, mode, ready string, extraEnv []string, args ...string) (*exec.Cmd, io.Writer, *lockedBuffer) {
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
	cmd.Env = append(cmd.Env, extraEnv...)
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
	return cmd, stdin, &stderr
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

// sleeperManifest is the manifest of the extension the shutdown tests call:
// its one command runs until it is killed.
const sleeperManifest = `schema_version: 1
name: sleeper
description: Sleeps until killed
commands:
  - name: wait
    description: Sleep
`

// installSleeper installs the sleeper extension below a fresh root and
// returns the root and the file the extension writes its PIDs to: its own
// and that of a child that sleeps far longer than any test runs.
func installSleeper(t *testing.T) (root, pidFile string) {
	t.Helper()
	root = t.TempDir()
	pidFile = filepath.Join(t.TempDir(), "pids")
	dir := filepath.Join(root, extmgr.Prefix+"sleeper")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncat >/dev/null\nsleep 300 &\necho $! >> '" + pidFile + "'\necho $$ >> '" + pidFile + "'\nwait\n"
	if err := os.WriteFile(filepath.Join(dir, extmgr.Prefix+"sleeper"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "redmine-extension.yml"), []byte(sleeperManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, pidFile
}

// waitForPIDs waits until pidFile lists n PIDs and returns them.
func waitForPIDs(t *testing.T, pidFile string, n int, stderr *lockedBuffer) []int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		data, _ := os.ReadFile(pidFile)
		if fields := strings.Fields(string(data)); len(fields) >= n {
			pids := make([]int, 0, len(fields))
			for _, f := range fields {
				pid, err := strconv.Atoi(f)
				if err != nil {
					t.Fatalf("bad pid %q", f)
				}
				pids = append(pids, pid)
			}
			return pids
		}
		if time.Now().After(deadline) {
			t.Fatalf("the extension never started; stderr:\n%s", stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// assertProcessesGone fails unless every pid has exited within a few
// seconds. Survivors are killed so they do not outlive the test.
func assertProcessesGone(t *testing.T, pids []int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for _, pid := range pids {
		for {
			err := syscall.Kill(pid, 0)
			if errors.Is(err, syscall.ESRCH) {
				break
			}
			if time.Now().After(deadline) {
				for _, p := range pids {
					_ = syscall.Kill(p, syscall.SIGKILL)
				}
				t.Fatalf("extension process %d outlived the MCP server", pid)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

// TestServeStopsRunningExtensionsOnSignal interrupts the server while an
// extension tool call is in flight: the extension runs in its own process
// group, so the server must kill it before it exits, on both transports.
func TestServeStopsRunningExtensionsOnSignal(t *testing.T) {
	const callTool = `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"sleeper_wait","arguments":{}}}`
	cases := []struct {
		name  string
		ready string
		args  []string
		// call starts the tool call and returns a channel that is closed
		// when its response arrived (never closed for stdio, whose
		// output the test does not read).
		call func(t *testing.T, stdin io.Writer, stderr *lockedBuffer) <-chan struct{}
	}{
		{
			name:  "stdio",
			ready: "helper ready",
			call: func(t *testing.T, stdin io.Writer, _ *lockedBuffer) <-chan struct{} {
				msgs := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}` + "\n" +
					`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" + callTool + "\n"
				if _, err := io.WriteString(stdin, msgs); err != nil {
					t.Fatal(err)
				}
				return nil
			},
		},
		{
			name:  "http",
			ready: "MCP server listening on",
			args:  []string{"--http", "127.0.0.1:0", "--no-auth", "--enable-extensions"},
			call: func(t *testing.T, _ io.Writer, stderr *lockedBuffer) <-chan struct{} {
				m := regexp.MustCompile(`MCP server listening on (http://\S+)`).FindStringSubmatch(stderr.String())
				if m == nil {
					t.Fatalf("no listen address in stderr:\n%s", stderr.String())
				}
				req, err := http.NewRequest(http.MethodPost, m[1], strings.NewReader(callTool))
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", "application/json, text/event-stream")
				answered := make(chan struct{})
				go func() {
					resp, err := http.DefaultClient.Do(req)
					if err == nil {
						_, _ = io.Copy(io.Discard, resp.Body)
						_ = resp.Body.Close()
						close(answered)
					}
				}()
				return answered
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, pidFile := installSleeper(t)
			cmd, stdin, stderr := startServeHelperWith(t, "caller", tc.ready, []string{serveHelperExtensionsEnv + "=" + root}, tc.args...)
			answered := tc.call(t, stdin, stderr)
			pids := waitForPIDs(t, pidFile, 2, stderr)

			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				t.Fatal(err)
			}
			ws := waitServeHelper(t, cmd, stderr)
			if ws.Signaled() || ws.ExitStatus() != 0 {
				t.Fatalf("wait status = %v, want a clean exit; stderr:\n%s", ws, stderr.String())
			}
			assertProcessesGone(t, pids)
			if answered != nil {
				select {
				case <-answered:
				case <-time.After(5 * time.Second):
					t.Error("the in-flight tool call got no response before the server exited")
				}
			}
		})
	}
}
