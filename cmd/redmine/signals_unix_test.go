//go:build unix

package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a bytes.Buffer that the test can read while the child
// process writes to it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// startCLI starts the CLI like runCLI does, but returns while it runs so the
// test can signal it. stdin stays open (and empty) until the test ends.
func startCLI(t *testing.T, serverURL string, args ...string) (*exec.Cmd, *syncBuffer, *syncBuffer) {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	content := "server: " + serverURL + "\napi_key: test\nauth_method: apikey\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], append([]string{"--config", cfgPath}, args...)...)
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "REDMINE_") {
			env = append(env, kv)
		}
	}
	cmd.Env = append(env, "REDMINE_MAIN_HELPER=1", "REDMINE_NO_UPDATE_CHECK=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close() })
	var stdout, stderr syncBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return cmd, &stdout, &stderr
}

// waitExit waits for cmd and returns its exit code.
func waitExit(t *testing.T, cmd *exec.Cmd, stderr *syncBuffer) int {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		if err != nil {
			t.Fatalf("wait: %v", err)
		}
		return 0
	case <-time.After(10 * time.Second):
		t.Fatalf("CLI did not exit after SIGINT; stderr:\n%s", stderr)
		return -1
	}
}

func TestInterruptCancelsInFlightRequest(t *testing.T) {
	arrived := make(chan struct{})
	aborted := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(arrived)
		select {
		case <-r.Context().Done():
			close(aborted)
		case <-time.After(30 * time.Second):
		}
	}))
	defer srv.Close()

	cmd, stdout, stderr := startCLI(t, srv.URL, "api", "/slow.json")
	select {
	case <-arrived:
	case <-time.After(10 * time.Second):
		t.Fatalf("request never reached the server; stderr:\n%s", stderr)
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if code := waitExit(t, cmd, stderr); code != 130 {
		t.Fatalf("exit code = %d, want 130; stderr:\n%s", code, stderr)
	}
	select {
	case <-aborted:
	case <-time.After(5 * time.Second):
		t.Fatal("the in-flight request was not cancelled")
	}
	if stdout.String() != "" || stderr.String() != "" {
		t.Fatalf("expected no output after an interrupt, got stdout %q, stderr %q", stdout, stderr)
	}
}

// A [y/N] prompt reads stdin in cooked mode and ignores the context; the
// first Ctrl-C must still end the command, as it did before the CLI caught
// signals.
func TestInterruptEndsBlockedPrompt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	cmd, _, stderr := startCLI(t, srv.URL, "issues", "delete", "1")
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(stderr.String(), "[y/N]") {
		if time.Now().After(deadline) {
			t.Fatalf("prompt never shown; stderr:\n%s", stderr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if code := waitExit(t, cmd, stderr); code != 130 {
		t.Fatalf("exit code = %d, want 130; stderr:\n%s", code, stderr)
	}
}
