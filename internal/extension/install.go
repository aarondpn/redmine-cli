package extension

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
	"github.com/aarondpn/redmine-cli/v2/internal/ghrelease"
)

// MaxBinarySize bounds a downloaded extension binary.
const MaxBinarySize = 200 << 20

// Source is what `extension install` was given: a GitHub repository or a
// local directory.
type Source struct {
	Owner string
	Repo  string
	// Name is the extension name, the repository (or directory) name
	// without the redmine-cli- prefix.
	Name string
	// LocalDir is set for a local install.
	LocalDir string
}

// String renders the source as owner/repo or the local directory.
func (s Source) String() string {
	if s.LocalDir != "" {
		return s.LocalDir
	}
	return s.Owner + "/" + s.Repo
}

// ParseSource parses an install argument: "owner/redmine-cli-<name>", a
// https://github.com/ URL of such a repository, or "." (or another path
// starting with "." or "/", or an absolute path) for a local directory named
// redmine-cli-<name>.
func ParseSource(arg string) (Source, error) {
	if arg == "." || strings.HasPrefix(arg, "./") || strings.HasPrefix(arg, "../") || strings.HasPrefix(arg, "/") || filepath.IsAbs(arg) ||
		(runtime.GOOS == "windows" && (strings.HasPrefix(arg, `.\`) || strings.HasPrefix(arg, `..\`))) {
		dir, err := filepath.Abs(arg)
		if err != nil {
			return Source{}, err
		}
		name, ok := strings.CutPrefix(filepath.Base(dir), Prefix)
		if !ok {
			return Source{}, fmt.Errorf("local extension directory %s must be named %s<name>", dir, Prefix)
		}
		return Source{Name: name, LocalDir: dir}, nil
	}

	ownerRepo := arg
	if strings.Contains(arg, "://") {
		u, err := url.Parse(arg)
		if err != nil {
			return Source{}, fmt.Errorf("invalid repository URL %q: %w", arg, err)
		}
		if u.Scheme != "https" || !strings.EqualFold(u.Host, "github.com") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return Source{}, fmt.Errorf("invalid repository URL %q: only https://github.com/<owner>/<repo> is supported", arg)
		}
		ownerRepo = strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
	}
	owner, repo, ok := strings.Cut(ownerRepo, "/")
	if !ok || strings.Contains(repo, "/") {
		return Source{}, fmt.Errorf("invalid extension %q: use <owner>/%s<name>, a GitHub URL or . for the current directory", arg, Prefix)
	}
	if err := ghrelease.ValidateRepo(owner, repo); err != nil {
		return Source{}, err
	}
	name, ok := strings.CutPrefix(repo, Prefix)
	if !ok {
		return Source{}, fmt.Errorf("repository %s/%s is not an extension: extension repositories are named %s<name>", owner, repo, Prefix)
	}
	return Source{Owner: owner, Repo: repo, Name: name}, nil
}

// AlreadyInstalledError reports an install over an existing extension
// without Force.
type AlreadyInstalledError struct{ Name string }

func (e *AlreadyInstalledError) Error() string {
	return fmt.Sprintf("extension %q is already installed; use --force to reinstall it or 'redmine extension upgrade %s' to update it", e.Name, e.Name)
}

// InstallOptions controls Install.
type InstallOptions struct {
	// Pin installs this release tag (or git tag) and excludes the
	// extension from upgrades.
	Pin string
	// Force replaces an installed extension of the same name.
	Force bool
}

// Install installs src. A GitHub extension is installed as a binary when the
// release (latest, or Pin) has an asset for this platform, and cloned with
// git otherwise. The manifest, if any, must be valid: an install never
// leaves a broken extension behind.
func (m *Manager) Install(ctx context.Context, src Source, opts InstallOptions) (*Extension, error) {
	if err := m.ValidateName(src.Name); err != nil {
		return nil, err
	}
	if src.LocalDir != "" {
		if opts.Pin != "" {
			return nil, errors.New("--pin cannot be used with a local extension")
		}
		return m.installLocal(src, opts.Force)
	}
	if !opts.Force {
		if _, err := os.Lstat(m.entryPath(src.Name)); err == nil {
			return nil, &AlreadyInstalledError{Name: src.Name}
		}
	}
	if err := os.MkdirAll(m.Root, 0o755); err != nil {
		return nil, err
	}

	release, err := m.github().FetchRelease(ctx, src.Owner, src.Repo, opts.Pin)
	switch {
	case err == nil:
		assetName := BinaryAssetName(src.Name, m.goos(), m.goarch())
		if asset, ok := release.FindAsset(assetName); ok {
			return m.installBinary(ctx, src, release, asset, opts.Pin != "")
		}
		if hasBinaryAssets(release, src.Name) {
			return nil, fmt.Errorf("release %s of %s has no binary for %s/%s (expected asset %s)", release.TagName, src, m.goos(), m.goarch(), assetName)
		}
	case errors.Is(err, ghrelease.ErrNotFound):
		// No releases (or no such tag): a script extension.
	default:
		return nil, fmt.Errorf("fetching release of %s: %w", src, err)
	}
	return m.installGit(ctx, src, opts.Pin)
}

// hasBinaryAssets reports whether the release ships binaries for some
// platform, i.e. the extension is precompiled and a clone would not run.
func hasBinaryAssets(r *ghrelease.Release, name string) bool {
	for _, a := range r.Assets {
		if strings.HasPrefix(a.Name, Prefix+name+"-") {
			return true
		}
	}
	return false
}

// checksumsAsset finds the checksums file of a release, if it has one.
func checksumsAsset(r *ghrelease.Release) (ghrelease.Asset, bool) {
	for _, a := range r.Assets {
		lower := strings.ToLower(a.Name)
		if lower == "sha256sums" || lower == "sha256sums.txt" || strings.HasSuffix(lower, "checksums.txt") {
			return a, true
		}
	}
	return ghrelease.Asset{}, false
}

func (m *Manager) installBinary(ctx context.Context, src Source, release *ghrelease.Release, asset ghrelease.Asset, pinned bool) (*Extension, error) {
	gh := m.github()
	data, err := gh.Download(ctx, asset.BrowserDownloadURL, MaxBinarySize)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", asset.Name, err)
	}
	var checksums []byte
	if cs, ok := checksumsAsset(release); ok {
		if checksums, err = gh.Download(ctx, cs.BrowserDownloadURL, ghrelease.MaxChecksumsBytes); err != nil {
			return nil, fmt.Errorf("downloading %s: %w", cs.Name, err)
		}
		if err := ghrelease.VerifySHA256(data, checksums, asset.Name); err != nil {
			return nil, fmt.Errorf("checksum verification of %s failed: %w", asset.Name, err)
		}
	}

	var manifest []byte
	if ma, ok := release.FindAsset(spec.FileName); ok {
		if manifest, err = gh.Download(ctx, ma.BrowserDownloadURL, spec.MaxManifestSize+1); err != nil {
			return nil, fmt.Errorf("downloading %s: %w", spec.FileName, err)
		}
		if _, listed := ghrelease.LookupChecksum(checksums, spec.FileName); listed {
			if err := ghrelease.VerifySHA256(manifest, checksums, spec.FileName); err != nil {
				return nil, fmt.Errorf("checksum verification of %s failed: %w", spec.FileName, err)
			}
		}
		opts := spec.Options{File: spec.FileName, ExtensionName: src.Name, CLIVersion: m.CLIVersion}
		if _, err := spec.Parse(manifest, opts); err != nil {
			return nil, &InvalidManifestError{Source: src.String() + "@" + release.TagName, Err: err}
		}
	}

	stage, err := os.MkdirTemp(m.Root, ".tmp-"+src.Name+"-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	exe := filepath.Join(stage, ExecutableName(src.Name, m.goos()))
	if err := os.WriteFile(exe, data, 0o755); err != nil {
		return nil, err
	}
	if manifest != nil {
		if err := os.WriteFile(filepath.Join(stage, spec.FileName), manifest, 0o644); err != nil {
			return nil, err
		}
	}
	info := &InstallInfo{
		Owner:       src.Owner,
		Repo:        src.Repo,
		Tag:         release.TagName,
		SHA256:      ghrelease.SHA256Hex(data),
		Pinned:      pinned,
		InstalledAt: m.now().UTC(),
		Kind:        KindBinary,
	}
	if err := writeInstallInfo(filepath.Join(stage, InstallFile), info); err != nil {
		return nil, err
	}
	if err := m.replace(stage, m.entryPath(src.Name)); err != nil {
		return nil, err
	}
	_ = os.Remove(m.gitMetaPath(src.Name))
	m.clearNotice(src.Name)
	return m.load(src.Name)
}

func (m *Manager) installGit(ctx context.Context, src Source, pin string) (*Extension, error) {
	stage, err := os.MkdirTemp(m.Root, ".tmp-"+src.Name+"-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	if err := m.git().Clone(ctx, m.cloneURL(src.Owner, src.Repo), stage, pin); err != nil {
		return nil, fmt.Errorf("cloning %s: %w", src, err)
	}
	if err := checkExecutable(stage, src.Name, m.goos()); err != nil {
		return nil, fmt.Errorf("%s is not installable: %w (and its release has no binary for %s/%s)", src, err, m.goos(), m.goarch())
	}
	if _, err := m.loadManifest(stage, src.Name); err != nil {
		return nil, &InvalidManifestError{Source: src.String(), Err: err}
	}
	info := &InstallInfo{
		Owner:       src.Owner,
		Repo:        src.Repo,
		Tag:         pin,
		Pinned:      pin != "",
		InstalledAt: m.now().UTC(),
		Kind:        KindGit,
	}
	// The metadata lives outside the clone, so it is not part of the swap:
	// keep the previous one to put back if the swap fails and the old
	// clone stays (a reinstall with --force must not leave the old clone
	// without metadata, which would make it impossible to upgrade).
	metaPath := m.gitMetaPath(src.Name)
	prevMeta, prevErr := os.ReadFile(metaPath)
	if err := writeInstallInfo(metaPath, info); err != nil {
		return nil, err
	}
	if err := m.replace(stage, m.entryPath(src.Name)); err != nil {
		if prevErr == nil {
			_ = writeFileAtomic(metaPath, prevMeta, 0o644)
		} else {
			_ = os.Remove(metaPath)
		}
		return nil, err
	}
	m.clearNotice(src.Name)
	return m.load(src.Name)
}

// installLocal links the directory of src into the extensions directory.
// The link is followed on every start, so edits to the manifest take effect
// immediately.
func (m *Manager) installLocal(src Source, force bool) (*Extension, error) {
	info, err := os.Stat(src.LocalDir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", src.LocalDir)
	}
	if err := checkExecutable(src.LocalDir, src.Name, m.goos()); err != nil {
		return nil, err
	}
	if _, err := m.loadManifest(src.LocalDir, src.Name); err != nil {
		return nil, &InvalidManifestError{Source: src.LocalDir, Err: err}
	}
	entry := m.entryPath(src.Name)
	if !force {
		if _, err := os.Lstat(entry); err == nil {
			return nil, &AlreadyInstalledError{Name: src.Name}
		}
	}
	if err := os.MkdirAll(m.Root, 0o755); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(m.Root, ".tmp-"+src.Name+"-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	link := filepath.Join(stage, "link")
	if m.goos() == "windows" {
		err = os.WriteFile(link, []byte(src.LocalDir+"\n"), 0o644)
	} else {
		err = os.Symlink(src.LocalDir, link)
	}
	if err != nil {
		return nil, err
	}
	if err := m.replace(link, entry); err != nil {
		return nil, err
	}
	_ = os.Remove(m.gitMetaPath(src.Name))
	m.clearNotice(src.Name)
	return m.load(src.Name)
}

// checkExecutable verifies that dir has the extension's executable.
func checkExecutable(dir, name, goos string) error {
	path := findExecutable(dir, name, goos)
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("executable %s not found in %s", filepath.Base(path), dir)
	}
	if goos != "windows" && info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("%s is not executable (run chmod +x %s)", path, path)
	}
	return nil
}

// InvalidManifestError reports an extension whose manifest failed
// validation; Err is usually a *spec.ValidationError.
type InvalidManifestError struct {
	Source string
	Err    error
}

func (e *InvalidManifestError) Error() string {
	return fmt.Sprintf("%s has an invalid %s:\n%s", e.Source, spec.FileName, e.Err)
}

func (e *InvalidManifestError) Unwrap() error { return e.Err }

// rename is os.Rename; tests replace it to make a swap fail.
var rename = os.Rename

// oldPrefix starts the name of the directory replace moves a replaced
// entry into.
const oldPrefix = ".old-"

// replace moves stage to final. An existing final is moved aside first and
// restored if the swap fails, so the extension is never half replaced.
//
// Once the swap succeeded the new version is in place, so deleting the old
// one is best effort: it fails for instance on Windows while the old
// executable still runs. What is left over is swept by the next replace of
// the same entry.
func (m *Manager) replace(stage, final string) error {
	m.sweepOld(final)
	if _, err := os.Lstat(final); err != nil {
		return rename(stage, final)
	}
	old, err := os.MkdirTemp(m.Root, oldPrefix+filepath.Base(final)+"-")
	if err != nil {
		return err
	}
	aside := filepath.Join(old, "entry")
	if err := rename(final, aside); err != nil {
		_ = os.Remove(old)
		return err
	}
	if err := rename(stage, final); err != nil {
		_ = rename(aside, final)
		_ = os.RemoveAll(old)
		return err
	}
	_ = removeEntry(old)
	return nil
}

// sweepOld deletes what earlier replaces of final could not remove.
func (m *Manager) sweepOld(final string) {
	entries, _ := os.ReadDir(m.Root)
	prefix := oldPrefix + filepath.Base(final) + "-"
	for _, e := range entries {
		// MkdirTemp only appends digits; anything else belongs to an
		// extension whose name starts like this one (pr and pr-x).
		suffix, ok := strings.CutPrefix(e.Name(), prefix)
		if ok && suffix != "" && strings.Trim(suffix, "0123456789") == "" {
			_ = os.RemoveAll(filepath.Join(m.Root, e.Name()))
		}
	}
}

// removeEntry deletes an extension entry. A link (symlink or Windows link
// file) is removed itself; its target is never touched.
func removeEntry(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || info.Mode().IsRegular() {
		return os.Remove(path)
	}
	return os.RemoveAll(path)
}

// Remove uninstalls name: the extension directory (or the link of a local
// extension, never its target) and its metadata.
func (m *Manager) Remove(name string) error {
	if err := spec.ValidateExtensionName(name); err != nil {
		return err
	}
	entry := m.entryPath(name)
	if _, err := os.Lstat(entry); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("extension %q: %w", name, ErrNotInstalled)
	}
	if err := removeEntry(entry); err != nil {
		return err
	}
	_ = os.Remove(m.gitMetaPath(name))
	m.clearNotice(name)
	return nil
}
