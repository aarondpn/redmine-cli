package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// ActionEnvelope is the JSON shape emitted by no-body mutators.
type ActionEnvelope struct {
	Ok       bool   `json:"ok"`
	Action   string `json:"action"`
	Resource string `json:"resource"`
	ID       any    `json:"id,omitempty"`
	Message  string `json:"message,omitempty"`
}

// ErrorEnvelope is the JSON shape emitted for command failures in JSON mode.
type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail describes a failed command execution.
type ErrorDetail struct {
	Message string   `json:"message"`
	Code    string   `json:"code,omitempty"`
	Details []string `json:"details,omitempty"`
}

// Error codes emitted in JSON error envelopes.
const (
	ErrCodeNotFound         = "not_found"
	ErrCodeAuthFailed       = "auth_failed"
	ErrCodeForbidden        = "forbidden"
	ErrCodeValidationFailed = "validation_failed"
	ErrCodeServerError      = "server_error"
	ErrCodeUnknown          = "unknown"
)

// Action actions exposed by no-body mutators. Keep the vocabulary small.
const (
	ActionCreated     = "created"
	ActionUpdated     = "updated"
	ActionDeleted     = "deleted"
	ActionClosed      = "closed"
	ActionReopened    = "reopened"
	ActionAssigned    = "assigned"
	ActionCommented   = "commented"
	ActionLogged      = "logged"
	ActionUserAdded   = "user_added"
	ActionUserRemoved = "user_removed"
	ActionLoggedIn    = "logged_in"
	ActionLoggedOut   = "logged_out"
	ActionSwitched    = "switched"
	ActionInstalled   = "installed"
	ActionOpened      = "opened"
)

// RenderActionJSON writes an action envelope as pretty-printed JSON.
func RenderActionJSON(w io.Writer, env ActionEnvelope) error {
	return renderJSONValue(w, env)
}

// RenderErrorJSON writes an error envelope as pretty-printed JSON.
func RenderErrorJSON(w io.Writer, env ErrorEnvelope) error {
	return renderJSONValue(w, env)
}

func renderJSONValue(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
