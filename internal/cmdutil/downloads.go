package cmdutil

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// linkFile is os.Link, swappable in tests to exercise the fallback used where
// hard links are unsupported.
var linkFile = os.Link

// SaveOptions controls how an attachment download is written to disk.
type SaveOptions struct {
	// Clobber allows replacing a file that already exists at the destination.
	// Without it, an existing entry (including a dangling symlink) is an error.
	Clobber bool
	// AllowHidden allows saving an attachment under its own filename when
	// that name starts with "." (--allow-hidden). Without it such a name is
	// an error; see CheckHiddenFilename.
	AllowHidden bool
}

// DestinationExistsError reports that a download target already exists and
// --clobber was not given. It unwraps to fs.ErrExist.
type DestinationExistsError struct {
	Path string
}

func (e *DestinationExistsError) Error() string {
	return fmt.Sprintf("%s already exists; use --clobber to overwrite it", e.Path)
}

func (e *DestinationExistsError) Unwrap() error { return fs.ErrExist }

// HiddenFilenameError reports that an attachment would be saved under its
// server-supplied filename, that name starts with "." and --allow-hidden
// was not given. A server could otherwise plant a dotfile such as .bashrc,
// .envrc or .npmrc in the destination directory.
type HiddenFilenameError struct {
	Name string
	Path string
}

func (e *HiddenFilenameError) Error() string {
	return fmt.Sprintf("refusing to save the hidden filename %q as %s; pass --allow-hidden to save it anyway", e.Name, e.Path)
}

// CheckHiddenFilename returns a *HiddenFilenameError when name, the
// sanitized filename an attachment would be saved under in dir, starts with
// "." and allowHidden is false.
func CheckHiddenFilename(dir, name string, allowHidden bool) error {
	if allowHidden || !strings.HasPrefix(name, ".") {
		return nil
	}
	return &HiddenFilenameError{Name: name, Path: filepath.Join(dir, name)}
}

// AttachmentFilename returns the server-supplied filename of att reduced to
// its base component, so a malicious or malformed name (e.g. one containing
// "../") cannot escape the destination directory.
func AttachmentFilename(att *models.Attachment) string {
	name := filepath.Base(att.Filename)
	// filepath.Base can only yield ".", "..", the separator, or "" for a
	// degenerate name; all of them would write outside dir (or to dir itself),
	// so fall back to a synthetic, contained name.
	if name == "." || name == ".." || name == string(os.PathSeparator) || name == "" {
		name = fmt.Sprintf("attachment-%d", att.ID)
	}
	return name
}

// UniqueAttachmentFilenames returns one sanitized filename per attachment,
// deduplicated so a bulk download never writes two attachments to the same
// path. Of the attachments sharing a name, the newest (highest ID) keeps it,
// since a corrected file is usually re-uploaded under the same name; older
// ones become "<stem> (attachment-<id>)<ext>". An attachment whose own name
// is unique keeps it even if it looks like such a suffixed name. Names are
// compared case-insensitively, whatever the destination filesystem, because
// the common desktop filesystems (APFS, NTFS) are case-insensitive.
func UniqueAttachmentFilenames(atts []models.Attachment) []string {
	newestFirst := make([]int, len(atts))
	for i := range newestFirst {
		newestFirst[i] = i
	}
	slices.SortStableFunc(newestFirst, func(a, b int) int { return cmp.Compare(atts[b].ID, atts[a].ID) })

	names := make([]string, len(atts))
	used := make(map[string]bool, len(atts))
	var older []int
	for _, i := range newestFirst {
		name := AttachmentFilename(&atts[i])
		if used[strings.ToLower(name)] {
			older = append(older, i)
			continue
		}
		used[strings.ToLower(name)] = true
		names[i] = name
	}
	for _, i := range older {
		name := withAttachmentSuffix(AttachmentFilename(&atts[i]), atts[i].ID)
		for used[strings.ToLower(name)] {
			name = withAttachmentSuffix(name, atts[i].ID)
		}
		used[strings.ToLower(name)] = true
		names[i] = name
	}
	return names
}

// withAttachmentSuffix inserts " (attachment-<id>)" before the extension of
// name. A leading dot is part of the stem, so ".env" becomes
// ".env (attachment-<id>)" rather than " (attachment-<id>).env".
func withAttachmentSuffix(name string, id int) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	if stem == "" {
		stem, ext = name, ""
	}
	return fmt.Sprintf("%s (attachment-%d)%s", stem, id, ext)
}

// SaveAttachmentToDir streams an attachment into dir using its sanitized real
// filename (see AttachmentFilename) and returns the path written and the
// number of bytes streamed. The directory is created if it does not exist.
func SaveAttachmentToDir(ctx context.Context, client *api.Client, att *models.Attachment, dir string, opts SaveOptions) (string, int64, error) {
	return SaveAttachmentToDirAs(ctx, client, att, dir, AttachmentFilename(att), opts)
}

// SaveAttachmentToDirAs is SaveAttachmentToDir with an explicit filename,
// which must already be a sanitized base name (see AttachmentFilename and
// UniqueAttachmentFilenames). A name starting with "." is refused unless
// opts.AllowHidden is set (see CheckHiddenFilename).
func SaveAttachmentToDirAs(ctx context.Context, client *api.Client, att *models.Attachment, dir, name string, opts SaveOptions) (string, int64, error) {
	if err := CheckHiddenFilename(dir, name, opts.AllowHidden); err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, fmt.Errorf("creating directory %s: %w", dir, err)
	}
	path := filepath.Join(dir, name)
	n, err := saveAttachmentFile(ctx, client, att, path, opts)
	return path, n, err
}

// CheckDownloadTarget reports whether path may be used as a download
// destination. A directory or other non-regular file (FIFO, device, socket)
// is always rejected; an existing file or symlink (dangling or not) is
// rejected unless clobber is set. A missing path is fine.
func CheckDownloadTarget(path string, clobber bool) error {
	_, err := checkDownloadTarget(path, clobber)
	return err
}

// checkDownloadTarget is CheckDownloadTarget, additionally returning the
// Lstat info of an existing target (nil when path does not exist).
func checkDownloadTarget(path string, clobber bool) (fs.FileInfo, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("checking %s: %w", path, err)
	}
	mode := info.Mode()
	switch {
	case mode.IsDir():
		return nil, fmt.Errorf("%s is a directory; refusing to replace it", path)
	case !mode.IsRegular() && mode&fs.ModeSymlink == 0:
		return nil, fmt.Errorf("%s is not a regular file; refusing to replace it", path)
	case !clobber:
		return nil, &DestinationExistsError{Path: path}
	}
	return info, nil
}

// createDownloadTemp creates a new, empty temporary file in dir. Unlike
// os.CreateTemp (always 0o600) it asks for 0o666 and lets the umask apply,
// so the saved file gets the same permissions os.Create always gave
// downloads (0o644 under the usual umask, 0o600 under umask 077).
func createDownloadTemp(dir string) (*os.File, error) {
	for range 10000 {
		name := filepath.Join(dir, ".redmine-download-"+strconv.FormatUint(rand.Uint64(), 36)+".tmp")
		f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return f, err
	}
	return nil, fmt.Errorf("creating temporary file in %s: too many name collisions", dir)
}

// SaveAttachmentToFile streams an attachment to the exact path given and
// returns the number of bytes written. The parent directory must already
// exist. A new file gets the permissions os.Create would give it (0o644
// under the usual umask); with opts.Clobber a replaced regular file keeps its
// previous permissions.
//
// The bytes are streamed into a temporary file in the destination directory,
// fsynced, and only then moved into place, so a failed download never
// touches an existing file and never leaves a partial one behind. Cancelling
// ctx (the first Ctrl-C) fails the download and removes the temporary file;
// only a process killed outright (SIGKILL, a second Ctrl-C) may leave a
// hidden .redmine-download-*.tmp. The move replaces the directory entry
// itself: a symlink at path is replaced, never written through.
//
// Without opts.Clobber an existing path is an error. The final move then uses
// a hard link, which fails atomically if path appeared during the download;
// only where hard links are unsupported (some filesystems, cross-device
// mounts) does it fall back to a re-check followed by a rename, which leaves
// a small window between the check and the rename.
//
// The exception is a path that is, or is a symlink to, a character device
// or a FIFO, such as /dev/null, /dev/stdout or a named pipe: there is no
// file to replace, so the bytes are written into it directly, with or
// without opts.Clobber. Other non-regular files, such as directories, are
// still an error.
func SaveAttachmentToFile(ctx context.Context, client *api.Client, att *models.Attachment, path string, opts SaveOptions) (int64, error) {
	stream, err := openStreamTarget(path)
	if err != nil {
		return 0, err
	}
	if stream != nil {
		n, err := client.Attachments.Download(ctx, att, stream)
		if closeErr := stream.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("closing %s: %w", path, closeErr)
		}
		return n, err
	}
	return saveAttachmentFile(ctx, client, att, path, opts)
}

// isStreamMode reports whether mode is that of a character device or a FIFO.
func isStreamMode(mode fs.FileMode) bool {
	return mode&(fs.ModeCharDevice|fs.ModeNamedPipe) != 0
}

// openStreamTarget opens path for writing when it is, or links to, a
// character device or a FIFO, and returns nil otherwise. Opening a FIFO
// blocks until a reader opens it, as it always has.
func openStreamTarget(path string) (*os.File, error) {
	info, err := os.Stat(path)
	if err != nil || !isStreamMode(info.Mode()) {
		return nil, nil
	}
	// No O_CREATE and no O_TRUNC: this never creates or truncates a file.
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	// path may have been replaced since the Stat; anything but a device or
	// FIFO goes through the regular no-clobber logic instead.
	if info, err := f.Stat(); err != nil || !isStreamMode(info.Mode()) {
		_ = f.Close()
		return nil, nil
	}
	return f, nil
}

// saveAttachmentFile is SaveAttachmentToFile without the device and FIFO
// exception, which only applies to a path the user chose.
func saveAttachmentFile(ctx context.Context, client *api.Client, att *models.Attachment, path string, opts SaveOptions) (int64, error) {
	existing, err := checkDownloadTarget(path, opts.Clobber)
	if err != nil {
		return 0, err
	}

	tmp, err := createDownloadTemp(filepath.Dir(path))
	if err != nil {
		return 0, fmt.Errorf("creating %s: %w", path, err)
	}
	tmpPath := tmp.Name()
	placed := false
	defer func() {
		// Only the temporary file is ever cleaned up; a file that existed
		// at path before the download is left untouched on failure.
		if !placed {
			_ = os.Remove(tmpPath)
		}
	}()

	// Replacing a regular file used to truncate it in place, which kept its
	// permissions; keep doing that so --clobber never widens access to a
	// file the user restricted. A replaced symlink is not followed. Windows
	// only maps the mode to the read-only attribute, which would merely stop
	// the temporary file from being renamed or removed, so it is skipped there.
	if existing != nil && existing.Mode().IsRegular() && runtime.GOOS != "windows" {
		err = tmp.Chmod(existing.Mode().Perm())
	}
	var n int64
	if err == nil {
		n, err = client.Attachments.Download(ctx, att, tmp)
	}
	if err == nil {
		if syncErr := tmp.Sync(); syncErr != nil {
			err = fmt.Errorf("syncing %s: %w", path, syncErr)
		}
	}
	if closeErr := tmp.Close(); closeErr != nil && err == nil {
		err = fmt.Errorf("closing %s: %w", path, closeErr)
	}
	if err != nil {
		return n, err
	}

	if opts.Clobber {
		if err := os.Rename(tmpPath, path); err != nil {
			return n, fmt.Errorf("replacing %s: %w", path, err)
		}
		placed = true
		return n, nil
	}

	// No clobber: link(2) refuses to replace an existing entry, closing the
	// race between CheckDownloadTarget and here. The temporary name is then
	// removed by the deferred cleanup.
	linkErr := linkFile(tmpPath, path)
	if linkErr == nil {
		return n, nil
	}
	if errors.Is(linkErr, fs.ErrExist) {
		return n, &DestinationExistsError{Path: path}
	}
	// Hard links are unsupported here (e.g. EXDEV, EPERM, FAT/exFAT): fall
	// back to check-then-rename.
	if err := CheckDownloadTarget(path, false); err != nil {
		return n, err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return n, fmt.Errorf("creating %s: %w", path, err)
	}
	placed = true
	return n, nil
}
