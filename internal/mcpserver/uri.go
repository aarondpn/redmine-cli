package mcpserver

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const redmineScheme = "redmine"

// URI template constants used by both registration and parsers.
const (
	tmplIssue     = "redmine://issue/{id}"
	tmplProject   = "redmine://project/{identifier}"
	tmplUser      = "redmine://user/{id}"
	tmplTimeEntry = "redmine://time-entry/{id}"
	tmplWiki      = "redmine://wiki/{project}/{page}"
	tmplVersion   = "redmine://version/{id}"
)

// parseRedmineURI splits a redmine://kind/... URI into its kind and the path
// segments following it. Segments are URL-decoded.
func parseRedmineURI(raw string) (kind string, parts []string, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", nil, fmt.Errorf("invalid URI: %w", err)
	}
	if u.Scheme != redmineScheme {
		return "", nil, fmt.Errorf("unexpected scheme %q (want %q)", u.Scheme, redmineScheme)
	}
	kind = u.Host
	if kind == "" {
		return "", nil, fmt.Errorf("URI missing resource kind")
	}

	path := strings.TrimPrefix(u.Path, "/")
	if path == "" {
		return kind, nil, nil
	}
	raws := strings.Split(path, "/")
	parts = make([]string, len(raws))
	for i, p := range raws {
		dec, err := url.PathUnescape(p)
		if err != nil {
			return "", nil, fmt.Errorf("invalid URI path segment %q: %w", p, err)
		}
		parts[i] = dec
	}
	return kind, parts, nil
}

// parseIntID returns the integer value of segment or an error.
func parseIntID(segment string) (int, error) {
	id, err := strconv.Atoi(segment)
	if err != nil {
		return 0, fmt.Errorf("%q is not a valid numeric ID", segment)
	}
	if id <= 0 {
		return 0, fmt.Errorf("%q is not a positive ID", segment)
	}
	return id, nil
}

// expectSingleSegment ensures exactly one path segment is present.
func expectSingleSegment(parts []string, kind string) (string, error) {
	if len(parts) != 1 || parts[0] == "" {
		return "", fmt.Errorf("%s URI must have exactly one path segment", kind)
	}
	return parts[0], nil
}
