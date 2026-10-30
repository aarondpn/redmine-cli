package wiki

import (
	"errors"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// parentNotAppliedError converts an *ops.WikiParentNotAppliedError into a
// coded command error, so the standard error path reports it: the
// explanation as the message, the dropped rename and the "other changes may
// already have been applied" note as details, code parent_not_applied, exit
// status 1. It returns nil when err is
// not a WikiParentNotAppliedError so callers can fall through to their usual
// error handling.
func parentNotAppliedError(err error) error {
	var notApplied *ops.WikiParentNotAppliedError
	if !errors.As(err, &notApplied) {
		return nil
	}
	return &cmdutil.CodedError{
		Code:    output.ErrCodeParentNotApplied,
		Message: notApplied.Summary(),
		Details: notApplied.Details(),
		Err:     err,
	}
}
