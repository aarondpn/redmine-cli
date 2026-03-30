package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aarondpn/redmine-cli/internal/models"
)

// clearStatusMsg is sent after a timeout to clear the status message.
type clearStatusMsg struct{}

// field is a key-value pair displayed in the detail header.
type field struct {
	key   string
	value string
}

// DetailPane is a scrollable detail pane with selectable fields.
type DetailPane struct {
	viewport      viewport.Model
	fields        []field
	title         string
	description   string // raw description for re-rendering on resize
	selectedField int    // -1 = no selection (scrolling description)
	width         int
	height        int
	statusMsg     string
	serverURL     string
	issueID       int
	issueURL      string
}

var (
	detailBorderFocused   = lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("6"))
	detailBorderUnfocused = lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8"))
	fieldSelectedStyle    = lipgloss.NewStyle().Background(lipgloss.Color("8")).Foreground(lipgloss.Color("15"))
)

// NewDetailPane creates a new detail pane.
func NewDetailPane(serverURL string) DetailPane {
	vp := viewport.New(0, 0)
	vp.SetContent("")
	return DetailPane{
		viewport:      vp,
		serverURL:     serverURL,
		selectedField: 0,
	}
}

// SetSize updates the pane dimensions and re-renders the description.
func (d *DetailPane) SetSize(width, height int) {
	if d.width == width && d.height == height {
		return
	}
	d.width = width
	d.height = height
	if d.description != "" || len(d.fields) > 0 {
		d.setViewportContent(d.description)
	}
}

func (d *DetailPane) contentWidth() int {
	// border (1 each side) + padding (2 each side) = 6
	w := d.width - 6
	if w < 1 {
		w = 1
	}
	return w
}

// SetIssueContent builds and sets the content for an issue.
func (d *DetailPane) SetIssueContent(issue *models.Issue) {
	d.issueID = issue.ID
	if d.serverURL != "" {
		d.issueURL = fmt.Sprintf("%s/issues/%d", strings.TrimRight(d.serverURL, "/"), issue.ID)
	}

	d.title = fmt.Sprintf("#%d %s", issue.ID, issue.Subject)

	assignee := "(unassigned)"
	if issue.AssignedTo != nil {
		assignee = issue.AssignedTo.Name
	}

	d.fields = []field{
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
	if d.issueURL != "" {
		d.fields = append(d.fields, field{"URL", d.issueURL})
	}

	d.selectedField = 0
	d.description = issue.Description
	d.setViewportContent(d.description)
}

// SetSearchContent builds and sets the content for a search result.
func (d *DetailPane) SetSearchContent(result *models.SearchResult) {
	d.issueID = result.ID
	d.issueURL = result.URL

	d.title = result.Title

	date := result.DateTime
	if len(date) >= 10 {
		date = date[:10]
	}

	d.fields = []field{
		{"ID", fmt.Sprintf("%d", result.ID)},
		{"Type", result.Type},
		{"Date", date},
		{"URL", result.URL},
	}

	d.selectedField = 0
	d.description = result.Description
	d.setViewportContent(d.description)
}

func (d *DetailPane) setViewportContent(description string) {
	cw := d.contentWidth()
	if cw < 10 {
		cw = 40
	}

	var content string
	if description != "" {
		content = RenderDescription(description, cw)
	}

	// Viewport height = total height - border/padding (4) - title (2) - fields - description label (2) - footer (1)
	vpHeight := d.height - 4 - 2 - len(d.fields) - 2 - 1
	if vpHeight < 3 {
		vpHeight = 3
	}
	d.viewport.Width = cw
	d.viewport.Height = vpHeight
	d.viewport.SetContent(content)
	d.viewport.GotoTop()
}

// Update handles key events for field navigation, scrolling, and clipboard.
func (d *DetailPane) Update(msg tea.Msg, keys KeyMap) tea.Cmd {
	switch msg := msg.(type) {
	case clearStatusMsg:
		d.statusMsg = ""
		return nil

	case tea.KeyMsg:
		inFields := d.selectedField >= 0 && d.selectedField < len(d.fields)
		inDescription := d.selectedField >= len(d.fields)

		switch {
		case key.Matches(msg, keys.Up):
			if inDescription {
				// If viewport is at top, move back to last field
				if d.viewport.AtTop() {
					d.selectedField = len(d.fields) - 1
					return nil
				}
				var cmd tea.Cmd
				d.viewport, cmd = d.viewport.Update(msg)
				return cmd
			}
			if d.selectedField > 0 {
				d.selectedField--
			}
			return nil

		case key.Matches(msg, keys.Down):
			if inDescription {
				var cmd tea.Cmd
				d.viewport, cmd = d.viewport.Update(msg)
				return cmd
			}
			if d.selectedField < len(d.fields)-1 {
				d.selectedField++
			} else if d.selectedField == len(d.fields)-1 && d.viewport.TotalLineCount() > 0 {
				// Move into description scroll
				d.selectedField = len(d.fields)
			}
			return nil

		case key.Matches(msg, keys.PageUp):
			if d.viewport.TotalLineCount() > 0 {
				d.selectedField = len(d.fields) // ensure we're in description mode
				d.viewport.ViewUp()
			}
			return nil

		case key.Matches(msg, keys.PageDown):
			if d.viewport.TotalLineCount() > 0 {
				d.selectedField = len(d.fields)
				d.viewport.ViewDown()
			}
			return nil

		case key.Matches(msg, keys.OpenBrowser):
			if d.issueURL != "" {
				if err := openURL(d.issueURL); err != nil {
					d.statusMsg = "Failed to open browser"
				} else {
					d.statusMsg = "Opened in browser"
				}
				return clearStatusAfter(2 * time.Second)
			}
			d.statusMsg = "No URL available"
			return clearStatusAfter(2 * time.Second)

		case key.Matches(msg, keys.CopyID):
			id := fmt.Sprintf("#%d", d.issueID)
			if err := clipboard.WriteAll(id); err != nil {
				d.statusMsg = "Copy failed"
			} else {
				d.statusMsg = fmt.Sprintf("Copied %s", id)
			}
			return clearStatusAfter(2 * time.Second)

		case key.Matches(msg, keys.CopyURL):
			if d.issueURL == "" {
				d.statusMsg = "No URL available"
				return clearStatusAfter(2 * time.Second)
			}
			if err := clipboard.WriteAll(d.issueURL); err != nil {
				d.statusMsg = "Copy failed"
			} else {
				d.statusMsg = "Copied URL"
			}
			return clearStatusAfter(2 * time.Second)

		case key.Matches(msg, keys.Enter):
			if inFields {
				f := d.fields[d.selectedField]
				if err := clipboard.WriteAll(f.value); err != nil {
					d.statusMsg = "Copy failed"
				} else {
					d.statusMsg = fmt.Sprintf("Copied %s", f.key)
				}
				return clearStatusAfter(2 * time.Second)
			}
			return nil
		}

		// Forward other keys (pgup/pgdn) to viewport when in description
		if inDescription {
			var cmd tea.Cmd
			d.viewport, cmd = d.viewport.Update(msg)
			return cmd
		}
	}

	return nil
}

// ViewFocused renders the detail pane. Pass focused=true to highlight the border.
func (d *DetailPane) ViewFocused(focused bool) string {
	cw := d.contentWidth()
	var b strings.Builder

	// Title (truncated to one line)
	title := d.title
	if len(title) > cw {
		title = title[:cw-1] + "…"
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// Fields (selectable when focused)
	maxValWidth := cw - 13 // 12 for label + 1 space
	if maxValWidth < 10 {
		maxValWidth = 10
	}
	for i, f := range d.fields {
		value := f.value
		if len(value) > maxValWidth {
			value = value[:maxValWidth-1] + "…"
		}

		if focused && i == d.selectedField {
			line := fmt.Sprintf("%-12s %s", f.key+":", value)
			b.WriteString(fieldSelectedStyle.Render(line))
		} else {
			label := subtitleStyle.Render(fmt.Sprintf("%-12s", f.key+":"))
			b.WriteString(label + " " + value)
		}
		b.WriteString("\n")
	}

	// Description
	if d.viewport.TotalLineCount() > 0 {
		descLabel := "Description:"
		if focused && d.selectedField >= len(d.fields) {
			descLabel = "Description: (scrolling)"
		}
		b.WriteString("\n" + subtitleStyle.Render(descLabel) + "\n")
		b.WriteString(d.viewport.View())
	}

	// Footer
	b.WriteString("\n")
	if d.statusMsg != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true).Render(d.statusMsg))
	} else if d.viewport.TotalLineCount() > 0 {
		scrollInfo := fmt.Sprintf(" %d%% ", int(d.viewport.ScrollPercent()*100))
		b.WriteString(subtitleStyle.Render(scrollInfo))
	}

	style := detailBorderUnfocused
	if focused {
		style = detailBorderFocused
	}
	return style.Width(d.width).Height(d.height).Render(
		lipgloss.NewStyle().Width(cw).Render(b.String()),
	)
}

func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

func openURL(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	default:
		return exec.Command("open", url).Start()
	}
}
