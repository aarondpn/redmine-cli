package output

import (
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"
)

// SanitizeTerminal neutralizes characters in untrusted text (anything that
// came from the Redmine server) that a terminal would interpret instead of
// display. It is applied to every human-readable output path; JSON, CSV,
// --jq results and raw response bodies are left untouched.
//
//   - ESC and the other C0 controls (except \n and \t) and DEL are rendered in
//     caret notation (ESC becomes "^[", BEL "^G", DEL "^?"), so OSC 52
//     clipboard writes, OSC 8 hyperlinks and CSI cursor movement show up as
//     inert text.
//   - A CR that ends a CRLF line ending is dropped; any other CR is shown as
//     "^M" so it cannot return the cursor and overwrite the line.
//   - C1 controls (U+0080-U+009F) are shown as "\u009b"-style escapes.
//   - Bidi embeddings, overrides and isolates (U+202A-U+202E, U+2066-U+2069)
//     and the LRM/RLM/ALM marks are removed.
//   - Invalid UTF-8 bytes become U+FFFD.
//
// Ordinary text, including non-ASCII letters, CJK and emoji, passes through
// unchanged.
func SanitizeTerminal(s string) string {
	if isTerminalSafe(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			b.WriteRune(utf8.RuneError)
		case r == '\n' || r == '\t':
			b.WriteByte(byte(r))
		case r == '\r':
			if i+1 >= len(s) || s[i+1] != '\n' {
				b.WriteString("^M")
			}
		case r < 0x20:
			b.WriteByte('^')
			b.WriteByte(byte(r) + 0x40)
		case r == 0x7f:
			b.WriteString("^?")
		case r >= 0x80 && r <= 0x9f:
			fmt.Fprintf(&b, `\u%04x`, r)
		case isBidiControl(r):
			// dropped
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

// SanitizeStyled is SanitizeTerminal for text that already went through a
// renderer such as glamour: SGR sequences (ESC [ params m), which only set
// colors and text attributes, are kept; every other control character or
// escape sequence is neutralized like in SanitizeTerminal. Use it when the
// renderer can turn escaped input back into control characters, as Markdown
// does with character references like "&#x1b;".
func SanitizeStyled(s string) string {
	if isTerminalSafe(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for len(s) > 0 {
		i := strings.IndexByte(s, 0x1b)
		if i < 0 {
			b.WriteString(SanitizeTerminal(s))
			break
		}
		b.WriteString(SanitizeTerminal(s[:i]))
		if n := sgrLen(s[i:]); n > 0 {
			b.WriteString(s[i : i+n])
			s = s[i+n:]
			continue
		}
		b.WriteString("^[")
		s = s[i+1:]
	}
	return b.String()
}

// sgrLen returns the length of the SGR sequence at the start of s, or 0 if s
// does not start with one.
func sgrLen(s string) int {
	if len(s) < 3 || s[0] != 0x1b || s[1] != '[' {
		return 0
	}
	for i := 2; i < len(s); i++ {
		switch c := s[i]; {
		case c == 'm':
			return i + 1
		case (c >= '0' && c <= '9') || c == ';' || c == ':':
		default:
			return 0
		}
	}
	return 0
}

// isTerminalSafe reports whether SanitizeTerminal would return s unchanged.
func isTerminalSafe(s string) bool {
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			if (c < 0x20 && c != '\n' && c != '\t') || c == 0x7f {
				return false
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if (r == utf8.RuneError && size == 1) || (r >= 0x80 && r <= 0x9f) || isBidiControl(r) {
			return false
		}
		i += size
	}
	return true
}

// isBidiControl reports whether r is an invisible bidirectional formatting
// character that can reorder how surrounding text is displayed.
func isBidiControl(r rune) bool {
	switch {
	case r >= 0x202a && r <= 0x202e: // LRE, RLE, PDF, LRO, RLO
		return true
	case r >= 0x2066 && r <= 0x2069: // LRI, RLI, FSI, PDI
		return true
	case r == 0x200e || r == 0x200f || r == 0x061c: // LRM, RLM, ALM
		return true
	}
	return false
}

// styledOutputs remembers every string produced by Style.Render that carries
// escape sequences. Those strings were built from sanitized input by trusted
// styling code, so table cells and detail values equal to one of them are
// passed through verbatim; any other value is sanitized.
var styledOutputs sync.Map

func rememberStyled(s string) {
	if strings.IndexByte(s, 0x1b) >= 0 {
		styledOutputs.Store(s, struct{}{})
	}
}

// sanitizeValue sanitizes a table cell or detail value, keeping the styling
// a caller applied through Style.Render.
func sanitizeValue(s string) string {
	if _, ok := styledOutputs.Load(s); ok {
		return s
	}
	return SanitizeTerminal(s)
}
