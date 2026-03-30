package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines key bindings for the TUI.
type KeyMap struct {
	Up          key.Binding
	Down        key.Binding
	Enter       key.Binding
	Back        key.Binding
	Filter      key.Binding
	Quit        key.Binding
	Help        key.Binding
	CopyID      key.Binding
	CopyURL     key.Binding
	ToggleFocus key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
	OpenBrowser key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/up", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/down", "move down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		CopyID: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "copy ID"),
		),
		CopyURL: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "copy URL"),
		),
		ToggleFocus: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch pane"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("shift+up", "pgup"),
			key.WithHelp("shift+up", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("shift+down", "pgdown"),
			key.WithHelp("shift+down", "page down"),
		),
		OpenBrowser: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open in browser"),
		),
	}
}
