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

// duplicateAttachmentsServer serves issue #3 with two attachments that share
// the filename screenshot.png; each download returns "body-<id>".
func duplicateAttachmentsServer(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/issues/3.json":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"issue":{"id":3,"project":{"id":1,"name":"P"},"tracker":{"id":1,"name":"T"},"status":{"id":1,"name":"New"},"priority":{"id":1,"name":"Normal"},"author":{"id":1,"name":"A"},"subject":"s","attachments":[`+
				`{"id":70,"filename":"screenshot.png","content_url":%q},`+
				`{"id":71,"filename":"screenshot.png","content_url":%q}]}}`,
				srv.URL+"/attachments/download/70/screenshot.png", srv.URL+"/attachments/download/71/screenshot.png")
		case "/attachments/download/70/screenshot.png":
			_, _ = w.Write([]byte("body-70"))
		case "/attachments/download/71/screenshot.png":
			_, _ = w.Write([]byte("body-71"))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestCmdIssueGet_DownloadAttachments_DuplicateFilenames is the regression
// test for same-named attachments silently overwriting each other: the first
// keeps its name, the second gets a deterministic suffix, and both paths are
// reported.
func TestCmdIssueGet_DownloadAttachments_DuplicateFilenames(t *testing.T) {
	srv := duplicateAttachmentsServer(t)
	f := testutil.NewFactory(t, srv.URL)
	dir := t.TempDir()
	cmd := NewCmdGet(f)
	cmd.SetArgs([]string{"3", "--download-attachments", dir, "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"screenshot.png":                 "body-70",
		"screenshot (attachment-71).png": "body-71",
	}
	for name, body := range want {
		path := filepath.Join(dir, name)
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
		if string(got) != body {
			t.Errorf("%s = %q, want %q", name, got, body)
		}
		if !strings.Contains(testutil.Stderr(f), path) {
			t.Errorf("stderr = %q, want the actual path %s reported", testutil.Stderr(f), path)
		}
	}
}

// TestCmdIssueGet_DownloadAttachments_ExistingFile refuses to overwrite a file
// that existed before the command ran, and fails before downloading anything;
// --clobber replaces it.
func TestCmdIssueGet_DownloadAttachments_ExistingFile(t *testing.T) {
	for _, clobber := range []bool{false, true} {
		t.Run(fmt.Sprintf("clobber=%v", clobber), func(t *testing.T) {
			srv := duplicateAttachmentsServer(t)
			f := testutil.NewFactory(t, srv.URL)
			dir := t.TempDir()
			existing := filepath.Join(dir, "screenshot (attachment-71).png")
			if err := os.WriteFile(existing, []byte("original"), 0o644); err != nil {
				t.Fatal(err)
			}

			args := []string{"3", "--download-attachments", dir, "--output", "json"}
			if clobber {
				args = append(args, "--clobber")
			}
			cmd := NewCmdGet(f)
			cmd.SetArgs(args)
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			err := cmd.Execute()

			got, _ := os.ReadFile(existing)
			if clobber {
				if err != nil {
					t.Fatalf("unexpected error with --clobber: %v", err)
				}
				if string(got) != "body-71" {
					t.Errorf("existing file = %q, want it replaced", got)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), existing) || !strings.Contains(err.Error(), "--clobber") {
				t.Fatalf("err = %v, want an error naming %s and suggesting --clobber", err, existing)
			}
			if string(got) != "original" {
				t.Errorf("existing file was modified: %q", got)
			}
			if _, err := os.Stat(filepath.Join(dir, "screenshot.png")); !os.IsNotExist(err) {
				t.Errorf("no attachment should be downloaded when a target exists (stat err = %v)", err)
			}
		})
	}
}
