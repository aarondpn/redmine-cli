package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
)

var (
	// Block-level patterns (applied per-line).
	reHeader      = regexp.MustCompile(`^h([1-6])\.\s+(.*)$`)
	reBulletList  = regexp.MustCompile(`^(\*+)\s+`)
	reOrderedList = regexp.MustCompile(`^(#+)\s+`)

	// Inline patterns.
	reBold       = regexp.MustCompile(`(?:^|(?P<pre>\s))` + `\*(\S[^*]*\S|\S)\*` + `(?:$|(?P<post>\s))`)
	reItalic     = regexp.MustCompile(`(?:^|(?P<pre>\s))` + `_(\S[^_]*\S|\S)_` + `(?:$|(?P<post>\s))`)
	reInlineCode = regexp.MustCompile(`@([^@\n]+)@`)
	reLink       = regexp.MustCompile(`"([^"]+)":(https?://\S+)`)
	reImage      = regexp.MustCompile(`!([^!\s]+)!`)

	// Multi-line patterns.
	rePreOpen  = regexp.MustCompile(`(?i)<pre>`)
	rePreClose = regexp.MustCompile(`(?i)</pre>`)
)

// TextileToMarkdown converts common Redmine textile markup to markdown.
func TextileToMarkdown(input string) string {
	lines := strings.Split(input, "\n")
	var out []string
	inPre := false

	for _, line := range lines {
		if inPre {
			if rePreClose.MatchString(line) {
				// Replace </pre> with closing fence.
				line = rePreClose.ReplaceAllString(line, "```")
				inPre = false
			}
			out = append(out, line)
			continue
		}

		if rePreOpen.MatchString(line) {
			line = rePreOpen.ReplaceAllString(line, "```")
			if rePreClose.MatchString(line) {
				// Inline <pre>...</pre> on one line.
				line = rePreClose.ReplaceAllString(line, "```")
			} else {
				inPre = true
			}
			out = append(out, line)
			continue
		}

		// Headers: h1. Title → # Title
		if m := reHeader.FindStringSubmatch(line); m != nil {
			level := m[1][0] - '0'
			line = strings.Repeat("#", int(level)) + " " + m[2]
			out = append(out, line)
			continue
		}

		// Bullet lists: * item → - item, ** item →  - item
		if m := reBulletList.FindStringSubmatch(line); m != nil {
			depth := len(m[1])
			indent := strings.Repeat("  ", depth-1)
			rest := line[len(m[0]):]
			line = indent + "- " + rest
			out = append(out, line)
			continue
		}

		// Ordered lists: # item → 1. item
		if m := reOrderedList.FindStringSubmatch(line); m != nil {
			depth := len(m[1])
			indent := strings.Repeat("  ", depth-1)
			rest := line[len(m[0]):]
			line = indent + "1. " + rest
			out = append(out, line)
			continue
		}

		// Inline replacements on non-list lines.
		line = convertInline(line)
		out = append(out, line)
	}

	return strings.Join(out, "\n")
}

func convertInline(line string) string {
	// Bold: *text* → **text** (only when surrounded by whitespace or at boundaries)
	line = reBold.ReplaceAllStringFunc(line, func(match string) string {
		m := reBold.FindStringSubmatch(match)
		pre := m[1]
		content := m[2]
		post := m[3]
		return pre + "**" + content + "**" + post
	})

	// Italic: _text_ → *text*
	line = reItalic.ReplaceAllStringFunc(line, func(match string) string {
		m := reItalic.FindStringSubmatch(match)
		pre := m[1]
		content := m[2]
		post := m[3]
		return pre + "*" + content + "*" + post
	})

	// Inline code: @code@ → `code`
	line = reInlineCode.ReplaceAllString(line, "`$1`")

	// Links: "text":url → [text](url)
	line = reLink.ReplaceAllString(line, "[$1]($2)")

	// Images: !image.png! → (image: image.png)
	line = reImage.ReplaceAllString(line, "(image: $1)")

	return line
}

// RenderDescription converts textile markup to styled terminal output.
func RenderDescription(textile string, width int) string {
	if width < 10 {
		width = 10
	}

	md := TextileToMarkdown(textile)

	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return wordWrap(textile, width)
	}

	out, err := r.Render(md)
	if err != nil {
		return wordWrap(textile, width)
	}

	return strings.TrimRight(out, "\n")
}

// wordWrap is a simple fallback wrapper.
func wordWrap(text string, width int) string {
	var result strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if len(line) <= width {
			result.WriteString(line + "\n")
			continue
		}
		words := strings.Fields(line)
		col := 0
		for i, w := range words {
			if i > 0 && col+1+len(w) > width {
				result.WriteString("\n")
				col = 0
			} else if i > 0 {
				result.WriteString(" ")
				col++
			}
			result.WriteString(w)
			col += len(w)
		}
		result.WriteString("\n")
	}
	return strings.TrimRight(result.String(), "\n")
}
