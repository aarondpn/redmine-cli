package ops

import (
	"context"
	"fmt"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// ProjectNotAppliedError reports that Redmine accepted an issue update but
// left the issue in its old project.
//
// Redmine only moves an issue to a project in which the user may add issues
// and that has at least one tracker; any other project_id is dropped and the
// update still answers 204 No Content. UpdateIssue reads the issue back
// whenever a move was requested to notice that.
type ProjectNotAppliedError struct {
	IssueID     int
	RequestedID int
	Current     models.IDName
	// OtherChanges reports whether the same request carried changes besides
	// the move. Redmine applies those even when it drops the move.
	OtherChanges bool
}

func (e *ProjectNotAppliedError) Error() string {
	if details := e.Details(); len(details) > 0 {
		return e.Summary() + " " + strings.Join(details, " ")
	}
	return e.Summary()
}

// Summary is the one-paragraph explanation of what happened, without the
// follow-up facts returned by Details.
func (e *ProjectNotAppliedError) Summary() string {
	return fmt.Sprintf("Issue #%d was not moved to project ID %d; it is still in project %q (ID %d). "+
		"Redmine silently ignores a move to a project in which you may not add issues (Add issues permission) or that has no trackers.",
		e.IssueID, e.RequestedID, e.Current.Name, e.Current.ID)
}

// Details lists the follow-up facts a caller needs to recover.
func (e *ProjectNotAppliedError) Details() []string {
	if !e.OtherChanges {
		return nil
	}
	return []string{"The other changes in this request (fields, notes, attachments) may already have been applied; do not resend them blindly."}
}

// verifyProjectApplied reads the issue back after an update that requested a
// move. A failed read-back is reported as a note: the update itself went
// through.
func verifyProjectApplied(ctx context.Context, client *api.Client, issueID, projectID int, otherChanges bool) (string, error) {
	issue, err := client.Issues.Get(ctx, issueID, nil)
	if err != nil {
		return fmt.Sprintf("could not verify that issue #%d was moved to project ID %d: %v", issueID, projectID, err), nil
	}
	if issue.Project.ID == projectID {
		return "", nil
	}
	return "", &ProjectNotAppliedError{
		IssueID:      issueID,
		RequestedID:  projectID,
		Current:      issue.Project,
		OtherChanges: otherChanges,
	}
}
