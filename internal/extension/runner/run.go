package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/aarondpn/redmine-cli/v2/internal/textsafe"
)

const (
	// MaxStdout bounds the captured result of a declarative command.
	MaxStdout = 10 << 20
	// MaxStderr bounds the captured stderr kept for the error message.
	MaxStderr = 64 << 10
	// maxErrorMessage bounds the stderr excerpt in an ExitError.
	maxErrorMessage = 4 << 10
	// waitDelay bounds how long Run waits for the output pipes after the
	// process exited or was killed: a grandchild that inherited them (and
	// escaped the process group kill) must not hang the caller.
	waitDelay = 2 * time.Second
)

// Request describes a bounded run of a declarative command.
type Request struct {
	// Path is the extension executable.
	Path string
	// Args follow the executable; declarative commands get none.
	Args []string
	// Env is the complete environment, usually Env.Environ(os.Environ()).
	Env []string
	// Dir is the working directory; empty means the caller's.
	Dir string
	// Stdin is the input, the validated input object as JSON.
	Stdin []byte
	// Stderr, when set, receives the extension's stderr as it is written
	// (the CLI streams it live). It is captured for the error message
	// either way. Write errors on it are ignored.
	Stderr io.Writer
	// Timeout kills the process (group) when exceeded. Zero means none.
	Timeout time.Duration
	// MaxStdout and MaxStderr override the package limits when positive.
	MaxStdout int64
	MaxStderr int64
}

// Result is the captured output of a run. Run returns it together with an
// *ExitError when the command failed, so the caller still sees its output.
type Result struct {
	Stdout []byte
	// Stderr is the captured stderr; only its last MaxStderr bytes when
	// StderrTruncated is set.
	Stderr          []byte
	StderrTruncated bool
	ExitCode        int
}

// ExitError reports a command that exited with a non-zero status.
type ExitError struct {
	Code int
	// Message is the sanitized, truncated stderr of the command.
	Message string
}

func (e *ExitError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("exit status %d", e.Code)
	}
	return fmt.Sprintf("exit status %d: %s", e.Code, e.Message)
}

// TimeoutError reports a command killed because it ran past its timeout.
type TimeoutError struct {
	Timeout time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("timed out after %s", e.Timeout)
}

// OutputTooLargeError reports a command killed because its stdout exceeded
// the limit.
type OutputTooLargeError struct {
	Limit int64
}

func (e *OutputTooLargeError) Error() string {
	return fmt.Sprintf("output exceeded %d bytes", e.Limit)
}

// Run executes a declarative command: Stdin on standard input, stdout
// captured (at most MaxStdout bytes, the process is killed beyond that),
// stderr captured and optionally streamed. The process runs in its own
// process group on Unix, so a timeout or cancellation of ctx kills
// everything it started; on Windows the process itself is killed.
//
// A non-zero exit returns the Result and an *ExitError. A timeout returns
// *TimeoutError, an oversized output *OutputTooLargeError, and a cancelled
// ctx an error wrapping ctx.Err().
func Run(ctx context.Context, req Request) (*Result, error) {
	maxOut := limitOr(req.MaxStdout, MaxStdout)
	maxErr := limitOr(req.MaxStderr, MaxStderr)

	runCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	if req.Timeout > 0 {
		var cancelTimeout context.CancelFunc
		runCtx, cancelTimeout = context.WithTimeoutCause(runCtx, req.Timeout, &TimeoutError{Timeout: req.Timeout})
		defer cancelTimeout()
	}

	name, argv, err := Command(req.Path, req.Args)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(runCtx, name, argv...)
	cmd.Env = req.Env
	cmd.Dir = req.Dir
	cmd.Stdin = bytes.NewReader(req.Stdin)
	stdout := &capWriter{limit: maxOut, onOverflow: func() {
		cancel(&OutputTooLargeError{Limit: maxOut})
	}}
	stderr := &tailWriter{limit: maxErr}
	cmd.Stdout = stdout
	if req.Stderr != nil {
		cmd.Stderr = io.MultiWriter(stderr, ignoreErrors{req.Stderr})
	} else {
		cmd.Stderr = stderr
	}
	startInGroup(cmd)
	cmd.Cancel = func() error { return killGroup(cmd) }
	cmd.WaitDelay = waitDelay

	runErr := cmd.Run()
	res := &Result{Stdout: stdout.bytes(), Stderr: stderr.bytes()}
	res.StderrTruncated = stderr.wasTruncated()
	// Checked before the exit status: a process that wrote its last chunk
	// and exited before the copy saw the overflow is not killed, so it can
	// exit 0, but its output is still truncated.
	if stdout.overflowed() {
		return res, &OutputTooLargeError{Limit: maxOut}
	}
	if runErr == nil {
		return res, nil
	}
	if runCtx.Err() != nil {
		var timeout *TimeoutError
		cause := context.Cause(runCtx)
		switch {
		case errors.As(cause, &timeout):
			return res, timeout
		case ctx.Err() != nil:
			return res, fmt.Errorf("extension cancelled: %w", ctx.Err())
		}
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		res.ExitCode = exitCode(exitErr.ProcessState)
		return res, &ExitError{Code: res.ExitCode, Message: ErrorMessage(res.Stderr)}
	}
	return res, runErr
}

// AttachedRequest describes a passthrough run with the caller's stdio.
type AttachedRequest struct {
	Path   string
	Args   []string
	Env    []string
	Dir    string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	// Timeout kills the process when exceeded. Zero means none, the
	// default for interactive passthrough commands.
	Timeout time.Duration
}

// SignalCause is implemented by a cancellation cause that stands for a
// signal the caller received (redmine's own SIGTERM, say). RunAttached
// passes that signal on to the process instead of killing it.
type SignalCause interface {
	error
	Signal() os.Signal
}

// RunAttached runs a raw command or an extension without a manifest with
// stdio attached and returns its exit code. The process stays in the
// caller's process group, so it can use the terminal and receives Ctrl-C
// from it directly. Cancelling ctx kills only the process itself, unless
// the cause is a SignalCause: then the process gets that signal to shut
// down by itself, and is killed only if it is still running waitDelay
// later.
func RunAttached(ctx context.Context, req AttachedRequest) (int, error) {
	runCtx := ctx
	if req.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeoutCause(ctx, req.Timeout, &TimeoutError{Timeout: req.Timeout})
		defer cancel()
	}
	name, argv, err := Command(req.Path, req.Args)
	if err != nil {
		return 0, err
	}
	cmd := exec.CommandContext(runCtx, name, argv...)
	cmd.Env = req.Env
	cmd.Dir = req.Dir
	cmd.Stdin = req.Stdin
	cmd.Stdout = req.Stdout
	cmd.Stderr = req.Stderr
	cmd.Cancel = func() error {
		var sc SignalCause
		if errors.As(context.Cause(runCtx), &sc) {
			// Signals other than Kill are not supported on Windows; the
			// process is killed there.
			if err := cmd.Process.Signal(sc.Signal()); err == nil || errors.Is(err, os.ErrProcessDone) {
				return err
			}
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = waitDelay

	runErr := cmd.Run()
	if runErr == nil {
		return 0, nil
	}
	if runCtx.Err() != nil {
		var timeout *TimeoutError
		if errors.As(context.Cause(runCtx), &timeout) {
			return 0, timeout
		}
		if ctx.Err() != nil {
			return 0, fmt.Errorf("extension cancelled: %w", ctx.Err())
		}
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return exitCode(exitErr.ProcessState), nil
	}
	return 0, runErr
}

// DecodeOutput parses a command's stdout as one JSON value (object, array
// or scalar), keeping numbers as json.Number so large IDs keep their exact
// digits. ok is false for empty or non-JSON output, which callers show as
// text.
func DecodeOutput(stdout []byte) (v any, ok bool) {
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 {
		return nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	// Trailing data (e.g. two JSON documents, or JSON followed by text)
	// is not a single result.
	if _, err := dec.Token(); err != io.EOF {
		return nil, false
	}
	return v, true
}

// ErrorMessage turns captured stderr into an error message: surrounding
// whitespace trimmed, at most the last 4 KiB (errors and stack traces end
// there), and neutralized for the terminal since an extension may relay
// text from anywhere.
func ErrorMessage(stderr []byte) string {
	s := strings.TrimSpace(string(stderr))
	if len(s) > maxErrorMessage {
		cut := len(s) - maxErrorMessage
		for cut < len(s) && !utf8.RuneStart(s[cut]) {
			cut++
		}
		s = "..." + s[cut:]
	}
	return textsafe.Terminal(s)
}

func limitOr(v, def int64) int64 {
	if v > 0 {
		return v
	}
	return def
}

// capWriter keeps the first limit bytes. Past the limit it reports the
// overflow once and swallows the rest, so the process does not block on a
// full pipe before it is killed.
type capWriter struct {
	mu         sync.Mutex
	buf        bytes.Buffer
	limit      int64
	overflow   bool
	onOverflow func()
}

func (w *capWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.overflow {
		return len(p), nil
	}
	if room := w.limit - int64(w.buf.Len()); int64(len(p)) > room {
		w.buf.Write(p[:room])
		w.overflow = true
		w.onOverflow()
		return len(p), nil
	}
	w.buf.Write(p)
	return len(p), nil
}

func (w *capWriter) overflowed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.overflow
}

func (w *capWriter) bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return bytes.Clone(w.buf.Bytes())
}

// tailWriter keeps the last limit bytes written.
type tailWriter struct {
	mu        sync.Mutex
	buf       []byte
	limit     int64
	truncated bool
}

func (w *tailWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	// Trim in batches so a chatty process does not cause a copy per write.
	if int64(len(w.buf)) > 2*w.limit {
		w.buf = append(w.buf[:0:0], w.buf[int64(len(w.buf))-w.limit:]...)
		w.truncated = true
	}
	return len(p), nil
}

func (w *tailWriter) wasTruncated() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.truncated
}

func (w *tailWriter) bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	if int64(len(w.buf)) > w.limit {
		w.truncated = true
		return bytes.Clone(w.buf[int64(len(w.buf))-w.limit:])
	}
	return bytes.Clone(w.buf)
}

// ignoreErrors keeps a failing live stream (e.g. a closed terminal) from
// aborting the copy into the capture buffer.
type ignoreErrors struct{ w io.Writer }

func (w ignoreErrors) Write(p []byte) (int, error) {
	_, _ = w.w.Write(p)
	return len(p), nil
}
