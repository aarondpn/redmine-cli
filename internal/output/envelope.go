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

// ErrorEnvelope is the JSON shape written to stderr for command failures in
// JSON mode, as a single line that is always the last line of stderr. It is
// never filtered by --jq.
type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail describes a failed command execution.
type ErrorDetail struct {
	Message string   `json:"message"`
	Code    string   `json:"code,omitempty"`
	Details []string `json:"details,omitempty"`
	// Hint is a short, actionable next step (typically a command to run) to
	// recover from the failure. Omitted when there is nothing useful to add.
	Hint string `json:"hint,omitempty"`
}

// Error codes emitted in JSON error envelopes.
const (
	ErrCodeNotFound         = "not_found"
	ErrCodeAuthFailed       = "auth_failed"
	ErrCodeForbidden        = "forbidden"
	ErrCodeValidationFailed = "validation_failed"
	ErrCodeConflict         = "conflict"
	ErrCodeServerError      = "server_error"
	// ErrCodeJQFailed means the command itself succeeded (any write already
	// happened) but the --jq expression failed on its output. Exception:
	// with `api -i` on a 4xx/5xx response the filter ran on the error
	// response and the request itself did not succeed.
	ErrCodeJQFailed = "jq_failed"
	// ErrCodeJQInvalid means the --jq expression is empty or malformed; the
	// command did not run.
	ErrCodeJQInvalid = "jq_invalid"
	ErrCodeUnknown   = "unknown"
	// ErrCodeStatusNotApplied: Redmine accepted an issue update but silently
	// kept the old status (workflow, open subtasks, open blockers).
	ErrCodeStatusNotApplied = "status_not_applied"
	// ErrCodeTimeout: a connection or request to the server timed out.
	ErrCodeTimeout = "timeout"
	// ErrCodeAlreadyExists: a download target exists and --clobber was not
	// given.
	ErrCodeAlreadyExists = "already_exists"
)

// Action actions exposed by no-body mutators. Keep the vocabulary small.
const (
	ActionCreated     = "created"
	ActionUpdated     = "updated"
	ActionDeleted     = "deleted"
	ActionClosed      = "closed"
	ActionReopened    = "reopened"
	ActionAssigned    = "assigned"
	ActionUnassigned  = "unassigned"
	ActionCommented   = "commented"
	ActionLogged      = "logged"
	ActionUserAdded   = "user_added"
	ActionUserRemoved = "user_removed"
	ActionLoggedIn    = "logged_in"
	ActionLoggedOut   = "logged_out"
	ActionSwitched    = "switched"
	ActionInstalled   = "installed"
	ActionOpened      = "opened"
	ActionUploaded    = "uploaded"
	ActionArchived    = "archived"
	ActionUnarchived  = "unarchived"
	ActionWatched     = "watched"
	ActionUnwatched   = "unwatched"
	ActionRelated     = "related"
	ActionUnrelated   = "unrelated"
)

// RenderActionJSON writes an action envelope as pretty-printed JSON.
func RenderActionJSON(w io.Writer, env ActionEnvelope) error {
	return renderJSONValue(w, env)
}

// RenderErrorJSON writes an error envelope as a single line of compact JSON,
// so it can be picked out of stderr even when warnings or --verbose debug
// output precede it.
func RenderErrorJSON(w io.Writer, env ErrorEnvelope) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(env)
}

func renderJSONValue(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
