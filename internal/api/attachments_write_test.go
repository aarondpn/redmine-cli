package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func TestAttachmentService_Update(t *testing.T) {
	c, rec := captureServer(t, 204, "")
	fn, desc := "new.png", ""
	if err := c.Attachments.Update(context.Background(), 7, models.AttachmentUpdate{Filename: &fn, Description: &desc}); err != nil {
		t.Fatal(err)
	}
	if rec.method != "PUT" || rec.path != "/attachments/7.json" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	if rec.body != `{"attachment":{"filename":"new.png","description":""}}` {
		t.Errorf("body = %s", rec.body)
	}
}

func TestAttachmentService_Delete(t *testing.T) {
	c, rec := captureServer(t, 204, "")
	if err := c.Attachments.Delete(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if rec.method != "DELETE" || rec.path != "/attachments/7.json" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
}

func TestAttachmentService_Thumbnail(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNGdata"))
	}))
	defer ts.Close()
	c := newTestClient(ts)
	c.Attachments = &AttachmentService{client: c}

	var buf strings.Builder
	n, err := c.Attachments.Thumbnail(context.Background(), 7, 0, &buf)
	if err != nil || n != 8 || buf.String() != "\x89PNGdata" {
		t.Fatalf("n=%d err=%v body=%q", n, err, buf.String())
	}
	if gotPath != "/attachments/thumbnail/7" {
		t.Errorf("path = %q", gotPath)
	}

	if _, err := c.Attachments.Thumbnail(context.Background(), 7, 150, io.Discard); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/attachments/thumbnail/7/150" {
		t.Errorf("path = %q", gotPath)
	}
}

func TestAttachmentService_Thumbnail_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.NotFoundHandler())
	defer ts.Close()
	c := newTestClient(ts)
	c.Attachments = &AttachmentService{client: c}

	_, err := c.Attachments.Thumbnail(context.Background(), 7, 0, io.Discard)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
		t.Fatalf("err = %v, want 404 APIError", err)
	}
}
