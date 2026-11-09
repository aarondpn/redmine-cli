//go:build unix

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// interruptFromTerminal reports whether a SIGINT came from the terminal:
// redmine's process group is the foreground group of its controlling
// terminal, so the terminal sent the signal to the extension as well.
// Without a controlling terminal, or in the background, the signal was
// sent to redmine alone.
func interruptFromTerminal() bool {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return false
	}
	defer tty.Close()
	fg, err := unix.IoctlGetInt(int(tty.Fd()), unix.TIOCGPGRP)
	return err == nil && fg == unix.Getpgrp()
}
