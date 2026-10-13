package issue

import (
	"errors"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// reportStatusNotApplied renders an update whose status change Redmine
// silently ignored. It returns nil when err is not an
// *ops.StatusNotAppliedError so callers can fall through to their usual error
// handling.
//
// The explanation always reaches stderr, JSON callers additionally get an
// ok:false action envelope on stdout, and the returned SilentError makes the
// command exit non-zero without main.go printing the problem a second time.
func reportStatusNotApplied(printer output.Printer, action string, id int, err error) error {
	var notApplied *ops.StatusNotAppliedError
	if !errors.As(err, &notApplied) {
		return nil
	}
	msg := notApplied.Error()
	if printer.Format() == output.FormatJSON {
		// Outcome only writes the envelope in JSON mode; in the other modes
		// it already prints msg as a warning on stderr.
		printer.Warning(msg)
	}
	printer.Outcome(false, action, "issue", id, msg)
	return &cmdutil.SilentError{Code: 1}
}

// printWarnings writes non-fatal follow-up warnings from an ops result to
// stderr.
func printWarnings(printer output.Printer, warnings []string) {
	for _, w := range warnings {
		printer.Warning(w)
	}
}
