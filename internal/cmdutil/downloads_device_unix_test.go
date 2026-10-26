//go:build unix

package cmdutil

import (
	"context"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestSaveAttachmentToFile_DevNull writes into /dev/null (directly and
// through a symlink) without --clobber, as `--path /dev/null` always did,
// and leaves the symlink and its directory as they were.
func TestSaveAttachmentToFile_DevNull(t *testing.T) {
	body := []byte("discarded bytes")
	dir := t.TempDir()
	link := filepath.Join(dir, "null")
	symlinkOrSkip(t, os.DevNull, link)

	for name, path := range map[string]string{"direct": os.DevNull, "symlink": link} {
		t.Run(name, func(t *testing.T) {
			client := newDownloadClient(t, body, http.StatusOK)
			n, err := SaveAttachmentToFile(context.Background(), client, attFor(client, "x.bin"), path, SaveOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if n != int64(len(body)) {
				t.Errorf("n = %d, want %d", n, len(body))
			}
		})
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&fs.ModeSymlink == 0 {
		t.Errorf("symlink was replaced (info = %v, err = %v)", info, err)
	}
	assertDirEntries(t, dir, "null")
}

// TestSaveAttachmentToFile_FIFO streams into a named pipe given as the
// explicit path instead of replacing it.
func TestSaveAttachmentToFile_FIFO(t *testing.T) {
	body := []byte("piped bytes")
	client := newDownloadClient(t, body, http.StatusOK)
	dir := t.TempDir()
	fifo := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}

	got := make(chan []byte, 1)
	go func() {
		r, err := os.Open(fifo)
		if err != nil {
			got <- nil
			return
		}
		defer r.Close()
		b, _ := io.ReadAll(r)
		got <- b
	}()

	n, err := SaveAttachmentToFile(context.Background(), client, attFor(client, "x.bin"), fifo, SaveOptions{})
	if err != nil {
		// Release the reader, which is still waiting for a writer.
		if w, openErr := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); openErr == nil {
			_ = w.Close()
		}
		t.Fatalf("unexpected error: %v", err)
	}
	if b := <-got; string(b) != string(body) || n != int64(len(body)) {
		t.Errorf("reader got %q (n = %d), want %q", b, n, body)
	}
	if info, err := os.Lstat(fifo); err != nil || info.Mode()&fs.ModeNamedPipe == 0 {
		t.Errorf("FIFO was replaced (info = %v, err = %v)", info, err)
	}
	assertDirEntries(t, dir, "pipe")
}

// A FIFO that merely has the attachment's name in a --dir destination was
// not chosen by the user, so it is still refused rather than written into.
func TestSaveAttachmentToDir_FIFORefused(t *testing.T) {
	client := newDownloadClient(t, []byte("x"), http.StatusOK)
	dir := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(dir, "report.pdf"), 0o600); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	for _, clobber := range []bool{false, true} {
		_, _, err := SaveAttachmentToDir(context.Background(), client, attFor(client, "report.pdf"), dir, SaveOptions{Clobber: clobber})
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Errorf("clobber=%v: err = %v, want a not-a-regular-file error", clobber, err)
		}
	}
}
