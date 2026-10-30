package api

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// JournalService handles journal (issue comment) API calls.
type JournalService struct {
	client *Client
}

// Update edits the notes and/or privacy of a journal. Redmine 5.0 or later
// only; it answers 204 even when the change was not applied.
func (s *JournalService) Update(ctx context.Context, id int, update models.JournalUpdate) error {
	body := map[string]interface{}{"journal": update}
	return s.client.Put(ctx, fmt.Sprintf("/journals/%d.json", id), body)
}
