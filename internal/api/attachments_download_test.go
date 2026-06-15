package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func TestAttachmentGet_Success(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"attachment":{"id":7,"filename":"diagram.png","filesize":2048,"content_type":"image/png","description":"arch","content_url":"http://example/attachments/download/7/diagram.png","author":{"id":3,"name":"Ada"},"created_on":"2026-01-02T03:04:05Z"}}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Attachments = &AttachmentService{client: c}

	att, err := c.Attachments.Get(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/attachments/7.json" {
		t.Errorf("path = %q, want /attachments/7.json", gotPath)
	}
	if att.ID != 7 || att.Filename != "diagram.png" || att.Filesize != 2048 {
		t.Errorf("attachment = %+v, want id=7 filename=diagram.png size=2048", att)
	}
	if att.ContentType != "image/png" || att.Author.Name != "Ada" {
		t.Errorf("attachment metadata = %+v, want content_type/image author Ada", att)
	}
}

func TestAttachmentGet_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Attachments = &AttachmentService{client: c}

	_, err := c.Attachments.Get(context.Background(), 999)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !apiErr.IsNotFound() {
		t.Fatalf("err = %v, want APIError 404", err)
	}
}

func TestAttachmentDownload_StreamsFromContentURL(t *testing.T) {
	want := []byte("\x89PNG\r\n\x1a\n raw binary \x00\x01\x02 bytes")
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(want)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Attachments = &AttachmentService{client: c}

	att := &models.Attachment{
		ID:         7,
		Filename:   "diagram.png",
		ContentURL: ts.URL + "/attachments/download/7/diagram.png",
	}

	var buf bytes.Buffer
	n, err := c.Attachments.Download(context.Background(), att, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/attachments/download/7/diagram.png" {
		t.Errorf("download path = %q, want /attachments/download/7/diagram.png", gotPath)
	}
	if n != int64(len(want)) {
		t.Errorf("bytes = %d, want %d", n, len(want))
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("body = %v, want %v (binary integrity)", buf.Bytes(), want)
	}
}

func TestAttachmentDownload_FallsBackToCanonicalPath(t *testing.T) {
	want := []byte("plain content")
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write(want)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Attachments = &AttachmentService{client: c}

	// No content_url: Download must build /attachments/download/:id/:filename
	// against the base URL and URL-escape the filename.
	att := &models.Attachment{ID: 9, Filename: "release notes.txt"}

	var buf bytes.Buffer
	if _, err := c.Attachments.Download(context.Background(), att, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/attachments/download/9/release notes.txt" {
		t.Errorf("download path = %q, want decoded /attachments/download/9/release notes.txt", gotPath)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("body = %q, want %q", buf.Bytes(), want)
	}
}

func TestAttachmentDownload_ErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Attachments = &AttachmentService{client: c}

	att := &models.Attachment{ID: 1, Filename: "x.bin", ContentURL: ts.URL + "/attachments/download/1/x.bin"}
	_, err := c.Attachments.Download(context.Background(), att, &bytes.Buffer{})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !apiErr.IsForbidden() {
		t.Fatalf("err = %v, want APIError 403", err)
	}
}

// TestAttachmentDownloadURL exercises the host-matching guard that keeps the
// API key (added to every request by authTransport) from being sent to a host
// other than the configured Redmine server.
func TestAttachmentDownloadURL(t *testing.T) {
	s := &AttachmentService{client: &Client{baseURL: "https://redmine.example.com/redmine"}}

	cases := []struct {
		name string
		att  *models.Attachment
		want string
	}{
		{
			name: "content_url on configured server is used verbatim",
			att:  &models.Attachment{ID: 1, Filename: "a.png", ContentURL: "https://redmine.example.com/redmine/attachments/download/1/a.png"},
			want: "https://redmine.example.com/redmine/attachments/download/1/a.png",
		},
		{
			name: "foreign content_url falls back to canonical base-URL path",
			att:  &models.Attachment{ID: 2, Filename: "b.png", ContentURL: "https://evil.example.net/steal"},
			want: "https://redmine.example.com/redmine/attachments/download/2/b.png",
		},
		{
			name: "empty content_url builds canonical path",
			att:  &models.Attachment{ID: 3, Filename: "c.png"},
			want: "https://redmine.example.com/redmine/attachments/download/3/c.png",
		},
		{
			name: "filename is URL-escaped in the built path",
			att:  &models.Attachment{ID: 4, Filename: "my file (1).png"},
			want: "https://redmine.example.com/redmine/attachments/download/4/my%20file%20%281%29.png",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := s.downloadURL(tc.att); got != tc.want {
				t.Errorf("downloadURL = %q, want %q", got, tc.want)
			}
		})
	}
}
