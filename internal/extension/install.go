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
	// Warn, when set, receives the warnings of a successful install: a
	// binary installed without checksum verification, or a release
	// without a manifest asset.
	Warn func(string)
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
			return m.installBinary(ctx, src, release, asset, binaryInstall{pinned: opts.Pin != "", warn: opts.Warn})
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

// binaryInstall controls installBinary.
type binaryInstall struct {
	pinned bool
	// requireVerified fails the install unless the binary (and manifest)
	// can be verified against the release's checksums file: an upgrade
	// of a verified install must not silently lose verification.
	requireVerified bool
	// warn receives the warnings of a successful install.
	warn func(string)
}

func (m *Manager) installBinary(ctx context.Context, src Source, release *ghrelease.Release, asset ghrelease.Asset, opts binaryInstall) (*Extension, error) {
	gh := m.github()
	label := "release " + release.TagName + " of " + src.String()
	data, err := gh.Download(ctx, asset.BrowserDownloadURL, MaxBinarySize)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", asset.Name, err)
	}
	var (
		checksums  []byte
		warnings   []string
		unverified string
	)
	if cs, ok := checksumsAsset(release); ok {
		if checksums, err = gh.Download(ctx, cs.BrowserDownloadURL, ghrelease.MaxChecksumsBytes); err != nil {
			return nil, fmt.Errorf("downloading %s: %w", cs.Name, err)
		}
		if err := ghrelease.VerifySHA256(data, checksums, asset.Name); err != nil {
			return nil, fmt.Errorf("checksum verification of %s failed: %w", asset.Name, err)
		}
	} else {
		unverified = label + " has no checksums file"
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
		} else if checksums != nil {
			unverified = "the checksums file of " + label + " does not list " + spec.FileName
			warnings = append(warnings, fmt.Sprintf("%s: it was installed without checksum verification", unverified))
		}
		parseOpts := spec.Options{File: spec.FileName, ExtensionName: src.Name, CLIVersion: m.CLIVersion}
		if _, err := spec.Parse(manifest, parseOpts); err != nil {
			return nil, &InvalidManifestError{Source: src.String() + "@" + release.TagName, Err: err}
		}
	} else {
		warnings = append(warnings, fmt.Sprintf("%s has no %s asset: installed as a passthrough extension, without declarative commands or MCP tools", label, spec.FileName))
	}
	if checksums == nil {
		files := asset.Name
		if manifest != nil {
			files += " and " + spec.FileName + " were"
		} else {
			files += " was"
		}
		warnings = append([]string{fmt.Sprintf("%s: %s installed without checksum verification", unverified, files)}, warnings...)
	}
	if opts.requireVerified && unverified != "" {
		return nil, fmt.Errorf("%s, but the installed version was checksum-verified; use --force to upgrade anyway", unverified)
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
		Owner:            src.Owner,
		Repo:             src.Repo,
		Tag:              release.TagName,
		SHA256:           ghrelease.SHA256Hex(data),
		ChecksumVerified: unverified == "",
		Pinned:           opts.pinned,
		InstalledAt:      m.now().UTC(),
		Kind:             KindBinary,
	}
	if err := m.swapIn(stage, src.Name, info); err != nil {
		return nil, err
	}
	if opts.warn != nil {
		for _, w := range warnings {
			opts.warn(w)
		}
	}
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
	if err := m.swapIn(stage, src.Name, info); err != nil {
		return nil, err
	}
	return m.load(src.Name)
}

// swapIn moves stage into place as extension name with the metadata info;
// nil (a local extension) removes the metadata. The metadata lives outside
// the extension directory, so it is not part of the swap: the previous one
// is put back if the swap fails and the old extension stays (a reinstall
// with --force must not leave the old one without metadata, which would
// make it impossible to upgrade).
func (m *Manager) swapIn(stage, name string, info *InstallInfo) error {
	metaPath := m.installMetaPath(name)
	if info != nil {
		prevMeta, prevErr := readLimited(metaPath, maxMetadataFile)
		if err := writeInstallInfo(metaPath, info); err != nil {
			return err
		}
		if err := m.replace(stage, m.entryPath(name)); err != nil {
			if prevErr == nil {
				_ = writeFileAtomic(metaPath, prevMeta, 0o644)
			} else {
				_ = os.Remove(metaPath)
			}
			return err
		}
	} else {
		if err := m.replace(stage, m.entryPath(name)); err != nil {
			return err
		}
		_ = os.Remove(metaPath)
	}
	m.clearNotice(name)
	return nil
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
	if err := m.CheckExecutable(src.LocalDir, src.Name); err != nil {
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
	if err := m.swapIn(link, src.Name, nil); err != nil {
		return nil, err
	}
	return m.load(src.Name)
}

// checkExecutable verifies that dir has the extension's executable.
func checkExecutable(dir, name, goos string) error {
	path := findExecutable(dir, name, goos)
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("executable %s not found in %s", filepath.Base(path), dir)
	}
	// The execute bit is a property of the host file system: Windows has
	// none, whatever extension platform goos stands for.
	if goos != "windows" && runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("%s is not executable (run chmod +x %s)", path, path)
	}
	return nil
}

// CheckExecutable verifies that the local directory dir has the executable
// of extension name. For a Go project (a go.mod in dir) a missing
// executable comes with the command that builds it.
func (m *Manager) CheckExecutable(dir, name string) error {
	err := checkExecutable(dir, name, m.goos())
	if err != nil && fileExists(filepath.Join(dir, "go.mod")) {
		if _, statErr := os.Stat(findExecutable(dir, name, m.goos())); statErr != nil {
			return fmt.Errorf("%w; build it first: go build -o %s", err, ExecutableName(name, m.goos()))
		}
	}
	return err
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

// rename is os.Rename unless a test set Manager.renameFunc to make a swap
// fail.
func (m *Manager) rename(from, to string) error {
	if m.renameFunc != nil {
		return m.renameFunc(from, to)
	}
	return os.Rename(from, to)
}

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
		return m.rename(stage, final)
	}
	old, err := os.MkdirTemp(m.Root, oldPrefix+filepath.Base(final)+"-")
	if err != nil {
		return err
	}
	aside := filepath.Join(old, "entry")
	if err := m.rename(final, aside); err != nil {
		_ = os.Remove(old)
		return err
	}
	if err := m.rename(stage, final); err != nil {
		_ = m.rename(aside, final)
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
	_ = os.Remove(m.installMetaPath(name))
	m.clearNotice(name)
	return nil
}
