package browser

import (
	"os/exec"
	"time"
)

// launcherWait bounds how long Open waits for the launcher to exit. It is a
// variable so tests can shorten it.
var launcherWait = time.Second

// Open validates rawURL with ValidateURL and opens it in the default
// browser. It waits for the launcher (open, xdg-open, rundll32) to exit so
// that a failure, such as xdg-open finding no browser, is reported. Some
// launchers keep running until the browser itself exits (xdg-open in a
// minimal desktop environment); Open returns nil after launcherWait in that
// case and reaps the launcher in the background, so neither the CLI nor
// the TUI hangs on it.
func Open(rawURL string) error {
	cmd, err := Command(rawURL)
	if err != nil {
		return err
	}
	return startAndWait(cmd, launcherWait)
}

// startAndWait starts cmd and returns its exit error if it exits within
// wait, or nil if it is still running by then.
func startAndWait(cmd *exec.Cmd, wait time.Duration) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		return nil
	}
}
