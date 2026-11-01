package auth

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripSGR removes color and style escapes.
func stripSGR(s string) string { return sgr.ReplaceAllString(s, "") }

func TestLoginUIDisabledPrintsNothing(t *testing.T) {
	var buf bytes.Buffer
	ui := newLoginUI(&buf, false)
	ui.start()
	ui.step("Server", "https://redmine.example.com")
	ui.fail("Connection", "failed")
	ui.done("work", "/tmp/config.yaml")
	if buf.Len() != 0 {
		t.Fatalf("disabled UI wrote %q", buf.String())
	}
}

func TestLoginUIPlainHeaderWithoutTerminal(t *testing.T) {
	var buf bytes.Buffer
	ui := newLoginUI(&buf, true)
	ui.start()
	if ui.header != nil {
		t.Fatal("logo header must not be pending when not writing to a terminal")
	}
	got := stripSGR(buf.String())
	if !strings.Contains(got, "Log in to a Redmine server") {
		t.Fatalf("header missing: %q", got)
	}
	if strings.Contains(got, "█") {
		t.Fatalf("logo drawn without a terminal: %q", got)
	}
}

func TestLoginUIQueuesStepsBehindPendingHeader(t *testing.T) {
	var buf bytes.Buffer
	ui := newLoginUI(&buf, true)
	ui.header = func([][2]int) string { return "HEADER\n" }

	ui.step("Server", "https://redmine.example.com")
	if buf.Len() != 0 {
		t.Fatalf("step printed before the pending header: %q", buf.String())
	}

	ui.flush()
	ui.step("Profile", "work")
	ui.flush() // no-op once the header is out

	got := stripSGR(buf.String())
	want := "HEADER\n" +
		"  ✓ Server            https://redmine.example.com\n" +
		"  ✓ Profile           work\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestLoginUIStepSanitizesValues(t *testing.T) {
	var buf bytes.Buffer
	ui := newLoginUI(&buf, true)
	ui.step("Connected as", "Eve \x1b]0;pwned\x07(eve)")
	if strings.Contains(buf.String(), "\x1b]0;") {
		t.Fatalf("terminal escape passed through: %q", buf.String())
	}
}

func TestWinkingFormClosesEyeOnlyWhileFormIsActive(t *testing.T) {
	var eyes [][][2]int
	m := &winkingForm{
		form: huh.NewForm(huh.NewGroup(huh.NewInput())),
		above: func(eye [][2]int) string {
			eyes = append(eyes, eye)
			return ""
		},
	}

	m.Update(winkMsg{closed: true})
	m.View()
	m.form.State = huh.StateCompleted
	m.View()
	m.Update(winkMsg{closed: false})
	m.form.State = huh.StateNormal
	m.View()

	want := [][][2]int{loginEyeClosed, loginEyeOpen, loginEyeOpen}
	for i := range want {
		if len(eyes[i]) != len(want[i]) || eyes[i][0] != want[i][0] {
			t.Fatalf("view %d drew eye %v, want %v", i, eyes[i], want[i])
		}
	}
}

func TestRenderLoginLogoEyeStatesKeepSize(t *testing.T) {
	open := strings.Split(stripSGR(renderLoginLogo(loginEyeOpen)), "\n")
	closed := strings.Split(stripSGR(renderLoginLogo(loginEyeClosed)), "\n")
	if len(open) != len(closed) {
		t.Fatalf("wink changes the logo height: %d vs %d lines", len(open), len(closed))
	}
	for i := range open {
		if lipgloss.Width(open[i]) != lipgloss.Width(closed[i]) {
			t.Fatalf("wink changes the width of line %d", i)
		}
	}
}

func TestMaskSecret(t *testing.T) {
	if got := maskSecret(""); got != "(empty)" {
		t.Errorf("maskSecret(\"\") = %q", got)
	}
	if maskSecret("short") != maskSecret("a-much-longer-secret-value") {
		t.Error("mask reveals the secret length")
	}
}

func TestProjectLabel(t *testing.T) {
	projects := []models.Project{{Name: "Website", Identifier: "website"}}
	if got := projectLabel(projects, "website"); got != "Website (website)" {
		t.Errorf("known project = %q", got)
	}
	if got := projectLabel(projects, "other"); got != "other" {
		t.Errorf("unknown project = %q", got)
	}
}
