package cmdutil

import (
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// DefaultProject returns the given project string if non-empty, otherwise
// falls back to the configured default project. If the config cannot be
// loaded the original (empty) value is returned.
func DefaultProject(f *Factory, project string) string {
	if project != "" {
		return project
	}
	cfg, err := f.Config()
	if err != nil || cfg.DefaultProject == "" {
		return project
	}
	return cfg.DefaultProject
}

// HandleEmpty checks whether items is empty. For empty slices it emits an
// empty JSON array (when the output format is JSON) or a warning (for other
// formats) and returns true. The caller should return nil when true.
func HandleEmpty[T any](p output.Printer, items []T, noun string) bool {
	if len(items) > 0 {
		return false
	}
	if p.Format() == output.FormatJSON {
		p.JSON(items)
		return true
	}
	if output.SupportsWarnings(p.Format()) {
		p.Warning(fmt.Sprintf("No %s found", noun))
	}
	return true
}

// PaginationResult holds the information needed to emit a pagination warning.
type PaginationResult struct {
	Shown  int    // Number of items displayed
	Total  int    // Total count from the API
	Limit  int    // The --limit value used
	Offset int    // The --offset value used
	Noun   string // Plural noun for the message, e.g. "issues"
}

// WarnPagination emits a pagination warning when there are more results than
// shown. No warning is emitted when limit is 0 (all results requested) or
// when the output format is JSON.
func WarnPagination(p output.Printer, r PaginationResult) {
	if r.Limit <= 0 {
		return
	}
	if r.Total <= r.Limit+r.Offset {
		return
	}
	if !output.SupportsWarnings(p.Format()) {
		return
	}
	p.Warning(fmt.Sprintf("Showing %d of %d %s. Use --offset to paginate.", r.Shown, r.Total, r.Noun))
}
