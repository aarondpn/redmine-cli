package cmdutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aarondpn/redmine-cli/internal/api"
	"github.com/aarondpn/redmine-cli/internal/debug"
)

// pngSignature is a 1x1 transparent PNG used to verify content-type sniffing
// for files with no recognizable extension.
var pngSignature = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG header
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89,
}

type uploadRecord struct {
	filename string
	body     []byte
	length   int64
	ct       string
}

func newUploadServer(t *testing.T, tokens []string) (*httptest.Server, *[]uploadRecord) {
	t.Helper()
	var recs []uploadRecord
	var idx int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, r.ContentLength)
		if r.ContentLength > 0 {
			_, _ = r.Body.Read(b)
		}
		recs = append(recs, uploadRecord{
			filename: r.URL.Query().Get("filename"),
			body:     b,
			length:   r.ContentLength,
			ct:       r.Header.Get("Content-Type"),
		})
		i := int(atomic.AddInt32(&idx, 1)) - 1
		tok := "tok"
		if i < len(tokens) {
			tok = tokens[i]
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"upload":{"token":"` + tok + `"}}`))
	}))
	t.Cleanup(ts.Close)
	return ts, &recs
}

func newClientFor(ts *httptest.Server) *api.Client {
	// Build a Client manually, bypassing NewClient (which requires a full
	// config). We only need the Attachments service wired to the test server.
	httpClient := ts.Client()
	u, _ := url.Parse(ts.URL)
	_ = u
	c := apiClientForTest(httpClient, ts.URL)
	return c
}

func writeTempFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return p
}

func TestUploadAttachments_Empty(t *testing.T) {
	// No server hit, no error, nil result.
	ups, err := UploadAttachments(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ups != nil {
		t.Errorf("ups = %v, want nil", ups)
	}
}

func TestUploadAttachments_SingleFile_ExtensionCT(t *testing.T) {
	ts, recs := newUploadServer(t, []string{"tkA"})
	c := newClientFor(ts)

	data := []byte("some text content")
	p := writeTempFile(t, "notes.txt", data)

	ups, err := UploadAttachments(context.Background(), c, []string{p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ups) != 1 {
		t.Fatalf("len(ups) = %d, want 1", len(ups))
	}
	u := ups[0]
	if u.Token != "tkA" {
		t.Errorf("token = %q, want tkA", u.Token)
	}
	if u.Filename != "notes.txt" {
		t.Errorf("filename = %q, want notes.txt", u.Filename)
	}
	if !strings.HasPrefix(u.ContentType, "text/plain") {
		t.Errorf("content-type = %q, want text/plain*", u.ContentType)
	}

	if len(*recs) != 1 {
		t.Fatalf("server got %d requests, want 1", len(*recs))
	}
	r := (*recs)[0]
	if r.filename != "notes.txt" {
		t.Errorf("server filename = %q, want notes.txt", r.filename)
	}
	if string(r.body) != string(data) {
		t.Errorf("server body mismatch")
	}
	if r.length != int64(len(data)) {
		t.Errorf("server length = %d, want %d", r.length, len(data))
	}
	if r.ct != "application/octet-stream" {
		t.Errorf("server content-type = %q, want octet-stream", r.ct)
	}
}

func TestUploadAttachments_MultipleFiles_PathWithComma(t *testing.T) {
	ts, recs := newUploadServer(t, []string{"tk1", "tk2"})
	c := newClientFor(ts)

	p1 := writeTempFile(t, "a,b.txt", []byte("one"))
	p2 := writeTempFile(t, "second.bin", []byte("two"))

	ups, err := UploadAttachments(context.Background(), c, []string{p1, p2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ups) != 2 {
		t.Fatalf("len(ups) = %d, want 2", len(ups))
	}
	if ups[0].Filename != "a,b.txt" {
		t.Errorf("filename[0] = %q, want a,b.txt", ups[0].Filename)
	}
	if ups[1].Token != "tk2" {
		t.Errorf("token[1] = %q, want tk2", ups[1].Token)
	}
	if (*recs)[0].filename != "a,b.txt" {
		t.Errorf("server filename[0] = %q, want a,b.txt", (*recs)[0].filename)
	}
}

func TestUploadAttachments_SniffsContentTypeWhenExtensionUnknown(t *testing.T) {
	ts, _ := newUploadServer(t, []string{"tk"})
	c := newClientFor(ts)

	p := writeTempFile(t, "blob", pngSignature)

	ups, err := UploadAttachments(context.Background(), c, []string{p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ups[0].ContentType != "image/png" {
		t.Errorf("content-type = %q, want image/png (sniffed)", ups[0].ContentType)
	}
}

func TestUploadAttachments_MissingFile(t *testing.T) {
	ts, _ := newUploadServer(t, []string{"tk"})
	c := newClientFor(ts)

	_, err := UploadAttachments(context.Background(), c, []string{"/nonexistent/path/does/not/exist.bin"})
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !strings.Contains(err.Error(), "uploading") {
		t.Errorf("error should wrap path, got: %v", err)
	}
}

func TestUploadAttachments_StopsOnError(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":["too big"]}`))
	}))
	defer ts.Close()
	c := apiClientForTest(ts.Client(), ts.URL)

	p1 := writeTempFile(t, "first.txt", []byte("one"))
	p2 := writeTempFile(t, "second.txt", []byte("two"))

	_, err := UploadAttachments(context.Background(), c, []string{p1, p2})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("server calls = %d, want 1 (must stop after first failure)", got)
	}
}

// apiClientForTest constructs an *api.Client wired to the test server without
// requiring a full config. Defined in attachments_helper_test.go.
var apiClientForTest = func(httpClient *http.Client, baseURL string) *api.Client {
	return api.NewTestClient(httpClient, baseURL, debug.New(nil))
}
