//go:build !windows

package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
)

// script writes an executable sh script and returns its path.
func script(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "redmine-cli-test")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		req        Request
		wantStdout string
		wantStderr string
		wantCode   int
		wantErr    string
	}{
		{
			name:       "stdin echoed",
			body:       `cat`,
			req:        Request{Stdin: []byte(`{"a":1}`)},
			wantStdout: `{"a":1}`,
		},
		{
			name:       "args and env",
			body:       `printf '%s|%s' "$1" "$FOO"`,
			req:        Request{Args: []string{"x y"}, Env: []string{"FOO=bar"}},
			wantStdout: "x y|bar",
		},
		{
			name:       "exit code and stderr",
			body:       `echo partial; echo "boom" >&2; exit 3`,
			wantStdout: "partial\n",
			wantStderr: "boom\n",
			wantCode:   3,
			wantErr:    "exit status 3: boom",
		},
		{
			name:     "exit without stderr",
			body:     `exit 7`,
			wantCode: 7,
			wantErr:  "exit status 7",
		},
		{
			name:     "killed by signal",
			body:     `kill -TERM $$; sleep 5`,
			wantCode: 128 + int(syscall.SIGTERM),
			wantErr:  "exit status 143",
		},
		{
			name:    "stdout cap kills the process",
			body:    `while :; do echo 0123456789; done`,
			req:     Request{MaxStdout: 1000, Timeout: 20 * time.Second},
			wantErr: "output exceeded 1000 bytes",
		},
		{
			name:    "timeout",
			body:    `sleep 30`,
			req:     Request{Timeout: 200 * time.Millisecond},
			wantErr: "timed out after 200ms",
		},
		{
			name:     "stderr keeps the tail",
			body:     `i=0; while [ $i -lt 200 ]; do echo "line $i" >&2; i=$((i+1)); done; exit 1`,
			req:      Request{MaxStderr: 100},
			wantCode: 1,
			wantErr:  "line 199",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.req
			req.Path = script(t, tt.body)
			res, err := Run(context.Background(), req)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Run: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
			if res == nil {
				t.Fatal("nil result")
			}
			if tt.wantStdout != "" && string(res.Stdout) != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", res.Stdout, tt.wantStdout)
			}
			if tt.wantStderr != "" && string(res.Stderr) != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", res.Stderr, tt.wantStderr)
			}
			if res.ExitCode != tt.wantCode {
				t.Errorf("exit code = %d, want %d", res.ExitCode, tt.wantCode)
			}
			if req.MaxStdout > 0 && int64(len(res.Stdout)) > req.MaxStdout {
				t.Errorf("stdout has %d bytes, cap %d", len(res.Stdout), req.MaxStdout)
			}
			if req.MaxStderr > 0 {
				if int64(len(res.Stderr)) > req.MaxStderr {
					t.Errorf("stderr has %d bytes, cap %d", len(res.Stderr), req.MaxStderr)
				}
				if !res.StderrTruncated {
					t.Error("StderrTruncated not set")
				}
			}
		})
	}
}

func TestRunErrorTypes(t *testing.T) {
	path := script(t, `sleep 30`)
	_, err := Run(context.Background(), Request{Path: path, Timeout: 50 * time.Millisecond})
	var timeout *TimeoutError
	if !errors.As(err, &timeout) || timeout.Timeout != 50*time.Millisecond {
		t.Fatalf("err = %#v, want *TimeoutError", err)
	}

	path = script(t, `exit 4`)
	_, err = Run(context.Background(), Request{Path: path})
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 4 {
		t.Fatalf("err = %#v, want *ExitError code 4", err)
	}

	path = script(t, `yes`)
	_, err = Run(context.Background(), Request{Path: path, MaxStdout: 10})
	var tooLarge *OutputTooLargeError
	if !errors.As(err, &tooLarge) || tooLarge.Limit != 10 {
		t.Fatalf("err = %#v, want *OutputTooLargeError", err)
	}
}

// TestRunTimeoutKillsProcessGroup checks that a child the extension started
// dies with it, so a timed-out "redmine api" call cannot linger.
func TestRunTimeoutKillsProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	path := script(t, `sleep 60 & echo $! > "`+pidFile+`"; wait`)
	start := time.Now()
	_, err := Run(context.Background(), Request{Path: path, Timeout: 300 * time.Millisecond})
	var timeout *TimeoutError
	if !errors.As(err, &timeout) {
		t.Fatalf("err = %v, want timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("Run took %s after the timeout", elapsed)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("grandchild %d still alive after the group kill", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestRunContextCancel(t *testing.T) {
	path := script(t, `sleep 30`)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	_, err := Run(ctx, Request{Path: path})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestRunStreamsStderr(t *testing.T) {
	path := script(t, `echo live >&2; echo '{}'`)
	var live bytes.Buffer
	res, err := Run(context.Background(), Request{Path: path, Stderr: &live})
	if err != nil {
		t.Fatal(err)
	}
	if live.String() != "live\n" || string(res.Stderr) != "live\n" {
		t.Fatalf("live = %q, captured = %q", live.String(), res.Stderr)
	}
}

func TestRunMissingExecutable(t *testing.T) {
	_, err := Run(context.Background(), Request{Path: filepath.Join(t.TempDir(), "missing")})
	if err == nil {
		t.Fatal("expected an error")
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("missing executable reported as exit: %v", err)
	}
}

func TestRunCommand(t *testing.T) {
	path := script(t, `cat`)
	c := &spec.Command{Name: "list", Timeout: "5s"}
	res, err := RunCommand(context.Background(), path, c, map[string]any{"issue_id": int64(12)}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(res.Stdout, &got); err != nil || got["issue_id"] != float64(12) {
		t.Fatalf("stdin = %s (%v)", res.Stdout, err)
	}

	res, err = RunCommand(context.Background(), path, c, nil, nil, nil)
	if err != nil || string(res.Stdout) != "{}" {
		t.Fatalf("nil input: stdout = %q, err = %v", res.Stdout, err)
	}

	slow := &spec.Command{Name: "slow", Timeout: "100ms"}
	_, err = RunCommand(context.Background(), script(t, `sleep 30`), slow, nil, nil, nil)
	var timeout *TimeoutError
	if !errors.As(err, &timeout) {
		t.Fatalf("err = %v, want the command timeout", err)
	}
}

func TestRunAttached(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		args       []string
		stdin      string
		timeout    time.Duration
		wantCode   int
		wantStdout string
		wantStderr string
		wantErr    string
	}{
		{name: "args and stdio", body: `printf '%s,' "$@"; cat; echo err >&2`, args: []string{"shell", "-x"}, stdin: "in", wantStdout: "shell,-x,in", wantStderr: "err\n"},
		{name: "exit code", body: `exit 42`, wantCode: 42},
		{name: "timeout", body: `sleep 30`, timeout: 100 * time.Millisecond, wantErr: "timed out"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code, err := RunAttached(context.Background(), AttachedRequest{
				Path:    script(t, tt.body),
				Args:    tt.args,
				Stdin:   strings.NewReader(tt.stdin),
				Stdout:  &stdout,
				Stderr:  &stderr,
				Timeout: tt.timeout,
			})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if code != tt.wantCode || stdout.String() != tt.wantStdout || stderr.String() != tt.wantStderr {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

// TestRunOverflowThenExitZero covers a process that writes just past the
// limit and exits 0 right away, before the overflow kill can reach it: the
// truncated output must still be an OutputTooLargeError, never a result.
func TestRunOverflowThenExitZero(t *testing.T) {
	const limit = 69000
	tests := []struct {
		name    string
		bytes   int
		wantErr bool
	}{
		{name: "one byte over", bytes: limit + 1, wantErr: true},
		{name: "well over", bytes: limit + 1000, wantErr: true},
		{name: "exactly the limit", bytes: limit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := script(t, `head -c `+strconv.Itoa(tt.bytes)+` /dev/zero`)
			// The race is timing dependent; repeat to hit both orders.
			for i := range 25 {
				res, err := Run(context.Background(), Request{Path: path, MaxStdout: limit})
				var tooLarge *OutputTooLargeError
				if tt.wantErr != errors.As(err, &tooLarge) {
					t.Fatalf("run %d: err = %v, want OutputTooLargeError: %v", i, err, tt.wantErr)
				}
				if !tt.wantErr && (err != nil || len(res.Stdout) != limit) {
					t.Fatalf("run %d: err = %v, stdout %d bytes", i, err, len(res.Stdout))
				}
			}
		})
	}
}

// lockedBuffer is a bytes.Buffer the test reads while the process writes.
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

// signalCause is the cancellation cause of a context cancelled by a signal.
type signalCause struct{ sig os.Signal }

func (c *signalCause) Error() string     { return c.sig.String() }
func (c *signalCause) Signal() os.Signal { return c.sig }

// TestRunAttachedPassesSignalCause cancels an attached run the way main
// does when redmine itself gets a SIGTERM: the extension must receive the
// signal (and may shut down cleanly), not be killed outright, while a plain
// cancellation still kills it.
func TestRunAttachedPassesSignalCause(t *testing.T) {
	body := `trap 'kill $pid 2>/dev/null; echo "got TERM"; exit 0' TERM
sleep 30 & pid=$!
echo ready
wait $pid`
	tests := []struct {
		name    string
		cause   error
		wantOut string
		maxTime time.Duration
	}{
		{name: "signal cause is passed on", cause: &signalCause{sig: syscall.SIGTERM}, wantOut: "got TERM", maxTime: waitDelay},
		{name: "plain cancellation kills", cause: errors.New("stop"), maxTime: waitDelay + 5*time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			var stdout lockedBuffer
			done := make(chan error, 1)
			go func() {
				_, err := RunAttached(ctx, AttachedRequest{Path: script(t, body), Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &bytes.Buffer{}})
				done <- err
			}()
			deadline := time.Now().Add(10 * time.Second)
			for !strings.Contains(stdout.String(), "ready") {
				if time.Now().After(deadline) {
					t.Fatal("the extension never started")
				}
				time.Sleep(10 * time.Millisecond)
			}
			start := time.Now()
			cancel(tt.cause)
			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), "extension cancelled") {
					t.Errorf("err = %v, want the cancellation", err)
				}
			case <-time.After(15 * time.Second):
				t.Fatal("RunAttached did not return after the cancellation")
			}
			if took := time.Since(start); took > tt.maxTime {
				t.Errorf("took %s, want at most %s", took, tt.maxTime)
			}
			if got := stdout.String(); tt.wantOut != "" && !strings.Contains(got, tt.wantOut) {
				t.Errorf("stdout = %q, want %q", got, tt.wantOut)
			} else if tt.wantOut == "" && strings.Contains(got, "got TERM") {
				t.Errorf("a plain cancellation sent SIGTERM: %q", got)
			}
		})
	}
}

// TestRunAttachedKillsWhenSignalIgnored covers an extension that ignores
// the passed-on signal: it is killed waitDelay later, so redmine cannot
// hang on it.
func TestRunAttachedKillsWhenSignalIgnored(t *testing.T) {
	body := `trap '' TERM
echo ready
while :; do sleep 1; done`
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	var stdout lockedBuffer
	done := make(chan error, 1)
	go func() {
		_, err := RunAttached(ctx, AttachedRequest{Path: script(t, body), Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &bytes.Buffer{}})
		done <- err
	}()
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(stdout.String(), "ready") {
		if time.Now().After(deadline) {
			t.Fatal("the extension never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel(&signalCause{sig: syscall.SIGTERM})
	select {
	case err := <-done:
		if err == nil {
			t.Error("want an error for the cancelled run")
		}
	case <-time.After(waitDelay + 10*time.Second):
		t.Fatal("an extension ignoring SIGTERM was never killed")
	}
}
