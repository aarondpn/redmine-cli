package output

import (
	"github.com/pterm/pterm"
)

// StartSpinner starts a spinner with the given message and returns a stop function.
func StartSpinner(msg string, isTTY bool) func() {
	if !isTTY {
		return func() {}
	}
	spinner, _ := pterm.DefaultSpinner.WithRemoveWhenDone(true).Start(msg)
	return func() {
		spinner.Stop()
	}
}
