package auth

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// loginPrompt is the input prompt used by all login fields.
const loginPrompt = "› "

// loginLabelWidth aligns the values of the step summary lines.
const loginLabelWidth = 18

// loginMinHeight is the terminal height the header with the logo needs
// next to the step lines and the first form.
const loginMinHeight = 20

var (
	loginTitleStyle = lipgloss.NewStyle().Bold(true)
	loginMutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	loginOKStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	loginFailStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	loginLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Width(loginLabelWidth)
	loginCmdStyle   = lipgloss.NewStyle().Foreground(cmdutil.BrandRed)
)

// loginUI renders the interactive login: a header with the logo, the
// prompts, a line per completed step so earlier answers stay visible, and a
// closing summary. It writes to stderr, like the printer's status messages.
// In the non-interactive mode it is disabled and every method except run is
// a no-op.
//
// A header with the logo is not printed right away: it is drawn together
// with the first prompt, so the logo can wink while the prompt already
// takes input, and stays on screen once the prompt is answered.
type loginUI struct {
	w       io.Writer
	enabled bool
	steps   int // step lines printed so far

	header func(eye [][2]int) string // set while the logo header is pending
	queued []string                  // step lines printed after the pending header
}

func newLoginUI(w io.Writer, interactive bool) *loginUI {
	return &loginUI{w: w, enabled: interactive}
}

// start shows the title block, with the logo to its right when the terminal
// is wide enough and shows colors. The logo waits for the first prompt.
func (u *loginUI) start() {
	if !u.enabled {
		return
	}
	text := lipgloss.JoinVertical(lipgloss.Left,
		loginTitleStyle.Render("Log in to a Redmine server"),
		loginMutedStyle.Render("Press ctrl+c to cancel at any time."),
	)
	const gap = 4
	block := func(eye [][2]int) string {
		var lines []string
		if eye == nil {
			lines = strings.Split(text, "\n")
		} else {
			logo := renderLoginLogo(eye)
			lines = strings.Split(lipgloss.JoinHorizontal(lipgloss.Center, text, strings.Repeat(" ", gap), logo), "\n")
		}
		var b strings.Builder
		b.WriteString("\n")
		for _, line := range lines {
			b.WriteString(strings.TrimRight("  "+line, " ") + "\n")
		}
		b.WriteString("\n")
		return b.String()
	}

	// The first form is drawn below the header in one view; bubbletea only
	// keeps as many lines as the terminal has rows, so a short terminal gets
	// the plain header.
	logoWidth := lipgloss.Width(renderLoginLogo(loginEyeOpen))
	width, height := terminalSize(u.w)
	if richTerminal(u.w) && 2+lipgloss.Width(text)+gap+logoWidth <= width && height >= loginMinHeight {
		u.header = block
		return
	}
	fmt.Fprint(u.w, block(nil))
}

// flush prints a pending header, without the wink, and the step lines
// queued after it.
func (u *loginUI) flush() {
	if u.header == nil {
		return
	}
	fmt.Fprint(u.w, u.header(loginEyeOpen)+strings.Join(u.queued, ""))
	u.header, u.queued = nil, nil
}

// run shows the fields as one themed form, the first one below the pending
// header.
func (u *loginUI) run(fields ...huh.Field) error {
	if u.header == nil {
		return runLoginForm(u.steps > 0, fields...)
	}
	above := u.header
	queued := strings.Join(u.queued, "")
	u.header, u.queued = nil, nil
	return runWinkingForm(u.w, func(eye [][2]int) string { return above(eye) + queued },
		newLoginForm(u.steps > 0, fields...))
}

// newLoginForm builds a themed form of the fields. Below step lines the
// form keeps a blank line to them; huh clears the form, margin included,
// once it is answered, so the step lines stay contiguous.
func newLoginForm(belowSteps bool, fields ...huh.Field) *huh.Form {
	theme := cmdutil.FormTheme()
	if belowSteps {
		theme.Form.Base = theme.Form.Base.MarginTop(1)
	}
	return huh.NewForm(huh.NewGroup(fields...)).WithTheme(theme)
}

// runLoginForm shows the fields as one themed form.
func runLoginForm(belowSteps bool, fields ...huh.Field) error {
	return newLoginForm(belowSteps, fields...).Run()
}

// step prints a completed step. value comes from the user or the server and
// is sanitized here.
func (u *loginUI) step(label, value string) {
	u.line(true, label, output.SanitizeTerminal(value), lipgloss.NewStyle())
}

// stepMuted prints a completed step whose value is a default or "none".
func (u *loginUI) stepMuted(label, value string) {
	u.line(true, label, output.SanitizeTerminal(value), loginMutedStyle)
}

// fail prints a failed step.
func (u *loginUI) fail(label, value string) {
	u.line(false, label, output.SanitizeTerminal(value), loginFailStyle)
}

func (u *loginUI) line(ok bool, label, value string, valueStyle lipgloss.Style) {
	if !u.enabled {
		return
	}
	mark := loginOKStyle.Render("✓")
	if !ok {
		mark = loginFailStyle.Render("✗")
	}
	line := fmt.Sprintf("  %s %s%s\n", mark, loginLabelStyle.Render(label), valueStyle.Render(value))
	u.steps++
	if u.header != nil {
		u.queued = append(u.queued, line)
		return
	}
	fmt.Fprint(u.w, line)
}

// done prints the closing summary after the profile was saved.
func (u *loginUI) done(profile, configPath string) {
	if !u.enabled {
		return
	}
	u.flush()
	fmt.Fprintln(u.w)
	fmt.Fprintf(u.w, "  %s\n", loginOKStyle.Bold(true).Render(
		fmt.Sprintf("Logged in. Profile %q is saved and active.", output.SanitizeTerminal(profile))))
	fmt.Fprintf(u.w, "  %s\n\n", loginMutedStyle.Render("Config: "+output.SanitizeTerminal(configPath)))
	fmt.Fprintf(u.w, "  %s %s\n", loginMutedStyle.Render("Next:"), loginCmdStyle.Render("redmine issue list"))
	fmt.Fprintln(u.w)
}

// The logo is the arch of the favicon (docs/public/favicon.svg) with the
// terminal prompt ">_" under it, whose ">" winks once when the logo appears.

// loginLogoArch is the left half of the arch, mirrored for the right half.
// Each character is a pixel: '.' empty, 'a' and 'b' the two reds of the
// alternating blocks. Two pixel rows make one terminal line, drawn with half
// blocks.
var loginLogoArch = []string{
	"..........bb",
	"......aaa.bb",
	"......aaa.bb",
	"...bbb.aa.bb",
	"..bbbbb.....",
	"..bbbb......",
	"............",
	".aaaa.......",
	"aaaaa.......",
	"aaaaa.......",
	"aaaaa.......",
	"aaaaa.......",
}

// Pixels {row, column} of the prompt in the full logo: the ">" open and
// closed (a bar at its middle), and the "_".
var (
	loginEyeOpen   = [][2]int{{8, 9}, {9, 10}, {10, 10}, {11, 9}}
	loginEyeClosed = [][2]int{{9, 9}, {9, 10}}
	loginCursor    = [][2]int{{11, 12}, {11, 13}, {11, 14}}
)

const (
	// loginWinkDelay is how long the logo shows before it winks, and
	// loginWinkTime how long the ">" stays closed.
	loginWinkDelay = 350 * time.Millisecond
	loginWinkTime  = 180 * time.Millisecond
)

// renderLoginLogo draws the logo with the given ">" pixels.
func renderLoginLogo(eye [][2]int) string {
	colors := map[byte]lipgloss.TerminalColor{
		'a': cmdutil.BrandRed,
		'b': cmdutil.BrandRedDim,
		'w': lipgloss.AdaptiveColor{Light: "#0b0c11", Dark: "#f5f1e8"},
	}

	rows := make([][]byte, len(loginLogoArch))
	for i, half := range loginLogoArch {
		row := []byte(half)
		for j := len(half) - 1; j >= 0; j-- {
			row = append(row, half[j])
		}
		rows[i] = row
	}
	for _, pixels := range [][][2]int{eye, loginCursor} {
		for _, p := range pixels {
			rows[p[0]][p[1]] = 'w'
		}
	}

	var lines []string
	for y := 0; y+1 < len(rows); y += 2 {
		var b strings.Builder
		top, bottom := rows[y], rows[y+1]
		for x := range top {
			tp, bp := top[x], bottom[x]
			switch {
			case tp == '.' && bp == '.':
				b.WriteByte(' ')
			case tp == bp:
				b.WriteString(lipgloss.NewStyle().Foreground(colors[tp]).Render("█"))
			case tp == '.':
				b.WriteString(lipgloss.NewStyle().Foreground(colors[bp]).Render("▄"))
			case bp == '.':
				b.WriteString(lipgloss.NewStyle().Foreground(colors[tp]).Render("▀"))
			default:
				b.WriteString(lipgloss.NewStyle().Foreground(colors[tp]).Background(colors[bp]).Render("▀"))
			}
		}
		lines = append(lines, b.String())
	}
	return strings.Join(lines, "\n")
}

// winkingForm runs a form below a header whose logo winks once: the header
// is part of the form's view, so the form takes input right away.
type winkingForm struct {
	form   *huh.Form
	above  func(eye [][2]int) string
	closed bool
}

// winkMsg closes or opens the logo's ">".
type winkMsg struct{ closed bool }

func (m *winkingForm) Init() tea.Cmd {
	return tea.Batch(m.form.Init(), tea.Tick(loginWinkDelay, func(time.Time) tea.Msg {
		return winkMsg{closed: true}
	}))
}

func (m *winkingForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if wink, ok := msg.(winkMsg); ok {
		m.closed = wink.closed
		if m.closed {
			return m, tea.Tick(loginWinkTime, func(time.Time) tea.Msg { return winkMsg{closed: false} })
		}
		return m, nil
	}
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}
	return m, cmd
}

// View draws the header and the form. Once the form is done, its view is
// empty and the header, with the ">" open, is what stays on screen.
func (m *winkingForm) View() string {
	eye := loginEyeOpen
	if m.closed && m.form.State == huh.StateNormal {
		eye = loginEyeClosed
	}
	return m.above(eye) + m.form.View()
}

// runWinkingForm runs form below the header drawn by above, like huh's
// Form.Run.
func runWinkingForm(w io.Writer, above func(eye [][2]int) string, form *huh.Form) error {
	form.SubmitCmd = tea.Quit
	form.CancelCmd = tea.Interrupt
	_, err := tea.NewProgram(&winkingForm{form: form, above: above},
		tea.WithOutput(w), tea.WithReportFocus()).Run()
	if form.State == huh.StateAborted || errors.Is(err, tea.ErrInterrupted) {
		return huh.ErrUserAborted
	}
	if err != nil {
		return fmt.Errorf("huh: %w", err)
	}
	return nil
}

// richTerminal reports whether w is a terminal that shows colors, which the
// logo needs. A dumb terminal makes huh fall back to its accessible mode,
// which the winking form does not have.
func richTerminal(w io.Writer) bool {
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	width, _ := terminalSize(w)
	return lipgloss.ColorProfile() != termenv.Ascii && width > 0
}

// terminalSize returns the size of the terminal w writes to, or zeros.
func terminalSize(w io.Writer) (width, height int) {
	if file, ok := w.(*os.File); ok {
		if width, height, err := term.GetSize(int(file.Fd())); err == nil {
			return width, height
		}
	}
	return 0, 0
}

// maskSecret returns a fixed-width mask so the length of the secret is not
// revealed.
func maskSecret(s string) string {
	if s == "" {
		return "(empty)"
	}
	return strings.Repeat("•", 8)
}

// authMethodLabel names an auth method for the step summary.
func authMethodLabel(method string) string {
	if method == "basic" {
		return "Username & password"
	}
	return "API key"
}
