package api

import (
	"context"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func TestJournalService_Update(t *testing.T) {
	c, rec := captureServer(t, 204, "")
	notes, private := "", true
	if err := c.Journals.Update(context.Background(), 12, models.JournalUpdate{Notes: &notes, PrivateNotes: &private}); err != nil {
		t.Fatal(err)
	}
	if rec.method != "PUT" || rec.path != "/journals/12.json" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if rec.body != `{"journal":{"notes":"","private_notes":true}}` {
		t.Errorf("body = %s", rec.body)
	}
}

func TestJournalService_UpdateOmitsUnsetFields(t *testing.T) {
	c, rec := captureServer(t, 204, "")
	private := false
	if err := c.Journals.Update(context.Background(), 12, models.JournalUpdate{PrivateNotes: &private}); err != nil {
		t.Fatal(err)
	}
	if rec.body != `{"journal":{"private_notes":false}}` {
		t.Errorf("body = %s", rec.body)
	}
}
