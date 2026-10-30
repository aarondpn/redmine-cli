package ops

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// StatusNotAppliedError reports that Redmine accepted an issue update but left
// the issue's status unchanged.
//
// Redmine answers 204 No Content to an issue PUT even when it drops the
// requested status_id: transitions the workflow does not allow for the
// current user's role(s) and the issue's tracker are silently ignored, and so
// is closing an issue that still has open subtasks or is blocked by an open
// issue. The only way to notice is to read the issue back, which UpdateIssue,
// CloseIssue and ReopenIssue do whenever a status change was requested.
type StatusNotAppliedError struct {
	IssueID   int
	Requested models.IDName
	Current   models.IDName
	// Allowed lists the statuses Redmine reports the current user may move the
	// issue to. Only meaningful when AllowedReported is true (Redmine 5.0+
	// supports include=allowed_statuses; older servers ignore it).
	Allowed         []models.IssueStatus
	AllowedReported bool
	// OtherChanges reports whether the same request carried changes besides
	// the status (fields, notes, attachments). Redmine applies those even when
	// it drops the status, so callers must not blindly resend them.
	OtherChanges bool
	// Transition is the kind of change that was requested; it selects the
	// explanation returned by Summary.
	Transition StatusTransition
}

// StatusTransition names the kind of status change that was requested.
type StatusTransition int

const (
	// StatusTransitionUpdate is an explicit status change (update --status).
	StatusTransitionUpdate StatusTransition = iota
	// StatusTransitionClose moves the issue to a closed status.
	StatusTransitionClose
	// StatusTransitionReopen moves the issue to an open status.
	StatusTransitionReopen
)

func (e *StatusNotAppliedError) Error() string {
	return e.Summary() + " " + strings.Join(e.Details(), " ")
}

// Summary is the one-paragraph explanation of what happened, without the
// follow-up facts returned by Details.
func (e *StatusNotAppliedError) Summary() string {
	var why string
	switch e.Transition {
	case StatusTransitionClose:
		why = "Redmine silently ignores status transitions that its workflow does not allow for your role and the issue's tracker, " +
			"and refuses to close issues that have open subtasks or are blocked by open issues."
	case StatusTransitionReopen:
		why = "Redmine silently ignores status transitions that its workflow does not allow for your role and the issue's tracker."
	default:
		why = "Redmine silently ignores status transitions that its workflow does not allow for your role and the issue's tracker; " +
			"moving to a closed status is also refused while the issue has open subtasks or is blocked by open issues."
	}
	return fmt.Sprintf("Issue #%d: status was not changed to %s; it is now %s. %s",
		e.IssueID, describeStatus(e.Requested), describeStatus(e.Current), why)
}

// Details lists the follow-up facts a caller needs to recover: the statuses
// the issue may move to and, when relevant, that the rest of the request may
// already have been applied.
func (e *StatusNotAppliedError) Details() []string {
	var details []string
	switch {
	case !e.AllowedReported:
		details = append(details, "This Redmine server does not report allowed transitions (requires Redmine 5.0+); check the workflow configuration or ask an administrator.")
	case len(e.Allowed) == 0:
		details = append(details, "Redmine reports no allowed status transitions for this issue.")
	default:
		names := make([]string, 0, len(e.Allowed))
		for _, s := range e.Allowed {
			names = append(names, fmt.Sprintf("%s (%d)", s.Name, s.ID))
		}
		details = append(details, fmt.Sprintf("Allowed statuses: %s.", strings.Join(names, ", ")))
	}
	if e.OtherChanges {
		details = append(details, "The other changes in this request (fields, notes, attachments) may already have been applied; do not resend them blindly.")
	}
	return details
}

func describeStatus(s models.IDName) string {
	if s.Name == "" {
		return fmt.Sprintf("status ID %d", s.ID)
	}
	return fmt.Sprintf("%q (ID %d)", s.Name, s.ID)
}

// statusCheck describes the status change verifyStatusApplied looks for.
type statusCheck struct {
	requested    models.IDName
	otherChanges bool
	// wantClosed, when set, accepts any status whose is_closed flag matches
	// (close/reopen): an issue that is already "Rejected" counts as closed
	// even though close asked for the first closed status.
	wantClosed *bool
	// statuses is the already-fetched status list, used to look up is_closed
	// and names without another request. May be nil.
	statuses []models.IssueStatus
}

// verifyStatusApplied reads the issue back after an update that requested a
// status change. It returns *StatusNotAppliedError when Redmine left the
// status untouched. When the read-back itself fails the update has still
// been applied, so the failure is reported as a note rather than an error; an
// issue that already had a status of the wanted open/closed kind also yields
// an informational note.
func verifyStatusApplied(ctx context.Context, client *api.Client, issueID int, check statusCheck) (note string, err error) {
	issue, allowed, reported, err := client.Issues.GetWithAllowedStatuses(ctx, issueID)
	if err != nil {
		return fmt.Sprintf("could not verify that issue #%d's status was changed: %v", issueID, err), nil
	}
	requested := check.requested
	if issue.Status.ID == requested.ID {
		return "", nil
	}
	if check.wantClosed != nil {
		for _, s := range check.statuses {
			if s.ID == issue.Status.ID && s.IsClosed == *check.wantClosed {
				state := "open"
				if s.IsClosed {
					state = "closed"
				}
				return fmt.Sprintf("Issue #%d is already %s with status %q; its status was left unchanged.", issueID, state, s.Name), nil
			}
		}
	}
	transition := StatusTransitionUpdate
	if check.wantClosed != nil {
		transition = StatusTransitionReopen
		if *check.wantClosed {
			transition = StatusTransitionClose
		}
	}
	if requested.Name == "" {
		requested.Name = lookupStatusName(ctx, client, requested.ID, append(append([]models.IssueStatus(nil), allowed...), check.statuses...))
	}
	return "", &StatusNotAppliedError{
		IssueID:         issueID,
		Requested:       requested,
		Current:         issue.Status.IDName(),
		Allowed:         allowed,
		AllowedReported: reported,
		OtherChanges:    check.otherChanges,
		Transition:      transition,
	}
}

// lookupStatusName names a status for error messages. It only runs on the
// failure path, so the extra statuses request is acceptable; any lookup
// failure just leaves the name empty.
func lookupStatusName(ctx context.Context, client *api.Client, id int, known []models.IssueStatus) string {
	for _, s := range known {
		if s.ID == id {
			return s.Name
		}
	}
	statuses, err := client.Statuses.List(ctx)
	if err != nil {
		return ""
	}
	for _, s := range statuses {
		if s.ID == id {
			return s.Name
		}
	}
	return ""
}

// hasNonStatusChanges reports whether an update carries anything besides the
// status itself.
func hasNonStatusChanges(upd models.IssueUpdate) bool {
	upd.StatusID = nil
	if len(upd.CustomFieldValues) == 0 {
		upd.CustomFieldValues = nil
	}
	if len(upd.Uploads) == 0 {
		upd.Uploads = nil
	}
	return !reflect.DeepEqual(upd, models.IssueUpdate{})
}
