package cmdutil

import (
	"strings"
	"time"
)

// ResolveDateKeyword expands the `today` keyword to today's date in the
// YYYY-MM-DD format the Redmine API expects. Any other input (including the
// empty string and a plain ISO date) is returned unchanged so callers can
// keep their existing defaulting logic.
func ResolveDateKeyword(input string) string {
	return resolveDateKeywordAt(input, time.Now())
}

func resolveDateKeywordAt(input string, now time.Time) string {
	if strings.EqualFold(strings.TrimSpace(input), "today") {
		return now.Format("2006-01-02")
	}
	return input
}
