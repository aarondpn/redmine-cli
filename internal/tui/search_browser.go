package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// SearchFunc performs a search and returns results.
type SearchFunc func(query string) ([]models.SearchResult, error)

// searchResultMsg is sent when a search completes.
type searchResultMsg struct {
	results []models.SearchResult
	query   string
	err     error
}

// searchTickMsg is used for debouncing.
type searchTickMsg struct {
	query string
}

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

var (
	searchBarStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("6"))
	searchBarUnfocusedStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("8"))
)

// SearchBrowserModel is the bubbletea model for the search result browser.
type SearchBrowserModel struct {
	searchInput  textinput.Model
	list         list.Model
	results      []models.SearchResult
	detail       DetailPane
	showDetail   bool
	focusDetail  bool
	searching    bool
	lastQuery    string // last query that was actually sent
	pendingQuery string // query waiting for debounce
	keys         KeyMap
	width        int
	height       int
	searchFn     SearchFunc
	serverURL    string
	err          error
}

// NewSearchBrowserModel creates a new search browser model.
func NewSearchBrowserModel(initialResults []models.SearchResult, initialQuery string, serverURL string, searchFn SearchFunc) SearchBrowserModel {
	items := make([]list.Item, len(initialResults))
	for i, r := range initialResults {
		items[i] = searchItem{result: r}
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "Search Results"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false) // We use our own search bar instead

	keys := DefaultKeyMap()
	searchKey := key.NewBinding(key.WithKeys("/", "s"), key.WithHelp("/ s", "search"))
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{searchKey, keys.ToggleFocus, keys.OpenBrowser}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			searchKey, keys.ToggleFocus, keys.OpenBrowser,
			keys.CopyID, keys.CopyURL,
			keys.PageUp, keys.PageDown,
		}
	}

	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.SetValue(initialQuery)
	ti.Focus()
	ti.CharLimit = 200

	return SearchBrowserModel{
		searchInput: ti,
		list:        l,
		results:     initialResults,
		detail:      NewDetailPane(serverURL),
		keys:        keys,
		searchFn:    searchFn,
		serverURL:   serverURL,
		lastQuery:   initialQuery,
	}
}

// Init implements tea.Model.
func (m SearchBrowserModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m SearchBrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
		return m, nil

	case clearStatusMsg:
		cmd := m.detail.Update(msg, m.keys)
		return m, cmd

	case searchResultMsg:
		m.searching = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Only apply if this result matches the current query.
		if msg.query == m.lastQuery {
			m.results = msg.results
			items := make([]list.Item, len(msg.results))
			for i, r := range msg.results {
				items[i] = searchItem{result: r}
			}
			m.list.SetItems(items)
			m.list.ResetSelected()
			// Close stale detail pane.
			if m.showDetail {
				m.showDetail = false
				m.focusDetail = false
				m.updateSizes()
			}
		}
		return m, nil

	case searchTickMsg:
		// Debounce: only search if query hasn't changed since tick was scheduled.
		if msg.query == m.searchInput.Value() && msg.query != m.lastQuery {
			m.lastQuery = msg.query
			m.searching = true
			return m, m.performSearch(msg.query)
		}
		return m, nil

	case tea.KeyMsg:
		// If the search input is focused, handle special keys.
		if m.searchInput.Focused() {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.searchInput.Blur()
				return m, nil
			case "down", "enter":
				m.searchInput.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				query := m.searchInput.Value()
				if query != m.lastQuery {
					m.pendingQuery = query
					return m, tea.Batch(cmd, m.scheduleSearch(query))
				}
				return m, cmd
			}
		}

		// List/detail key handling (search bar not focused).
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case msg.String() == "/" || msg.String() == "s":
			m.searchInput.Focus()
			return m, textinput.Blink

		case key.Matches(msg, m.keys.Enter) && !m.showDetail:
			if item, ok := m.list.SelectedItem().(searchItem); ok {
				m.showDetail = true
				m.focusDetail = false
				m.updateSizes()
				m.detail.SetSearchContent(&item.result)
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
				m.updateSizes()
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

	// List navigation.
	prevIdx := m.list.Index()
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	if m.showDetail && m.list.Index() != prevIdx {
		if item, ok := m.list.SelectedItem().(searchItem); ok {
			m.detail.SetSearchContent(&item.result)
		}
	}

	return m, cmd
}

func (m *SearchBrowserModel) updateSizes() {
	// Search bar takes 3 rows (border top + content + border bottom).
	searchBarHeight := 3
	if m.showDetail {
		pw := m.paneWidth()
		ph := m.paneHeight()
		m.list.SetSize(pw-2, ph-2-searchBarHeight)
		m.detail.SetSize(pw, ph)
	} else {
		m.list.SetSize(m.width, m.height-4-searchBarHeight)
	}
}

func (m SearchBrowserModel) performSearch(query string) tea.Cmd {
	fn := m.searchFn
	return func() tea.Msg {
		results, err := fn(query)
		return searchResultMsg{results: results, query: query, err: err}
	}
}

func (m SearchBrowserModel) scheduleSearch(query string) tea.Cmd {
	return tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg {
		return searchTickMsg{query: query}
	})
}

// paneWidth returns the width for each pane (excluding the 1-char gap).
func (m SearchBrowserModel) paneWidth() int {
	return (m.width - 1) / 2
}

// paneHeight returns the fixed height for both panes.
func (m SearchBrowserModel) paneHeight() int {
	return m.height - 2
}

// View implements tea.Model.
func (m SearchBrowserModel) View() string {
	// Search bar.
	searchBarW := m.width - 4
	if m.showDetail {
		searchBarW = m.paneWidth() - 4
	}
	if searchBarW < 10 {
		searchBarW = 10
	}
	m.searchInput.Width = searchBarW

	barStyle := searchBarUnfocusedStyle
	if m.searchInput.Focused() {
		barStyle = searchBarStyle
	}

	statusSuffix := ""
	if m.searching {
		statusSuffix = " (searching...)"
	}
	if m.err != nil {
		statusSuffix = " (error)"
	}

	searchBar := barStyle.Width(searchBarW + 2).Render(m.searchInput.View() + statusSuffix)

	if m.showDetail {
		pw := m.paneWidth()
		ph := m.paneHeight()

		listBorder := listFocusStyle
		if m.focusDetail {
			listBorder = listUnfocusStyle
		}

		leftContent := searchBar + "\n" + m.list.View()
		listView := listBorder.Width(pw).Height(ph).Render(leftContent)
		detailView := m.detail.ViewFocused(m.focusDetail)
		return lipgloss.JoinHorizontal(lipgloss.Top, listView, " ", detailView)
	}

	return docStyle.Render(searchBar + "\n" + m.list.View())
}

// RunSearchBrowser starts the interactive search result browser.
func RunSearchBrowser(initialResults []models.SearchResult, initialQuery string, serverURL string, searchFn SearchFunc) error {
	m := NewSearchBrowserModel(initialResults, initialQuery, serverURL, searchFn)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
