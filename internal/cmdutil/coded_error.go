package cmdutil

import "strings"

// CodedError is a command failure that carries its own stable JSON error
// code (see the output.ErrCode* constants) and optional details. The normal
// error path renders it like any other error: Error() in text mode, and an
// error envelope with Code, Message and Details in JSON mode.
type CodedError struct {
	Code    string
	Message string
	Details []string
	// Err is the underlying error, kept for errors.As/errors.Is.
	Err error
}

// Error renders the message followed by the details, one per line, in the
// same style FormatError uses for validation errors.
func (e *CodedError) Error() string {
	if len(e.Details) == 0 {
		return e.Message
	}
	return e.Message + "\n  - " + strings.Join(e.Details, "\n  - ")
}

func (e *CodedError) Unwrap() error { return e.Err }
