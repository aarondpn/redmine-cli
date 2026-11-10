//go:build unix

package main

import (
	"os"
	"os/signal"
	"syscall"
)

// hangupSignals is SIGHUP, unless redmine was started with it ignored (by
// nohup, say): catching it would undo that, for redmine and for the
// extensions it starts.
func hangupSignals() []os.Signal {
	return hangupSignalsUnless(signal.Ignored)
}

// hangupSignalsUnless implements hangupSignals; ignored reports whether a
// signal was ignored at startup.
func hangupSignalsUnless(ignored func(os.Signal) bool) []os.Signal {
	if ignored(syscall.SIGHUP) {
		return nil
	}
	return []os.Signal{syscall.SIGHUP}
}
