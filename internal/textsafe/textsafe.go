// Package textsafe neutralizes untrusted text before it reaches a terminal.
// It has no dependencies, so low-level packages (the API client, the debug
// logger) apply the same rules as the output layer.
package textsafe

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Terminal neutralizes characters in untrusted text (anything that came from
// the Redmine server) that a terminal would interpret instead of display.
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
// unchanged. The result is terminal safe, so applying Terminal again is a
// no-op.
func Terminal(s string) string {
	if IsTerminalSafe(s) {
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

// IsTerminalSafe reports whether Terminal would return s unchanged.
func IsTerminalSafe(s string) bool {
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
