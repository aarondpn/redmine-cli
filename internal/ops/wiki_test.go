package ops

import (
	"strings"
	"testing"
)

// TestUpdateWikiPage_RejectsZeroVersion guards the ops-layer boundary: the
// CLI already validates --expect-version, but MCP clients (or any other
// programmatic caller) reach this function directly and could send a
// pointer-to-zero. The JSON omitempty rule on *int would serialize that as
// `"version":0`, which Redmine rejects — we'd rather catch it locally with
// a clear message than round-trip a useless request.
func TestUpdateWikiPage_RejectsZeroVersion(t *testing.T) {
	zero := 0
	negative := -3

	for name, version := range map[string]*int{
		"zero":     &zero,
		"negative": &negative,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := UpdateWikiPage(t.Context(), nil, UpdateWikiPageInput{
				ProjectID: "proj",
				Page:      "MyPage",
				Version:   version,
			})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), ">= 1") {
				t.Errorf("error = %q, want it to mention the >= 1 constraint", err.Error())
			}
		})
	}
}

// TestUpdateWikiPage_SectionGuards covers the ops-layer section validation.
// These guards matter most for MCP/programmatic callers that bypass the CLI
// flag checks: a non-positive section, a section edit without replacement
// text (which would otherwise resend the whole page into one section), or a
// section_hash with no section to anchor it. A nil client is safe because
// validation runs before any request is made.
func TestUpdateWikiPage_SectionGuards(t *testing.T) {
	str := func(s string) *string { return &s }
	intp := func(i int) *int { return &i }
	text := str("section body")

	cases := []struct {
		name    string
		input   UpdateWikiPageInput
		wantSub string
	}{
		{
			name:    "zero section",
			input:   UpdateWikiPageInput{ProjectID: "proj", Page: "MyPage", Text: text, Section: intp(0)},
			wantSub: "section must be >= 1",
		},
		{
			name:    "negative section",
			input:   UpdateWikiPageInput{ProjectID: "proj", Page: "MyPage", Text: text, Section: intp(-2)},
			wantSub: "section must be >= 1",
		},
		{
			name:    "section without text",
			input:   UpdateWikiPageInput{ProjectID: "proj", Page: "MyPage", Section: intp(2)},
			wantSub: "section update requires text",
		},
		{
			name:    "section_hash without section",
			input:   UpdateWikiPageInput{ProjectID: "proj", Page: "MyPage", Text: text, SectionHash: str("abc123")},
			wantSub: "section_hash requires section",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := UpdateWikiPage(t.Context(), nil, tc.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}
