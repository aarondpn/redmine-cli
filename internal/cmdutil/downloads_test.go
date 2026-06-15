package cmdutil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func newDownloadClient(t *testing.T, body []byte, status int) *api.Client {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if status != 0 && status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(ts.Close)
	return api.NewTestClient(ts.Client(), ts.URL, debug.New(nil))
}

func attFor(client *api.Client, filename string) *models.Attachment {
	// content_url is left empty so Download builds the canonical path against
	// the test client's base URL (any path resolves to the stub handler).
	return &models.Attachment{ID: 1, Filename: filename}
}

func TestSaveAttachmentToDir_WritesRealFilename(t *testing.T) {
	want := []byte("file body bytes")
	client := newDownloadClient(t, want, http.StatusOK)
	dir := t.TempDir()

	path, n, err := SaveAttachmentToDir(context.Background(), client, attFor(client, "report.pdf"), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := filepath.Join(dir, "report.pdf"); path != got {
		t.Errorf("path = %q, want %q", path, got)
	}
	if n != int64(len(want)) {
		t.Errorf("bytes = %d, want %d", n, len(want))
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("content = %q, want %q", got, want)
	}
}

// TestSaveAttachmentToDir_SanitizesFilename guards against a server-supplied
// filename escaping the destination directory via path traversal.
func TestSaveAttachmentToDir_SanitizesFilename(t *testing.T) {
	client := newDownloadClient(t, []byte("x"), http.StatusOK)
	dir := t.TempDir()

	path, _, err := SaveAttachmentToDir(context.Background(), client, attFor(client, "../../../etc/evil"), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Dir(path) != filepath.Clean(dir) {
		t.Errorf("written outside dir: path=%q dir=%q", path, dir)
	}
	if filepath.Base(path) != "evil" {
		t.Errorf("base = %q, want sanitized to evil", filepath.Base(path))
	}
}

// TestSaveAttachmentToDir_FallsBackForDegenerateNames covers names that
// filepath.Base reduces to a value that would escape dir or name the directory
// itself ("..", "/", ""); each must fall back to a contained synthetic name.
func TestSaveAttachmentToDir_FallsBackForDegenerateNames(t *testing.T) {
	for _, name := range []string{"..", "../..", "/", ""} {
		t.Run("name="+name, func(t *testing.T) {
			client := newDownloadClient(t, []byte("x"), http.StatusOK)
			dir := t.TempDir()

			path, _, err := SaveAttachmentToDir(context.Background(), client, attFor(client, name), dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if filepath.Dir(path) != filepath.Clean(dir) {
				t.Errorf("written outside dir: path=%q dir=%q", path, dir)
			}
			if filepath.Base(path) != "attachment-1" {
				t.Errorf("base = %q, want synthetic attachment-1", filepath.Base(path))
			}
			if _, err := os.Stat(path); err != nil {
				t.Errorf("expected file at %q: %v", path, err)
			}
		})
	}
}

// TestSaveAttachmentToDir_CreatesDir confirms the destination directory is
// created when missing (used by both `attachments download --dir` and
// `issues get --download-attachments`).
func TestSaveAttachmentToDir_CreatesDir(t *testing.T) {
	client := newDownloadClient(t, []byte("data"), http.StatusOK)
	dir := filepath.Join(t.TempDir(), "nested", "created")

	path, _, err := SaveAttachmentToDir(context.Background(), client, attFor(client, "a.bin"), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file at %q: %v", path, err)
	}
}

// TestSaveAttachmentToFile_CleansUpPartialOnError verifies a failed download
// does not leave a truncated artifact behind.
func TestSaveAttachmentToFile_CleansUpPartialOnError(t *testing.T) {
	client := newDownloadClient(t, nil, http.StatusForbidden)
	path := filepath.Join(t.TempDir(), "partial.bin")

	if _, err := SaveAttachmentToFile(context.Background(), client, attFor(client, "partial.bin"), path); err == nil {
		t.Fatal("expected error on 403 download")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("partial file should have been removed, stat err = %v", err)
	}
}
