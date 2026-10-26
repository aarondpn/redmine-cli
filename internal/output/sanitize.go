package output

import (
	"strings"
	"sync"

	"github.com/aarondpn/redmine-cli/v2/internal/textsafe"
)

// SanitizeTerminal neutralizes characters in untrusted text (anything that
// came from the Redmine server) that a terminal would interpret instead of
// display. It is applied to every human-readable output path; JSON,
// --jq results and raw response bodies are left untouched. See
// textsafe.Terminal for the rules.
func SanitizeTerminal(s string) string {
	return textsafe.Terminal(s)
}

// SanitizeStyled is SanitizeTerminal for text that already went through a
// renderer such as glamour: SGR sequences (ESC [ params m), which only set
// colors and text attributes, are kept; every other control character or
// escape sequence is neutralized like in SanitizeTerminal. Use it when the
// renderer can turn escaped input back into control characters, as Markdown
// does with character references like "&#x1b;".
func SanitizeStyled(s string) string {
	if textsafe.IsTerminalSafe(s) {
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
