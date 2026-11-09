package spec

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Error is one problem found in a manifest or an input object. Line is the
// 1-based line in File, or 0 when the position is unknown (manifests built
// in code, MCP input).
type Error struct {
	File    string
	Line    int
	Path    string // dotted location such as commands[0].params[1].default
	Message string
}

// Error renders "file:line: path: message", leaving out unknown parts.
func (e *Error) Error() string {
	var b strings.Builder
	if e.File != "" {
		b.WriteString(e.File)
		if e.Line > 0 {
			b.WriteString(":")
			b.WriteString(strconv.Itoa(e.Line))
		}
		b.WriteString(": ")
	} else if e.Line > 0 {
		fmt.Fprintf(&b, "line %d: ", e.Line)
	}
	if e.Path != "" {
		b.WriteString(e.Path)
		b.WriteString(": ")
	}
	b.WriteString(e.Message)
	return b.String()
}

// ValidationError collects every problem found in one pass, so authors can
// fix a manifest without a round trip per mistake.
type ValidationError struct {
	Errors []*Error
}

// Error renders one problem per line.
func (e *ValidationError) Error() string {
	lines := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		lines[i] = err.Error()
	}
	return strings.Join(lines, "\n")
}

// Details returns the rendered problems, e.g. for cmdutil.CodedError.
func (e *ValidationError) Details() []string {
	out := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		out[i] = err.Error()
	}
	return out
}

// issues accumulates problems while validating.
type issues struct {
	file string
	pos  positions
	list []*Error
}

// addf records a problem at the JSON pointer ptr (see pointer).
func (is *issues) addf(ptr string, format string, args ...any) {
	is.list = append(is.list, &Error{
		File:    is.file,
		Line:    is.pos.line(ptr),
		Path:    displayPath(ptr),
		Message: fmt.Sprintf(format, args...),
	})
}

// err returns nil when nothing was recorded. Problems are ordered by line so
// the output reads top to bottom; the sort is stable to keep the discovery
// order for problems on the same line.
func (is *issues) err() error {
	if len(is.list) == 0 {
		return nil
	}
	slices.SortStableFunc(is.list, func(a, b *Error) int { return cmp.Compare(a.Line, b.Line) })
	return &ValidationError{Errors: is.list}
}

// positions maps a JSON pointer ("/commands/0/name") to the source line of
// the key (for mapping members) or of the item (for sequence items).
type positions map[string]int

// line returns the line for ptr, falling back to the closest ancestor that
// has one: a missing field is reported where its parent object starts.
func (p positions) line(ptr string) int {
	for {
		if l, ok := p[ptr]; ok {
			return l
		}
		if ptr == "" {
			return 0
		}
		i := strings.LastIndexByte(ptr, '/')
		if i < 0 {
			return 0
		}
		ptr = ptr[:i]
	}
}

// pointer joins JSON pointer segments. Segments never need ~ escaping here:
// unknown keys are reported through their parent plus a quoted name.
func pointer(base string, segs ...any) string {
	var b strings.Builder
	b.WriteString(base)
	for _, s := range segs {
		b.WriteByte('/')
		switch v := s.(type) {
		case int:
			b.WriteString(strconv.Itoa(v))
		default:
			fmt.Fprint(&b, v)
		}
	}
	return b.String()
}

// displayPath turns "/commands/0/params/1/type" into
// "commands[0].params[1].type".
func displayPath(ptr string) string {
	if ptr == "" {
		return ""
	}
	var b strings.Builder
	for _, seg := range strings.Split(strings.TrimPrefix(ptr, "/"), "/") {
		if _, err := strconv.Atoi(seg); err == nil && b.Len() > 0 {
			b.WriteString("[" + seg + "]")
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(seg)
	}
	return b.String()
}
