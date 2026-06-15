package ops

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

func TestGetAttachment_DecodesResponse(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"attachment":{"id":42,"filename":"screenshot.png","filesize":1234,"content_type":"image/png","author":{"id":1,"name":"Grace"},"created_on":"2026-05-01T00:00:00Z"}}`))
	}))
	defer ts.Close()

	client := api.NewTestClient(ts.Client(), ts.URL, debug.New(nil))

	att, err := GetAttachment(context.Background(), client, GetAttachmentInput{ID: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/attachments/42.json" {
		t.Errorf("path = %q, want /attachments/42.json", gotPath)
	}
	if att.ID != 42 || att.Filename != "screenshot.png" || att.ContentType != "image/png" {
		t.Errorf("attachment = %+v, want id=42 screenshot.png image/png", att)
	}
}
