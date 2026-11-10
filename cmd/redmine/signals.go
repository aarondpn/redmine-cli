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

// interruptSignals cancel the command context: SIGINT, SIGTERM and, on Unix,
// SIGHUP (the terminal closed or the SSH connection dropped), so an
// extension running in a process group of its own is killed rather than
// orphaned. Ctrl-C in a raw-mode TUI is a key press, not a signal;
// bubbletea handles those itself.
func interruptSignals() []os.Signal {
	return append([]os.Signal{os.Interrupt, syscall.SIGTERM}, hangupSignals()...)
}

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

// Signal returns the signal. An extension run with the terminal attached
// gets it passed on when it cancels the context (see runner.RunAttached).
func (e *interruptedError) Signal() os.Signal { return e.sig }

// exitCode follows the shell convention for a process killed by a signal
// (129 for SIGHUP, 130 for SIGINT, 143 for SIGTERM).
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

// notifyInterrupt returns a context that is cancelled on the first of the
// interruptSignals. The first signal also restores default signal handling, so a
// second Ctrl-C kills the process immediately. Call stop once the command
// has returned.
func notifyInterrupt(parent context.Context) (ctx context.Context, stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, interruptSignals()...)
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

// passInterrupt replaces notifyInterrupt while an extension runs with the
// terminal attached. The extension shares redmine's process group, so a
// Ctrl-C at the terminal reaches it directly; redmine only has to survive
// that signal and wait for the extension to decide. Any other signal (a
// SIGTERM from `timeout`, systemd or a CI runner, or a SIGINT sent to
// redmine alone) reaches redmine only: it cancels the context with the
// signal as the cause, and the runner passes the signal on to the
// extension, so redmine still ends when it is asked to. A SIGHUP is passed
// on as well: the extension may get it twice (a hangup reaches the whole
// foreground process group), but is killed if it outlives the hangup.
//
// The signals are caught rather than ignored, because an ignored signal
// would be inherited by the extension process.
func passInterrupt(parent context.Context) (ctx context.Context, stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, interruptSignals()...)
	return relaySignals(parent, ch, func() { signal.Stop(ch) }, interruptFromTerminal)
}

// relaySignals implements passInterrupt on top of a signal channel.
// fromTerminal reports whether a SIGINT was sent by the terminal to the
// whole foreground process group, extension included.
func relaySignals(parent context.Context, sigs <-chan os.Signal, release func(), fromTerminal func() bool) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(parent)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-sigs:
				if sig == os.Interrupt && fromTerminal() {
					continue
				}
				// Only the first cause counts; the runner kills the
				// extension if it ignores the signal.
				cancel(&interruptedError{sig: sig})
			case <-done:
				return
			}
		}
	}()
	var stopOnce sync.Once
	return ctx, func() {
		stopOnce.Do(func() {
			release()
			close(done)
			cancel(context.Canceled)
		})
	}
}
