package issue

import (
	"errors"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// statusNotAppliedError converts an *ops.StatusNotAppliedError into a coded
// command error, so the standard error path reports it: the explanation as
// the message, the allowed statuses and the "other changes may already have
// been applied" note as details, code status_not_applied, exit status 1. It
// returns nil when err is not a StatusNotAppliedError so callers can fall
// through to their usual error handling.
func statusNotAppliedError(err error) error {
	var notApplied *ops.StatusNotAppliedError
	if !errors.As(err, &notApplied) {
		return nil
	}
	return &cmdutil.CodedError{
		Code:    output.ErrCodeStatusNotApplied,
		Message: notApplied.Summary(),
		Details: notApplied.Details(),
		Err:     err,
	}
}

// printWarnings writes non-fatal follow-up notes from an ops result to
// stderr.
func printWarnings(printer output.Printer, warnings []string) {
	for _, w := range warnings {
		printer.Warning(w)
	}
}
