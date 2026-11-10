//go:build unix

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	script := `#!/bin/sh
trap 'kill $pid 2>/dev/null; echo "got TERM"; exit 0' TERM
trap 'kill $pid 2>/dev/null; echo "got INT"; exit 0' INT
trap 'kill $pid 2>/dev/null; echo "got HUP"; exit 0' HUP
sleep 30 & pid=$!
echo ready
wait $pid
`
	return startExtensionCLI(t, "sleeper", script, "", nil, args...)
}

// startExtensionCLI installs extension name (script as its executable,
// manifest unless empty) in a fresh extensions directory and starts the CLI
// with args and the extra environment env, in a process group of its own.
// It returns once the extension printed "ready" (on stdout or stderr).
func startExtensionCLI(t *testing.T, name, script, manifest string, env []string, args ...string) (*exec.Cmd, *syncBuffer, *syncBuffer) {
	t.Helper()
	extRoot := filepath.Join(t.TempDir(), "extensions")
	dir := filepath.Join(extRoot, "redmine-cli-"+name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "redmine-cli-"+name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if manifest != "" {
		if err := os.WriteFile(filepath.Join(dir, "redmine-extension.yml"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(os.Args[0], args...)
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
	// A declarative command's stdout is captured; its stderr is streamed.
	for !strings.Contains(stdout.String(), "ready") && !strings.Contains(stderr.String(), "ready") {
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
		{name: "sighup", sig: syscall.SIGHUP, args: []string{"sleeper"}, wantOut: "got HUP", wantCode: 129},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, stdout, stderr := startWithExtension(t, tt.args...)
			if err := cmd.Process.Signal(tt.sig); err != nil {
				t.Fatal(err)
			}
			// The extension would sleep for 30 seconds: exiting within
			// waitExit's deadline with the trap's output shows the signal
			// was passed on, without a tighter wall-clock bound.
			if code := waitExit(t, cmd, stderr); code != tt.wantCode {
				t.Errorf("exit code = %d, want %d; stderr:\n%s", code, tt.wantCode, stderr)
			}
			if !strings.Contains(stdout.String(), tt.wantOut) {
				t.Errorf("stdout = %q, want %q (the extension did not get the signal)", stdout.String(), tt.wantOut)
			}
		})
	}
}

// TestHangupKillsDeclarativeRun sends SIGHUP (the terminal closed) to
// redmine while a declarative command runs in its own process group: the
// group must be killed, not left running without its parent.
func TestHangupKillsDeclarativeRun(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	script := `#!/bin/sh
sleep 300 &
echo $! > "$HUP_PID_FILE"
echo ready >&2
wait
`
	manifest := `schema_version: 1
name: hup
description: Test extension
commands:
  - name: wait
    description: Waits
`
	cmd, _, stderr := startExtensionCLI(t, "hup", script, manifest, []string{"HUP_PID_FILE=" + pidFile}, "hup", "wait")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })

	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	if code := waitExit(t, cmd, stderr); code != 129 {
		t.Errorf("exit code = %d, want 129; stderr:\n%s", code, stderr)
	}
	deadline := time.Now().Add(10 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			t.Fatal("the extension's process group survived the hangup")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
