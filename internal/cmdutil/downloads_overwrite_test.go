package cmdutil

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// newTruncatingClient returns a client whose download advertises more bytes
// than it sends, so the stream fails part-way through.
func newTruncatingClient(t *testing.T) *api.Client {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte("only a few bytes"))
	}))
	t.Cleanup(ts.Close)
	return api.NewTestClient(ts.Client(), ts.URL, debug.New(nil))
}

// assertDirEntries fails unless dir contains exactly the named entries, which
// catches leftover temporary files.
func assertDirEntries(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(entries))
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("dir entries = %v, want %v", got, want)
	}
}

func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
}

// stubLink makes the hard-link step fail as it would on a filesystem without
// hard-link support, forcing the check-then-rename fallback.
func stubLink(t *testing.T) {
	t.Helper()
	orig := linkFile
	linkFile = func(oldname, newname string) error {
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: errors.New("operation not supported")}
	}
	t.Cleanup(func() { linkFile = orig })
}

// TestSaveAttachmentToFile_FailedDownloadKeepsExistingFile is the regression
// test for the old cleanup deleting a file that existed before the download:
// with clobber, a download that fails (before or during streaming) must leave
// the original bytes intact and no temporary file behind.
func TestSaveAttachmentToFile_FailedDownloadKeepsExistingFile(t *testing.T) {
	tests := []struct {
		name   string
		client func(t *testing.T) *api.Client
	}{
		{"http error", func(t *testing.T) *api.Client { return newDownloadClient(t, nil, http.StatusForbidden) }},
		{"truncated stream", newTruncatingClient},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.client(t)
			dir := t.TempDir()
			path := filepath.Join(dir, "keep.txt")
			if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := SaveAttachmentToFile(context.Background(), client, attFor(client, "keep.txt"), path, SaveOptions{Clobber: true}); err == nil {
				t.Fatal("expected download error")
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("pre-existing file was removed: %v", err)
			}
			if string(got) != "original" {
				t.Errorf("pre-existing file content = %q, want %q", got, "original")
			}
			assertDirEntries(t, dir, "keep.txt")
		})
	}
}

// TestSaveAttachmentToFile_CancelledMidStream covers Ctrl-C during a
// download: main cancels the command context, the stream fails, and the
// temporary file is removed instead of being left behind.
func TestSaveAttachmentToFile_CancelledMidStream(t *testing.T) {
	for _, clobber := range []bool{false, true} {
		t.Run(fmt.Sprintf("clobber=%v", clobber), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "1000")
				_, _ = w.Write([]byte("first chunk"))
				w.(http.Flusher).Flush()
				// The client has started streaming into the temp file;
				// interrupt it and hold the rest of the body back.
				cancel()
				select {
				case <-r.Context().Done():
				case <-time.After(10 * time.Second):
				}
			}))
			defer ts.Close()
			client := api.NewTestClient(ts.Client(), ts.URL, debug.New(nil))

			dir := t.TempDir()
			path := filepath.Join(dir, "big.bin")
			_, err := SaveAttachmentToFile(ctx, client, attFor(client, "big.bin"), path, SaveOptions{Clobber: clobber})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("SaveAttachmentToFile() error = %v, want context.Canceled", err)
			}
			assertDirEntries(t, dir)
		})
	}
}

func TestSaveAttachmentToFile_AlreadyCancelled(t *testing.T) {
	client := newDownloadClient(t, []byte("payload"), http.StatusOK)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if _, err := SaveAttachmentToFile(ctx, client, attFor(client, "a.txt"), path, SaveOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("SaveAttachmentToFile() error = %v, want context.Canceled", err)
	}
	assertDirEntries(t, dir)
}

// TestSaveAttachmentToFile_ExistingTarget reproduces the H-03 PoC: an
// existing ./notes.txt must not be truncated unless clobber is requested.
func TestSaveAttachmentToFile_ExistingTarget(t *testing.T) {
	tests := []struct {
		name    string
		clobber bool
		want    string
	}{
		{"refused without clobber", false, "original"},
		{"replaced with clobber", true, "server bytes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newDownloadClient(t, []byte("server bytes"), http.StatusOK)
			dir := t.TempDir()
			path := filepath.Join(dir, "notes.txt")
			if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
				t.Fatal(err)
			}

			_, err := SaveAttachmentToFile(context.Background(), client, attFor(client, "notes.txt"), path, SaveOptions{Clobber: tt.clobber})
			if tt.clobber && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.clobber {
				var exists *DestinationExistsError
				if !errors.As(err, &exists) || !errors.Is(err, fs.ErrExist) {
					t.Fatalf("err = %v, want DestinationExistsError", err)
				}
				if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "--clobber") {
					t.Errorf("error %q should name the path and suggest --clobber", err)
				}
			}
			got, _ := os.ReadFile(path)
			if string(got) != tt.want {
				t.Errorf("content = %q, want %q", got, tt.want)
			}
			assertDirEntries(t, dir, "notes.txt")
		})
	}
}

// TestSaveAttachmentToFile_SymlinkTarget reproduces the H-03 PoC of writing
// through a symlink: the file the link points at must never change. Without
// clobber the download is refused; with clobber the link itself is replaced.
func TestSaveAttachmentToFile_SymlinkTarget(t *testing.T) {
	for _, clobber := range []bool{false, true} {
		t.Run(fmt.Sprintf("clobber=%v", clobber), func(t *testing.T) {
			client := newDownloadClient(t, []byte("server bytes"), http.StatusOK)
			victim := filepath.Join(t.TempDir(), "authorized_keys")
			if err := os.WriteFile(victim, []byte("ssh-ed25519 AAAA"), 0o600); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "notes.txt")
			symlinkOrSkip(t, victim, path)

			_, err := SaveAttachmentToFile(context.Background(), client, attFor(client, "notes.txt"), path, SaveOptions{Clobber: clobber})
			if clobber && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !clobber && !errors.Is(err, fs.ErrExist) {
				t.Fatalf("err = %v, want an already-exists error", err)
			}

			if got, _ := os.ReadFile(victim); string(got) != "ssh-ed25519 AAAA" {
				t.Errorf("symlink target was written through: %q", got)
			}
			info, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if isLink := info.Mode()&fs.ModeSymlink != 0; isLink == clobber {
				t.Errorf("symlink at target: isLink=%v, want %v", isLink, !clobber)
			}
			if clobber {
				if got, _ := os.ReadFile(path); string(got) != "server bytes" {
					t.Errorf("replaced file content = %q", got)
				}
			}
			assertDirEntries(t, dir, "notes.txt")
		})
	}
}

// TestSaveAttachmentToFile_DanglingSymlink counts a dangling symlink as an
// existing entry and never creates the file it points at.
func TestSaveAttachmentToFile_DanglingSymlink(t *testing.T) {
	client := newDownloadClient(t, []byte("server bytes"), http.StatusOK)
	dir := t.TempDir()
	missing := filepath.Join(t.TempDir(), "created-through-link")
	path := filepath.Join(dir, "notes.txt")
	symlinkOrSkip(t, missing, path)

	_, err := SaveAttachmentToFile(context.Background(), client, attFor(client, "notes.txt"), path, SaveOptions{})
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("err = %v, want an already-exists error", err)
	}
	if _, err := os.Lstat(missing); !os.IsNotExist(err) {
		t.Errorf("file was created through the dangling symlink (stat err = %v)", err)
	}
	assertDirEntries(t, dir, "notes.txt")
}

// TestSaveAttachmentToFile_DirectoryTarget refuses a directory at the target,
// even with clobber.
func TestSaveAttachmentToFile_DirectoryTarget(t *testing.T) {
	for _, clobber := range []bool{false, true} {
		t.Run(fmt.Sprintf("clobber=%v", clobber), func(t *testing.T) {
			client := newDownloadClient(t, []byte("server bytes"), http.StatusOK)
			dir := t.TempDir()
			path := filepath.Join(dir, "sub")
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}

			_, err := SaveAttachmentToFile(context.Background(), client, attFor(client, "sub"), path, SaveOptions{Clobber: clobber})
			if err == nil || !strings.Contains(err.Error(), "is a directory") {
				t.Fatalf("err = %v, want an is-a-directory error", err)
			}
			if info, err := os.Lstat(path); err != nil || !info.IsDir() {
				t.Errorf("directory was replaced (info=%v, err=%v)", info, err)
			}
			assertDirEntries(t, dir, "sub")
		})
	}
}

// TestSaveAttachmentToFile_TargetAppearsDuringDownload covers the window
// between the up-front existence check and the final move: a file created
// while the bytes are streaming must not be replaced without clobber, both on
// the hard-link path and on the check-then-rename fallback.
func TestSaveAttachmentToFile_TargetAppearsDuringDownload(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		t.Run(fmt.Sprintf("linkFallback=%v", fallback), func(t *testing.T) {
			if fallback {
				stubLink(t)
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "race.txt")
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if err := os.WriteFile(path, []byte("planted"), 0o644); err != nil {
					t.Errorf("plant file: %v", err)
				}
				_, _ = w.Write([]byte("server bytes"))
			}))
			t.Cleanup(ts.Close)
			client := api.NewTestClient(ts.Client(), ts.URL, debug.New(nil))

			_, err := SaveAttachmentToFile(context.Background(), client, attFor(client, "race.txt"), path, SaveOptions{})
			if !errors.Is(err, fs.ErrExist) {
				t.Fatalf("err = %v, want an already-exists error", err)
			}
			if got, _ := os.ReadFile(path); string(got) != "planted" {
				t.Errorf("content = %q, want the file created during the download kept", got)
			}
			assertDirEntries(t, dir, "race.txt")
		})
	}
}

// TestSaveAttachmentToFile_LinkFallbackWrites confirms the rename fallback
// still saves the file when hard links are unsupported.
func TestSaveAttachmentToFile_LinkFallbackWrites(t *testing.T) {
	stubLink(t)
	client := newDownloadClient(t, []byte("server bytes"), http.StatusOK)
	dir := t.TempDir()
	path := filepath.Join(dir, "out.bin")

	if _, err := SaveAttachmentToFile(context.Background(), client, attFor(client, "out.bin"), path, SaveOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "server bytes" {
		t.Errorf("content = %q", got)
	}
	assertDirEntries(t, dir, "out.bin")
}

// TestSaveAttachmentToFile_FileMode gives a new file the permissions os.Create
// would (umask applied, so 0o644 under the usual umask) rather than the 0o600
// of the temporary file, and keeps the permissions of a regular file replaced
// with clobber.
func TestSaveAttachmentToFile_FileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions")
	}
	dir := t.TempDir()
	ref, err := os.Create(filepath.Join(dir, "reference"))
	if err != nil {
		t.Fatal(err)
	}
	_ = ref.Close()
	refInfo, err := os.Stat(ref.Name())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		existing os.FileMode // 0 means no file at the target
		clobber  bool
		want     os.FileMode
	}{
		{"new file", 0, false, refInfo.Mode().Perm()},
		{"clobber keeps restricted mode", 0o600, true, 0o600},
		{"clobber keeps wider mode", 0o664, true, 0o664},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newDownloadClient(t, []byte("x"), http.StatusOK)
			path := filepath.Join(t.TempDir(), "mode.bin")
			if tt.existing != 0 {
				if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, tt.existing); err != nil {
					t.Fatal(err)
				}
			}

			if _, err := SaveAttachmentToFile(context.Background(), client, attFor(client, "mode.bin"), path, SaveOptions{Clobber: tt.clobber}); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != tt.want {
				t.Errorf("mode = %o, want %o", got, tt.want)
			}
		})
	}
}

// TestSaveAttachmentToFile_ClobberSymlinkDoesNotCopyTargetMode replaces a
// symlink with a file that has the default mode, not the mode of whatever the
// link pointed at.
func TestSaveAttachmentToFile_ClobberSymlinkDoesNotCopyTargetMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions")
	}
	client := newDownloadClient(t, []byte("x"), http.StatusOK)
	victim := filepath.Join(t.TempDir(), "script.sh")
	if err := os.WriteFile(victim, []byte("#!/bin/sh"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(victim, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "notes.txt")
	symlinkOrSkip(t, victim, path)

	if _, err := SaveAttachmentToFile(context.Background(), client, attFor(client, "notes.txt"), path, SaveOptions{Clobber: true}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 != 0 {
		t.Errorf("mode = %o, want the default mode, not the link target's", info.Mode().Perm())
	}
}

// TestSaveAttachmentToFile_ExistingTargetNotDownloaded refuses an existing
// target (file, symlink or dangling symlink) before any download request is
// made, also on the check-then-rename fallback where no hard link would catch
// it later.
func TestSaveAttachmentToFile_ExistingTargetNotDownloaded(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, path string)
	}{
		{"file", func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink", func(t *testing.T, path string) {
			victim := filepath.Join(t.TempDir(), "victim")
			if err := os.WriteFile(victim, []byte("original"), 0o644); err != nil {
				t.Fatal(err)
			}
			symlinkOrSkip(t, victim, path)
		}},
		{"dangling symlink", func(t *testing.T, path string) {
			symlinkOrSkip(t, filepath.Join(t.TempDir(), "missing"), path)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubLink(t)
			requests := 0
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests++
				_, _ = w.Write([]byte("server bytes"))
			}))
			t.Cleanup(ts.Close)
			client := api.NewTestClient(ts.Client(), ts.URL, debug.New(nil))
			dir := t.TempDir()
			path := filepath.Join(dir, "notes.txt")
			tt.setup(t, path)

			_, err := SaveAttachmentToFile(context.Background(), client, attFor(client, "notes.txt"), path, SaveOptions{})
			if !errors.Is(err, fs.ErrExist) {
				t.Fatalf("err = %v, want an already-exists error", err)
			}
			if requests != 0 {
				t.Errorf("download requests = %d, want 0", requests)
			}
			info, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if tt.name != "file" && info.Mode()&fs.ModeSymlink == 0 {
				t.Error("symlink at target was replaced")
			}
			assertDirEntries(t, dir, "notes.txt")
		})
	}
}

// TestSaveAttachmentToFile_TargetAppearsBeforeLink covers the last window,
// after the fallback re-check would have run: a file planted right before the
// final hard link must make the link fail atomically and be kept.
func TestSaveAttachmentToFile_TargetAppearsBeforeLink(t *testing.T) {
	orig := linkFile
	linkFile = func(oldname, newname string) error {
		if err := os.WriteFile(newname, []byte("planted"), 0o644); err != nil {
			t.Errorf("plant file: %v", err)
		}
		return orig(oldname, newname)
	}
	t.Cleanup(func() { linkFile = orig })

	client := newDownloadClient(t, []byte("server bytes"), http.StatusOK)
	dir := t.TempDir()
	path := filepath.Join(dir, "race.txt")

	_, err := SaveAttachmentToFile(context.Background(), client, attFor(client, "race.txt"), path, SaveOptions{})
	var exists *DestinationExistsError
	if !errors.As(err, &exists) {
		t.Fatalf("err = %v, want DestinationExistsError", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "planted" {
		t.Errorf("content = %q, want the planted file kept", got)
	}
	assertDirEntries(t, dir, "race.txt")
}

// TestSaveAttachmentToDir_RefusesHiddenFilename refuses a server-supplied
// dotfile name such as .bashrc, without creating the directory or
// contacting the server, unless opts.AllowHidden is set.
func TestSaveAttachmentToDir_RefusesHiddenFilename(t *testing.T) {
	tests := []struct {
		filename string
		hidden   bool
	}{
		{".bashrc", true},
		{"../.gitconfig", true},
		{"report.pdf", false},
		{"..", false}, // saved as attachment-<id>
	}
	for _, tt := range tests {
		for _, allow := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/allow=%v", tt.filename, allow), func(t *testing.T) {
				requests := 0
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					requests++
					_, _ = w.Write([]byte("x"))
				}))
				t.Cleanup(ts.Close)
				client := api.NewTestClient(ts.Client(), ts.URL, debug.New(nil))
				dir := filepath.Join(t.TempDir(), "out")

				path, _, err := SaveAttachmentToDir(context.Background(), client, attFor(client, tt.filename), dir, SaveOptions{AllowHidden: allow})
				if !tt.hidden || allow {
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					if _, err := os.Stat(path); err != nil {
						t.Errorf("expected file at %q: %v", path, err)
					}
					return
				}
				var hidden *HiddenFilenameError
				if !errors.As(err, &hidden) {
					t.Fatalf("err = %v, want HiddenFilenameError", err)
				}
				name := filepath.Base(tt.filename)
				if msg := err.Error(); !strings.Contains(msg, name) || !strings.Contains(msg, filepath.Join(dir, name)) || !strings.Contains(msg, "--allow-hidden") {
					t.Errorf("error %q should name the file, its path and --allow-hidden", msg)
				}
				if requests != 0 {
					t.Errorf("server was asked %d times, want 0", requests)
				}
				if _, err := os.Stat(dir); !os.IsNotExist(err) {
					t.Errorf("destination directory was created (stat err = %v)", err)
				}
			})
		}
	}
}

// An explicit path is the user's choice, so a hidden name is fine there.
func TestSaveAttachmentToFile_HiddenPathAllowed(t *testing.T) {
	client := newDownloadClient(t, []byte("x"), http.StatusOK)
	path := filepath.Join(t.TempDir(), ".bashrc")
	if _, err := SaveAttachmentToFile(context.Background(), client, attFor(client, ".bashrc"), path, SaveOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file at %q: %v", path, err)
	}
}

func TestUniqueAttachmentFilenames_NewestKeepsName(t *testing.T) {
	// The API does not promise any order, so the result must not depend on it.
	for _, atts := range [][]models.Attachment{
		{{ID: 5, Filename: "log.txt"}, {ID: 9, Filename: "LOG.txt"}, {ID: 7, Filename: "log.txt"}},
		{{ID: 9, Filename: "LOG.txt"}, {ID: 7, Filename: "log.txt"}, {ID: 5, Filename: "log.txt"}},
	} {
		got := map[int]string{}
		for i, name := range UniqueAttachmentFilenames(atts) {
			got[atts[i].ID] = name
		}
		want := map[int]string{9: "LOG.txt", 7: "log (attachment-7).txt", 5: "log (attachment-5).txt"}
		for id, name := range want {
			if got[id] != name {
				t.Errorf("attachment %d saved as %q, want %q (input %v)", id, got[id], name, atts)
			}
		}
	}
}

func TestUniqueAttachmentFilenames(t *testing.T) {
	atts := []models.Attachment{
		{ID: 10, Filename: "screenshot.png"},
		{ID: 11, Filename: "screenshot.png"},
		{ID: 12, Filename: "Screenshot.PNG"},
		{ID: 13, Filename: "notes"},
		{ID: 14, Filename: "notes"},
		{ID: 15, Filename: ".env"},
		{ID: 16, Filename: ".env"},
		{ID: 17, Filename: "../screenshot.png"},
		{ID: 18, Filename: "screenshot (attachment-11).png"},
		{ID: 19, Filename: "archive.tar.gz"},
		{ID: 20, Filename: "archive.tar.gz"},
		{ID: 21, Filename: ".."},
	}
	// The newest attachment of each name keeps it; 18 keeps its own name
	// although it looks like the suffixed name of 11.
	want := []string{
		"screenshot (attachment-10).png",
		"screenshot (attachment-11) (attachment-11).png",
		"Screenshot (attachment-12).PNG",
		"notes (attachment-13)",
		"notes",
		".env (attachment-15)",
		".env",
		"screenshot.png",
		"screenshot (attachment-11).png",
		"archive.tar (attachment-19).gz",
		"archive.tar.gz",
		"attachment-21",
	}
	got := UniqueAttachmentFilenames(atts)
	if len(got) != len(want) {
		t.Fatalf("got %d names, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("name[%d] (id %d) = %q, want %q", i, atts[i].ID, got[i], want[i])
		}
	}
}
