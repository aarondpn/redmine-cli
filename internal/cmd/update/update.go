package update

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	repoOwner    = "aarondpn"
	repoName     = "redmine-cli"
	binaryName   = "redmine"
	releaseURL   = "https://api.github.com/repos/" + repoOwner + "/" + repoName + "/releases/latest"
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// NewCmdUpdate creates the update command.
func NewCmdUpdate(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update redmine CLI to the latest version",
		Long:  "Check GitHub releases for a newer version and replace the current binary.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(version)
		},
	}
	return cmd
}

func runUpdate(currentVersion string) error {
	fmt.Printf("Current version: %s\n", currentVersion)
	fmt.Println("Checking for updates...")

	release, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentClean := strings.TrimPrefix(currentVersion, "v")

	if currentClean != "dev" && !isNewer(latestVersion, currentClean) {
		fmt.Printf("Already up to date (%s).\n", currentVersion)
		return nil
	}

	assetName := expectedAssetName()
	var downloadURL, checksumsURL string
	for _, asset := range release.Assets {
		switch asset.Name {
		case assetName:
			downloadURL = asset.BrowserDownloadURL
		case "checksums.txt":
			checksumsURL = asset.BrowserDownloadURL
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no release asset found for %s/%s (expected %s)", runtime.GOOS, runtime.GOARCH, assetName)
	}

	execPath, err := installPath()
	if err != nil {
		return err
	}

	fmt.Printf("Downloading %s...\n", release.TagName)

	archiveData, err := downloadBytes(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}

	if checksumsURL != "" {
		fmt.Println("Verifying checksum...")
		if err := verifyChecksum(archiveData, assetName, checksumsURL); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	binaryData, err := extractBinary(archiveData, assetName)
	if err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}

	if err := replaceBinary(execPath, binaryData); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	fmt.Printf("Updated successfully: %s → %s\n", currentVersion, release.TagName)
	return nil
}

func fetchLatestRelease() (*githubRelease, error) {
	resp, err := http.Get(releaseURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
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
	return fmt.Sprintf("redmine-%s-%s%s", runtime.GOOS, runtime.GOARCH, ext)
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
			dir, repoOwner, repoName, repoOwner, repoName,
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

// isNewer returns true if version a is newer than version b.
// Versions are expected as "major.minor.patch" (no "v" prefix).
func isNewer(a, b string) bool {
	aParts := parseVersion(a)
	bParts := parseVersion(b)
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

func parseVersion(v string) [3]int {
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
