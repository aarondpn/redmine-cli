package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// private_notes is always emitted by Redmine's journal views, including false.
func TestJournalPrivateNotesFalseRoundTrips(t *testing.T) {
	var j Journal
	if err := json.Unmarshal([]byte(`{"id":1,"user":{"id":1,"name":"A"},"notes":"hi","created_on":"2025-01-01T00:00:00Z","private_notes":false,"details":[]}`), &j); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(j)
	if !strings.Contains(string(b), `"private_notes":false`) {
		t.Errorf("private_notes dropped: %s", b)
	}
}

func TestIssueIsPrivateFalseRoundTrips(t *testing.T) {
	var i Issue
	if err := json.Unmarshal([]byte(`{"id":1,"is_private":false}`), &i); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(i)
	if !strings.Contains(string(b), `"is_private":false`) {
		t.Errorf("is_private dropped: %s", b)
	}
}

func TestProjectFileDownloadsZeroRoundTrips(t *testing.T) {
	var f ProjectFile
	if err := json.Unmarshal([]byte(`{"id":1,"filename":"a.txt","downloads":0}`), &f); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(f)
	if !strings.Contains(string(b), `"downloads":0`) {
		t.Errorf("downloads dropped: %s", b)
	}
}

func TestUserAdminStatusPresence(t *testing.T) {
	var u User
	if err := json.Unmarshal([]byte(`{"id":5,"login":"bob"}`), &u); err != nil {
		t.Fatal(err)
	}
	if u.Admin != nil || u.Status != nil {
		t.Fatalf("absent keys must stay nil: %+v", u)
	}
	b, _ := json.Marshal(u)
	if strings.Contains(string(b), `"admin"`) || strings.Contains(string(b), `"status"`) {
		t.Errorf("nil admin/status must be omitted: %s", b)
	}
	if err := json.Unmarshal([]byte(`{"id":5,"admin":false,"status":1}`), &u); err != nil {
		t.Fatal(err)
	}
	b, _ = json.Marshal(u)
	if !strings.Contains(string(b), `"admin":false`) || !strings.Contains(string(b), `"status":1`) {
		t.Errorf("present values dropped: %s", b)
	}
}
