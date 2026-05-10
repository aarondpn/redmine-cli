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
