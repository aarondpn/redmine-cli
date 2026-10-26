package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// maxSymlinkHops bounds symlink resolution for a config path whose link
// target does not exist yet, mirroring the usual ELOOP limit.
const maxSymlinkHops = 40

// resolveConfigPath returns the file a write to path should replace. A
// symlinked config (e.g. into a dotfiles repository) resolves to its target so
// the link keeps pointing at the real file; a missing path is returned as-is.
func resolveConfigPath(path string) (string, error) {
	for hops := 0; ; hops++ {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		// Either nothing exists at path yet, or it is a link whose target
		// has not been created: follow the link by hand.
		info, lerr := os.Lstat(path)
		if lerr != nil || info.Mode()&os.ModeSymlink == 0 {
			return path, nil
		}
		if hops >= maxSymlinkHops {
			return "", fmt.Errorf("resolving config path %s: too many levels of symbolic links", path)
		}
		target, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = target
	}
}

// renameFile is os.Rename, replaceable in tests.
var renameFile = os.Rename

// writeFileAtomic replaces the already-resolved path with data, mode 0o600.
// The data goes to a temp file in the same directory that is synced and then
// renamed over path, so readers and crashes see either the old or the new
// file, never a truncated one. Missing parent directories are created 0o700;
// existing ones are left alone. An existing file that cannot be replaced by a
// rename (its directory is not writable, or it is a mount point such as a
// config bind-mounted into a container) is overwritten in place, as before.
func writeFileAtomic(path string, data []byte) error {
	info, statErr := os.Stat(path)
	exists := statErr == nil
	// A rename would replace a file the user made read-only; keep refusing
	// as the in-place write did.
	if exists && info.Mode().Perm()&0o200 == 0 {
		return &os.PathError{Op: "open", Path: path, Err: os.ErrPermission}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	// CreateTemp opens the file 0o600, so the secret is never world-readable.
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		if exists && errors.Is(err, os.ErrPermission) {
			return writeFileInPlace(path, data)
		}
		return err
	}
	tmpName := tmp.Name()
	// After a successful rename both are no-ops: the file is already closed
	// and nothing is left at tmpName.
	defer func() { _ = os.Remove(tmpName) }()
	defer func() { _ = tmp.Close() }()

	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := renameFile(tmpName, path); err != nil {
		if exists && (errors.Is(err, syscall.EBUSY) || errors.Is(err, syscall.EXDEV)) {
			return writeFileInPlace(path, data)
		}
		return err
	}
	syncDir(dir)
	return nil
}

// writeFileInPlace overwrites path with data and tightens its mode to 0o600.
// Not atomic; only used when the file cannot be replaced by a rename.
func writeFileInPlace(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	// WriteFile keeps an existing file's mode (older versions wrote 0o644).
	_ = os.Chmod(path, 0o600)
	return nil
}

// syncDir makes a completed rename durable. Best-effort: some platforms
// (Windows) cannot open or sync a directory.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}
