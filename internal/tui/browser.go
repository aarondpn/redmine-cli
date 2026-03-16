package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aarondpn/redmine-cli/internal/models"
)

var (
	docStyle      = lipgloss.NewStyle().Margin(1, 2)
	detailStyle   = lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder())
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
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
	list       list.Model
	issues     []models.Issue
	selected   *models.Issue
	showDetail bool
	keys       KeyMap
	width      int
	height     int
}

// NewBrowserModel creates a new browser model.
func NewBrowserModel(issues []models.Issue) BrowserModel {
	items := make([]list.Item, len(issues))
	for i, iss := range issues {
		items[i] = issueItem{issue: iss}
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "Issues"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	return BrowserModel{
		list:   l,
		issues: issues,
		keys:   DefaultKeyMap(),
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
			m.list.SetSize(msg.Width/2, msg.Height-4)
		} else {
			m.list.SetSize(msg.Width, msg.Height-4)
		}
		return m, nil

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Enter):
			if item, ok := m.list.SelectedItem().(issueItem); ok {
				m.selected = &item.issue
				m.showDetail = true
				m.list.SetSize(m.width/2, m.height-4)
			}
			return m, nil
		case key.Matches(msg, m.keys.Back):
			if m.showDetail {
				m.showDetail = false
				m.selected = nil
				m.list.SetSize(m.width, m.height-4)
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model.
func (m BrowserModel) View() string {
	if m.showDetail && m.selected != nil {
		listView := m.list.View()
		detailView := renderDetail(m.selected, m.width/2-6, m.height-6)
		return lipgloss.JoinHorizontal(lipgloss.Top, listView, "  ", detailView)
	}
	return docStyle.Render(m.list.View())
}

func renderDetail(issue *models.Issue, width, height int) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("#%d %s", issue.ID, issue.Subject)))
	b.WriteString("\n\n")

	assignee := "(unassigned)"
	if issue.AssignedTo != nil {
		assignee = issue.AssignedTo.Name
	}

	fields := []struct{ k, v string }{
		{"Project", issue.Project.Name},
		{"Tracker", issue.Tracker.Name},
		{"Status", issue.Status.Name},
		{"Priority", issue.Priority.Name},
		{"Assignee", assignee},
		{"Author", issue.Author.Name},
		{"Done", fmt.Sprintf("%d%%", issue.DoneRatio)},
		{"Created", issue.CreatedOn},
		{"Updated", issue.UpdatedOn},
	}

	for _, f := range fields {
		b.WriteString(subtitleStyle.Render(fmt.Sprintf("%-12s", f.k+":")) + " " + f.v + "\n")
	}

	if issue.Description != "" {
		b.WriteString("\n" + subtitleStyle.Render("Description:") + "\n")
		b.WriteString(issue.Description + "\n")
	}

	return detailStyle.Width(width).Height(height).Render(b.String())
}

// RunBrowser starts the interactive issue browser.
func RunBrowser(issues []models.Issue) error {
	m := NewBrowserModel(issues)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
