package api

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

// InvalidPathSegmentError is returned when a caller-supplied value cannot be
// used safely as a single URL path segment. It is returned before any request
// is sent.
type InvalidPathSegmentError struct {
	Kind   string // human-readable name of the value, e.g. "project identifier"
	Value  string
	Reason string
}

func (e *InvalidPathSegmentError) Error() string {
	return fmt.Sprintf("invalid %s %q: %s", e.Kind, e.Value, e.Reason)
}

// pathSegment validates s as an opaque identifier and returns it escaped for
// use as one URL path segment. It rejects empty values, dot segments and any
// character that could split, terminate or re-encode the path ('/', '\', '?',
// '#', '%' and control characters), so a value like "../users/2" or
// "%2e%2e" can never address a different endpoint, even behind a proxy that
// normalizes or decodes paths. Redmine project identifiers ([a-z0-9_-]) and
// numeric IDs always pass.
func pathSegment(kind, s string) (string, error) {
	if s == "" {
		return "", &InvalidPathSegmentError{Kind: kind, Value: s, Reason: "must not be empty"}
	}
	if s == "." || s == ".." {
		return "", &InvalidPathSegmentError{Kind: kind, Value: s, Reason: "must not be a dot segment"}
	}
	if i := strings.IndexAny(s, `/\?#%`); i >= 0 {
		return "", &InvalidPathSegmentError{Kind: kind, Value: s, Reason: fmt.Sprintf("must not contain %q", s[i])}
	}
	if strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return "", &InvalidPathSegmentError{Kind: kind, Value: s, Reason: "must not contain control characters"}
	}
	return url.PathEscape(s), nil
}

// projectSegment validates and escapes a project identifier or numeric ID.
func projectSegment(identifier string) (string, error) {
	return pathSegment("project identifier", identifier)
}

// wikiPageSegment escapes a wiki page title for use as one URL path segment.
// url.PathEscape already encodes '/', '\', '?', '#', '%' and control
// characters, so other titles keep their existing behaviour. It rejects empty
// titles and any title with a "." or ".." component between '/' or '\': a
// proxy that decodes %2F before normalizing the path would otherwise turn
// "x/../../../../users/2" into /users/2.json. Redmine strips '.', '/' and '\'
// from titles anyway, so no reachable page needs such a title.
func wikiPageSegment(page string) (string, error) {
	if page == "" {
		return "", &InvalidPathSegmentError{Kind: "wiki page", Value: page, Reason: "must not be empty"}
	}
	for _, part := range strings.FieldsFunc(page, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == "." || part == ".." {
			return "", &InvalidPathSegmentError{Kind: "wiki page", Value: page, Reason: "must not contain a dot segment"}
		}
	}
	return url.PathEscape(page), nil
}
