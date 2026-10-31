package config

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLockConfigWaitsAndTimesOut(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	unlock, err := LockConfig(context.Background(), cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	orig := lockTimeout
	lockTimeout = 100 * time.Millisecond
	t.Cleanup(func() { lockTimeout = orig })

	_, err = LockConfig(context.Background(), cfgPath)
	if err == nil || !strings.Contains(err.Error(), "timed out waiting for another redmine process to finish updating") {
		t.Fatalf("second LockConfig error = %v, want timeout", err)
	}

	unlock()
	again, err := LockConfig(context.Background(), cfgPath)
	if err != nil {
		t.Fatalf("LockConfig after unlock: %v", err)
	}
	again()

	if _, err := os.Stat(cfgPath + ".lock"); err != nil {
		t.Fatalf("lock file should be kept: %v", err)
	}
}

func TestLockConfigReadOnlyDirIsNoOp(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("directory permissions are not enforced here")
	}
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	unlock, err := LockConfig(context.Background(), cfgPath)
	if err != nil {
		t.Fatalf("LockConfig error = %v, want nil", err)
	}
	unlock()
}
