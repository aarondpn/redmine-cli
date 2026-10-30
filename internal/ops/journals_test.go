package ops

import (
	"context"
	"testing"
)

func TestUpdateJournal_Validation(t *testing.T) {
	client, calls := opsServer(t, nil)
	for name, in := range map[string]UpdateJournalInput{
		"no id":     {Notes: ptr("x")},
		"no fields": {ID: 1},
	} {
		if _, err := UpdateJournal(context.Background(), client, in); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if len(*calls) != 0 {
		t.Errorf("unexpected requests: %v", *calls)
	}
}

func TestUpdateJournal_Sends(t *testing.T) {
	client, calls := opsServer(t, nil)
	private := true
	res, err := UpdateJournal(context.Background(), client, UpdateJournalInput{ID: 9, Notes: ptr(""), PrivateNotes: &private})
	if err != nil {
		t.Fatal(err)
	}
	if res.Message != "Updated journal 9" {
		t.Errorf("message = %q", res.Message)
	}
	c := (*calls)[0]
	if c.method != "PUT" || c.uri != "/journals/9.json" || c.body != `{"journal":{"notes":"","private_notes":true}}` {
		t.Errorf("call = %+v", c)
	}
}
