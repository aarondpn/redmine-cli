package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/filelock"
)

// lockTimeout bounds how long LockConfig waits for another process. A
// variable so tests can shorten it.
var lockTimeout = 30 * time.Second

// LockConfig takes the exclusive lock that serializes config writes and OAuth
// token refreshes across processes: <resolved config path>.lock. When the
// lock file cannot be created (read-only directory, as with a bind-mounted
// config) it returns a no-op unlock so writes keep working as before. The
// lock file is never deleted, since removing it would reintroduce races.
func LockConfig(ctx context.Context, path string) (unlock func(), err error) {
	if path == "" {
		path = DefaultConfigPath()
	}
	resolved, err := resolveConfigPath(path)
	if err != nil {
		return nil, err
	}

	// Same mode writeFileAtomic gives a missing config directory.
	if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
		if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EROFS) {
			return func() {}, nil
		}
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	l, err := filelock.Acquire(ctx, resolved+".lock")
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("timed out waiting for another redmine process to finish updating %s", resolved)
		}
		if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EROFS) {
			return func() {}, nil
		}
		return nil, err
	}
	return func() { _ = l.Unlock() }, nil
}
