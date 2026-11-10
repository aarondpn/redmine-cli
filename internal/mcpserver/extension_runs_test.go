//go:build unix

package mcpserver

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/runner"
	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
)

// writeSleeper writes an extension that records its own PID and the PID of
// a child it starts in pidFile, then waits for the child, which sleeps far
// longer than any test runs.
func writeSleeper(t *testing.T, pidFile string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "redmine-cli-sleeper")
	body := "#!/bin/sh\ncat >/dev/null\nsleep 300 &\necho $! >> '" + pidFile + "'\necho $$ >> '" + pidFile + "'\nwait\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// waitForPIDs waits until pidFile lists n PIDs and returns them.
func waitForPIDs(t *testing.T, pidFile string, n int) []int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		data, _ := os.ReadFile(pidFile)
		fields := strings.Fields(string(data))
		if len(fields) >= n {
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
			t.Fatalf("the extension never wrote %d PIDs to %s (got %q)", n, pidFile, data)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// assertProcessesGone fails unless every pid has exited (and was reaped)
// within a few seconds.
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
				_ = syscall.Kill(pid, syscall.SIGKILL)
				t.Fatalf("process %d is still running after the server stopped (kill: %v)", pid, err)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func TestExtensionRuns_StopKillsRunningExtensions(t *testing.T) {
	unsetEnv(t, protocolEnv...)
	pidFile := filepath.Join(t.TempDir(), "pids")
	tools := []ExtensionTool{{
		Extension: "sleeper",
		Path:      writeSleeper(t, pidFile),
		Command:   &spec.Command{Name: "wait", Description: "Sleeps"},
		Env:       runner.Env{},
	}}
	runs := NewExtensionRuns()
	cs := newExtensionSession(t, Options{ExtensionTools: tools, ExtensionRuns: runs})

	type callResult struct {
		res *mcp.CallToolResult
		err error
	}
	called := make(chan callResult, 1)
	go func() {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "sleeper_wait", Arguments: map[string]any{}})
		called <- callResult{res, err}
	}()
	pids := waitForPIDs(t, pidFile, 2)

	start := time.Now()
	if !runs.Stop(ExtensionShutdownTimeout) {
		t.Fatal("Stop timed out waiting for the extension run")
	}
	if elapsed := time.Since(start); elapsed > ExtensionShutdownTimeout {
		t.Errorf("Stop took %s", elapsed)
	}
	assertProcessesGone(t, pids)

	select {
	case got := <-called:
		if got.err != nil {
			t.Fatalf("CallTool: %v", got.err)
		}
		if !got.res.IsError || !strings.Contains(resultText(t, got.res), "cancelled") {
			t.Errorf("result = %+v, want a cancellation error", got.res.Content)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the tool call did not return after Stop")
	}

	// A stopped tracker starts nothing new.
	if err := os.Remove(pidFile); err != nil {
		t.Fatal(err)
	}
	res := callTool(t, cs, "sleeper_wait", map[string]any{})
	if !res.IsError || !strings.Contains(resultText(t, res), "shutting down") {
		t.Errorf("result after Stop = %+v, want a shutting down error", res.Content)
	}
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Errorf("the extension ran after Stop (stat: %v)", err)
	}
	if !runs.Stop(time.Second) {
		t.Error("a second Stop should return at once")
	}
}

func TestExtensionRuns_NilTracksNothing(t *testing.T) {
	var runs *ExtensionRuns
	ctx := context.Background()
	got, done, ok := runs.begin(ctx)
	if !ok || got != ctx {
		t.Fatalf("begin on nil = %v, %v", got, ok)
	}
	done()
	if !runs.Stop(0) {
		t.Error("Stop on nil should report success")
	}
}

func TestExtensionRuns_StopWaitsOnlyUntilTimeout(t *testing.T) {
	runs := NewExtensionRuns()
	ctx, done, ok := runs.begin(context.Background())
	if !ok {
		t.Fatal("begin refused before Stop")
	}
	defer done()
	if runs.Stop(50 * time.Millisecond) {
		t.Error("Stop reported success while a run had not finished")
	}
	if ctx.Err() == nil || !errors.Is(context.Cause(ctx), errShuttingDown) {
		t.Errorf("run context not cancelled by Stop: %v", context.Cause(ctx))
	}
}
