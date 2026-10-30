package ops

import (
	"context"
	"errors"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

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
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Updated journal %d", input.ID)}, nil
}
