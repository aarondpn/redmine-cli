package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/zalando/go-keyring"
)

func skipWithoutUnixPermissions(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("relies on Unix permissions and symlinks")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission checks")
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", path, got, want)
	}
}

func TestSaveProfilePlainFileIsPrivate(t *testing.T) {
	skipWithoutUnixPermissions(t)
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	// Older versions wrote 0o644; a re-save must tighten it.
	if err := os.WriteFile(cfgPath, []byte("active_profile: old\nprofiles:\n  old:\n    server: https://old.example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SaveProfile("work", &Config{Server: "https://work.example.com", APIKey: "k"}, cfgPath); err != nil {
		t.Fatal(err)
	}

	assertMode(t, cfgPath, 0o600)
	pc, err := LoadProfiles(cfgPath, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := pc.Profiles["old"]; !ok {
		t.Fatal("existing profile lost on save")
	}
	if pc.Profiles["work"].APIKey != "k" {
		t.Fatalf("saved profile = %+v", pc.Profiles["work"])
	}
	assertNoTempFiles(t, filepath.Dir(cfgPath))
}

func TestSaveProfileThroughSymlinkKeepsLink(t *testing.T) {
	skipWithoutUnixPermissions(t)
	dotfiles := t.TempDir()
	target := filepath.Join(dotfiles, "redmine-cli.yaml")
	if err := os.WriteFile(target, []byte("active_profile: old\nprofiles:\n  old:\n    server: https://old.example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	link := filepath.Join(home, ".redmine-cli.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := SaveProfile("work", &Config{Server: "https://work.example.com", APIKey: "k"}, link); err != nil {
		t.Fatal(err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("config symlink was replaced by a regular file")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "work.example.com") {
		t.Fatalf("symlink target not updated:\n%s", data)
	}
	assertMode(t, target, 0o600)
	assertNoTempFiles(t, home)
	assertNoTempFiles(t, dotfiles)
}

func TestSaveProfileThroughRelativeDanglingSymlinkCreatesTarget(t *testing.T) {
	skipWithoutUnixPermissions(t)
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "dotfiles"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, ".redmine-cli.yaml")
	if err := os.Symlink(filepath.Join("dotfiles", "redmine-cli.yaml"), link); err != nil {
		t.Fatal(err)
	}

	if err := SaveProfile("work", &Config{Server: "https://work.example.com", APIKey: "k"}, link); err != nil {
		t.Fatal(err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("dangling config symlink was replaced by a regular file")
	}
	assertMode(t, filepath.Join(dir, "dotfiles", "redmine-cli.yaml"), 0o600)
}

func TestSaveProfileCreatesPrivateConfigDir(t *testing.T) {
	skipWithoutUnixPermissions(t)
	base := t.TempDir()
	existing := filepath.Join(base, "existing")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(existing, "redmine", "config.yaml")

	if err := SaveProfile("work", &Config{Server: "https://work.example.com", APIKey: "k"}, cfgPath); err != nil {
		t.Fatal(err)
	}

	assertMode(t, filepath.Join(existing, "redmine"), 0o700)
	assertMode(t, existing, 0o755)
	assertMode(t, cfgPath, 0o600)
}

func TestSaveProfileFailedWriteKeepsOldFile(t *testing.T) {
	skipWithoutUnixPermissions(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := SaveProfile("work", &Config{Server: "https://work.example.com", APIKey: "old-key"}, cfgPath); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	// The new file is fully written, but replacing the old one fails.
	orig := renameFile
	renameFile = func(string, string) error { return errors.New("injected rename failure") }
	t.Cleanup(func() { renameFile = orig })

	if err := SaveProfile("work", &Config{Server: "https://work.example.com", APIKey: "new-key"}, cfgPath); err == nil {
		t.Fatal("SaveProfile error = nil, want write failure")
	}

	after, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("config changed by a failed write:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	assertNoTempFiles(t, dir)
}

func TestSaveProfileFallsBackToInPlaceWrite(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, dir string)
	}{
		{
			// e.g. a user-owned config inside a directory the user cannot write.
			name: "directory not writable",
			setup: func(t *testing.T, dir string) {
				if err := os.Chmod(dir, 0o500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
			},
		},
		{
			// A config bind-mounted into a container cannot be renamed over.
			name: "mount point",
			setup: func(t *testing.T, dir string) {
				orig := renameFile
				renameFile = func(oldpath, newpath string) error {
					return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: syscall.EBUSY}
				}
				t.Cleanup(func() { renameFile = orig })
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			skipWithoutUnixPermissions(t)
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(cfgPath, []byte("active_profile: work\nprofiles:\n  work:\n    server: https://work.example.com\n    api_key: old-key\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			tc.setup(t, dir)

			if err := SaveProfile("work", &Config{Server: "https://work.example.com", APIKey: "new-key"}, cfgPath); err != nil {
				t.Fatalf("SaveProfile error = %v, want the in-place fallback to succeed", err)
			}

			pc, err := LoadProfiles(cfgPath, debug.New(nil))
			if err != nil {
				t.Fatal(err)
			}
			if got := pc.Profiles["work"].APIKey; got != "new-key" {
				t.Fatalf("APIKey = %q, want %q", got, "new-key")
			}
			assertMode(t, cfgPath, 0o600)
			assertNoTempFiles(t, dir)
		})
	}
}

func TestSaveProfileRefusesReadOnlyFile(t *testing.T) {
	skipWithoutUnixPermissions(t)
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := SaveProfile("work", &Config{Server: "https://work.example.com", APIKey: "old-key"}, cfgPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cfgPath, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cfgPath, 0o600) })

	err := SaveProfile("work", &Config{Server: "https://work.example.com", APIKey: "new-key"}, cfgPath)
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("SaveProfile error = %v, want permission error for a read-only config", err)
	}
}

func TestSaveKeyringFailureRestoresSymlinkedConfig(t *testing.T) {
	skipWithoutUnixPermissions(t)
	keyring.MockInitWithError(errors.New("keychain locked"))
	dotfiles := t.TempDir()
	target := filepath.Join(dotfiles, "redmine-cli.yaml")
	link := filepath.Join(t.TempDir(), ".redmine-cli.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	plain := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "plain-key"}
	if err := SaveProfile("work", plain, link); err != nil {
		t.Fatal(err)
	}

	convert := &Config{Server: "https://work.example.com", AuthMethod: "apikey", APIKey: "plain-key", CredentialStore: CredentialStoreKeyring}
	if err := SaveProfile("work", convert, link); err == nil {
		t.Fatal("SaveProfile error = nil, want keyring failure")
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("rollback replaced the config symlink with a regular file")
	}
	loaded, err := Load(link, "", debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.APIKey != "plain-key" || loaded.CredentialStore == CredentialStoreKeyring {
		t.Fatalf("rollback did not restore the plaintext profile: %+v", loaded)
	}
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}
