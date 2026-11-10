//go:build !unix

package main

import "os"

// hangupSignals is empty: there is no SIGHUP outside Unix.
func hangupSignals() []os.Signal { return nil }
