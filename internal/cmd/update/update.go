package update

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/spf13/cobra"
)

const (
	RepoOwner  = "aarondpn"
	RepoName   = "redmine-cli"
	binaryName = "redmine"
	ReleaseURL = "https://api.github.com/repos/" + RepoOwner + "/" + RepoName + "/releases/latest"
)

const (
	// maxArchiveBytes bounds the release archive download. Release archives
	// are a few MiB; a response this large is not a real release.
	maxArchiveBytes = 200 << 20
	// maxChecksumsBytes bounds the checksums file, which has one short line
	// per release asset.
	maxChecksumsBytes = 1 << 20
	// releaseHTTPTimeout bounds each release metadata, checksums and archive
	// request end to end, including reading the body.
	releaseHTTPTimeout = 5 * time.Minute
)

// releaseHTTPClient fetches release metadata, archives and checksums. Like
// the attestation client it never waits forever: the server must start
// answering within attestationHTTPTimeout, and a whole request (body
// included) may take at most releaseHTTPTimeout.
var releaseHTTPClient = newReleaseHTTPClient()

func newReleaseHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = attestationHTTPTimeout
	return &http.Client{Transport: transport, Timeout: releaseHTTPTimeout}
}

// GithubRelease represents a GitHub release.
type GithubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []GithubAsset `json:"assets"`
}

// GithubAsset represents a release asset.
type GithubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// updateOptions carries the security-relevant knobs of runUpdateWithFormat.
type updateOptions struct {
	// SkipAttestation downgrades verification to checksum-only.
	SkipAttestation bool
	// Warn prints a security warning. Required when SkipAttestation is set.
	Warn func(msg string)
}

// NewCmdUpdate creates the update command.
func NewCmdUpdate(f *cmdutil.Factory, version string) *cobra.Command {
	var format string
	var opts updateOptions

	cmd := &cobra.Command{
		Use:   "update",
		Args:  cobra.NoArgs,
		Short: "Update redmine CLI to the latest version",
		Long: "Check GitHub releases for a newer version and replace the current binary.\n\n" +
			"The downloaded archive must match the release checksums and carry a valid\n" +
			"GitHub artifact attestation (SLSA build provenance signed via Sigstore) from\n" +
			"this repository's release workflow for the exact release tag. The update is\n" +
			"refused if either check fails.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Warn = f.Warn
			return runUpdateWithFormat(cmdutil.Context(cmd), version, cmd.OutOrStdout(), cmd.ErrOrStderr(), f.Printer(format), opts)
		},
	}
	cmdutil.AddOutputFlag(cmd, &format)
	cmd.Flags().BoolVar(&opts.SkipAttestation, "skip-attestation", false,
		"Install with checksum verification only, skipping the attestation check (not recommended)")
	return cmd
}

// checkHomebrew and upgradeHomebrew are package-level functions to allow
// test overrides.
var checkHomebrew = defaultCheckHomebrew
var checkHomebrewOutdated = defaultCheckHomebrewOutdated
var upgradeHomebrew = defaultUpgradeHomebrew
var resolveExecPath = defaultResolveExecPath
var brewPrefix = defaultBrewPrefix

func defaultResolveExecPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(execPath)
}

func defaultBrewPrefix() (string, error) {
	if _, err := exec.LookPath("brew"); err != nil {
		return "", err
	}
	out, err := exec.Command("brew", "--prefix").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// defaultCheckHomebrew reports whether the currently running binary resolves
// into Homebrew's cask installation for redmine. Users may have a cask
// installed alongside a manual or go-installed copy, including inside
// /opt/homebrew/bin or /usr/local/bin, so checking for brew presence or the
// top-level prefix alone would misroute self-update to `brew upgrade`.
func defaultCheckHomebrew() bool {
	execPath, err := resolveExecPath()
	if err != nil {
		return false
	}
	prefix, err := brewPrefix()
	if err != nil || prefix == "" {
		return false
	}
	caskDir := filepath.Join(prefix, "Caskroom", "redmine")
	return strings.HasPrefix(execPath, caskDir+string(filepath.Separator))
}

func defaultCheckHomebrewOutdated() (bool, error) {
	// `brew outdated --cask <name>` exits 0 with empty stdout when the cask
	// is up to date, exits 1 with the cask name on stdout when outdated, and
	// exits non-zero with stderr output on actual errors (e.g. unknown cask).
	// Treating any non-zero exit as an error would misclassify the common
	// "outdated" case, blocking self-update right when it should run.
	cmd := exec.Command("brew", "outdated", "--cask", "redmine")
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	stdout := strings.TrimSpace(out.String())

	if stdout != "" {
		return true, nil
	}
	if runErr != nil {
		stderr := strings.TrimSpace(errBuf.String())
		if stderr != "" {
			return false, fmt.Errorf("brew outdated failed: %w: %s", runErr, stderr)
		}
		return false, fmt.Errorf("brew outdated failed: %w", runErr)
	}
	return false, nil
}

func defaultUpgradeHomebrew(stdout, stderr io.Writer) error {
	fmt.Fprintln(stderr, "Installed via Homebrew, running: brew upgrade redmine")
	cmd := exec.Command("brew", "upgrade", "redmine")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew upgrade failed: %w", err)
	}
	return nil
}

var fetchRelease = FetchLatestRelease

// resolveInstallPath is a variable so tests can install into a temp file.
var resolveInstallPath = installPath

func runUpdateWithFormat(ctx context.Context, currentVersion string, stdout, stderr io.Writer, printer output.Printer, opts updateOptions) error {
	if checkHomebrew() {
		outdated, err := checkHomebrewOutdated()
		if err != nil {
			return err
		}
		if !outdated {
			printer.Outcome(false, output.ActionUpdated, "cli", "homebrew", "Already up to date via Homebrew.")
			return nil
		}

		brewStdout := stdout
		brewStderr := stderr
		if printer.Format() == output.FormatJSON {
			brewStdout = io.Discard
			brewStderr = io.Discard
		}
		if err := upgradeHomebrew(brewStdout, brewStderr); err != nil {
			return err
		}
		printer.Action(output.ActionUpdated, "cli", "homebrew", "Updated redmine via Homebrew")
		return nil
	}

	if opts.SkipAttestation {
		msg := "--skip-attestation: the update will only be verified against the release checksums, which come from the same place as the archive; a tampered release would NOT be detected"
		opts.Warn(msg)
	}

	logStep(printer, stderr, "Current version: %s\n", currentVersion)
	logLine(printer, stderr, "Checking for updates...")

	release, err := fetchRelease(ctx)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentClean := strings.TrimPrefix(currentVersion, "v")

	if currentClean != "dev" && !IsNewer(latestVersion, currentClean) {
		printer.Outcome(false, output.ActionUpdated, "cli", currentVersion, fmt.Sprintf("Already up to date (%s).", currentVersion))
		return nil
	}

	assetName := expectedAssetName()
	checksumsName := expectedChecksumsName(release.TagName)
	var downloadURL, checksumsURL string
	for _, asset := range release.Assets {
		switch asset.Name {
		case assetName:
			downloadURL = asset.BrowserDownloadURL
		case checksumsName:
			checksumsURL = asset.BrowserDownloadURL
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no release asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}
	if checksumsURL == "" {
		return fmt.Errorf("checksums asset %q not found in release %s; refusing to install unverified binary", checksumsName, release.TagName)
	}

	execPath, err := resolveInstallPath(release.TagName)
	if err != nil {
		return err
	}

	logStep(printer, stderr, "Downloading %s...\n", release.TagName)

	archiveData, err := downloadBytes(ctx, downloadURL, maxArchiveBytes)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}

	logLine(printer, stderr, "Verifying checksum...")
	if err := verifyChecksum(ctx, archiveData, assetName, checksumsURL); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	if !opts.SkipAttestation {
		logLine(printer, stderr, "Verifying attestation...")
		digest := sha256.Sum256(archiveData)
		if err := verifyAttestation(ctx, digest[:], release.TagName); err != nil {
			return err
		}
	}

	binaryData, err := extractBinary(archiveData, assetName)
	if err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}

	if err := replaceBinary(execPath, binaryData); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	printer.Action(output.ActionUpdated, "cli", release.TagName,
		fmt.Sprintf("Updated successfully: %s -> %s", currentVersion, release.TagName))
	return nil
}

// verifyAttestation runs the attestation check and turns its outcome into the
// user-facing error. Every failure refuses the install; only an unreachable
// attestations API or TUF repository points at --skip-attestation.
func verifyAttestation(ctx context.Context, digest []byte, tag string) error {
	err := newAttestationVerifier().Verify(ctx, digest, tag)
	if err == nil {
		return nil
	}
	var unavailable *attestationUnavailableError
	if errors.As(err, &unavailable) {
		return fmt.Errorf("attestation verification failed: %w\n\n"+
			"Could not reach GitHub or the Sigstore TUF repository to verify the release attestation; refusing to install.\n"+
			"Retry later, or re-run with --skip-attestation to install with checksum verification only (not recommended).", err)
	}
	return fmt.Errorf("attestation verification failed: %w; refusing to install %s", err, tag)
}

func logLine(printer output.Printer, w io.Writer, msg string) {
	if printer.Format() == output.FormatJSON {
		return
	}
	fmt.Fprintln(w, msg)
}

func logStep(printer output.Printer, w io.Writer, format string, args ...any) {
	if printer.Format() == output.FormatJSON {
		return
	}
	fmt.Fprintf(w, format, args...)
}

// FetchLatestRelease fetches the latest release from GitHub.
func FetchLatestRelease(ctx context.Context) (*GithubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ReleaseURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := releaseHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func expectedAssetName() string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("%s-%s-%s%s", RepoName, runtime.GOOS, runtime.GOARCH, ext)
}

// expectedChecksumsName returns the GoReleaser-default checksums filename
// for the given release tag (e.g. "redmine-cli_1.2.3_checksums.txt").
func expectedChecksumsName(tag string) string {
	return fmt.Sprintf("%s_%s_checksums.txt", RepoName, strings.TrimPrefix(tag, "v"))
}

// getLimited fetches url with releaseHTTPClient and returns the status code
// and, for 200 OK, the body. It fails with a *responseTooLargeError instead
// of reading more than limit bytes.
func getLimited(ctx context.Context, url string, limit int64) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}
	resp, err := releaseHTTPClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil, nil
	}
	if resp.ContentLength > limit {
		return 0, nil, &responseTooLargeError{limit: limit}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return 0, nil, err
	}
	if int64(len(data)) > limit {
		return 0, nil, &responseTooLargeError{limit: limit}
	}
	return resp.StatusCode, data, nil
}

// responseTooLargeError reports a download that exceeded its size cap.
type responseTooLargeError struct {
	limit int64
}

func (e *responseTooLargeError) Error() string {
	return fmt.Sprintf("response is larger than %d bytes; refusing to read it", e.limit)
}

// downloadBytes downloads url into memory, reading at most limit bytes.
func downloadBytes(ctx context.Context, url string, limit int64) ([]byte, error) {
	status, data, err := getLimited(ctx, url, limit)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", status)
	}
	return data, nil
}

func verifyChecksum(ctx context.Context, data []byte, assetName, checksumsURL string) error {
	status, checksums, err := getLimited(ctx, checksumsURL, maxChecksumsBytes)
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("checksums download returned status %d", status)
	}

	var expectedHash string
	scanner := bufio.NewScanner(bytes.NewReader(checksums))
	scanner.Buffer(nil, maxChecksumsBytes)
	for scanner.Scan() {
		// GoReleaser format: "<hash>  <filename>"
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[1] == assetName {
			expectedHash = fields[0]
			break
		}
	}
	if expectedHash == "" {
		return fmt.Errorf("no checksum found for %s", assetName)
	}

	actualHash := sha256.Sum256(data)
	actualHex := hex.EncodeToString(actualHash[:])

	if actualHex != expectedHash {
		return fmt.Errorf("expected %s, got %s", expectedHash, actualHex)
	}
	return nil
}

func extractBinary(archiveData []byte, assetName string) ([]byte, error) {
	r := bytes.NewReader(archiveData)
	if strings.HasSuffix(assetName, ".zip") {
		return extractFromZipReader(r, r.Size())
	}
	return extractFromTarGz(r)
}

func extractFromTarGz(r io.Reader) ([]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := filepath.Base(header.Name)
		if name == binaryName || name == binaryName+".exe" {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("binary not found in archive")
}

func extractFromZipReader(r io.ReaderAt, size int64) ([]byte, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	for _, f := range zr.File {
		name := filepath.Base(f.Name)
		if name == binaryName || name == binaryName+".exe" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("binary not found in zip archive")
}

// installPath returns the resolved path of the current binary, or an error
// if the directory is not writable by the current user. tag selects the
// install.sh revision suggested in that error.
func installPath(tag string) (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to find current executable: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable path: %w", err)
	}

	dir := filepath.Dir(execPath)
	tmp, err := os.CreateTemp(dir, ".redmine-write-test-*")
	if err != nil {
		return "", notWritableError(dir, tag)
	}
	tmp.Close()
	os.Remove(tmp.Name())

	return execPath, nil
}

// safeTagPattern matches release tags that are safe to embed in a suggested
// shell command and URL path.
var safeTagPattern = regexp.MustCompile(`^v` + releaseTagPattern + `$`)

// notWritableError explains how to reinstall when the binary's directory is
// not writable. The suggested install.sh is pinned to the release tag rather
// than the mutable main branch.
func notWritableError(dir, tag string) error {
	if !safeTagPattern.MatchString(tag) {
		return fmt.Errorf(
			"cannot update: %s is not writable\n\n"+
				"Reinstall to a user-writable location by downloading from:\n"+
				"  https://github.com/%s/%s/releases/latest",
			dir, RepoOwner, RepoName,
		)
	}
	return fmt.Errorf(
		"cannot update: %s is not writable\n\n"+
			"Reinstall to a user-writable location:\n"+
			"  curl -fsSL https://raw.githubusercontent.com/%s/%s/%s/install.sh | REDMINE_CLI_VERSION=%s bash\n\n"+
			"Or download manually from:\n"+
			"  https://github.com/%s/%s/releases/tag/%s",
		dir, RepoOwner, RepoName, tag, tag, RepoOwner, RepoName, tag,
	)
}

func replaceBinary(execPath string, newBinary []byte) error {
	dir := filepath.Dir(execPath)

	// Write new binary to temp file in same directory for atomic rename
	tmp, err := os.CreateTemp(dir, ".redmine-update-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(newBinary); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Preserve original file permissions
	info, err := os.Stat(execPath)
	if err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, info.Mode()); err != nil {
		os.Remove(tmpPath)
		return err
	}

	// Atomic rename
	if err := os.Rename(tmpPath, execPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

// IsNewer returns true if version a is newer than version b.
// Versions are expected as "major.minor.patch" (no "v" prefix).
func IsNewer(a, b string) bool {
	aParts := ParseVersion(a)
	bParts := ParseVersion(b)
	for i := 0; i < 3; i++ {
		if aParts[i] > bParts[i] {
			return true
		}
		if aParts[i] < bParts[i] {
			return false
		}
	}
	return false
}

// ParseVersion parses a semver string into [major, minor, patch].
func ParseVersion(v string) [3]int {
	var parts [3]int
	segments := strings.SplitN(v, ".", 3)
	for i, s := range segments {
		if i >= 3 {
			break
		}
		// Extract leading digits (e.g. "3-rc1" → "3")
		numStr := s
		if idx := strings.IndexFunc(s, func(r rune) bool {
			return r < '0' || r > '9'
		}); idx >= 0 {
			numStr = s[:idx]
		}
		if n, err := strconv.Atoi(numStr); err == nil {
			parts[i] = n
		}
	}
	return parts
}
