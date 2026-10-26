package output

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestSanitizeTerminal(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain ascii", "Crash on save", "Crash on save"},
		{"umlauts", "Größenänderung für Übersicht", "Größenänderung für Übersicht"},
		{"cjk", "保存時にクラッシュ 崩溃", "保存時にクラッシュ 崩溃"},
		{"emoji", "Deploy 🚀 👩‍💻 🇩🇪", "Deploy 🚀 👩‍💻 🇩🇪"},
		{"rtl text", "שלום עולם مرحبا", "שלום עולם مرحبا"},
		{"newline and tab kept", "line1\n\tline2", "line1\n\tline2"},
		{"empty", "", ""},
		{"osc 52 clipboard write", "x\x1b]52;c;Y3VybCBldmlsLnNofHNo\x07y", "x^[]52;c;Y3VybCBldmlsLnNofHNo^Gy"},
		{"osc 52 with ST terminator", "\x1b]52;c;ZXZpbA==\x1b\\", "^[]52;c;ZXZpbA==^[\\"},
		{"osc 8 hyperlink", "\x1b]8;;https://evil.example/\x1b\\Click\x1b]8;;\x1b\\", "^[]8;;https://evil.example/^[\\Click^[]8;;^[\\"},
		{"osc 0 window title", "\x1b]0;pwned\x07", "^[]0;pwned^G"},
		{"csi cursor up and erase line", "ok\x1b[1A\x1b[2KFAKE", "ok^[[1A^[[2KFAKE"},
		{"sgr color", "\x1b[31mred\x1b[0m", "^[[31mred^[[0m"},
		{"bel", "ding\x07", "ding^G"},
		{"carriage return overwrite", "rm -rf /\rharmless", "rm -rf /^Mharmless"},
		{"trailing carriage return", "text\r", "text^M"},
		{"crlf line endings", "a\r\nb\r\n", "a\nb\n"},
		{"backspace", "abc\b\b\bxyz", "abc^H^H^Hxyz"},
		{"nul", "a\x00b", "a^@b"},
		{"del", "a\x7fb", "a^?b"},
		{"c1 csi", "a\u009b31mb", `a\u009b31mb`},
		{"c1 osc and st", "\u009d52;c;eA==\u009c", `\u009d52;c;eA==\u009c`},
		{"bidi override", "invoice_\u202Efdp.exe", "invoice_fdp.exe"},
		{"bidi isolates", "a\u2066b\u2067c\u2068d\u2069e", "abcde"},
		{"bidi embeddings and pop", "\u202Aa\u202Bb\u202Cc\u202Dd", "abcd"},
		{"lrm rlm alm", "a\u200Eb\u200Fc\u061Cd", "abcd"},
		{"invalid utf-8", "a\xffb\x9bc", "a�b�c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeTerminal(tt.in)
			if got != tt.want {
				t.Errorf("SanitizeTerminal(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if strings.ContainsRune(got, 0x1b) {
				t.Errorf("SanitizeTerminal(%q) still contains ESC: %q", tt.in, got)
			}
			if again := SanitizeTerminal(got); again != got {
				t.Errorf("SanitizeTerminal is not idempotent: %q -> %q", got, again)
			}
		})
	}
}

// ansiStyle returns a Style that always emits ANSI colors, independent of
// whether the test runs in a terminal.
func ansiStyle() Style {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(2) // termenv.ANSI
	return NewStyle(r.NewStyle().Foreground(lipgloss.Color("6")))
}

func TestStyleRender_SanitizesInput(t *testing.T) {
	out := ansiStyle().Render("Closed\x1b]52;c;ZXZpbA==\x07")
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("expected styled output, got %q", out)
	}
	if strings.Contains(out, "\x1b]") || strings.Contains(out, "\x07") {
		t.Errorf("styled output kept data escape sequences: %q", out)
	}
	if !strings.Contains(out, "Closed^[]52;c;ZXZpbA==^G") {
		t.Errorf("expected neutralized data inside styling, got %q", out)
	}
}

func TestPrinter_HumanOutputIsSanitized(t *testing.T) {
	const evil = "Subject\x1b]52;c;ZXZpbA==\x07\x1b[2K\rFAKE\u202E"
	const want = "Subject^[]52;c;ZXZpbA==^G^[[2K^MFAKE"

	t.Run("table", func(t *testing.T) {
		var out bytes.Buffer
		p := NewStdPrinter(&out, io.Discard, false, true, FormatTable)
		p.Table([]string{"ID", "SUBJECT"}, [][]string{{"1", evil}})
		assertNeutralized(t, out.String(), want)
	})
	t.Run("detail", func(t *testing.T) {
		var out bytes.Buffer
		p := NewStdPrinter(&out, io.Discard, false, true, FormatTable)
		p.Detail([]KeyValue{{Key: "Subject", Value: evil}, {Key: evil, Value: "x"}})
		assertNeutralized(t, out.String(), want)
	})
	for name, emit := range map[string]func(p *StdPrinter){
		"success": func(p *StdPrinter) { p.Success(evil) },
		"error":   func(p *StdPrinter) { p.Error(evil) },
		"warning": func(p *StdPrinter) { p.Warning(evil) },
		"outcome": func(p *StdPrinter) { p.Outcome(false, ActionUpdated, "issue", 1, evil) },
		"action":  func(p *StdPrinter) { p.Action(ActionCreated, "issue", 1, evil) },
	} {
		for _, noColor := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/noColor=%v", name, noColor), func(t *testing.T) {
				var errOut bytes.Buffer
				p := NewStdPrinter(io.Discard, &errOut, true, noColor, FormatTable)
				emit(p)
				assertNeutralized(t, errOut.String(), want)
			})
		}
	}
}

func TestPrinter_JSONIsNotSanitized(t *testing.T) {
	var out bytes.Buffer
	p := NewStdPrinter(&out, io.Discard, false, true, FormatJSON)
	p.JSON(map[string]string{"subject": "a\u202Eb"})
	if !strings.Contains(out.String(), "a\u202Eb") {
		t.Errorf("JSON output was altered: %q", out.String())
	}
}

func TestPrinter_KeepsTrustedStyling(t *testing.T) {
	styled := ansiStyle().Render("#42")
	forged := strings.Replace(styled, "#42", "#43", 1)

	var out bytes.Buffer
	p := NewStdPrinter(&out, io.Discard, true, false, FormatTable)
	p.Detail([]KeyValue{{Key: "ID", Value: styled}, {Key: "Other", Value: forged}})
	got := out.String()
	if !strings.Contains(got, styled) {
		t.Errorf("styled value was altered: %q", got)
	}
	if strings.Contains(got, forged) {
		t.Errorf("escape sequences in an unstyled value were kept: %q", got)
	}
}

func TestSanitizeStyled(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "Größe 保存 🚀", "Größe 保存 🚀"},
		{"sgr kept", "\x1b[38;5;252mtext\x1b[0m", "\x1b[38;5;252mtext\x1b[0m"},
		{"sgr with colon params kept", "\x1b[38:2:1:2:3mx\x1b[m", "\x1b[38:2:1:2:3mx\x1b[m"},
		{"osc 52 inside sgr", "\x1b[38;5;252m\x1b]52;c;ZXZpbA==\x07\x1b[0m", "\x1b[38;5;252m^[]52;c;ZXZpbA==^G\x1b[0m"},
		{"osc 8 hyperlink", "\x1b]8;;https://evil.example/\x1b\\x", "^[]8;;https://evil.example/^[\\x"},
		{"csi erase", "\x1b[2Jx\x1b[1A", "^[[2Jx^[[1A"},
		{"unterminated csi", "x\x1b[31", "x^[[31"},
		{"lone esc", "x\x1b", "x^["},
		{"c1 csi", "\u009b31m", `\u009b31m`},
		{"bidi", "a\u202Eb", "ab"},
		{"carriage return", "a\rb", "a^Mb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeStyled(tt.in); got != tt.want {
				t.Errorf("SanitizeStyled(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func assertNeutralized(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("output %q does not contain neutralized text %q", got, want)
	}
	for _, bad := range []string{"\x1b]", "\x07", "\x1b[2K", "\r", "\u202E"} {
		if strings.Contains(got, bad) {
			t.Errorf("output contains %q: %q", bad, got)
		}
	}
}
