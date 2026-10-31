package update

import (
	"archive/tar"
	"archive/zip"
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
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// --- Homebrew detection ---

// skipHomebrewOnWindows skips tests that need Unix paths or a shell-script
// brew shim. Homebrew does not exist on Windows.
func skipHomebrewOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Homebrew is not available on Windows")
	}
}

func stubBrewEnv(t *testing.T, execPath, prefix string, prefixErr error) {
	t.Helper()
	origExec := resolveExecPath
	origPrefix := brewPrefix
	t.Cleanup(func() {
		resolveExecPath = origExec
		brewPrefix = origPrefix
	})
	resolveExecPath = func() (string, error) { return execPath, nil }
	brewPrefix = func() (string, error) { return prefix, prefixErr }
}

func TestDefaultCheckHomebrew_ExecUnderCaskDir(t *testing.T) {
	skipHomebrewOnWindows(t)
	stubBrewEnv(t, "/opt/homebrew/Caskroom/redmine/2.1.1/redmine", "/opt/homebrew", nil)
	if !defaultCheckHomebrew() {
		t.Error("expected true when exec path resolves into the redmine cask dir")
	}
}

func TestDefaultCheckHomebrew_ExecOutsidePrefix(t *testing.T) {
	// Simulates a curl- or go-install binary on a machine that also has the
	// brew cask. Previously this returned true and misrouted to brew upgrade.
	stubBrewEnv(t, "/Users/someone/.local/bin/redmine", "/opt/homebrew", nil)
	if defaultCheckHomebrew() {
		t.Error("expected false when exec path is outside brew prefix")
	}
}

func TestDefaultCheckHomebrew_ExecUnderPrefixButOutsideCaskDir(t *testing.T) {
	// A manually copied binary in /opt/homebrew/bin must not be treated as the
	// brew-managed cask binary.
	stubBrewEnv(t, "/opt/homebrew/bin/redmine", "/opt/homebrew", nil)
	if defaultCheckHomebrew() {
		t.Error("expected false when exec path is under brew prefix but outside the cask dir")
	}
}

func TestDefaultCheckHomebrew_CaskDirPrefixMatch(t *testing.T) {
	// Guards against naive string prefix match: "/opt/homebrew/Caskroom/redmine-sibling/..."
	// must not count as being under "/opt/homebrew/Caskroom/redmine".
	stubBrewEnv(t, "/opt/homebrew/Caskroom/redmine-sibling/2.1.1/redmine", "/opt/homebrew", nil)
	if defaultCheckHomebrew() {
		t.Error("expected false for sibling dir sharing redmine cask dir as string prefix")
	}
}

func TestDefaultCheckHomebrew_BrewNotInstalled(t *testing.T) {
	stubBrewEnv(t, "/opt/homebrew/Caskroom/redmine/2.1.1/redmine", "", errors.New("brew not found"))
	if defaultCheckHomebrew() {
		t.Error("expected false when brew is unavailable")
	}
}

// withBrewShim prepends a temp dir containing a fake `brew` shell script to
// PATH. The script's body is the argument verbatim. Returns nothing; the
// original PATH is restored via t.Cleanup.
func withBrewShim(t *testing.T, body string) {
	t.Helper()
	skipHomebrewOnWindows(t)
	dir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	shimPath := filepath.Join(dir, "brew")
	if err := os.WriteFile(shimPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write brew shim: %v", err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)
}

func TestDefaultCheckHomebrewOutdated_Outdated(t *testing.T) {
	// brew outdated --cask <name> exits 1 and prints the cask name when
	// the cask is outdated. Must be read as outdated=true, not as an error.
	withBrewShim(t, `echo redmine; exit 1`)

	outdated, err := defaultCheckHomebrewOutdated()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !outdated {
		t.Error("expected outdated=true when brew prints cask name and exits 1")
	}
}

func TestDefaultCheckHomebrewOutdated_UpToDate(t *testing.T) {
	withBrewShim(t, `exit 0`)

	outdated, err := defaultCheckHomebrewOutdated()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outdated {
		t.Error("expected outdated=false for exit 0 with empty stdout")
	}
}

func TestDefaultCheckHomebrewOutdated_ActualError(t *testing.T) {
	withBrewShim(t, `echo "Error: Cask unavailable" >&2; exit 1`)

	outdated, err := defaultCheckHomebrewOutdated()
	if err == nil {
		t.Fatal("expected error when stdout is empty and stderr has a message")
	}
	if outdated {
		t.Error("expected outdated=false on error path")
	}
	if !strings.Contains(err.Error(), "Cask unavailable") {
		t.Errorf("expected error to surface stderr, got: %v", err)
	}
}

func stubHomebrew(t *testing.T, isBrew bool, upgradeErr error) {
	t.Helper()
	origCheck := checkHomebrew
	origOutdated := checkHomebrewOutdated
	origUpgrade := upgradeHomebrew
	t.Cleanup(func() {
		checkHomebrew = origCheck
		checkHomebrewOutdated = origOutdated
		upgradeHomebrew = origUpgrade
	})
	checkHomebrew = func() bool { return isBrew }
	checkHomebrewOutdated = func() (bool, error) { return true, nil }
	upgradeHomebrew = func(stdout, stderr io.Writer) error { return upgradeErr }
}

func runUpdateForTest(t *testing.T, version string) error {
	t.Helper()
	var out, errOut bytes.Buffer
	return runUpdateWithFormat(context.Background(), version, &out, &errOut, output.NewStdPrinter(&out, &errOut, false, true, output.FormatTable), updateOptions{})
}

func TestRunUpdate_DelegatesToBrewWhenInstalled(t *testing.T) {
	var brewUpgradeCalled bool
	origCheck := checkHomebrew
	origOutdated := checkHomebrewOutdated
	origUpgrade := upgradeHomebrew
	t.Cleanup(func() {
		checkHomebrew = origCheck
		checkHomebrewOutdated = origOutdated
		upgradeHomebrew = origUpgrade
	})
	checkHomebrew = func() bool { return true }
	checkHomebrewOutdated = func() (bool, error) { return true, nil }
	upgradeHomebrew = func(stdout, stderr io.Writer) error {
		brewUpgradeCalled = true
		return nil
	}

	err := runUpdateForTest(t, "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !brewUpgradeCalled {
		t.Error("expected brew upgrade to be called, but it was not")
	}
}

func TestRunUpdate_BrewUpgradeError(t *testing.T) {
	stubHomebrew(t, true, errors.New("brew is broken"))

	err := runUpdateForTest(t, "1.0.0")
	if err == nil {
		t.Fatal("expected error from brew upgrade, got nil")
	}
	if !strings.Contains(err.Error(), "brew is broken") {
		t.Errorf("expected error to contain 'brew is broken', got: %v", err)
	}
}

func TestRunUpdate_SkipsBrewWhenNotInstalled(t *testing.T) {
	// When not a brew install, runUpdate should proceed to check GitHub.
	// We stub fetchRelease to return a version equal to current,
	// so it exits with "Already up to date" without downloading.
	stubHomebrew(t, false, nil)

	origFetch := fetchRelease
	t.Cleanup(func() { fetchRelease = origFetch })
	fetchRelease = func(ctx context.Context) (*GithubRelease, error) {
		return &GithubRelease{
			TagName: "v1.0.0",
			Assets:  []GithubAsset{},
		}, nil
	}

	err := runUpdateForTest(t, "1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunUpdate_JSONOutput_AlreadyUpToDate(t *testing.T) {
	stubHomebrew(t, false, nil)

	origFetch := fetchRelease
	t.Cleanup(func() { fetchRelease = origFetch })
	fetchRelease = func(ctx context.Context) (*GithubRelease, error) {
		return &GithubRelease{
			TagName: "v1.0.0",
			Assets:  []GithubAsset{},
		}, nil
	}

	var out, errOut bytes.Buffer
	err := runUpdateWithFormat(context.Background(), "1.0.0", &out, &errOut, output.NewStdPrinter(&out, &errOut, false, true, output.FormatJSON), updateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var env struct {
		Ok      bool   `json:"ok"`
		Action  string `json:"action"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("expected JSON output, got: %v\n%s", err, out.String())
	}
	if env.Ok || env.Action != "updated" || !strings.Contains(env.Message, "Already up to date") {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no stderr in JSON mode, got %q", errOut.String())
	}
}

func TestRunUpdate_HomebrewNoopReturnsFalseOutcome(t *testing.T) {
	origCheck := checkHomebrew
	origOutdated := checkHomebrewOutdated
	origUpgrade := upgradeHomebrew
	t.Cleanup(func() {
		checkHomebrew = origCheck
		checkHomebrewOutdated = origOutdated
		upgradeHomebrew = origUpgrade
	})
	checkHomebrew = func() bool { return true }
	checkHomebrewOutdated = func() (bool, error) { return false, nil }
	upgradeHomebrew = func(stdout, stderr io.Writer) error {
		t.Fatal("did not expect brew upgrade to run")
		return nil
	}

	var out, errOut bytes.Buffer
	err := runUpdateWithFormat(context.Background(), "1.0.0", &out, &errOut, output.NewStdPrinter(&out, &errOut, false, true, output.FormatJSON), updateOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var env struct {
		Ok      bool   `json:"ok"`
		Action  string `json:"action"`
		ID      string `json:"id"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("expected JSON output, got: %v\n%s", err, out.String())
	}
	if env.Ok || env.Action != "updated" || env.ID != "homebrew" || !strings.Contains(env.Message, "Already up to date") {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected no stderr in JSON mode, got %q", errOut.String())
	}
}

// --- verifyChecksum ---

func TestVerifyChecksum_Valid(t *testing.T) {
	data := []byte("hello world")
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	checksums := fmt.Sprintf(
		"aaaa  other-file.tar.gz\n%s  redmine-linux-amd64.tar.gz\nbbbb  another.zip\n",
		hashHex,
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, checksums)
	}))
	defer ts.Close()

	err := verifyChecksum(context.Background(), data, "redmine-linux-amd64.tar.gz", ts.URL)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	data := []byte("hello world")
	checksums := "0000000000000000000000000000000000000000000000000000000000000000  redmine-linux-amd64.tar.gz\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, checksums)
	}))
	defer ts.Close()

	err := verifyChecksum(context.Background(), data, "redmine-linux-amd64.tar.gz", ts.URL)
	if err == nil {
		t.Fatal("expected checksum mismatch error, got nil")
	}
}

func TestVerifyChecksum_AssetNotFound(t *testing.T) {
	checksums := "abcd1234  some-other-file.tar.gz\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, checksums)
	}))
	defer ts.Close()

	err := verifyChecksum(context.Background(), []byte("data"), "redmine-linux-amd64.tar.gz", ts.URL)
	if err == nil {
		t.Fatal("expected error for missing asset, got nil")
	}
}

func TestVerifyChecksum_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	err := verifyChecksum(context.Background(), []byte("data"), "redmine-linux-amd64.tar.gz", ts.URL)
	if err == nil {
		t.Fatal("expected error for server error, got nil")
	}
}

func TestVerifyChecksum_RejectsOversizedFile(t *testing.T) {
	data := []byte("hello world")
	hash := sha256.Sum256(data)
	line := hex.EncodeToString(hash[:]) + "  redmine-linux-amd64.tar.gz\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The matching line comes first: the cap must apply to the whole
		// response, not only to the part scanned before a match.
		_, _ = io.WriteString(w, line)
		_, _ = w.Write(bytes.Repeat([]byte("x"), maxChecksumsBytes))
	}))
	defer ts.Close()

	err := verifyChecksum(context.Background(), data, "redmine-linux-amd64.tar.gz", ts.URL)
	var tooLarge *responseTooLargeError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("verifyChecksum() error = %v, want responseTooLargeError", err)
	}
}

// --- downloadBytes ---

func TestDownloadBytes_Limit(t *testing.T) {
	const limit = 1024
	tests := []struct {
		name      string
		size      int
		chunked   bool
		wantError bool
	}{
		{name: "at the limit", size: limit},
		{name: "declared length over the limit", size: limit + 1, wantError: true},
		{name: "chunked body over the limit", size: limit + 1, chunked: true, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.chunked {
					// Flushing before the body forces chunked encoding,
					// so no Content-Length reveals the size up front.
					w.(http.Flusher).Flush()
				}
				_, _ = w.Write(bytes.Repeat([]byte("a"), tt.size))
			}))
			defer ts.Close()

			data, err := downloadBytes(context.Background(), ts.URL, limit)
			if tt.wantError {
				var tooLarge *responseTooLargeError
				if !errors.As(err, &tooLarge) {
					t.Fatalf("downloadBytes() error = %v, want responseTooLargeError", err)
				}
				return
			}
			if err != nil || len(data) != tt.size {
				t.Fatalf("downloadBytes() = %d bytes, %v; want %d bytes", len(data), err, tt.size)
			}
		})
	}
}

func TestDownloadBytes_HonorsContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("archive"))
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := downloadBytes(ctx, ts.URL, maxArchiveBytes); !errors.Is(err, context.Canceled) {
		t.Fatalf("downloadBytes() error = %v, want context.Canceled", err)
	}
}

func TestReleaseHTTPClientHasTimeouts(t *testing.T) {
	if releaseHTTPClient.Timeout <= 0 {
		t.Error("releaseHTTPClient has no overall timeout")
	}
	transport, ok := releaseHTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("releaseHTTPClient.Transport = %T, want *http.Transport", releaseHTTPClient.Transport)
	}
	if transport.ResponseHeaderTimeout <= 0 || transport.TLSHandshakeTimeout <= 0 {
		t.Errorf("transport timeouts: response header %v, TLS handshake %v; want both set",
			transport.ResponseHeaderTimeout, transport.TLSHandshakeTimeout)
	}
}

// --- extractBinary (tar.gz) ---

func buildTarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, content := range files {
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func TestExtractBinary_TarGz(t *testing.T) {
	expected := []byte("binary-content")
	archive := buildTarGz(t, map[string][]byte{
		"redmine": expected,
	})

	got, err := extractBinary(archive, "redmine-linux-amd64.tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, expected) {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestExtractBinary_TarGzWithPath(t *testing.T) {
	expected := []byte("binary-in-subdir")
	archive := buildTarGz(t, map[string][]byte{
		"dist/redmine": expected,
	})

	got, err := extractBinary(archive, "redmine-linux-amd64.tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, expected) {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestExtractBinary_TarGzMissingBinary(t *testing.T) {
	archive := buildTarGz(t, map[string][]byte{
		"not-the-binary": []byte("something"),
	})

	_, err := extractBinary(archive, "redmine-linux-amd64.tar.gz")
	if err == nil {
		t.Fatal("expected error for missing binary, got nil")
	}
}

// --- extractBinary (zip) ---

func buildZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	zw.Close()
	return buf.Bytes()
}

func TestExtractBinary_Zip(t *testing.T) {
	expected := []byte("windows-binary")
	archive := buildZip(t, map[string][]byte{
		"redmine.exe": expected,
	})

	got, err := extractBinary(archive, "redmine-windows-amd64.zip")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, expected) {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestExtractBinary_ZipMissingBinary(t *testing.T) {
	archive := buildZip(t, map[string][]byte{
		"readme.txt": []byte("hello"),
	})

	_, err := extractBinary(archive, "redmine-windows-amd64.zip")
	if err == nil {
		t.Fatal("expected error for missing binary, got nil")
	}
}

// --- IsNewer ---

func TestIsNewer(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.6.0", "1.5.0", true},
		{"1.5.0", "1.6.0", false},
		{"1.5.0", "1.5.0", false},
		{"2.0.0", "1.9.9", true},
		{"1.5.1", "1.5.0", true},
		{"1.5.0", "1.5.1", false},
		{"1.10.0", "1.9.0", true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_vs_%s", tt.a, tt.b), func(t *testing.T) {
			got := IsNewer(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// --- runUpdate: checksum asset is mandatory ---

// TestRunUpdate_RefusesWithoutChecksumsAsset guards against silent checksum
// skip when the release omits the checksums asset. Installing an unverified
// binary would be a downgrade attack vector.
func TestRunUpdate_RefusesWithoutChecksumsAsset(t *testing.T) {
	stubHomebrew(t, false, nil)

	var downloadCalls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloadCalls++
		fmt.Fprint(w, "tampered payload")
	}))
	defer ts.Close()

	origFetch := fetchRelease
	t.Cleanup(func() { fetchRelease = origFetch })
	fetchRelease = func(ctx context.Context) (*GithubRelease, error) {
		return &GithubRelease{
			TagName: "v99.0.0",
			Assets: []GithubAsset{
				{Name: expectedAssetName(), BrowserDownloadURL: ts.URL},
				// No checksums asset.
			},
		}, nil
	}

	err := runUpdateForTest(t, "1.0.0")
	if err == nil {
		t.Fatal("expected error when checksums asset is missing, got nil")
	}
	if !strings.Contains(err.Error(), "checksums asset") {
		t.Errorf("expected error to mention missing checksums asset, got: %v", err)
	}
	if downloadCalls != 0 {
		t.Errorf("expected no archive download when checksums missing, got %d calls", downloadCalls)
	}
}

// TestRunUpdate_RefusesOnChecksumMismatch ensures a tampered archive fails
// verification and does not get written to disk.
func TestRunUpdate_RefusesOnChecksumMismatch(t *testing.T) {
	stubHomebrew(t, false, nil)

	archive := []byte("tampered payload")
	// Checksum for a DIFFERENT payload — simulates modified archive.
	bogusHash := sha256.Sum256([]byte("original payload"))
	checksumLine := fmt.Sprintf("%s  %s\n", hex.EncodeToString(bogusHash[:]), expectedAssetName())

	archiveSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive)
	}))
	defer archiveSrv.Close()
	checksumSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, checksumLine)
	}))
	defer checksumSrv.Close()

	origFetch := fetchRelease
	t.Cleanup(func() { fetchRelease = origFetch })
	fetchRelease = func(ctx context.Context) (*GithubRelease, error) {
		return &GithubRelease{
			TagName: "v99.0.0",
			Assets: []GithubAsset{
				{Name: expectedAssetName(), BrowserDownloadURL: archiveSrv.URL},
				{Name: expectedChecksumsName("v99.0.0"), BrowserDownloadURL: checksumSrv.URL},
			},
		}, nil
	}

	err := runUpdateForTest(t, "1.0.0")
	if err == nil {
		t.Fatal("expected checksum verification error, got nil")
	}
	if !strings.Contains(err.Error(), "checksum verification failed") {
		t.Errorf("expected error to mention checksum verification failure, got: %v", err)
	}
}

func TestFetchLatestRelease_SendsTokenAndHintsRateLimit(t *testing.T) {
	var gotAuth string
	status := http.StatusOK
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(status)
		fmt.Fprint(w, `{"tag_name":"v9.9.9"}`)
	}))
	defer ts.Close()

	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	if _, err := fetchLatestRelease(context.Background(), ts.URL); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "" {
		t.Errorf("anonymous request sent Authorization %q", gotAuth)
	}

	t.Setenv("GH_TOKEN", "gh-token")
	release, err := fetchLatestRelease(context.Background(), ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer gh-token" {
		t.Errorf("Authorization = %q, want the GH_TOKEN bearer", gotAuth)
	}
	if release.TagName != "v9.9.9" {
		t.Errorf("TagName = %q", release.TagName)
	}

	status = http.StatusForbidden
	_, err = fetchLatestRelease(context.Background(), ts.URL)
	if err == nil || !strings.Contains(err.Error(), "rate limited? set GITHUB_TOKEN or GH_TOKEN") {
		t.Errorf("403 error = %v, want the rate limit hint", err)
	}
}

func TestFetchLatestRelease_RetriesWithoutRejectedToken(t *testing.T) {
	var auths []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auths = append(auths, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") != "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"tag_name":"v9.9.9"}`)
	}))
	defer ts.Close()

	t.Setenv("GITHUB_TOKEN", "stale")
	release, err := fetchLatestRelease(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("expected the anonymous retry to succeed, got %v", err)
	}
	if release.TagName != "v9.9.9" {
		t.Errorf("TagName = %q", release.TagName)
	}
	if len(auths) != 2 || auths[0] != "Bearer stale" || auths[1] != "" {
		t.Errorf("requests sent Authorization %q, want the token once, then none", auths)
	}
}

func TestFetchLatestRelease_UnauthorizedHint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	t.Setenv("GITHUB_TOKEN", "stale")
	_, err := fetchLatestRelease(context.Background(), ts.URL)
	if err == nil || !strings.Contains(err.Error(), "GITHUB_TOKEN or GH_TOKEN rejected") {
		t.Errorf("401 error = %v, want the rejected token hint", err)
	}
}
