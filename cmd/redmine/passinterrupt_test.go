package main

import (
	"context"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// TestRelaySignals checks the context used while an extension runs with
// the terminal attached: a Ctrl-C from the terminal (which the extension
// got as well) neither cancels it nor ends the process, while a signal sent
// to redmine alone cancels it with the signal as the cause, which the
// runner passes on to the extension.
func TestRelaySignals(t *testing.T) {
	tests := []struct {
		name         string
		sig          os.Signal
		fromTerminal bool
		wantCancel   bool
		wantCode     int
	}{
		{name: "terminal ctrl-c is left to the extension", sig: os.Interrupt, fromTerminal: true},
		{name: "sigint sent to redmine alone", sig: os.Interrupt, wantCancel: true, wantCode: 130},
		{name: "sigterm in the foreground", sig: syscall.SIGTERM, fromTerminal: true, wantCancel: true, wantCode: 143},
		{name: "sigterm without a terminal", sig: syscall.SIGTERM, wantCancel: true, wantCode: 143},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sigs := make(chan os.Signal)
			var released atomic.Int32
			ctx, stop := relaySignals(context.Background(), sigs, func() { released.Add(1) }, func() bool { return tt.fromTerminal })
			for range 2 {
				select {
				case sigs <- tt.sig:
				case <-time.After(5 * time.Second):
					t.Fatal("signal was not consumed")
				}
			}
			if tt.wantCancel {
				select {
				case <-ctx.Done():
				case <-time.After(5 * time.Second):
					t.Fatal("the signal did not cancel the context")
				}
				code, ok := interruptExitCode(ctx)
				if !ok || code != tt.wantCode {
					t.Fatalf("interruptExitCode = %d, %v; want %d", code, ok, tt.wantCode)
				}
				if cause, ok := context.Cause(ctx).(interface{ Signal() os.Signal }); !ok || cause.Signal() != tt.sig {
					t.Fatalf("cause %v does not carry %v", context.Cause(ctx), tt.sig)
				}
			} else {
				select {
				case <-ctx.Done():
					t.Fatal("a terminal signal cancelled the context")
				case <-time.After(20 * time.Millisecond):
				}
				if _, ok := interruptExitCode(ctx); ok {
					t.Fatal("interruptExitCode reports an interrupt")
				}
			}

			stop()
			stop()
			if ctx.Err() == nil {
				t.Fatal("stop did not cancel the context")
			}
			if released.Load() != 1 {
				t.Fatalf("release called %d times, want 1", released.Load())
			}
		})
	}
}
