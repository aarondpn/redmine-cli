package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/term"
)

// CheckResult holds the outcome of a background update check.
type CheckResult struct {
	NewVersion string // e.g. "1.3.0" (without "v" prefix)
	ReleaseURL string // full GitHub release page URL
}

// updateCheckCache is the on-disk cache for update check results.
// Version-aware: the cache is invalidated when the binary version changes
// (e.g. after running `redmine update`).
type updateCheckCache struct {
	CheckedAt          time.Time `json:"checked_at"`
	CheckedFromVersion string    `json:"checked_from_version"` // normalized current version the check was run from
	LatestVersion      string    `json:"latest_version"`       // normalized latest version per GitHub
	ReleaseURL         string    `json:"release_url"`
	UpdateAvailable    bool      `json:"update_available"` // true if LatestVersion is newer than CheckedFromVersion
}

// isTerminal reports whether stderr is a terminal. It is a variable so tests
// can override it.
var isTerminal = func() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}

// ShouldCheck returns true if the background update check should run.
func ShouldCheck(version string, args []string) bool {
	if version == "dev" {
		return false
	}
	if v := os.Getenv("REDMINE_NO_UPDATE_CHECK"); v == "1" || strings.EqualFold(v, "true") {
		return false
	}
	if !isTerminal() {
		return false
	}
	if hasSubcommand(args, "update") {
		return false
	}
	return true
}

// hasSubcommand returns true if name appears as a non-flag argument in args.
// This handles invocations like "redmine --verbose update" where flags
// precede the subcommand.
func hasSubcommand(args []string, name string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		if a == name {
			return true
		}
	}
	return false
}

// CheckForUpdate checks GitHub for a newer release. It returns nil on any
// error or if the current version is already up to date.
func CheckForUpdate(ctx context.Context, currentVersion string) *CheckResult {
	release, err := fetchRelease(ctx)
	if err != nil {
		return nil
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentClean := strings.TrimPrefix(currentVersion, "v")

	if !IsNewer(latestVersion, currentClean) {
		return nil
	}

	return &CheckResult{
		NewVersion: latestVersion,
		ReleaseURL: fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", RepoOwner, RepoName, release.TagName),
	}
}

// releasePagePrefix is the only URL prefix an update notice may link to.
const releasePagePrefix = "https://github.com/" + RepoOwner + "/" + RepoName + "/releases/"

// releaseTagPattern matches the characters a release tag may contain: a
// letter or digit, then letters, digits, '.', '+' and '-'. That rules out
// whitespace, control characters, shell metacharacters and path separators.
// Callers anchor it.
const releaseTagPattern = `[0-9A-Za-z][0-9A-Za-z.+-]*`

var (
	// safeVersionPattern matches a normalized release version such as
	// "2.14.1" or "3.0.0-rc.1".
	safeVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.+-]*)?$`)
	// safeReleaseURLPattern only accepts a single release tag page. It also
	// rules out whitespace and control characters (terminal escape
	// sequences), and dot segments ("releases/../../other/repo") that a
	// browser would resolve outside this repository's releases.
	safeReleaseURLPattern = regexp.MustCompile(`^` + regexp.QuoteMeta(releasePagePrefix) + `tag/` + releaseTagPattern + `$`)
)

// validNotice reports whether a version and release URL are safe to print.
func validNotice(version, releaseURL string) bool {
	return safeVersionPattern.MatchString(version) && safeReleaseURLPattern.MatchString(releaseURL)
}

// PrintNotice writes an update notice to w. Results whose version or release
// URL do not look like a release of this repository are not printed.
func PrintNotice(w io.Writer, currentVersion string, result *CheckResult) {
	if result == nil || !validNotice(result.NewVersion, result.ReleaseURL) {
		return
	}
	current := strings.TrimPrefix(currentVersion, "v")
	fmt.Fprintf(w, "\nA new version of redmine is available: v%s → v%s\n%s\nRun \"redmine update\" to upgrade\n",
		current, result.NewVersion, result.ReleaseURL)
}

// cachePath returns the path to the update check cache file.
// It is a variable so tests can override it.
var cachePath = defaultCachePath

func defaultCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".redmine-cli-update-check.json")
}

const updateCheckCacheAge = 24 * time.Hour

func normalizeVersion(version string) string {
	return strings.TrimPrefix(version, "v")
}

// maxCacheBytes bounds the cache file read; a valid entry is a few hundred
// bytes.
const maxCacheBytes = 64 << 10

// maxCacheClockSkew tolerates small clock differences when rejecting cache
// entries timestamped in the future.
const maxCacheClockSkew = 5 * time.Minute

// readCache loads the update check cache from disk.
// Returns nil if the file does not exist, is corrupt, or fails validation.
// The file lives in the home directory and may have been written by someone
// else, so it is treated as untrusted input.
func readCache() *updateCheckCache {
	path := cachePath()
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxCacheBytes+1))
	if err != nil || len(data) > maxCacheBytes {
		return nil
	}

	var cache updateCheckCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil
	}
	if !cache.valid(time.Now()) {
		return nil
	}

	return &cache
}

// valid reports whether a decoded cache entry is well-formed: a plausible
// timestamp (a future one would pin the entry forever), the versions it
// compares, and a release URL pointing at this repository's releases.
func (c *updateCheckCache) valid(now time.Time) bool {
	if c.CheckedAt.IsZero() || c.CheckedAt.After(now.Add(maxCacheClockSkew)) {
		return false
	}
	if c.CheckedFromVersion == "" {
		return false
	}
	return validNotice(c.LatestVersion, c.ReleaseURL)
}

// writeCache saves the update check cache to disk, readable only by the
// current user. Errors are silently ignored.
//
// The entry is written to a temp file (created 0600) in the same directory
// and renamed into place, so concurrent invocations never read a truncated
// file and a cache created 0644 by an older version, or a symlink planted at
// the path, is replaced rather than written through.
func writeCache(cache *updateCheckCache) {
	path := cachePath()
	if path == "" {
		return
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return
	}

	tmp, err := os.CreateTemp(dir, ".redmine-cli-update-check-*.tmp")
	if err != nil {
		return
	}
	tmpPath := tmp.Name()
	_, err = tmp.Write(data)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmpPath, path)
	}
	if err != nil {
		_ = os.Remove(tmpPath)
	}
}

// CheckForUpdateCached checks GitHub for a newer release, caching the result
// for 24 hours. Cache is version-aware: entries from a different binary
// version are ignored.
//
// Failure semantics (Option B): Only successful checks are cached.
// A "successful check" means the GitHub API responded. The result may indicate
// no update is available (update_available=false) or an update is available
// (update_available=true, with version fields). Network errors and timeouts
// are NOT cached, so a retry happens on the next invocation.
//
// Cached positive results are defensively revalidated with IsNewer before
// being returned, ensuring stale cache entries (e.g. after a GitHub release
// rollback) do not surface a false upgrade notice.
func CheckForUpdateCached(ctx context.Context, currentVersion string) *CheckResult {
	currentClean := normalizeVersion(currentVersion)

	// Try version-aware cache first.
	if cache := readCache(); cache != nil {
		// Invalidate if cache is expired.
		if time.Since(cache.CheckedAt) > updateCheckCacheAge {
			cache = nil
		}
		// Invalidate if binary version changed (e.g. after running `redmine update`).
		if cache != nil && cache.CheckedFromVersion != currentClean {
			cache = nil
		}
		// If we have a positive cached result, defensively revalidate.
		if cache != nil && cache.UpdateAvailable {
			if !IsNewer(cache.LatestVersion, currentClean) {
				// Cache is stale (e.g. version was bumped, then rolled back).
				// Ignore it and perform a fresh check.
				cache = nil
			} else {
				return &CheckResult{
					NewVersion: cache.LatestVersion,
					ReleaseURL: cache.ReleaseURL,
				}
			}
		}
		if cache != nil {
			// Valid negative cache (checked successfully, no update available).
			return nil
		}
	}

	// Perform a live check.
	release, err := fetchRelease(ctx)
	if err != nil {
		// Network error or timeout: do not cache, return nil.
		return nil
	}

	latestVersion := normalizeVersion(release.TagName)
	updateAvailable := IsNewer(latestVersion, currentClean)
	releaseURL := fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", RepoOwner, RepoName, release.TagName)
	if !validNotice(latestVersion, releaseURL) {
		// Unexpected tag format: neither cache nor print it.
		return nil
	}

	// Persist the cache entry (even when no update available).
	writeCache(&updateCheckCache{
		CheckedAt:          time.Now(),
		CheckedFromVersion: currentClean,
		LatestVersion:      latestVersion,
		ReleaseURL:         releaseURL,
		UpdateAvailable:    updateAvailable,
	})

	if !updateAvailable {
		return nil
	}

	return &CheckResult{
		NewVersion: latestVersion,
		ReleaseURL: releaseURL,
	}
}
