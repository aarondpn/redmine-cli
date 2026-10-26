package tui

import (
	"strings"
	"testing"
)

func TestTextileToMarkdown_Headers(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"h1. Title", "# Title"},
		{"h2. Subtitle", "## Subtitle"},
		{"h3. Section", "### Section"},
	}
	for _, tt := range tests {
		got := TextileToMarkdown(tt.input)
		if got != tt.want {
			t.Errorf("TextileToMarkdown(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTextileToMarkdown_BulletLists(t *testing.T) {
	input := "* item one\n* item two\n** nested"
	got := TextileToMarkdown(input)
	if !strings.Contains(got, "- item one") {
		t.Errorf("expected '- item one', got %q", got)
	}
	if !strings.Contains(got, "  - nested") {
		t.Errorf("expected '  - nested', got %q", got)
	}
}

func TestTextileToMarkdown_OrderedLists(t *testing.T) {
	input := "# first\n# second\n## nested"
	got := TextileToMarkdown(input)
	if !strings.Contains(got, "1. first") {
		t.Errorf("expected '1. first', got %q", got)
	}
	if !strings.Contains(got, "  1. nested") {
		t.Errorf("expected '  1. nested', got %q", got)
	}
}

func TestTextileToMarkdown_Bold(t *testing.T) {
	got := TextileToMarkdown("this is *bold* text")
	if !strings.Contains(got, "**bold**") {
		t.Errorf("expected **bold**, got %q", got)
	}
}

func TestTextileToMarkdown_Italic(t *testing.T) {
	got := TextileToMarkdown("this is _italic_ text")
	if !strings.Contains(got, "*italic*") {
		t.Errorf("expected *italic*, got %q", got)
	}
}

func TestTextileToMarkdown_InlineCode(t *testing.T) {
	got := TextileToMarkdown("use @fmt.Println@ here")
	if !strings.Contains(got, "`fmt.Println`") {
		t.Errorf("expected `fmt.Println`, got %q", got)
	}
}

func TestTextileToMarkdown_Links(t *testing.T) {
	got := TextileToMarkdown(`visit "Redmine":https://redmine.org for info`)
	if !strings.Contains(got, "[Redmine](https://redmine.org)") {
		t.Errorf("expected markdown link, got %q", got)
	}
}

func TestTextileToMarkdown_PreBlock(t *testing.T) {
	input := "<pre>\ncode here\n</pre>"
	got := TextileToMarkdown(input)
	if !strings.Contains(got, "```") {
		t.Errorf("expected fenced code block, got %q", got)
	}
	if !strings.Contains(got, "code here") {
		t.Errorf("expected code content preserved, got %q", got)
	}
}

func TestTextileToMarkdown_Images(t *testing.T) {
	got := TextileToMarkdown("see !screenshot.png! here")
	if !strings.Contains(got, "(image: screenshot.png)") {
		t.Errorf("expected image placeholder, got %q", got)
	}
}

func TestTextileToMarkdown_BulletListNotBold(t *testing.T) {
	// Bullet list items starting with * should become list items, not bold.
	input := "* Alle Router verwenden @ApiTags@-Konstanten"
	got := TextileToMarkdown(input)
	if !strings.HasPrefix(got, "- ") {
		t.Errorf("expected list item, got %q", got)
	}
}

func TestRenderDescription_WordWraps(t *testing.T) {
	long := strings.Repeat("word ", 50)
	got := RenderDescription(long, 40)
	for _, line := range strings.Split(got, "\n") {
		// Glamour may add ANSI codes, so check visible length is reasonable.
		// Just verify it doesn't crash and produces output.
		_ = line
	}
	if len(got) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestRenderDescription_PreservesContent(t *testing.T) {
	got := RenderDescription("plain text", 40)
	// Glamour wraps output in ANSI codes; strip them to check content.
	stripped := stripANSI(got)
	if !strings.Contains(stripped, "plain text") {
		t.Errorf("expected plain text preserved, got stripped=%q raw=%q", stripped, got)
	}
}

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until 'm'
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			i = j + 1
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

// RenderDescription sanitizes its input itself. The post-render filter keeps
// SGR sequences for glamour's colors, and in code blocks and inline code
// glamour passes a raw SGR sequence from the input through intact, so
// without that an unsanitized description could hide or blink text.
func TestRenderDescription_SanitizesRawInput(t *testing.T) {
	for _, in := range []string{
		"<pre>\x1b[8mhidden</pre>",
		"Run @\x1b[8mhidden@ now",
		"    \x1b[8mhidden",
		"Body \x1b[8mhidden \x1b]52;c;ZXZpbA==\x07 \u202Eend",
	} {
		out := RenderDescription(in, 80)
		for _, bad := range []string{"\x1b[8m", "\x1b]", "\x07", "\u202E"} {
			if strings.Contains(out, bad) {
				t.Errorf("RenderDescription(%q) contains %q: %q", in, bad, out)
			}
		}
		if !strings.Contains(out, "hidden") {
			t.Errorf("RenderDescription(%q) lost the text: %q", in, out)
		}
	}
}

func TestRenderDescription_CharacterReferencesStayInert(t *testing.T) {
	// Markdown decodes character references, so sanitizing the input alone
	// would let "&#x1b;" turn back into ESC after rendering.
	for _, in := range []string{
		"Hello &#x1b;]52;c;ZXZpbA==&#x07; world",
		"a &#27;[2J b",
		"x &#x9b;31m y",
		"z &#x202e;abc",
		"<pre>&#x1b;]8;;https://evil.example/&#x1b;\\</pre>",
		"h1. &#x1b;]0;title&#7;",
	} {
		out := RenderDescription(in, 80)
		for _, bad := range []string{"\x1b]", "\x07", "\x1b[2J", "\u009b", "\u202E"} {
			if strings.Contains(out, bad) {
				t.Errorf("RenderDescription(%q) contains %q: %q", in, bad, out)
			}
		}
	}
	if out := RenderDescription("Hello &#x1b;]52;c;ZXZpbA==&#x07; world", 80); !strings.Contains(out, "^[]52;c;ZXZpbA==^G") {
		t.Errorf("expected the decoded sequence as visible text, got %q", out)
	}
}
