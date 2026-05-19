//go:build e2e

package e2e

import (
	"strconv"
	"testing"
)

// TestMyAccount_Get verifies the GET /my/account.json passthrough. The e2e
// stack authenticates as admin via API key, so api_key must come back
// populated.
func TestMyAccount_Get(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	var got struct {
		ID     int    `json:"id"`
		Login  string `json:"login"`
		APIKey string `json:"api_key"`
	}
	r.runJSON(t, &got, "my-account", "get")
	if got.ID == 0 {
		t.Fatalf("my-account get returned id 0: %+v", got)
	}
	if got.Login == "" {
		t.Errorf("my-account get returned empty login")
	}
	if got.APIKey == "" {
		t.Errorf("my-account get must return api_key (admin auth uses one)")
	}
}

// TestMyAccount_Update changes the authenticated user's firstname, verifies
// the round-trip, and restores the original value in t.Cleanup. The cleanup
// is required because the admin user is shared across the suite.
func TestMyAccount_Update(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	var before struct {
		ID        int    `json:"id"`
		FirstName string `json:"firstname"`
	}
	r.runJSON(t, &before, "my-account", "get")
	if before.FirstName == "" {
		t.Fatal("expected non-empty firstname before update")
	}

	original := before.FirstName
	suffix := uniqueShortSuffix(t)
	wanted := "E2E-" + suffix

	t.Cleanup(func() {
		var restored actionEnvelope
		r.runJSON(t, &restored, "my-account", "update", "--firstname", original)
		if !restored.Ok || restored.Action != "updated" || restored.Resource != "my-account" {
			t.Errorf("restore envelope unexpected: %+v", restored)
		}
	})

	var updated actionEnvelope
	r.runJSON(t, &updated, "my-account", "update", "--firstname", wanted)
	if !updated.Ok || updated.Action != "updated" || updated.Resource != "my-account" {
		t.Fatalf("unexpected update envelope: %+v", updated)
	}

	var after struct {
		FirstName string `json:"firstname"`
	}
	r.runJSON(t, &after, "my-account", "get")
	if after.FirstName != wanted {
		t.Errorf("firstname round-trip = %q, want %q (id %s)", after.FirstName, wanted, strconv.Itoa(before.ID))
	}
}
