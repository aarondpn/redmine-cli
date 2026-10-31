package filelock

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireWaitsForUnlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")
	first, err := Acquire(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}

	got := make(chan error, 1)
	go func() {
		second, err := Acquire(context.Background(), path)
		if err == nil {
			err = second.Unlock()
		}
		got <- err
	}()

	select {
	case err := <-got:
		t.Fatalf("second Acquire returned while the lock was held: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	if err := first.Unlock(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-got:
		if err != nil {
			t.Fatalf("second Acquire: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("second Acquire did not get the lock after Unlock")
	}
}

func TestAcquireContextDeadline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")
	held, err := Acquire(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Unlock() }()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := Acquire(ctx, path); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Acquire error = %v, want DeadlineExceeded", err)
	}
}

func TestAcquireCreatesFile0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")
	l, err := Acquire(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Unlock(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file should remain after Unlock: %v", err)
	}
}

// TestHelperProcess holds the lock named by FILELOCK_HELPER_PATH until its
// stdin closes. It only runs when re-executed by TestAcquireAcrossProcesses.
func TestHelperProcess(t *testing.T) {
	path := os.Getenv("FILELOCK_HELPER_PATH")
	if path == "" {
		t.Skip("helper process only")
	}
	l, err := Acquire(context.Background(), path)
	if err != nil {
		os.Exit(2)
	}
	_, _ = os.Stdout.WriteString("locked\n")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	_ = l.Unlock()
	os.Exit(0)
}

func TestAcquireAcrossProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.lock")

	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	cmd.Env = append(os.Environ(), "FILELOCK_HELPER_PATH="+path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close(); _ = cmd.Wait() })

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "locked\n" {
		t.Fatalf("helper did not report the lock: %q, %v", line, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if _, err := Acquire(ctx, path); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Acquire while child holds the lock: error = %v, want DeadlineExceeded", err)
	}

	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper exit: %v", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	l, err := Acquire(ctx, path)
	if err != nil {
		t.Fatalf("Acquire after child exit: %v", err)
	}
	_ = l.Unlock()
}
