package issue

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

var issuePNG = []byte{0x89, 'P', 'N', 'G', 0x00, 0x01, 0x02, 0xff}

// issueWithAttachmentsServer serves issue #1 with one attachment (when
// include=attachments is requested) plus the raw bytes at the content_url.
func issueWithAttachmentsServer(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/issues/1.json":
			if !strings.Contains(r.URL.RawQuery, "attachments") {
				t.Errorf("expected include=attachments, got query %q", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"issue":{"id":1,"project":{"id":1,"name":"P"},"tracker":{"id":1,"name":"T"},"status":{"id":1,"name":"New"},"priority":{"id":1,"name":"Normal"},"author":{"id":1,"name":"A"},"subject":"s","attachments":[{"id":55,"filename":"shot.png","filesize":%d,"content_type":"image/png","content_url":%q,"author":{"id":1,"name":"A"},"created_on":"2026-01-01T00:00:00Z"}]}}`,
				len(issuePNG), srv.URL+"/attachments/download/55/shot.png")
		case "/attachments/download/55/shot.png":
			_, _ = w.Write(issuePNG)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCmdIssueGet_AttachmentsIncludedInJSON(t *testing.T) {
	srv := issueWithAttachmentsServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdGet(f)
	cmd.SetArgs([]string{"1", "--attachments", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := testutil.Stdout(f)
	if !strings.Contains(out, `"filename": "shot.png"`) || !strings.Contains(out, `"id": 55`) {
		t.Fatalf("json stdout = %q, want attachment shot.png id 55", out)
	}
}

func TestCmdIssueGet_DownloadAttachments_NoAttachments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issues/2.json" {
			t.Errorf("unexpected request %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issue":{"id":2,"project":{"id":1,"name":"P"},"tracker":{"id":1,"name":"T"},"status":{"id":1,"name":"New"},"priority":{"id":1,"name":"Normal"},"author":{"id":1,"name":"A"},"subject":"s","attachments":[]}}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	dir := t.TempDir()
	cmd := NewCmdGet(f)
	cmd.SetArgs([]string{"2", "--download-attachments", dir, "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected success for an issue with no attachments: %v", err)
	}
	if !strings.Contains(testutil.Stderr(f), "no attachments to download") {
		t.Errorf("stderr = %q, want a no-attachments warning", testutil.Stderr(f))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files written, got %d", len(entries))
	}
}

func TestCmdIssueGet_DownloadAttachments(t *testing.T) {
	srv := issueWithAttachmentsServer(t)
	f := testutil.NewFactory(t, srv.URL)
	dir := t.TempDir()
	cmd := NewCmdGet(f)
	cmd.SetArgs([]string{"1", "--download-attachments", dir, "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "shot.png"))
	if err != nil {
		t.Fatalf("expected downloaded attachment: %v", err)
	}
	if string(got) != string(issuePNG) {
		t.Error("downloaded attachment bytes differ from source")
	}
	// The issue JSON still lands on stdout; download confirmation on stderr.
	if !strings.Contains(testutil.Stdout(f), `"subject": "s"`) {
		t.Errorf("stdout should still carry the issue JSON, got %q", testutil.Stdout(f))
	}
	if !strings.Contains(testutil.Stderr(f), "shot.png") {
		t.Errorf("stderr = %q, want download confirmation", testutil.Stderr(f))
	}
}
