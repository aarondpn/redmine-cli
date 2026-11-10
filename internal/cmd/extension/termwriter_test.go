package extension

import (
	"bytes"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTerminalWriter(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "warning: slow\n", "warning: slow\n"},
		{"osc 52 clipboard", "x\x1b]52;c;ZXZpbA==\x07y\n", "x^[]52;c;ZXZpbA==^Gy\n"},
		{"osc 8 hyperlink", "\x1b]8;;https://evil\x1b\\link\x1b]8;;\x1b\\\n", "^[]8;;https://evil^[\\link^[]8;;^[\\\n"},
		{"cursor movement", "a\x1b[2Ab\x1b[2J\n", "a^[[2Ab^[[2J\n"},
		{"colors kept", "\x1b[31merror\x1b[0m\n", "\x1b[31merror\x1b[0m\n"},
		{"progress carriage return kept", "10%\r50%\r100%\n", "10%\r50%\r100%\n"},
		{"c1 and bidi", "a\u009b2Jb\u202ec\n", "a\\u009b2Jbc\n"},
		{"unicode", "Größe ✓ 崩溃 🚀\n", "Größe ✓ 崩溃 🚀\n"},
		{"no final newline", "tail\x1b", "tail^["},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Every way of splitting the stream in two must give the same
			// result as one write: escape sequences and UTF-8 characters
			// cut by a write boundary must survive.
			for split := 0; split <= len(tt.in); split++ {
				var out bytes.Buffer
				w := &terminalWriter{w: &out}
				if _, err := w.Write([]byte(tt.in[:split])); err != nil {
					t.Fatal(err)
				}
				if _, err := w.Write([]byte(tt.in[split:])); err != nil {
					t.Fatal(err)
				}
				w.Flush()
				if out.String() != tt.want {
					t.Fatalf("split at %d: got %q, want %q", split, out.String(), tt.want)
				}
			}
		})
	}
}

func TestTerminalWriterBoundsHeldBack(t *testing.T) {
	var out bytes.Buffer
	w := &terminalWriter{w: &out}
	// An endless "SGR" never ends; it must not be buffered forever.
	_, _ = w.Write([]byte("\x1b[" + strings.Repeat("1;", 100)))
	if out.Len() == 0 {
		t.Fatal("an unterminated escape sequence was held back without limit")
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Fatalf("raw ESC written: %q", out.String())
	}
}

// TestDeclarativeStderrSanitizedAtTerminal runs a command whose stderr
// carries an OSC 52 clipboard write: at a terminal it is neutralized, when
// redirected the bytes pass unchanged.
func TestDeclarativeStderrSanitizedAtTerminal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fixture extension is a shell script")
	}
	for _, tty := range []bool{true, false} {
		te := newTestEnv(t)
		copyFixture(t, filepath.Join(te.extRoot, "redmine-cli-fixture"))
		setStderrTerminal(t, tty)
		t.Setenv("FIXTURE_STDERR", "evil \x1b]52;c;ZXZpbA==\x07 \x1b[31mred\x1b[0m")
		if err := te.run("extension", "exec", "fixture", "show", "1", "-o", "json"); err != nil {
			t.Fatalf("%v\n%s", err, te.stderr)
		}
		got := te.stderr.String()
		if tty {
			if strings.Contains(got, "\x1b]") || !strings.Contains(got, "evil ^[]52;c;ZXZpbA==^G \x1b[31mred\x1b[0m") {
				t.Errorf("terminal stderr = %q", got)
			}
		} else if !strings.Contains(got, "evil \x1b]52;c;ZXZpbA==\x07") {
			t.Errorf("redirected stderr = %q, want the bytes unchanged", got)
		}
	}
}
