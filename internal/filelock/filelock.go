// Package filelock provides an exclusive advisory lock on a file that works
// across processes on Unix and Windows. The OS releases it when the process
// exits, so a crash never leaves a stale lock.
package filelock

import (
	"context"
	"os"
	"time"
)

// pollInterval is how often Acquire retries a held lock.
const pollInterval = 25 * time.Millisecond

// Lock is a held file lock.
type Lock struct{ f *os.File }

// Acquire opens (creating, 0o600) path and polls until the lock is taken or
// ctx is done.
func Acquire(ctx context.Context, path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		ok, err := tryLock(f)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		if ok {
			return &Lock{f: f}, nil
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}

// Unlock releases the lock. The file itself is left in place.
func (l *Lock) Unlock() error {
	return unlock(l.f)
}
