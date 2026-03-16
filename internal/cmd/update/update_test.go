package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Homebrew detection ---

func stubHomebrew(t *testing.T, isBrew bool, upgradeErr error) {
	t.Helper()
	origCheck := checkHomebrew
	origUpgrade := upgradeHomebrew
	t.Cleanup(func() {
		checkHomebrew = origCheck
		upgradeHomebrew = origUpgrade
	})
	checkHomebrew = func() bool { return isBrew }
	upgradeHomebrew = func() error { return upgradeErr }
}

func TestRunUpdate_DelegatesToBrewWhenInstalled(t *testing.T) {
	var brewUpgradeCalled bool
	origCheck := checkHomebrew
	origUpgrade := upgradeHomebrew
	t.Cleanup(func() {
		checkHomebrew = origCheck
		upgradeHomebrew = origUpgrade
	})
	checkHomebrew = func() bool { return true }
	upgradeHomebrew = func() error {
		brewUpgradeCalled = true
		return nil
	}

	err := runUpdate("1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !brewUpgradeCalled {
		t.Error("expected brew upgrade to be called, but it was not")
	}
}

func TestRunUpdate_BrewUpgradeError(t *testing.T) {
	stubHomebrew(t, true, errors.New("brew is broken"))

	err := runUpdate("1.0.0")
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
	fetchRelease = func() (*githubRelease, error) {
		return &githubRelease{
			TagName: "v1.0.0",
			Assets:  []githubAsset{},
		}, nil
	}

	err := runUpdate("1.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

	err := verifyChecksum(data, "redmine-linux-amd64.tar.gz", ts.URL)
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

	err := verifyChecksum(data, "redmine-linux-amd64.tar.gz", ts.URL)
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

	err := verifyChecksum([]byte("data"), "redmine-linux-amd64.tar.gz", ts.URL)
	if err == nil {
		t.Fatal("expected error for missing asset, got nil")
	}
}

func TestVerifyChecksum_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	err := verifyChecksum([]byte("data"), "redmine-linux-amd64.tar.gz", ts.URL)
	if err == nil {
		t.Fatal("expected error for server error, got nil")
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

// --- isNewer ---

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
			got := isNewer(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("isNewer(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
