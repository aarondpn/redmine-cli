package extension

import (
	"io"
	"strings"
	"unicode/utf8"

	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// maxHeldBack bounds how much of an unfinished escape sequence or UTF-8
// character terminalWriter keeps for the next write.
const maxHeldBack = 64

// terminalWriter streams an extension's stderr to the user's terminal with
// control sequences neutralized: an extension may relay Redmine data
// (subjects, journal text, a nested `redmine api` error body), and that
// must not be able to set the window title, write the clipboard (OSC 52),
// add hyperlinks or move the cursor. Colors (SGR) are kept, as is a
// carriage return, which progress output uses to redraw its line; both
// only change how the current line looks.
//
// A write may end in the middle of an escape sequence or a UTF-8
// character; that tail is held back until the next write or Flush, so
// splitting the stream does not garble it.
type terminalWriter struct {
	w       io.Writer
	pending []byte
}

func (t *terminalWriter) Write(p []byte) (int, error) {
	t.pending = append(t.pending, p...)
	if cut := safeCut(t.pending); cut > 0 {
		t.emit(t.pending[:cut])
		t.pending = append(t.pending[:0], t.pending[cut:]...)
	}
	return len(p), nil
}

// Flush writes what is held back. Call it once the stream has ended.
func (t *terminalWriter) Flush() {
	if len(t.pending) > 0 {
		t.emit(t.pending)
		t.pending = nil
	}
}

func (t *terminalWriter) emit(b []byte) {
	parts := strings.Split(string(b), "\r")
	for i, part := range parts {
		parts[i] = output.SanitizeStyled(part)
	}
	// Write errors (a closed terminal) must not fail the run; the runner
	// ignores them too.
	_, _ = io.WriteString(t.w, strings.Join(parts, "\r"))
}

// safeCut returns how much of b can be sanitized now: all of it, except a
// trailing escape sequence that may still become a color (SGR) sequence
// and a trailing incomplete UTF-8 character, as long as that tail is
// shorter than maxHeldBack.
func safeCut(b []byte) int {
	cut := len(b)
	if i := lastIndexByte(b, 0x1b, maxHeldBack); i >= 0 && maybeSGRPrefix(b[i:]) {
		cut = i
	}
	for start := cut - 1; start >= 0 && start >= cut-utf8.UTFMax; start-- {
		if utf8.RuneStart(b[start]) {
			if !utf8.FullRune(b[start:cut]) {
				cut = start
			}
			break
		}
	}
	return cut
}

// lastIndexByte is bytes.LastIndexByte limited to the last n bytes.
func lastIndexByte(b []byte, c byte, n int) int {
	for i := len(b) - 1; i >= 0 && i >= len(b)-n; i-- {
		if b[i] == c {
			return i
		}
	}
	return -1
}

// maybeSGRPrefix reports whether s (starting with ESC) is an unfinished
// SGR sequence: ESC, ESC [ or ESC [ followed by parameters only.
func maybeSGRPrefix(s []byte) bool {
	if len(s) == 1 {
		return true
	}
	if s[1] != '[' {
		return false
	}
	for _, c := range s[2:] {
		if (c < '0' || c > '9') && c != ';' && c != ':' {
			return false
		}
	}
	return true
}
