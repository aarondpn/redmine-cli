package update

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// CheckResult holds the outcome of a background update check.
type CheckResult struct {
	NewVersion string // e.g. "1.3.0" (without "v" prefix)
	ReleaseURL string // full GitHub release page URL
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

// PrintNotice writes an update notice to w.
func PrintNotice(w io.Writer, currentVersion string, result *CheckResult) {
	if result == nil {
		return
	}
	current := strings.TrimPrefix(currentVersion, "v")
	fmt.Fprintf(w, "\nA new version of redmine is available: v%s → v%s\n%s\nRun \"redmine update\" to upgrade\n",
		current, result.NewVersion, result.ReleaseURL)
}
