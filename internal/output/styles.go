package output

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Style is a lipgloss style for rendering server data. Render sanitizes its
// input with SanitizeTerminal before applying the style, so escape sequences
// in the output only ever come from the style itself. The printer recognizes
// such output and keeps its styling, while neutralizing escape sequences in
// any other value.
type Style struct {
	lipgloss.Style
}

// NewStyle wraps a lipgloss style.
func NewStyle(s lipgloss.Style) Style {
	return Style{Style: s}
}

// Render sanitizes strs and renders them with the style.
func (s Style) Render(strs ...string) string {
	clean := make([]string, len(strs))
	for i, str := range strs {
		clean[i] = SanitizeTerminal(str)
	}
	out := s.Style.Render(clean...)
	rememberStyled(out)
	return out
}

var (
	StyleNew        = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("10")))
	StyleInProgress = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("11")))
	StyleResolved   = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("12")))
	StyleClosed     = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("8")))
	StyleRejected   = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("9")))

	StyleHighPrio   = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true))
	StyleNormalPrio = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("7")))
	StyleLowPrio    = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("8")))

	StyleID      = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true))
	StyleHeader  = NewStyle(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")))
	StyleLabel   = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("8")))
	StyleSuccess = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("10")))
	StyleError   = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true))
	StyleWarning = NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("11")))
)

// StatusStyle returns the appropriate style for a status name.
func StatusStyle(name string) Style {
	lower := strings.ToLower(name)
	switch {
	case lower == "new":
		return StyleNew
	case strings.Contains(lower, "progress"):
		return StyleInProgress
	case lower == "resolved" || lower == "feedback":
		return StyleResolved
	case lower == "closed":
		return StyleClosed
	case lower == "rejected":
		return StyleRejected
	default:
		return NewStyle(lipgloss.NewStyle())
	}
}

// PriorityStyle returns the appropriate style for a priority name.
func PriorityStyle(name string) Style {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "urgent") || strings.Contains(lower, "immediate"):
		return StyleHighPrio
	case strings.Contains(lower, "high"):
		return StyleHighPrio
	case strings.Contains(lower, "low"):
		return StyleLowPrio
	default:
		return StyleNormalPrio
	}
}
