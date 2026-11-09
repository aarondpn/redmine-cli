//go:build unix

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// startWithExtension starts the CLI running a manifest-less extension that
// reports the signals it gets, in a process group of its own so the test
// can signal redmine alone, the way `kill <pid>`, `timeout` or a
// supervisor does.
func startWithExtension(t *testing.T, args ...string) (*exec.Cmd, *syncBuffer, *syncBuffer) {
	t.Helper()
	extRoot := filepath.Join(t.TempDir(), "extensions")
	dir := filepath.Join(extRoot, "redmine-cli-sleeper")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
trap 'kill $pid 2>/dev/null; echo "got TERM"; exit 0' TERM
trap 'kill $pid 2>/dev/null; echo "got INT"; exit 0' INT
sleep 30 & pid=$!
echo ready
wait $pid
`
	if err := os.WriteFile(filepath.Join(dir, "redmine-cli-sleeper"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], args...)
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "REDMINE_") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, "REDMINE_MAIN_HELPER=1", "REDMINE_NO_UPDATE_CHECK=1",
		"REDMINE_EXTENSIONS_DIR="+extRoot, "REDMINE_CONFIG="+filepath.Join(t.TempDir(), "none.yaml"))
	// Not the terminal's foreground group: a SIGINT can only come from the
	// test, as from any sender other than the terminal.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var stdout, stderr syncBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) })
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(stdout.String(), "ready") {
		if time.Now().After(deadline) {
			t.Fatalf("the extension never started; stderr:\n%s", stderr.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cmd, &stdout, &stderr
}

// TestSignalToRedmineReachesAttachedExtension sends a signal to the redmine
// process only, not its process group: redmine must pass it on to the
// extension and exit, instead of swallowing it and waiting forever.
func TestSignalToRedmineReachesAttachedExtension(t *testing.T) {
	tests := []struct {
		name     string
		sig      syscall.Signal
		args     []string
		wantOut  string
		wantCode int
	}{
		{name: "sigterm", sig: syscall.SIGTERM, args: []string{"sleeper"}, wantOut: "got TERM", wantCode: 143},
		{name: "sigint from outside the terminal", sig: syscall.SIGINT, args: []string{"sleeper"}, wantOut: "got INT", wantCode: 130},
		{name: "sigterm through exec", sig: syscall.SIGTERM, args: []string{"extension", "exec", "sleeper"}, wantOut: "got TERM", wantCode: 143},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, stdout, stderr := startWithExtension(t, tt.args...)
			start := time.Now()
			if err := cmd.Process.Signal(tt.sig); err != nil {
				t.Fatal(err)
			}
			if code := waitExit(t, cmd, stderr); code != tt.wantCode {
				t.Errorf("exit code = %d, want %d; stderr:\n%s", code, tt.wantCode, stderr)
			}
			if took := time.Since(start); took > 5*time.Second {
				t.Errorf("redmine took %s to exit", took)
			}
			if !strings.Contains(stdout.String(), tt.wantOut) {
				t.Errorf("stdout = %q, want %q (the extension did not get the signal)", stdout.String(), tt.wantOut)
			}
		})
	}
}
