package mcpserver

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ExtensionShutdownTimeout bounds how long Stop waits for the extension
// runs it cancelled. Cancelling a run kills its process group at once; the
// runner then waits up to two seconds for output pipes a stray grandchild
// may still hold, so this leaves room for that and a little more.
const ExtensionShutdownTimeout = 5 * time.Second

// errShuttingDown is the cancellation cause of the runs Stop cancels.
var errShuttingDown = errors.New("the MCP server is shutting down")

// ExtensionRuns tracks the extension processes a server has running, so
// the process serving MCP can stop them before it exits. An extension runs
// in its own process group: if the server just returned, a tool call in
// flight would leave the extension running on its own, unbounded.
//
// The zero value is not usable; use NewExtensionRuns. A nil *ExtensionRuns
// tracks nothing: runs are bounded by their request context alone.
type ExtensionRuns struct {
	ctx    context.Context
	cancel context.CancelCauseFunc

	mu      sync.Mutex
	stopped bool
	wg      sync.WaitGroup
}

// NewExtensionRuns returns an empty tracker.
func NewExtensionRuns() *ExtensionRuns {
	ctx, cancel := context.WithCancelCause(context.Background())
	return &ExtensionRuns{ctx: ctx, cancel: cancel}
}

// begin registers a run. The returned context is ctx, also cancelled by
// Stop; done must be called when the run is over. ok is false once Stop
// was called: the run must not start.
func (r *ExtensionRuns) begin(ctx context.Context) (runCtx context.Context, done func(), ok bool) {
	if r == nil {
		return ctx, func() {}, true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return nil, nil, false
	}
	r.wg.Add(1)
	runCtx, cancel := context.WithCancelCause(ctx)
	stop := context.AfterFunc(r.ctx, func() { cancel(context.Cause(r.ctx)) })
	return runCtx, func() {
		stop()
		cancel(nil)
		r.wg.Done()
	}, true
}

// Stop cancels every run in flight, which kills their process groups,
// refuses new ones and waits up to timeout for the cancelled runs to
// finish. It reports whether they all did. Calling it again only waits.
func (r *ExtensionRuns) Stop(timeout time.Duration) bool {
	if r == nil {
		return true
	}
	r.mu.Lock()
	r.stopped = true
	r.mu.Unlock()
	r.cancel(errShuttingDown)

	finished := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(finished)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-finished:
		return true
	case <-timer.C:
		return false
	}
}
