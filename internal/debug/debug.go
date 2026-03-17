package debug

import (
	"fmt"
	"io"
	"net/url"
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

// ScrubURL removes sensitive query parameters (key) from a URL string.
func ScrubURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if q.Has("key") {
		q.Set("key", "***")
		u.RawQuery = q.Encode()
	}
	return u.String()
}
