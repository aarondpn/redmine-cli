package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

// deprecationNow is the clock used by TestDeprecatedFlagsNotExpired.
var deprecationNow = time.Now

// markerLocation is a "DEPRECATED(remove-after: ...)" comment in the sources.
type markerLocation struct {
	File string
	Line int
	Date string
	Text string
}

// scanDeprecationMarkers finds every marker comment in non-test Go files below
// root.
func scanDeprecationMarkers(t *testing.T, root string) []markerLocation {
	t.Helper()
	var found []markerLocation
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "// "+cmdutil.DeprecationMarker) {
				continue
			}
			rest := strings.TrimPrefix(trimmed, "// "+cmdutil.DeprecationMarker)
			date, _, _ := strings.Cut(rest, ")")
			found = append(found, markerLocation{File: path, Line: i + 1, Date: date, Text: trimmed})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// deprecationProblems reports every violation of the deprecation convention:
// missing or malformed dates, missing marker comments and flags that are past
// their removal date.
func deprecationProblems(flags []cmdutil.DeprecatedFlag, markers []markerLocation, now time.Time) []string {
	var problems []string
	for _, fl := range flags {
		removeAfter, err := time.Parse(cmdutil.DeprecationDateLayout, fl.RemoveAfter)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s --%s: invalid removeAfter date %q", fl.Command, fl.Name, fl.RemoveAfter))
			continue
		}
		var locs []string
		for _, m := range markers {
			if m.Date == fl.RemoveAfter && strings.Contains(m.Text, "--"+fl.Name) {
				locs = append(locs, fmt.Sprintf("%s:%d", m.File, m.Line))
			}
		}
		if len(locs) == 0 {
			problems = append(problems, fmt.Sprintf("%s --%s: no `// %s%s): ... --%s ...` marker comment found at its call site", fl.Command, fl.Name, cmdutil.DeprecationMarker, fl.RemoveAfter, fl.Name))
			continue
		}
		// The flag stays valid through removeAfter itself.
		if !now.Before(removeAfter.AddDate(0, 0, 1)) {
			problems = append(problems, fmt.Sprintf("%s --%s expired on %s: delete the flag, the code reading it and its tests (see %s; also grep the repo for %s%s) to find tests)", fl.Command, fl.Name, fl.RemoveAfter, strings.Join(locs, ", "), cmdutil.DeprecationMarker, fl.RemoveAfter))
		}
	}
	return problems
}

func TestDeprecatedFlagsNotExpired(t *testing.T) {
	root := NewRootCmd("test")
	flags := cmdutil.DeprecatedFlags(root)
	if len(flags) == 0 {
		t.Fatal("expected at least one deprecated flag in the command tree")
	}
	markers := scanDeprecationMarkers(t, filepath.Join("..", "..", "internal"))
	if problems := deprecationProblems(flags, markers, deprecationNow()); len(problems) > 0 {
		t.Fatalf("deprecated flags need attention:\n  %s", strings.Join(problems, "\n  "))
	}
}

func TestDeprecationProblems(t *testing.T) {
	flag := cmdutil.DeprecatedFlag{Command: "redmine groups get", Name: "include-users", RemoveAfter: "2027-01-31"}
	marker := markerLocation{
		File: "internal/cmd/group/get.go", Line: 90, Date: "2027-01-31",
		Text: "// DEPRECATED(remove-after: 2027-01-31): --include-users alias",
	}
	day := func(s string) time.Time {
		tm, err := time.Parse(cmdutil.DeprecationDateLayout, s)
		if err != nil {
			t.Fatal(err)
		}
		return tm
	}

	t.Run("valid before and on the removal date", func(t *testing.T) {
		for _, now := range []string{"2026-10-30", "2027-01-31"} {
			if got := deprecationProblems([]cmdutil.DeprecatedFlag{flag}, []markerLocation{marker}, day(now)); len(got) != 0 {
				t.Errorf("now=%s: unexpected problems: %v", now, got)
			}
		}
	})

	t.Run("expired names command flag and location", func(t *testing.T) {
		got := deprecationProblems([]cmdutil.DeprecatedFlag{flag}, []markerLocation{marker}, day("2027-02-01"))
		if len(got) != 1 {
			t.Fatalf("problems = %v", got)
		}
		for _, want := range []string{"redmine groups get", "--include-users", "2027-01-31", "internal/cmd/group/get.go:90"} {
			if !strings.Contains(got[0], want) {
				t.Errorf("message %q missing %q", got[0], want)
			}
		}
	})

	t.Run("missing marker", func(t *testing.T) {
		got := deprecationProblems([]cmdutil.DeprecatedFlag{flag}, nil, day("2026-10-30"))
		if len(got) != 1 || !strings.Contains(got[0], "marker") {
			t.Errorf("problems = %v", got)
		}
	})

	t.Run("invalid date", func(t *testing.T) {
		bad := flag
		bad.RemoveAfter = ""
		got := deprecationProblems([]cmdutil.DeprecatedFlag{bad}, []markerLocation{marker}, day("2026-10-30"))
		if len(got) != 1 || !strings.Contains(got[0], "invalid removeAfter") {
			t.Errorf("problems = %v", got)
		}
	})
}
