package cmdutil

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Palette shared by interactive prompts: the Redmine red of the logo and the
// docs site, darker on light terminals. lipgloss maps the hex colors to the
// nearest ANSI color on terminals without true color.
var (
	// BrandRed is the accent color of interactive prompts.
	BrandRed = lipgloss.AdaptiveColor{Light: "#c23030", Dark: "#e04040"}
	// BrandRedDim is the darker red used next to BrandRed, e.g. in the logo.
	BrandRedDim = lipgloss.AdaptiveColor{Light: "#8f1f1f", Dark: "#a82a2a"}

	formAccent = BrandRed
	formMuted  = lipgloss.Color("8")
	formError  = lipgloss.AdaptiveColor{Light: "#b85c00", Dark: "#f0a050"}
)

// FormTheme returns the huh theme used by all interactive prompts.
func FormTheme() *huh.Theme {
	t := huh.ThemeBase()

	t.Focused.Base = t.Focused.Base.BorderForeground(formAccent)
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = lipgloss.NewStyle().Foreground(formAccent).Bold(true)
	t.Focused.NoteTitle = t.Focused.Title.MarginBottom(1)
	t.Focused.Description = lipgloss.NewStyle().Foreground(formMuted)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(formError)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(formError)

	t.Focused.SelectSelector = lipgloss.NewStyle().Foreground(formAccent).SetString("› ")
	t.Focused.Option = lipgloss.NewStyle()
	t.Focused.SelectedOption = lipgloss.NewStyle().Foreground(formAccent)
	t.Focused.MultiSelectSelector = t.Focused.SelectSelector
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(formAccent).SetString("✓ ")
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(formMuted).SetString("• ")
	t.Focused.UnselectedOption = lipgloss.NewStyle()
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(formAccent)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(formAccent)

	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(lipgloss.Color("#f5f1e8")).Background(formAccent).Bold(true)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(lipgloss.Color("7")).Background(lipgloss.Color("0"))
	t.Focused.Next = t.Focused.FocusedButton

	t.Focused.TextInput.Prompt = lipgloss.NewStyle().Foreground(formAccent)
	t.Focused.TextInput.Cursor = lipgloss.NewStyle().Foreground(formAccent)
	t.Focused.TextInput.Placeholder = lipgloss.NewStyle().Foreground(formMuted)

	// Fields that are not focused stay readable but recede.
	t.Blurred = t.Focused
	t.Blurred.Base = t.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.Title = lipgloss.NewStyle().Foreground(formMuted).Bold(true)
	t.Blurred.SelectSelector = lipgloss.NewStyle().Foreground(formMuted).SetString("› ")
	t.Blurred.SelectedOption = lipgloss.NewStyle()
	t.Blurred.MultiSelectSelector = lipgloss.NewStyle().SetString("  ")
	t.Blurred.TextInput.Prompt = lipgloss.NewStyle().Foreground(formMuted)
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	t.Group.Title = t.Focused.Title
	t.Group.Description = t.Focused.Description
	return t
}

// NewForm builds a huh form with the shared theme.
func NewForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithTheme(FormTheme())
}
