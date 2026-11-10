// Package extension manages installed redmine-cli extensions: where they
// live, how they are discovered (local only, no network), and installing,
// upgrading and removing them from GitHub releases, git clones or local
// directories. The manifest model is in the spec subpackage and process
// execution in the runner subpackage.
package extension

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
	"github.com/aarondpn/redmine-cli/v2/internal/ghrelease"
)

const (
	// DirEnv overrides the extensions directory (used by tests and for
	// portable setups).
	DirEnv = "REDMINE_EXTENSIONS_DIR"
	// Prefix starts every extension repository, directory and executable
	// name: repository owner/redmine-cli-<name> provides `redmine <name>`.
	Prefix = "redmine-cli-"
	// Topic is the GitHub topic extension repositories are tagged with.
	Topic = "redmine-cli-extension"
	// metaDirName holds the install metadata of every extension and the
	// update-notice cache. It lives outside the extension directories: the
	// content of a git clone comes from its repository, so a file in it
	// must never be able to supply the provenance (owner, repo, tag,
	// pinned) of the extension. It also keeps `git pull` clean.
	metaDirName = ".meta"
	// maxMetadataFile bounds metadata and link files, which are a few
	// hundred bytes.
	maxMetadataFile = 64 << 10
)

// Kind is how an extension was installed.
type Kind string

const (
	// KindBinary is a release asset downloaded from GitHub.
	KindBinary Kind = "binary"
	// KindGit is a git clone of a script extension.
	KindGit Kind = "git"
	// KindLocal is a link to a local directory, for extension authors.
	KindLocal Kind = "local"
)

// InstallInfo is the install metadata of a binary or git extension.
type InstallInfo struct {
	Owner string `yaml:"owner"`
	Repo  string `yaml:"repo"`
	// Tag is the release tag of a binary extension, or the pinned tag of a
	// git extension.
	Tag string `yaml:"tag,omitempty"`
	// SHA256 is the digest of the installed binary.
	SHA256 string `yaml:"sha256,omitempty"`
	// ChecksumVerified is set when the binary, and the manifest if the
	// release has one, were verified against the release's checksums
	// file. An upgrade refuses to drop it without --force.
	ChecksumVerified bool      `yaml:"checksum_verified,omitempty"`
	Pinned           bool      `yaml:"pinned"`
	InstalledAt      time.Time `yaml:"installed_at"`
	Kind             Kind      `yaml:"kind"`
}

// Extension is an installed extension as found on disk.
type Extension struct {
	Name string
	Kind Kind
	// Dir is the directory holding the executable and manifest; for a
	// local extension the linked directory.
	Dir string
	// Path is the executable.
	Path string
	// Install is the install metadata, read from <root>/.meta; nil for
	// local extensions and when the metadata is missing.
	Install *InstallInfo
	// Revision is the checked-out commit of a git extension.
	Revision string
	// Manifest is the parsed redmine-extension.yml, nil when the extension
	// has none (a passthrough extension) or it is invalid.
	Manifest *spec.Manifest
	// Warnings are the unknown fields of the manifest, which loading
	// skips (a manifest written for a newer redmine-cli still runs);
	// install, upgrade and `extension check` reject them.
	Warnings []*spec.Error
	// Err is set when the extension cannot be used: its executable is
	// missing, its link is broken or its manifest is invalid.
	Err error
}

// Repo returns "owner/repo", or "" when unknown.
func (e *Extension) Repo() string {
	if e.Install == nil || e.Install.Owner == "" {
		return ""
	}
	return e.Install.Owner + "/" + e.Install.Repo
}

// Pinned reports whether upgrades skip the extension.
func (e *Extension) Pinned() bool {
	return e.Install != nil && e.Install.Pinned
}

// Version is the installed version for display: the release tag of a binary
// extension, the pinned tag or short commit of a git one, "" otherwise.
func (e *Extension) Version() string {
	if e.Install != nil && e.Install.Tag != "" {
		return e.Install.Tag
	}
	if e.Revision != "" {
		return shortRevision(e.Revision)
	}
	return ""
}

func shortRevision(rev string) string {
	if len(rev) > 7 {
		return rev[:7]
	}
	return rev
}

// ErrNotInstalled is matched by errors about an extension that is not
// installed.
var ErrNotInstalled = errors.New("extension is not installed")

// Manager manages the extensions below Root. The zero value of every field
// except Root has a sensible default, so tests only set what they fake.
type Manager struct {
	// Root is the extensions directory (see DefaultRoot).
	Root string
	// CLIVersion is the running redmine-cli version, for min_cli_version.
	CLIVersion string
	// GitHub serves release metadata, downloads and search. nil uses the
	// public API.
	GitHub *ghrelease.Client
	// Git runs git for script extensions. nil uses the git binary.
	Git Git
	// CoreNames are the names and aliases of the core commands; an
	// extension may not take one.
	CoreNames []string
	// GOOS and GOARCH select the release asset; empty means the running
	// platform.
	GOOS, GOARCH string
	// Now is the clock; nil means time.Now.
	Now func() time.Time
	// CloneURL builds the git URL of a repository; nil means github.com.
	CloneURL func(owner, repo string) string

	// renameFunc replaces os.Rename in tests that make a swap fail.
	renameFunc func(from, to string) error
}

// NewManager returns a Manager for the default extensions directory.
func NewManager(cliVersion string) (*Manager, error) {
	root, err := DefaultRoot()
	if err != nil {
		return nil, err
	}
	return &Manager{Root: root, CLIVersion: cliVersion}, nil
}

// DefaultRoot returns the extensions directory: $REDMINE_EXTENSIONS_DIR,
// else %LOCALAPPDATA%\redmine-cli\extensions on Windows, else
// $XDG_DATA_HOME/redmine-cli/extensions, else
// ~/.local/share/redmine-cli/extensions.
func DefaultRoot() (string, error) {
	return defaultRoot(runtime.GOOS, os.Getenv, os.UserHomeDir)
}

func defaultRoot(goos string, getenv func(string) string, home func() (string, error)) (string, error) {
	if dir := getenv(DirEnv); dir != "" {
		return filepath.Abs(dir)
	}
	if goos == "windows" {
		if dir := getenv("LOCALAPPDATA"); dir != "" {
			return filepath.Join(dir, "redmine-cli", "extensions"), nil
		}
	}
	// The XDG spec says relative values are invalid and must be ignored.
	if dir := getenv("XDG_DATA_HOME"); dir != "" && filepath.IsAbs(dir) {
		return filepath.Join(dir, "redmine-cli", "extensions"), nil
	}
	h, err := home()
	if err != nil || h == "" {
		return "", fmt.Errorf("cannot determine the extensions directory (no home directory); set %s", DirEnv)
	}
	return filepath.Join(h, ".local", "share", "redmine-cli", "extensions"), nil
}

func (m *Manager) github() *ghrelease.Client {
	if m.GitHub != nil {
		return m.GitHub
	}
	return &ghrelease.Client{}
}

func (m *Manager) git() Git {
	if m.Git != nil {
		return m.Git
	}
	return ExecGit{}
}

func (m *Manager) goos() string {
	if m.GOOS != "" {
		return m.GOOS
	}
	return runtime.GOOS
}

func (m *Manager) goarch() string {
	if m.GOARCH != "" {
		return m.GOARCH
	}
	return runtime.GOARCH
}

func (m *Manager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func (m *Manager) cloneURL(owner, repo string) string {
	if m.CloneURL != nil {
		return m.CloneURL(owner, repo)
	}
	return "https://github.com/" + owner + "/" + repo + ".git"
}

// entryPath is <root>/redmine-cli-<name>: the extension directory, or the
// link of a local extension.
func (m *Manager) entryPath(name string) string {
	return filepath.Join(m.Root, Prefix+name)
}

func (m *Manager) metaDir() string {
	return filepath.Join(m.Root, metaDirName)
}

// installMetaPath is where the InstallInfo of a binary or git extension
// lives, outside its directory.
func (m *Manager) installMetaPath(name string) string {
	return filepath.Join(m.metaDir(), name+".yml")
}

// ExecutableName is the executable of extension name on goos.
func ExecutableName(name, goos string) string {
	if goos == "windows" {
		return Prefix + name + ".exe"
	}
	return Prefix + name
}

// BinaryAssetName is the release asset of extension name for a platform.
func BinaryAssetName(name, goos, goarch string) string {
	asset := Prefix + name + "-" + goos + "-" + goarch
	if goos == "windows" {
		asset += ".exe"
	}
	return asset
}

// ValidateName checks the extension name rule, including clashes with core
// commands: a core command always wins, so such an extension could only be
// run through `redmine extension exec`.
func (m *Manager) ValidateName(name string) error {
	if err := spec.ValidateExtensionName(name); err != nil {
		return err
	}
	if slices.Contains(m.CoreNames, name) {
		return fmt.Errorf("extension name %q is already taken by the core command %q", name, name)
	}
	return nil
}

// List returns the installed extensions sorted by name. It only reads the
// local disk. Extensions that cannot be used are included with Err set, so
// they can be shown and reported instead of silently disappearing.
func (m *Manager) List() ([]*Extension, error) {
	entries, err := os.ReadDir(m.Root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Extension
	for _, e := range entries {
		name, ok := strings.CutPrefix(e.Name(), Prefix)
		if !ok || spec.ValidateExtensionName(name) != nil {
			continue
		}
		ext, err := m.load(name)
		if err != nil {
			continue
		}
		out = append(out, ext)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Get returns the installed extension name. The error matches
// ErrNotInstalled when there is none.
func (m *Manager) Get(name string) (*Extension, error) {
	if err := spec.ValidateExtensionName(name); err != nil {
		return nil, err
	}
	return m.load(name)
}

// load reads the extension at its entry path.
func (m *Manager) load(name string) (*Extension, error) {
	entry := m.entryPath(name)
	info, err := os.Lstat(entry)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("extension %q: %w", name, ErrNotInstalled)
	}
	if err != nil {
		return nil, err
	}
	ext := &Extension{Name: name}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		ext.Kind = KindLocal
		target, err := filepath.EvalSymlinks(entry)
		if err != nil {
			ext.Dir = entry
			ext.Err = fmt.Errorf("the linked directory of local extension %q is gone: %w", name, err)
			return ext, nil
		}
		ext.Dir = target
	case info.Mode().IsRegular():
		// Windows link file: the path of the linked directory.
		ext.Kind = KindLocal
		data, err := readLimited(entry, maxMetadataFile)
		if err != nil {
			return nil, err
		}
		ext.Dir = strings.TrimSpace(string(data))
		if st, err := os.Stat(ext.Dir); err != nil || !st.IsDir() {
			ext.Err = fmt.Errorf("the linked directory %s of local extension %q is gone", ext.Dir, name)
			return ext, nil
		}
	case info.IsDir():
		ext.Dir = entry
		// Only the metadata redmine wrote itself counts; nothing inside
		// the directory (a clone's install.yml, say) is consulted.
		meta, err := readInstallInfo(m.installMetaPath(name))
		if err == nil && (meta.Kind == KindBinary || meta.Kind == KindGit) {
			ext.Kind = meta.Kind
			ext.Install = meta
		} else if _, statErr := os.Stat(filepath.Join(entry, ".git")); statErr == nil {
			ext.Kind = KindGit
		} else {
			ext.Kind = KindBinary
		}
		if ext.Kind == KindGit {
			if rev, err := readGitHead(entry); err == nil {
				ext.Revision = rev
			}
		}
	default:
		return nil, fmt.Errorf("extension %q: unexpected file type at %s", name, entry)
	}

	ext.Path = findExecutable(ext.Dir, name, m.goos())
	if _, err := os.Stat(ext.Path); err != nil {
		ext.Err = fmt.Errorf("extension %q has no executable %s", name, ext.Path)
		return ext, nil
	}
	ext.Manifest, ext.Warnings, ext.Err = m.loadManifestLenient(ext.Dir, name)
	return ext, nil
}

// loadManifest parses the manifest in dir strictly, as install and upgrade
// do; (nil, nil) when there is none.
func (m *Manager) loadManifest(dir, name string) (*spec.Manifest, error) {
	path := filepath.Join(dir, spec.FileName)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return spec.ParseFile(path, spec.Options{ExtensionName: name, CLIVersion: m.CLIVersion})
}

// loadManifestLenient parses the manifest in dir for running the
// extension: unknown fields are skipped and returned as warnings.
func (m *Manager) loadManifestLenient(dir, name string) (*spec.Manifest, []*spec.Error, error) {
	path := filepath.Join(dir, spec.FileName)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	return spec.ParseFileLenient(path, spec.Options{ExtensionName: name, CLIVersion: m.CLIVersion})
}

// ExecutablePath returns the executable of extension name in dir on the
// running platform, whether or not it exists.
func ExecutablePath(dir, name string) string {
	return findExecutable(dir, name, runtime.GOOS)
}

// findExecutable returns the executable in dir. On Windows a binary has the
// .exe suffix while a script extension's executable has none.
func findExecutable(dir, name, goos string) string {
	exe := filepath.Join(dir, ExecutableName(name, goos))
	if goos == "windows" {
		if _, err := os.Stat(exe); err != nil {
			if script := filepath.Join(dir, Prefix+name); fileExists(script) {
				return script
			}
		}
	}
	return exe
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func readLimited(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s is larger than %d bytes", path, limit)
	}
	return data, nil
}

func readInstallInfo(path string) (*InstallInfo, error) {
	data, err := readLimited(path, maxMetadataFile)
	if err != nil {
		return nil, err
	}
	var info InstallInfo
	if err := yaml.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &info, nil
}

func writeInstallInfo(path string, info *InstallInfo) error {
	data, err := yaml.Marshal(info)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

// writeFileAtomic writes data to a temp file next to path and renames it
// into place, so readers never see a partial file.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_, err = tmp.Write(data)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Chmod(tmpPath, perm)
	}
	if err == nil {
		err = os.Rename(tmpPath, path)
	}
	if err != nil {
		_ = os.Remove(tmpPath)
	}
	return err
}

// readGitHead resolves HEAD of the clone in dir by reading .git directly, so
// discovery and the update notice never spawn git.
func readGitHead(dir string) (string, error) {
	gitDir := filepath.Join(dir, ".git")
	head, err := readLimited(filepath.Join(gitDir, "HEAD"), maxMetadataFile)
	if err != nil {
		return "", err
	}
	ref, ok := strings.CutPrefix(strings.TrimSpace(string(head)), "ref: ")
	if !ok {
		return strings.TrimSpace(string(head)), nil
	}
	if !strings.HasPrefix(ref, "refs/") || strings.Contains(ref, "..") {
		return "", fmt.Errorf("unexpected HEAD ref %q", ref)
	}
	if data, err := readLimited(filepath.Join(gitDir, filepath.FromSlash(ref)), maxMetadataFile); err == nil {
		return strings.TrimSpace(string(data)), nil
	}
	packed, err := readLimited(filepath.Join(gitDir, "packed-refs"), 16<<20)
	if err != nil {
		return "", err
	}
	for _, line := range bytes.Split(packed, []byte("\n")) {
		fields := strings.Fields(string(line))
		if len(fields) == 2 && fields[1] == ref {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("ref %s not found", ref)
}
