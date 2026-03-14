package output

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	StyleNew        = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	StyleInProgress = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	StyleResolved   = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	StyleClosed     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	StyleRejected   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	StyleHighPrio   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	StyleNormalPrio = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	StyleLowPrio    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	StyleID      = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	StyleHeader  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	StyleLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	StyleSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	StyleError   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	StyleWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

// StatusStyle returns the appropriate style for a status name.
func StatusStyle(name string) lipgloss.Style {
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
		return lipgloss.NewStyle()
	}
}

// PriorityStyle returns the appropriate style for a priority name.
func PriorityStyle(name string) lipgloss.Style {
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
