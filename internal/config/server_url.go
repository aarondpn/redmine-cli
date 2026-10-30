package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var schemePrefix = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

// redmineRoutes are the first path segments of Redmine's own routes. A pasted
// browser URL is cut at the first of them, so a sub-path install prefix
// (https://host/redmine/issues/5 -> https://host/redmine) is preserved.
var redmineRoutes = map[string]bool{
	"activity": true, "admin": true, "attachments": true, "boards": true,
	"custom_fields": true, "documents": true, "enumerations": true,
	"files": true, "groups": true, "issue_categories": true,
	"issue_statuses": true, "issues": true, "journals": true, "login": true,
	"logout": true, "my": true, "news": true, "projects": true, "queries": true,
	"repositories": true, "roles": true, "search": true, "settings": true,
	"time_entries": true, "trackers": true, "users": true, "versions": true,
	"watchers": true, "wiki": true, "workflows": true,
}

// EnsureScheme prepends https:// to a server value that names no scheme
// (redmine.example.com, localhost:3000). Anything else is returned unchanged.
func EnsureScheme(server string) string {
	s := strings.TrimSpace(server)
	if s == "" || schemePrefix.MatchString(s) {
		return server
	}
	return "https://" + s
}

// NormalizeServerURL turns what a user typed or pasted into a Redmine base
// URL: whitespace is trimmed, https:// is added when no scheme is given
// (an explicit http:// is kept), query, fragment and trailing slashes are
// dropped, and a trailing Redmine route copied from the browser such as
// /projects/foo/issues or /my/account is removed. A sub-path prefix the
// instance is installed under is preserved.
func NormalizeServerURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("server URL is required")
	}
	s = EnsureScheme(s)

	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("invalid server URL %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid server URL %q: scheme must be http or https", raw)
	}
	if u.Hostname() == "" {
		return "", fmt.Errorf("invalid server URL %q: no host", raw)
	}

	u.Scheme = strings.ToLower(u.Scheme)
	u.RawQuery, u.Fragment, u.RawFragment, u.ForceQuery = "", "", "", false

	var kept []string
	for _, seg := range strings.Split(u.Path, "/") {
		if seg == "" {
			continue
		}
		if redmineRoutes[strings.ToLower(seg)] {
			break
		}
		kept = append(kept, seg)
	}
	u.Path, u.RawPath = "", ""
	if len(kept) > 0 {
		u.Path = "/" + strings.Join(kept, "/")
	}
	return u.String(), nil
}
