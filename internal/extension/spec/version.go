package spec

import (
	"regexp"
	"strconv"
)

var semverRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)([-+].*)?$`)

// parseVersion parses "1.2.3", "v1.2.3" and suffixed forms such as
// "v1.2.3-4-gabcdef-dirty" (git describe). Pre-release suffixes are ignored:
// min_cli_version is a coarse compatibility gate, not a release ordering.
func parseVersion(v string) ([3]int, bool) {
	m := semverRe.FindStringSubmatch(v)
	if m == nil {
		return [3]int{}, false
	}
	var out [3]int
	for i := range out {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

func compareVersions(a, b [3]int) int {
	for i := range a {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	return 0
}
