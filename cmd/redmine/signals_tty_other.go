//go:build !unix

package main

// interruptFromTerminal reports whether a Ctrl-C came from the console.
// Windows sends console Ctrl-C to every process attached to the console,
// the extension included, so it always did.
func interruptFromTerminal() bool { return true }
