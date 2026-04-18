package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

var (
	docStyle         = lipgloss.NewStyle().Margin(1, 2)
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	subtitleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	listFocusStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("6"))
	listUnfocusStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8"))
)

// issueItem implements list.Item for the issue browser.
type issueItem struct {
	issue models.Issue
}

func (i issueItem) Title() string {
	return fmt.Sprintf("#%d %s", i.issue.ID, i.issue.Subject)
}

func (i issueItem) Description() string {
	assignee := ""
	if i.issue.AssignedTo != nil {
		assignee = " -> " + i.issue.AssignedTo.Name
	}
	return fmt.Sprintf("%s | %s | %s%s",
		i.issue.Tracker.Name,
		i.issue.Status.Name,
		i.issue.Priority.Name,
		assignee,
	)
}

func (i issueItem) FilterValue() string {
	return i.issue.Subject
}

// BrowserModel is the bubbletea model for the issue browser.
type BrowserModel struct {
	list        list.Model
	issues      []models.Issue
	detail      DetailPane
	showDetail  bool
	focusDetail bool // true = right pane (detail) has focus
	keys        KeyMap
	width       int
	height      int
}

// NewBrowserModel creates a new browser model.
func NewBrowserModel(issues []models.Issue, serverURL string) BrowserModel {
	items := make([]list.Item, len(issues))
	for i, iss := range issues {
		items[i] = issueItem{issue: iss}
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "Issues"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	keys := DefaultKeyMap()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keys.ToggleFocus, keys.OpenBrowser}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			keys.ToggleFocus, keys.OpenBrowser,
			keys.CopyID, keys.CopyURL,
			keys.PageUp, keys.PageDown,
		}
	}

	return BrowserModel{
		list:   l,
		issues: issues,
		detail: NewDetailPane(serverURL),
		keys:   keys,
	}
}

// Init implements tea.Model.
func (m BrowserModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m BrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.showDetail {
			pw := m.paneWidth()
			ph := m.paneHeight()
			// List inner size: pane width minus border (2) minus any padding
			m.list.SetSize(pw-2, ph-2)
			m.detail.SetSize(pw, ph)
		} else {
			m.list.SetSize(msg.Width, msg.Height-4)
		}
		return m, nil

	case clearStatusMsg:
		cmd := m.detail.Update(msg, m.keys)
		return m, cmd

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Enter) && !m.showDetail:
			if item, ok := m.list.SelectedItem().(issueItem); ok {
				pw := m.paneWidth()
				ph := m.paneHeight()
				m.showDetail = true
				m.focusDetail = false
				m.list.SetSize(pw-2, ph-2)
				m.detail.SetSize(pw, ph)
				m.detail.SetIssueContent(&item.issue)
			}
			return m, nil

		case key.Matches(msg, m.keys.ToggleFocus) && m.showDetail:
			m.focusDetail = !m.focusDetail
			return m, nil

		case key.Matches(msg, m.keys.Back):
			if m.showDetail {
				if m.focusDetail {
					m.focusDetail = false
					return m, nil
				}
				m.showDetail = false
				m.list.SetSize(m.width, m.height-4)
				return m, nil
			}

		case m.showDetail && (key.Matches(msg, m.keys.OpenBrowser) ||
			key.Matches(msg, m.keys.CopyID) ||
			key.Matches(msg, m.keys.CopyURL)):
			cmd := m.detail.Update(msg, m.keys)
			return m, cmd

		case m.showDetail && m.focusDetail:
			cmd := m.detail.Update(msg, m.keys)
			return m, cmd
		}
	}

	// List has focus: handle navigation.
	prevIdx := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	// Update detail pane when selection changes.
	if m.showDetail && m.list.Index() != prevIdx {
		if item, ok := m.list.SelectedItem().(issueItem); ok {
			m.detail.SetIssueContent(&item.issue)
		}
	}

	return m, cmd
}

// paneWidth returns the width for each pane (excluding the 1-char gap).
func (m BrowserModel) paneWidth() int {
	return (m.width - 1) / 2
}

// paneHeight returns the fixed height for both panes.
func (m BrowserModel) paneHeight() int {
	return m.height - 2
}

// View implements tea.Model.
func (m BrowserModel) View() string {
	if m.showDetail {
		pw := m.paneWidth()
		ph := m.paneHeight()

		listBorder := listFocusStyle
		if m.focusDetail {
			listBorder = listUnfocusStyle
		}
		listView := listBorder.Width(pw).Height(ph).Render(m.list.View())
		detailView := m.detail.ViewFocused(m.focusDetail)
		return lipgloss.JoinHorizontal(lipgloss.Top, listView, " ", detailView)
	}
	return docStyle.Render(m.list.View())
}

// RunBrowser starts the interactive issue browser.
func RunBrowser(issues []models.Issue, serverURL string) error {
	m := NewBrowserModel(issues, serverURL)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
