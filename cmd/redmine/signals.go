package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// interruptSignals cancel the command context. Ctrl-C in a raw-mode TUI is a
// key press, not a signal; bubbletea handles those itself.
var interruptSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

// interruptGrace is how long a command may keep running after the first
// signal cancelled its context. Anything still running by then is blocked on
// something that ignores the context, such as a [y/N] prompt reading stdin in
// cooked mode, and is ended the way the default signal handler would have.
const interruptGrace = time.Second

// interruptedError is the cancellation cause of the command context after a
// signal arrived.
type interruptedError struct {
	sig os.Signal
}

func (e *interruptedError) Error() string {
	return e.sig.String() + " signal received"
}

// exitCode follows the shell convention for a process killed by a signal
// (130 for SIGINT, 143 for SIGTERM).
func (e *interruptedError) exitCode() int {
	if s, ok := e.sig.(syscall.Signal); ok {
		return 128 + int(s)
	}
	return 1
}

// interruptExitCode reports whether ctx was cancelled by a signal and, if so,
// the exit code to use.
func interruptExitCode(ctx context.Context) (int, bool) {
	var intr *interruptedError
	if errors.As(context.Cause(ctx), &intr) {
		return intr.exitCode(), true
	}
	return 0, false
}

// notifyInterrupt returns a context that is cancelled on the first SIGINT or
// SIGTERM. The first signal also restores default signal handling, so a
// second Ctrl-C kills the process immediately. Call stop once the command
// has returned.
func notifyInterrupt(parent context.Context) (ctx context.Context, stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, interruptSignals...)
	return watchInterrupt(parent, ch, func() { signal.Stop(ch) }, interruptGrace, os.Exit)
}

// watchInterrupt implements notifyInterrupt on top of a signal channel.
// release restores default signal handling; exit ends the process when the
// command outlives grace after the first signal.
func watchInterrupt(parent context.Context, sigs <-chan os.Signal, release func(), grace time.Duration, exit func(int)) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	var releaseOnce sync.Once
	releaseSignals := func() { releaseOnce.Do(release) }
	done := make(chan struct{})

	go func() {
		select {
		case sig := <-sigs:
			intr := &interruptedError{sig: sig}
			cancel(intr)
			releaseSignals()
			timer := time.NewTimer(grace)
			defer timer.Stop()
			select {
			case <-done:
			case <-timer.C:
				exit(intr.exitCode())
			}
		case <-done:
		}
	}()

	var stopOnce sync.Once
	return ctx, func() {
		stopOnce.Do(func() {
			releaseSignals()
			close(done)
			cancel(context.Canceled)
		})
	}
}
