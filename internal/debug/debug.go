package debug

import (
	"fmt"
	"io"
	"net/url"
	"strings"
)

// Logger writes debug messages to a writer. A nil Logger is safe to use and
// silently discards all output.
type Logger struct {
	w io.Writer
}

// New creates a Logger that writes to w. If w is nil the logger is a no-op.
func New(w io.Writer) *Logger {
	if w == nil {
		return &Logger{}
	}
	return &Logger{w: w}
}

// Printf writes a formatted debug message followed by a newline.
func (l *Logger) Printf(format string, args ...interface{}) {
	if l == nil || l.w == nil {
		return
	}
	fmt.Fprintf(l.w, "[debug] "+format+"\n", args...)
}

// Enabled reports whether debug output is active.
func (l *Logger) Enabled() bool {
	return l != nil && l.w != nil
}

// sensitiveQueryKeys lists query parameters whose values ScrubURL redacts,
// compared case-insensitively. Besides Redmine's own "key" it covers common
// token parameters and the signature parts of presigned object-storage URLs.
var sensitiveQueryKeys = map[string]bool{
	"key":                  true,
	"api_key":              true,
	"apikey":               true,
	"token":                true,
	"access_token":         true,
	"password":             true,
	"secret":               true,
	"signature":            true,
	"x-amz-signature":      true,
	"x-amz-credential":     true,
	"x-amz-security-token": true,
}

// redacted replaces a scrubbed secret. It is "***" percent-encoded, which is
// how url.URL.String renders the redacted userinfo.
const redacted = "%2A%2A%2A"

// ScrubURL redacts credentials from a URL string: userinfo (user:pass@) and
// the values of sensitive query parameters (see sensitiveQueryKeys) are
// replaced with "***". URLs without credentials are returned unchanged. A URL
// that cannot be parsed is still scrubbed, on a best-effort textual basis.
func ScrubURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return scrubUnparsedURL(raw)
	}
	changed := false
	if u.User != nil {
		u.User = url.User("***")
		changed = true
	}
	if q, ok := scrubQuery(u.RawQuery); ok {
		u.RawQuery = q
		changed = true
	}
	if !changed {
		return raw
	}
	return u.String()
}

// scrubQuery redacts the values of sensitive parameters in a raw query
// string, keeping everything else byte-for-byte. It works on the raw text
// because url.ParseQuery silently drops pairs it cannot decode (an invalid
// escape, or ';' used as a separator), which would leave their values in
// place. Both '&' and ';' are treated as separators.
func scrubQuery(rawQuery string) (string, bool) {
	if rawQuery == "" {
		return rawQuery, false
	}
	var b strings.Builder
	changed := false
	for {
		i := strings.IndexAny(rawQuery, "&;")
		pair := rawQuery
		if i >= 0 {
			pair = rawQuery[:i]
		}
		name, _, _ := strings.Cut(pair, "=")
		if isSensitiveQueryKey(name) {
			pair = name + "=" + redacted
			changed = true
		}
		b.WriteString(pair)
		if i < 0 {
			break
		}
		b.WriteByte(rawQuery[i])
		rawQuery = rawQuery[i+1:]
	}
	return b.String(), changed
}

func isSensitiveQueryKey(name string) bool {
	if decoded, err := url.QueryUnescape(name); err == nil {
		name = decoded
	}
	return sensitiveQueryKeys[strings.ToLower(name)]
}

// scrubUnparsedURL is ScrubURL's fallback for strings url.Parse rejects (for
// example an invalid port or host escape): it redacts everything before an
// '@' in the authority, and the sensitive query parameters.
func scrubUnparsedURL(raw string) string {
	s, fragment := raw, ""
	if i := strings.IndexByte(s, '#'); i >= 0 {
		s, fragment = s[:i], s[i:]
	}
	if i := strings.Index(s, "://"); i >= 0 {
		start := i + len("://")
		end := len(s)
		if j := strings.IndexAny(s[start:], "/?"); j >= 0 {
			end = start + j
		}
		if at := strings.LastIndexByte(s[start:end], '@'); at >= 0 {
			s = s[:start] + redacted + s[start+at:]
		}
	}
	if i := strings.IndexByte(s, '?'); i >= 0 {
		if q, ok := scrubQuery(s[i+1:]); ok {
			s = s[:i+1] + q
		}
	}
	return s + fragment
}
