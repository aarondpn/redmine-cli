//go:build unix

package cmdutil

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestSaveAttachmentToFile_HonorsUmask keeps a restrictive umask in effect: a
// user running with umask 077 got private (0o600) files from os.Create and
// must not get world-readable ones now that downloads go via a temp file.
func TestSaveAttachmentToFile_HonorsUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(old) })

	client := newDownloadClient(t, []byte("x"), http.StatusOK)
	path := filepath.Join(t.TempDir(), "private.bin")
	if _, err := SaveAttachmentToFile(context.Background(), client, attFor(client, "private.bin"), path, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 600 under umask 077", got)
	}
}
