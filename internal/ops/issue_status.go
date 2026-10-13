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
}

func (e *StatusNotAppliedError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Issue #%d: status was not changed to %s; it is still %s. "+
		"Redmine silently ignores status transitions that its workflow does not allow for your role and the issue's tracker, "+
		"and refuses to close issues that have open subtasks or are blocked by open issues.",
		e.IssueID, describeStatus(e.Requested), describeStatus(e.Current))
	if e.OtherChanges {
		b.WriteString(" The other changes in this request (fields, notes, attachments) may have been applied; do not resend them blindly.")
	}
	switch {
	case !e.AllowedReported:
		b.WriteString(" This Redmine server does not report allowed transitions (requires Redmine 5.0+); check the workflow configuration or ask an administrator.")
	case len(e.Allowed) == 0:
		b.WriteString(" Redmine reports no allowed status transitions for this issue.")
	default:
		names := make([]string, 0, len(e.Allowed))
		for _, s := range e.Allowed {
			names = append(names, fmt.Sprintf("%s (%d)", s.Name, s.ID))
		}
		fmt.Fprintf(&b, " Allowed statuses: %s.", strings.Join(names, ", "))
	}
	return b.String()
}

func describeStatus(s models.IDName) string {
	if s.Name == "" {
		return fmt.Sprintf("status ID %d", s.ID)
	}
	return fmt.Sprintf("%q (ID %d)", s.Name, s.ID)
}

// verifyStatusApplied reads the issue back after an update that requested a
// status change. It returns *StatusNotAppliedError when Redmine left the
// status untouched. When the read-back itself fails the update has still
// been applied, so the failure is reported as a warning rather than an error.
func verifyStatusApplied(ctx context.Context, client *api.Client, issueID int, requested models.IDName, otherChanges bool) (warning string, err error) {
	issue, allowed, reported, err := client.Issues.GetWithAllowedStatuses(ctx, issueID)
	if err != nil {
		return fmt.Sprintf("could not verify that issue #%d's status was changed: %v", issueID, err), nil
	}
	if issue.Status.ID == requested.ID {
		return "", nil
	}
	if requested.Name == "" {
		requested.Name = lookupStatusName(ctx, client, requested.ID, allowed)
	}
	return "", &StatusNotAppliedError{
		IssueID:         issueID,
		Requested:       requested,
		Current:         issue.Status,
		Allowed:         allowed,
		AllowedReported: reported,
		OtherChanges:    otherChanges,
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
