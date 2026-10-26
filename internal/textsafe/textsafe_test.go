package textsafe

import (
	"strings"
	"testing"
)

// The full rule set is also exercised through output.SanitizeTerminal; these
// cases pin the core behavior for packages that call Terminal directly.
func TestTerminal(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain", "Größe ✓ 崩溃 🚀", "Größe ✓ 崩溃 🚀"},
		{"newline and tab kept", "a\n\tb", "a\n\tb"},
		{"osc 52", "x\x1b]52;c;ZXZpbA==\x07y", "x^[]52;c;ZXZpbA==^Gy"},
		{"crlf and stray cr", "a\r\nb\rc", "a\nb^Mc"},
		{"del and nul", "a\x7fb\x00", "a^?b^@"},
		{"c1", "a\u009b2J", `a\u009b2J`},
		{"bidi override", "invoice_\u202Efdp.exe", "invoice_fdp.exe"},
		{"invalid utf-8", "a\xffb", "a�b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Terminal(tt.in)
			if got != tt.want {
				t.Errorf("Terminal(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if strings.ContainsRune(got, 0x1b) || !IsTerminalSafe(got) {
				t.Errorf("Terminal(%q) = %q is not terminal safe", tt.in, got)
			}
			if IsTerminalSafe(tt.in) != (got == tt.in) {
				t.Errorf("IsTerminalSafe(%q) = %v disagrees with Terminal", tt.in, IsTerminalSafe(tt.in))
			}
		})
	}
}
