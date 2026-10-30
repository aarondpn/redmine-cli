package ops

import (
	"context"
	"errors"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// JournalEditHint explains 401/404 answers: Redmine before 5.0 has no journal
// update endpoint.
const JournalEditHint = "Editing comments through the REST API needs Redmine 5.0 or later; older servers reject the request. " +
	"Check the journal ID with get_issue (includes=[\"journals\"]) or `redmine issues get <issue> --journals`."

// JournalEditForbiddenHint explains 403 answers. Redmine 4.2 ignores the API key
// for this endpoint and answers 403 to the anonymous request, so it names both
// possible causes.
const JournalEditForbiddenHint = "Redmine denied this edit: the account may lack the Edit issue notes permission (or Edit own issue notes for its own comments), or the server is older than 5.0 (Redmine 5.0+ is required), which does not accept comment edits over the API. Don't retry with other credentials; ask the user or a Redmine administrator."

// HintedError carries a remediation hint next to the underlying error. Its
// message includes the hint so MCP clients see it; the CLI unwraps it to render
// the hint on its own line.
type HintedError struct {
	Err  error
	Hint string
}

func (e *HintedError) Error() string { return e.Err.Error() + " (" + e.Hint + ")" }
func (e *HintedError) Unwrap() error { return e.Err }

type UpdateJournalInput struct {
	ID           int     `json:"id" jsonschema:"Journal ID (get_issue with includes=[\"journals\"] lists them)."`
	Notes        *string `json:"notes,omitempty" jsonschema:"New note text. An empty string removes the comment's text (the comment then disappears from the issue history)."`
	PrivateNotes *bool   `json:"private_notes,omitempty" jsonschema:"Make the note private (true) or public (false). Needs the set_notes_private permission; silently ignored otherwise."`
}

//mcpgen:tool update_journal
//mcpgen:description Edit the notes or privacy of an issue journal (comment) by journal ID; find IDs via get_issue with includes=["journals"]. Needs Redmine 5.0+. An empty notes string removes the comment's text. Requires --enable-writes.
//mcpgen:category issues
//mcpgen:writes
func UpdateJournal(ctx context.Context, client *api.Client, input UpdateJournalInput) (MessageResult, error) {
	if input.ID <= 0 {
		return MessageResult{}, errors.New("id must be a positive journal ID")
	}
	if input.Notes == nil && input.PrivateNotes == nil {
		return MessageResult{}, errors.New("nothing to update: provide notes and/or private_notes")
	}
	err := client.Journals.Update(ctx, input.ID, models.JournalUpdate{Notes: input.Notes, PrivateNotes: input.PrivateNotes})
	if err != nil {
		var apiErr *api.APIError
		if errors.As(err, &apiErr) {
			switch {
			case apiErr.IsAuthError() || apiErr.IsNotFound():
				return MessageResult{}, &HintedError{Err: err, Hint: JournalEditHint}
			case apiErr.IsForbidden():
				return MessageResult{}, &HintedError{Err: err, Hint: JournalEditForbiddenHint}
			}
		}
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Updated journal %d", input.ID)}, nil
}
