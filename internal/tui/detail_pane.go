package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/aarondpn/redmine-cli/v2/internal/browser"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// clearStatusMsg is sent after a timeout to clear the status message. seq
// identifies the status it belongs to, so an older timer cannot clear a
// newer message (or a pending open confirmation).
type clearStatusMsg struct {
	seq int
}

// openURL launches a URL in the default browser. It is a variable so tests
// can stub it.
var openURL = browser.Open

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
	statusSeq     int
	serverURL     string
	issueID       int
	issueURL      string
	// confirmURL is the off-server URL the user was asked to confirm by
	// pressing the open key a second time.
	confirmURL string
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
	d.confirmURL = ""
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
	d.issueURL = searchResultURL(d.serverURL, result)
	d.confirmURL = ""

	d.title = result.Title

	date := result.DateTime
	if len(date) >= 10 {
		date = date[:10]
	}

	d.fields = []field{
		{"ID", fmt.Sprintf("%d", result.ID)},
		{"Type", result.Type},
		{"Date", date},
		{"URL", d.issueURL},
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
		if msg.seq == d.statusSeq {
			d.statusMsg = ""
			d.confirmURL = ""
		}
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
				d.viewport.PageUp()
			}
			return nil

		case key.Matches(msg, keys.PageDown):
			if d.viewport.TotalLineCount() > 0 {
				d.selectedField = len(d.fields)
				d.viewport.PageDown()
			}
			return nil

		case key.Matches(msg, keys.OpenBrowser):
			return d.openInBrowser(keys)

		case key.Matches(msg, keys.CopyID):
			id := fmt.Sprintf("#%d", d.issueID)
			if err := clipboard.WriteAll(id); err != nil {
				return d.setStatus("Copy failed", 2*time.Second)
			}
			return d.setStatus(fmt.Sprintf("Copied %s", id), 2*time.Second)

		case key.Matches(msg, keys.CopyURL):
			if d.issueURL == "" {
				return d.setStatus("No URL available", 2*time.Second)
			}
			if err := clipboard.WriteAll(d.issueURL); err != nil {
				return d.setStatus("Copy failed", 2*time.Second)
			}
			return d.setStatus("Copied URL", 2*time.Second)

		case key.Matches(msg, keys.Enter):
			if inFields {
				f := d.fields[d.selectedField]
				if err := clipboard.WriteAll(f.value); err != nil {
					return d.setStatus("Copy failed", 2*time.Second)
				}
				return d.setStatus(fmt.Sprintf("Copied %s", f.key), 2*time.Second)
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
	title := output.SanitizeTerminal(d.title)
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
		value := output.SanitizeTerminal(f.value)
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

// setStatus shows msg in the footer and clears it after timeout. A new
// status replaces any pending open confirmation.
func (d *DetailPane) setStatus(msg string, timeout time.Duration) tea.Cmd {
	d.statusMsg = msg
	d.confirmURL = ""
	d.statusSeq++
	seq := d.statusSeq
	return tea.Tick(timeout, func(time.Time) tea.Msg {
		return clearStatusMsg{seq: seq}
	})
}

// openInBrowser opens the current URL. URLs that are not plain http(s)
// links are refused, and a URL on a host other than the configured server
// is only opened after the user saw it and pressed the open key again.
func (d *DetailPane) openInBrowser(keys KeyMap) tea.Cmd {
	if d.issueURL == "" {
		return d.setStatus("No URL available", 2*time.Second)
	}
	shown := output.SanitizeTerminal(d.issueURL)
	u, err := browser.ValidateURL(d.issueURL)
	if err != nil {
		return d.setStatus("Refusing to open unsafe URL: "+shown, 5*time.Second)
	}
	if !browser.SameHost(u, d.serverURL) && d.confirmURL != d.issueURL {
		// Name the host explicitly: a URL such as
		// https://redmine.example.com@evil.example/ reads like the server.
		prompt := fmt.Sprintf("Link leaves the Redmine server for %s: %s (press %s again to open)",
			output.SanitizeTerminal(u.Host), shown, keys.OpenBrowser.Help().Key)
		cmd := d.setStatus(prompt, 5*time.Second)
		d.confirmURL = d.issueURL
		return cmd
	}
	d.confirmURL = ""
	if err := openURL(d.issueURL); err != nil {
		return d.setStatus("Failed to open browser", 2*time.Second)
	}
	return d.setStatus("Opened in browser", 2*time.Second)
}

// searchResultURL returns the URL to open for a search result. Issue results
// link to the issue on the configured server; everything else falls back to
// the URL the server reported, which openInBrowser validates.
func searchResultURL(serverURL string, result *models.SearchResult) string {
	if serverURL != "" && result.ID > 0 {
		switch result.Type {
		case "issue", "issue-closed":
			return fmt.Sprintf("%s/issues/%d", strings.TrimRight(serverURL, "/"), result.ID)
		}
	}
	return result.URL
}
