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

// searchItem implements list.Item for the search browser.
type searchItem struct {
	result models.SearchResult
}

func (i searchItem) Title() string {
	return i.result.Title
}

func (i searchItem) Description() string {
	date := i.result.DateTime
	if len(date) >= 10 {
		date = date[:10]
	}
	return fmt.Sprintf("%s | %s", i.result.Type, date)
}

func (i searchItem) FilterValue() string {
	return i.result.Title
}

// SearchBrowserModel is the bubbletea model for the search result browser.
type SearchBrowserModel struct {
	list       list.Model
	results    []models.SearchResult
	selected   *models.SearchResult
	showDetail bool
	keys       KeyMap
	width      int
	height     int
}

// NewSearchBrowserModel creates a new search browser model.
func NewSearchBrowserModel(results []models.SearchResult) SearchBrowserModel {
	items := make([]list.Item, len(results))
	for i, r := range results {
		items[i] = searchItem{result: r}
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "Search Results"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	return SearchBrowserModel{
		list:    l,
		results: results,
		keys:    DefaultKeyMap(),
	}
}

// Init implements tea.Model.
func (m SearchBrowserModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m SearchBrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if item, ok := m.list.SelectedItem().(searchItem); ok {
				m.selected = &item.result
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
func (m SearchBrowserModel) View() string {
	if m.showDetail && m.selected != nil {
		listView := m.list.View()
		detailView := renderSearchDetail(m.selected, m.width/2-6, m.height-6)
		return lipgloss.JoinHorizontal(lipgloss.Top, listView, "  ", detailView)
	}
	return docStyle.Render(m.list.View())
}

func renderSearchDetail(result *models.SearchResult, width, height int) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(result.Title))
	b.WriteString("\n\n")

	date := result.DateTime
	if len(date) >= 10 {
		date = date[:10]
	}

	fields := []struct{ k, v string }{
		{"ID", fmt.Sprintf("%d", result.ID)},
		{"Type", result.Type},
		{"Date", date},
		{"URL", result.URL},
	}

	for _, f := range fields {
		b.WriteString(subtitleStyle.Render(fmt.Sprintf("%-12s", f.k+":")) + " " + f.v + "\n")
	}

	if result.Description != "" {
		b.WriteString("\n" + subtitleStyle.Render("Description:") + "\n")
		b.WriteString(result.Description + "\n")
	}

	return detailStyle.Width(width).Height(height).Render(b.String())
}

// RunSearchBrowser starts the interactive search result browser.
func RunSearchBrowser(results []models.SearchResult) error {
	m := NewSearchBrowserModel(results)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
