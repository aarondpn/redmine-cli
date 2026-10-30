package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Scoop / winget detection ---

func TestManagedInstallFor(t *testing.T) {
	tests := []struct {
		name     string
		execPath string
		env      map[string]string
		want     string // expected Command, "" for not managed
	}{
		{
			name:     "scoop default root",
			execPath: `C:\Users\ada\scoop\apps\redmine\2.17.0\redmine.exe`,
			want:     "scoop update redmine",
		},
		{
			name:     "scoop current junction",
			execPath: `C:\Users\ada\scoop\apps\redmine\current\redmine.exe`,
			want:     "scoop update redmine",
		},
		{
			name:     "scoop custom root from SCOOP",
			execPath: `D:\tools\apps\redmine\2.17.0\redmine.exe`,
			env:      map[string]string{"SCOOP": `D:\Tools\`},
			want:     "scoop update redmine",
		},
		{
			name:     "scoop global root from SCOOP_GLOBAL",
			execPath: `C:\ProgramData\pkgs\apps\redmine\2.17.0\redmine.exe`,
			env:      map[string]string{"SCOOP_GLOBAL": `C:\ProgramData\pkgs`},
			want:     "scoop update redmine",
		},
		{
			name:     "scoop app with another name",
			execPath: `C:\Users\ada\scoop\apps\redmine-tools\1.0.0\redmine.exe`,
		},
		{
			name:     "winget user scope",
			execPath: `C:\Users\ada\AppData\Local\Microsoft\WinGet\Packages\aarondpn.redmine-cli_Microsoft.Winget.Source_8wekyb3d8bbwe\redmine.exe`,
			want:     "winget upgrade aarondpn.redmine-cli",
		},
		{
			name:     "winget machine scope",
			execPath: `C:\Program Files\WinGet\Packages\aarondpn.redmine-cli_Microsoft.Winget.Source_8wekyb3d8bbwe\redmine.exe`,
			want:     "winget upgrade aarondpn.redmine-cli",
		},
		{
			name:     "winget package with a longer id",
			execPath: `C:\Users\ada\AppData\Local\Microsoft\WinGet\Packages\aarondpn.redmine-cli-nightly_Microsoft.Winget.Source_8wekyb3d8bbwe\redmine.exe`,
		},
		{
			name:     "zip install on windows",
			execPath: `C:\Users\ada\bin\redmine.exe`,
		},
		{
			name:     "install script on linux",
			execPath: "/home/ada/.local/bin/redmine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := managedInstallFor(tt.execPath, func(k string) string { return tt.env[k] })
			if tt.want == "" {
				if ok {
					t.Fatalf("expected not managed, got %+v", got)
				}
				return
			}
			if !ok {
				t.Fatalf("expected %q, got not managed", tt.want)
			}
			if got.Command != tt.want {
				t.Errorf("Command = %q, want %q", got.Command, tt.want)
			}
		})
	}
}

func stubManagedInstall(t *testing.T, m managedInstall, ok bool) {
	t.Helper()
	orig := detectManagedInstall
	t.Cleanup(func() { detectManagedInstall = orig })
	detectManagedInstall = func() (managedInstall, bool) { return m, ok }
}

func TestRunUpdate_RefusesManagedInstall(t *testing.T) {
	stubHomebrew(t, false, nil)
	stubManagedInstall(t, managedInstall{Manager: "Scoop", Command: "scoop update redmine"}, true)

	var downloadCalls int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloadCalls++
	}))
	defer ts.Close()

	origFetch := fetchRelease
	t.Cleanup(func() { fetchRelease = origFetch })
	fetchRelease = func(ctx context.Context) (*GithubRelease, error) {
		return &GithubRelease{
			TagName: "v99.0.0",
			Assets: []GithubAsset{
				{Name: expectedAssetName(), BrowserDownloadURL: ts.URL},
				{Name: expectedChecksumsName("v99.0.0"), BrowserDownloadURL: ts.URL},
			},
		}, nil
	}

	err := runUpdateForTest(t, "1.0.0")
	if err == nil {
		t.Fatal("expected an error for a Scoop install, got nil")
	}
	for _, want := range []string{"v99.0.0", "Scoop", "scoop update redmine"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if downloadCalls != 0 {
		t.Errorf("expected no download for a managed install, got %d calls", downloadCalls)
	}
}

func TestRunUpdate_ManagedInstallAlreadyUpToDate(t *testing.T) {
	stubHomebrew(t, false, nil)
	stubManagedInstall(t, managedInstall{Manager: "winget", Command: "winget upgrade aarondpn.redmine-cli"}, true)

	origFetch := fetchRelease
	t.Cleanup(func() { fetchRelease = origFetch })
	fetchRelease = func(ctx context.Context) (*GithubRelease, error) {
		return &GithubRelease{TagName: "v1.0.0"}, nil
	}

	if err := runUpdateForTest(t, "1.0.0"); err != nil {
		t.Fatalf("expected no error when already up to date, got %v", err)
	}
}

// --- Windows-style replacement of the running binary ---

func TestSwapMovingAside(t *testing.T) {
	dir := t.TempDir()
	execPath := filepath.Join(dir, "redmine.exe")
	tmpPath := filepath.Join(dir, ".redmine-update-1")
	writeFile(t, execPath, "old")
	writeFile(t, execPath+".old", "from a previous update")
	writeFile(t, tmpPath, "new")

	if err := swapMovingAside(tmpPath, execPath); err != nil {
		t.Fatalf("swapMovingAside: %v", err)
	}
	if got := readFile(t, execPath); got != "new" {
		t.Errorf("exec content = %q, want %q", got, "new")
	}
	if got := readFile(t, execPath+".old"); got != "old" {
		t.Errorf(".old content = %q, want %q", got, "old")
	}
	if _, err := os.Stat(tmpPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("temp file still present: %v", err)
	}
}

func TestSwapMovingAside_RestoresOnFailure(t *testing.T) {
	dir := t.TempDir()
	execPath := filepath.Join(dir, "redmine.exe")
	writeFile(t, execPath, "old")

	// A missing temp file makes the second rename fail.
	err := swapMovingAside(filepath.Join(dir, "missing"), execPath)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if got := readFile(t, execPath); got != "old" {
		t.Errorf("exec content = %q, want the restored %q", got, "old")
	}
}

func TestReplaceBinary_MovesAsideWhenRequired(t *testing.T) {
	orig := moveRunningBinaryAside
	t.Cleanup(func() { moveRunningBinaryAside = orig })
	moveRunningBinaryAside = true

	dir := t.TempDir()
	execPath := filepath.Join(dir, "redmine.exe")
	writeFile(t, execPath, "old")

	if err := replaceBinary(execPath, []byte("new")); err != nil {
		t.Fatalf("replaceBinary: %v", err)
	}
	if got := readFile(t, execPath); got != "new" {
		t.Errorf("exec content = %q, want %q", got, "new")
	}
	if got := readFile(t, execPath+".old"); got != "old" {
		t.Errorf(".old content = %q, want %q", got, "old")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
