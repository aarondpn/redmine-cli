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
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

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

// NewCmdUpdate creates the update command.
func NewCmdUpdate(f *cmdutil.Factory, version string) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update redmine CLI to the latest version",
		Long:  "Check GitHub releases for a newer version and replace the current binary.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdateWithFormat(version, cmd.OutOrStdout(), cmd.ErrOrStderr(), f.Printer(format))
		},
	}
	cmdutil.AddOutputFlag(cmd, &format)
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

// defaultCheckHomebrew reports whether the currently running binary lives
// inside Homebrew's prefix. A user may have the cask installed alongside a
// non-brew copy (curl/go install), so checking cask presence alone would
// misroute that binary's self-update to `brew upgrade`.
func defaultCheckHomebrew() bool {
	execPath, err := resolveExecPath()
	if err != nil {
		return false
	}
	prefix, err := brewPrefix()
	if err != nil || prefix == "" {
		return false
	}
	return strings.HasPrefix(execPath, prefix+string(filepath.Separator))
}

func defaultCheckHomebrewOutdated() (bool, error) {
	cmd := exec.Command("brew", "outdated", "--cask", "redmine")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("brew outdated failed: %w", err)
	}
	return strings.TrimSpace(out.String()) != "", nil
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

func runUpdateWithFormat(currentVersion string, stdout, stderr io.Writer, printer output.Printer) error {
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

	logStep(printer, stderr, "Current version: %s\n", currentVersion)
	logLine(printer, stderr, "Checking for updates...")

	release, err := fetchRelease(context.Background())
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

	execPath, err := installPath()
	if err != nil {
		return err
	}

	logStep(printer, stderr, "Downloading %s...\n", release.TagName)

	archiveData, err := downloadBytes(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}

	logLine(printer, stderr, "Verifying checksum...")
	if err := verifyChecksum(archiveData, assetName, checksumsURL); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
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

	resp, err := http.DefaultClient.Do(req)
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

func downloadBytes(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func verifyChecksum(data []byte, assetName, checksumsURL string) error {
	resp, err := http.Get(checksumsURL)
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checksums download returned status %d", resp.StatusCode)
	}

	var expectedHash string
	scanner := bufio.NewScanner(resp.Body)
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
// if the directory is not writable by the current user.
func installPath() (string, error) {
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
		return "", fmt.Errorf(
			"cannot update: %s is not writable\n\n"+
				"Reinstall to a user-writable location:\n"+
				"  curl -fsSL https://raw.githubusercontent.com/%s/%s/main/install.sh | bash\n\n"+
				"Or download manually from:\n"+
				"  https://github.com/%s/%s/releases/latest",
			dir, RepoOwner, RepoName, RepoOwner, RepoName,
		)
	}
	tmp.Close()
	os.Remove(tmp.Name())

	return execPath, nil
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
