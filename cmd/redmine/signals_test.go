package main

import (
	"context"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestWatchInterrupt_FirstSignalCancelsAndReleases(t *testing.T) {
	sigs := make(chan os.Signal, 1)
	var released atomic.Int32
	exited := make(chan int, 1)
	ctx, stop := watchInterrupt(context.Background(), sigs, func() { released.Add(1) }, time.Hour, func(code int) { exited <- code })
	defer stop()

	sigs <- os.Interrupt
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("context not cancelled after the first signal")
	}
	code, ok := interruptExitCode(ctx)
	if !ok || code != 130 {
		t.Fatalf("interruptExitCode() = %d, %v; want 130, true", code, ok)
	}

	// Default handling must be restored right away so a second Ctrl-C kills
	// the process, not only once the command returns.
	deadline := time.Now().Add(5 * time.Second)
	for released.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if released.Load() != 1 {
		t.Fatalf("release called %d times after the first signal, want 1", released.Load())
	}

	stop()
	if released.Load() != 1 {
		t.Fatalf("release called %d times after stop, want 1", released.Load())
	}
	select {
	case code := <-exited:
		t.Fatalf("exit(%d) called although the command returned within the grace period", code)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestWatchInterrupt_ExitsWhenCommandOutlivesGrace(t *testing.T) {
	sigs := make(chan os.Signal, 1)
	exited := make(chan int, 1)
	_, stop := watchInterrupt(context.Background(), sigs, func() {}, 10*time.Millisecond, func(code int) { exited <- code })
	defer stop()

	sigs <- syscall.SIGTERM
	select {
	case code := <-exited:
		if code != 143 {
			t.Fatalf("exit code = %d, want 143", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("exit not called after the grace period")
	}
}

func TestWatchInterrupt_StopWithoutSignal(t *testing.T) {
	sigs := make(chan os.Signal, 1)
	var released atomic.Int32
	ctx, stop := watchInterrupt(context.Background(), sigs, func() { released.Add(1) }, time.Hour, func(code int) {
		t.Errorf("exit(%d) called without a signal", code)
	})
	if ctx.Err() != nil {
		t.Fatalf("context cancelled before any signal: %v", ctx.Err())
	}
	stop()
	stop()
	if released.Load() != 1 {
		t.Fatalf("release called %d times, want 1", released.Load())
	}
	if _, ok := interruptExitCode(ctx); ok {
		t.Fatal("interruptExitCode() reports an interrupt although none happened")
	}
}
