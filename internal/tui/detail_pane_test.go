package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

const testServer = "https://redmine.example.com"

var openKey = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}}

// stubOpenURL replaces openURL for the test and returns the URLs it was
// asked to open.
func stubOpenURL(t *testing.T) *[]string {
	t.Helper()
	var opened []string
	orig := openURL
	t.Cleanup(func() { openURL = orig })
	openURL = func(rawURL string) error {
		opened = append(opened, rawURL)
		return nil
	}
	return &opened
}

func TestSearchResultURL(t *testing.T) {
	tests := []struct {
		name   string
		server string
		result models.SearchResult
		want   string
	}{
		{
			name:   "issue is built from the configured server",
			server: testServer + "/",
			result: models.SearchResult{ID: 7, Type: "issue", URL: "https://evil.example/issues/7"},
			want:   testServer + "/issues/7",
		},
		{
			name:   "closed issue is built from the configured server",
			server: testServer,
			result: models.SearchResult{ID: 8, Type: "issue-closed", URL: "-aNoSuchApp"},
			want:   testServer + "/issues/8",
		},
		{
			name:   "other types fall back to the reported URL",
			server: testServer,
			result: models.SearchResult{ID: 3, Type: "wiki-page", URL: testServer + "/projects/p/wiki/Home"},
			want:   testServer + "/projects/p/wiki/Home",
		},
		{
			name:   "no server falls back to the reported URL",
			result: models.SearchResult{ID: 7, Type: "issue", URL: testServer + "/issues/7"},
			want:   testServer + "/issues/7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := searchResultURL(tt.server, &tt.result); got != tt.want {
				t.Errorf("searchResultURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetailPane_OpenBrowser(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		afterFirst  int // launches after the first press of the open key
		afterSecond int // launches after the second press
		status      string
	}{
		{name: "configured server opens directly", url: testServer + "/news/1", afterFirst: 1, afterSecond: 2, status: "Opened in browser"},
		{name: "other host needs confirmation", url: "https://evil.example/phish", afterFirst: 0, afterSecond: 1, status: "https://evil.example/phish"},
		{name: "userinfo lookalike names the real host", url: "https://redmine.example.com@evil.example/", afterFirst: 0, afterSecond: 1, status: "for evil.example:"},
		{name: "leading dash is refused", url: "-aNoSuchApp", status: "Refusing to open unsafe URL"},
		{name: "file URL is refused", url: "file:///etc/passwd", status: "Refusing to open unsafe URL"},
		{name: "javascript URL is refused", url: "javascript:alert(1)", status: "Refusing to open unsafe URL"},
		{name: "custom scheme is refused", url: "vscode://file/etc/passwd", status: "Refusing to open unsafe URL"},
		{name: "scheme relative URL is refused", url: "//evil.example/", status: "Refusing to open unsafe URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opened := stubOpenURL(t)
			d := NewDetailPane(testServer)
			d.SetSearchContent(&models.SearchResult{ID: 1, Type: "news", Title: "t", URL: tt.url})
			keys := DefaultKeyMap()

			d.Update(openKey, keys)
			if !strings.Contains(d.statusMsg, tt.status) {
				t.Errorf("status after first press = %q, want it to contain %q", d.statusMsg, tt.status)
			}
			if len(*opened) != tt.afterFirst {
				t.Fatalf("launched %d times after first press (%q), want %d", len(*opened), *opened, tt.afterFirst)
			}

			d.Update(openKey, keys)
			if len(*opened) != tt.afterSecond {
				t.Fatalf("launched %d times after second press (%q), want %d", len(*opened), *opened, tt.afterSecond)
			}
		})
	}
}

func TestDetailPane_ConfirmationResets(t *testing.T) {
	keys := DefaultKeyMap()
	result := &models.SearchResult{ID: 1, Type: "news", Title: "t", URL: "https://evil.example/"}

	t.Run("new content", func(t *testing.T) {
		opened := stubOpenURL(t)
		d := NewDetailPane(testServer)
		d.SetSearchContent(result)
		d.Update(openKey, keys)
		d.SetSearchContent(result)
		d.Update(openKey, keys)
		if len(*opened) != 0 {
			t.Fatalf("opened %q without a fresh confirmation", *opened)
		}
	})
	t.Run("status timeout", func(t *testing.T) {
		opened := stubOpenURL(t)
		d := NewDetailPane(testServer)
		d.SetSearchContent(result)
		d.Update(openKey, keys)
		d.Update(clearStatusMsg{seq: d.statusSeq}, keys)
		d.Update(openKey, keys)
		if len(*opened) != 0 {
			t.Fatalf("opened %q after the confirmation expired", *opened)
		}
	})
	t.Run("stale timer does not clear the prompt", func(t *testing.T) {
		opened := stubOpenURL(t)
		d := NewDetailPane(testServer)
		d.SetSearchContent(result)
		stale := d.statusSeq
		d.Update(openKey, keys)
		d.Update(clearStatusMsg{seq: stale}, keys)
		d.Update(openKey, keys)
		if len(*opened) != 1 {
			t.Fatalf("opened %q, want one open after confirming", *opened)
		}
	})
}

func TestDetailPane_SanitizesServerData(t *testing.T) {
	const evil = "\x1b]52;c;ZXZpbA==\x07\x1b[2K\r\u202E"
	d := NewDetailPane(testServer)
	d.SetSize(120, 40)
	d.SetIssueContent(&models.Issue{
		ID:          1,
		Subject:     "Subject" + evil,
		Description: "Body" + evil,
		Project:     models.IDName{Name: "Project" + evil},
		Status:      models.IssueStatusRef{Name: "New"},
		Author:      models.IDName{Name: "Author" + evil},
	})
	view := d.ViewFocused(true)
	for _, bad := range []string{"\x1b]", "\x07", "\x1b[2K", "\r", "\u202E"} {
		if strings.Contains(view, bad) {
			t.Errorf("detail view contains %q: %q", bad, view)
		}
	}
	if !strings.Contains(view, "^[]52;c;") {
		t.Errorf("expected neutralized escape sequence in view: %q", view)
	}
	// The raw value is still what gets copied.
	if d.fields[0].value != "Project"+evil {
		t.Errorf("field value was modified: %q", d.fields[0].value)
	}
}

func TestListItems_SanitizeServerData(t *testing.T) {
	const evil = "\x1b]8;;https://evil.example/\x1b\\x\u202E"
	issue := issueItem{issue: models.Issue{
		ID:       1,
		Subject:  evil,
		Tracker:  models.IDName{Name: evil},
		Status:   models.IssueStatusRef{Name: evil},
		Priority: models.IDName{Name: evil},
	}}
	search := searchItem{result: models.SearchResult{Title: evil, Type: evil}}
	for name, s := range map[string]string{
		"issue title":        issue.Title(),
		"issue description":  issue.Description(),
		"issue filter":       issue.FilterValue(),
		"search title":       search.Title(),
		"search description": search.Description(),
		"search filter":      search.FilterValue(),
	} {
		if strings.ContainsAny(s, "\x1b\u202E") {
			t.Errorf("%s not sanitized: %q", name, s)
		}
	}
}
