package update

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestShouldCheck(t *testing.T) {
	// Override isTerminal to always return true for these tests.
	origTerminal := isTerminal
	t.Cleanup(func() { isTerminal = origTerminal })
	isTerminal = func() bool { return true }

	tests := []struct {
		name    string
		version string
		args    []string
		env     string // REDMINE_NO_UPDATE_CHECK value
		tty     bool
		want    bool
	}{
		{name: "normal", version: "1.0.0", args: []string{"issue", "list"}, tty: true, want: true},
		{name: "dev version", version: "dev", args: nil, tty: true, want: false},
		{name: "env 1", version: "1.0.0", args: nil, env: "1", tty: true, want: false},
		{name: "env true", version: "1.0.0", args: nil, env: "true", tty: true, want: false},
		{name: "env TRUE", version: "1.0.0", args: nil, env: "TRUE", tty: true, want: false},
		{name: "update command", version: "1.0.0", args: []string{"update"}, tty: true, want: false},
		{name: "update after flags", version: "1.0.0", args: []string{"--verbose", "update"}, tty: true, want: false},
		{name: "update after flag with value", version: "1.0.0", args: []string{"--config", "cfg.yaml", "update"}, tty: true, want: false},
		{name: "update after short flag", version: "1.0.0", args: []string{"-v", "update"}, tty: true, want: false},
		{name: "no tty", version: "1.0.0", args: nil, tty: false, want: false},
		{name: "no args", version: "1.0.0", args: nil, tty: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isTerminal = func() bool { return tt.tty }

			if tt.env != "" {
				t.Setenv("REDMINE_NO_UPDATE_CHECK", tt.env)
			}
			got := ShouldCheck(tt.version, tt.args)
			if got != tt.want {
				t.Errorf("ShouldCheck(%q, %v) = %v, want %v", tt.version, tt.args, got, tt.want)
			}
		})
	}
}

func stubFetchRelease(t *testing.T, fn func(ctx context.Context) (*GithubRelease, error)) {
	t.Helper()
	orig := fetchRelease
	t.Cleanup(func() { fetchRelease = orig })
	fetchRelease = fn
}

func TestCheckForUpdate_NewerAvailable(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		return &GithubRelease{TagName: "v2.0.0"}, nil
	})

	result := CheckForUpdate(context.Background(), "1.0.0")
	if result == nil {
		t.Fatal("expected non-nil result")
		return
	}
	if result.NewVersion != "2.0.0" {
		t.Errorf("NewVersion = %q, want %q", result.NewVersion, "2.0.0")
	}
	if !strings.Contains(result.ReleaseURL, "v2.0.0") {
		t.Errorf("ReleaseURL = %q, want it to contain v2.0.0", result.ReleaseURL)
	}
}

func TestCheckForUpdate_AlreadyUpToDate(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		return &GithubRelease{TagName: "v1.0.0"}, nil
	})

	result := CheckForUpdate(context.Background(), "1.0.0")
	if result != nil {
		t.Errorf("expected nil result for same version, got %+v", result)
	}
}

func TestCheckForUpdate_NetworkError(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		return nil, fmt.Errorf("network error")
	})

	result := CheckForUpdate(context.Background(), "1.0.0")
	if result != nil {
		t.Errorf("expected nil result on error, got %+v", result)
	}
}

func TestCheckForUpdate_Timeout(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return &GithubRelease{TagName: "v2.0.0"}, nil
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result := CheckForUpdate(ctx, "1.0.0")
	if result != nil {
		t.Errorf("expected nil result on timeout, got %+v", result)
	}
}

func TestPrintNotice(t *testing.T) {
	var buf bytes.Buffer
	result := &CheckResult{
		NewVersion: "2.0.0",
		ReleaseURL: "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0",
	}
	PrintNotice(&buf, "1.0.0", result)

	out := buf.String()
	for _, want := range []string{"v1.0.0", "v2.0.0", "redmine update", result.ReleaseURL} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintNotice_Nil(t *testing.T) {
	var buf bytes.Buffer
	PrintNotice(&buf, "1.0.0", nil)
	if buf.Len() != 0 {
		t.Errorf("expected no output for nil result, got %q", buf.String())
	}
}

// --- Cache tests ---

func setCachePath(t *testing.T, dir string) {
	t.Helper()
	orig := cachePath
	cachePath = func() string { return filepath.Join(dir, ".redmine-cli-update-check.json") }
	t.Cleanup(func() { cachePath = orig })
}

func writeTestCache(t *testing.T, c *updateCheckCache) {
	t.Helper()
	writeCache(c)
}

func readTestCache(t *testing.T) *updateCheckCache {
	t.Helper()
	return readCache()
}

func clearTestCache(t *testing.T) {
	t.Helper()
	path := cachePath()
	if path == "" {
		return
	}
	os.Remove(path)
}

func setEmptyCachePath(t *testing.T) {
	t.Helper()
	orig := cachePath
	cachePath = func() string { return "" }
	t.Cleanup(func() { cachePath = orig })
}

// Test 1: positive cache hit
func TestCheckForUpdateCached_PositiveCacheHit(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		t.Fatal("unexpected network call")
		return nil, nil
	})

	setCachePath(t, t.TempDir())
	clearTestCache(t)
	writeTestCache(t, &updateCheckCache{
		CheckedAt:          time.Now(),
		CheckedFromVersion: "1.0.0",
		LatestVersion:      "2.0.0",
		ReleaseURL:         "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0",
		UpdateAvailable:    true,
	})

	result := CheckForUpdateCached(context.Background(), "1.0.0")
	if result == nil {
		t.Fatal("expected cached result")
		return
	}
	if result.NewVersion != "2.0.0" {
		t.Errorf("NewVersion = %q, want %q", result.NewVersion, "2.0.0")
	}
}

// Test 2: stale cache after version mismatch (simulates self-upgrade)
func TestCheckForUpdateCached_VersionMismatch(t *testing.T) {
	var fetchCalled bool
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		fetchCalled = true
		return &GithubRelease{TagName: "v2.0.0"}, nil
	})

	setCachePath(t, t.TempDir())
	clearTestCache(t)
	writeTestCache(t, &updateCheckCache{
		CheckedAt:          time.Now(),
		CheckedFromVersion: "1.0.0",
		LatestVersion:      "2.0.0",
		ReleaseURL:         "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0",
		UpdateAvailable:    true,
	})

	result := CheckForUpdateCached(context.Background(), "2.0.0")
	if result != nil {
		t.Errorf("expected nil result for same version, got %+v", result)
	}
	if !fetchCalled {
		t.Error("expected live fetch to run due to version mismatch")
	}
}

// Test 3: invalid positive cache should fall back to a live check.
func TestCheckForUpdateCached_InvalidPositiveCacheFallsBackToFetch(t *testing.T) {
	var fetchCalls int
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		fetchCalls++
		return &GithubRelease{TagName: "v1.0.0"}, nil
	})

	setCachePath(t, t.TempDir())
	clearTestCache(t)
	writeTestCache(t, &updateCheckCache{
		CheckedAt:          time.Now(),
		CheckedFromVersion: "1.0.0",
		LatestVersion:      "1.0.0",
		ReleaseURL:         "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0",
		UpdateAvailable:    true,
	})

	result := CheckForUpdateCached(context.Background(), "1.0.0")
	if result != nil {
		t.Errorf("expected nil result after refreshing invalid cache, got %+v", result)
	}
	if fetchCalls != 1 {
		t.Errorf("expected 1 live fetch, got %d", fetchCalls)
	}

	cache := readTestCache(t)
	if cache == nil {
		t.Fatal("expected cache to be rewritten")
		return
	}
	if cache.UpdateAvailable {
		t.Errorf("cache should reflect no update available, got update_available=true")
	}
}

// Test 4: negative cache hit
func TestCheckForUpdateCached_NegativeCacheHit(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		t.Fatal("unexpected network call")
		return nil, nil
	})

	setCachePath(t, t.TempDir())
	clearTestCache(t)
	writeTestCache(t, &updateCheckCache{
		CheckedAt:          time.Now(),
		CheckedFromVersion: "1.0.0",
		LatestVersion:      "1.0.0",
		ReleaseURL:         "https://github.com/aarondpn/redmine-cli/releases/tag/v1.0.0",
		UpdateAvailable:    false,
	})

	result := CheckForUpdateCached(context.Background(), "1.0.0")
	if result != nil {
		t.Errorf("expected nil result for negative cache, got %+v", result)
	}
}

// Test 5: expired cache forces live fetch
func TestCheckForUpdateCached_ExpiredCache(t *testing.T) {
	var fetchCalled bool
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		fetchCalled = true
		return &GithubRelease{TagName: "v1.0.0"}, nil
	})

	setCachePath(t, t.TempDir())
	clearTestCache(t)
	writeTestCache(t, &updateCheckCache{
		CheckedAt:          time.Now().Add(-updateCheckCacheAge - time.Hour),
		CheckedFromVersion: "1.0.0",
		LatestVersion:      "1.0.0",
		ReleaseURL:         "https://github.com/aarondpn/redmine-cli/releases/tag/v1.0.0",
		UpdateAvailable:    false,
	})

	result := CheckForUpdateCached(context.Background(), "1.0.0")
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if !fetchCalled {
		t.Error("expected live fetch for expired cache")
	}
}

// Test 6: corrupt cache file falls back to live fetch
func TestCheckForUpdateCached_CorruptCache(t *testing.T) {
	var fetchCalled bool
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		fetchCalled = true
		return &GithubRelease{TagName: "v1.0.0"}, nil
	})

	setCachePath(t, t.TempDir())
	clearTestCache(t)
	if err := os.WriteFile(cachePath(), []byte("not valid json{"), 0o644); err != nil {
		t.Fatalf("write corrupt cache: %v", err)
	}

	result := CheckForUpdateCached(context.Background(), "1.0.0")
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}
	if !fetchCalled {
		t.Error("expected live fetch for corrupt cache")
	}
}

// Test 7: cache write path on positive result
func TestCheckForUpdateCached_CacheWritePositive(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		return &GithubRelease{TagName: "v2.0.0"}, nil
	})

	setCachePath(t, t.TempDir())
	clearTestCache(t)
	result := CheckForUpdateCached(context.Background(), "1.0.0")
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	cache := readTestCache(t)
	if cache == nil {
		t.Fatal("expected cache to be written")
		return
	}
	if cache.CheckedFromVersion != "1.0.0" {
		t.Errorf("CheckedFromVersion = %q, want %q", cache.CheckedFromVersion, "1.0.0")
	}
	if cache.LatestVersion != "2.0.0" {
		t.Errorf("LatestVersion = %q, want %q", cache.LatestVersion, "2.0.0")
	}
	if cache.UpdateAvailable != true {
		t.Errorf("UpdateAvailable = %v, want true", cache.UpdateAvailable)
	}
	if cache.ReleaseURL == "" {
		t.Error("ReleaseURL should be non-empty")
	}
}

// Test 8: cache write path on no-update result
func TestCheckForUpdateCached_CacheWriteNoUpdate(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		return &GithubRelease{TagName: "v1.0.0"}, nil
	})

	setCachePath(t, t.TempDir())
	clearTestCache(t)
	result := CheckForUpdateCached(context.Background(), "1.0.0")
	if result != nil {
		t.Errorf("expected nil result, got %+v", result)
	}

	cache := readTestCache(t)
	if cache == nil {
		t.Fatal("expected cache to be written")
		return
	}
	if cache.UpdateAvailable {
		t.Errorf("UpdateAvailable = %v, want false", cache.UpdateAvailable)
	}
}

// Test 9: network error does NOT write cache
func TestCheckForUpdateCached_NetworkErrorNoCache(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		return nil, fmt.Errorf("network error")
	})

	setCachePath(t, t.TempDir())
	clearTestCache(t)
	result := CheckForUpdateCached(context.Background(), "1.0.0")
	if result != nil {
		t.Errorf("expected nil result on network error, got %+v", result)
	}

	cache := readTestCache(t)
	if cache != nil {
		t.Errorf("expected no cache on network error, got %+v", cache)
	}
}

func TestCheckForUpdateCached_EmptyCachePathDisablesCaching(t *testing.T) {
	var fetchCalls int
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		fetchCalls++
		return &GithubRelease{TagName: "v2.0.0"}, nil
	})

	setEmptyCachePath(t)

	result := CheckForUpdateCached(context.Background(), "1.0.0")
	if result == nil {
		t.Fatal("expected live result")
	}
	if fetchCalls != 1 {
		t.Fatalf("expected exactly one live fetch, got %d", fetchCalls)
	}

	cache := readTestCache(t)
	if cache != nil {
		t.Fatalf("expected caching to be disabled when cache path is empty, got %+v", cache)
	}
}

// Test 10: cache bypass by version mismatch
func TestCheckForUpdateCached_CacheBypassByVersionMismatch(t *testing.T) {
	var fetchCalled bool
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		fetchCalled = true
		return &GithubRelease{TagName: "v2.0.0"}, nil
	})

	setCachePath(t, t.TempDir())
	clearTestCache(t)
	writeTestCache(t, &updateCheckCache{
		CheckedAt:          time.Now(),
		CheckedFromVersion: "1.0.0",
		LatestVersion:      "1.0.0",
		ReleaseURL:         "https://github.com/aarondpn/redmine-cli/releases/tag/v1.0.0",
		UpdateAvailable:    false,
	})

	result := CheckForUpdateCached(context.Background(), "1.1.0")
	if result == nil {
		t.Fatal("expected live result after version mismatch invalidated the cache")
		return
	}
	if result.NewVersion != "2.0.0" {
		t.Errorf("NewVersion = %q, want %q", result.NewVersion, "2.0.0")
	}
	if !fetchCalled {
		t.Error("expected live fetch for version mismatch")
	}
}

func TestCheckForUpdateCached_PositiveCacheHitWithVPrefixVersion(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		t.Fatal("unexpected network call")
		return nil, nil
	})

	setCachePath(t, t.TempDir())
	writeTestCache(t, &updateCheckCache{
		CheckedAt:          time.Now(),
		CheckedFromVersion: "1.0.0",
		LatestVersion:      "2.0.0",
		ReleaseURL:         "https://github.com/aarondpn/redmine-cli/releases/tag/v2.0.0",
		UpdateAvailable:    true,
	})

	result := CheckForUpdateCached(context.Background(), "v1.0.0")
	if result == nil {
		t.Fatal("expected cached result")
		return
	}
	if result.NewVersion != "2.0.0" {
		t.Errorf("NewVersion = %q, want %q", result.NewVersion, "2.0.0")
	}
}

// Regression: CheckForUpdate still works without cache
func TestCheckForUpdate_Regression(t *testing.T) {
	stubFetchRelease(t, func(ctx context.Context) (*GithubRelease, error) {
		return &GithubRelease{TagName: "v2.0.0"}, nil
	})

	result := CheckForUpdate(context.Background(), "1.0.0")
	if result == nil {
		t.Fatal("expected non-nil result")
		return
	}
	if result.NewVersion != "2.0.0" {
		t.Errorf("NewVersion = %q, want %q", result.NewVersion, "2.0.0")
	}
}
