package cmdutil

import (
	"testing"

	"github.com/aarondpn/redmine-cli/internal/output"
)

// spyPrinter records calls made to the Printer interface.
type spyPrinter struct {
	format       string
	jsonCalls    []interface{}
	warningCalls []string
}

func newSpyPrinter(format string) *spyPrinter {
	return &spyPrinter{format: format}
}

func (s *spyPrinter) Format() string                          { return s.format }
func (s *spyPrinter) JSON(v interface{})                      { s.jsonCalls = append(s.jsonCalls, v) }
func (s *spyPrinter) Warning(msg string)                      { s.warningCalls = append(s.warningCalls, msg) }
func (s *spyPrinter) Table(headers []string, rows [][]string) {}
func (s *spyPrinter) Detail(pairs []output.KeyValue)          {}
func (s *spyPrinter) CSV(headers []string, rows [][]string)   {}
func (s *spyPrinter) Success(msg string)                      {}
func (s *spyPrinter) Error(msg string)                        {}
func (s *spyPrinter) Action(action, resource string, id any, humanMsg string) {
	_ = action
	_ = resource
	_ = id
	_ = humanMsg
}
func (s *spyPrinter) Spinner(msg string) func() { return func() {} }

// --- DefaultProject tests ---

func TestDefaultProject_NonEmpty(t *testing.T) {
	f := testFactoryWithConfig(t, "default_project: fallback\n")

	got := DefaultProject(f, "explicit")
	if got != "explicit" {
		t.Errorf("DefaultProject() = %q, want %q", got, "explicit")
	}
}

func TestDefaultProject_EmptyWithDefault(t *testing.T) {
	f := testFactoryWithConfig(t, "default_project: fallback\n")

	got := DefaultProject(f, "")
	if got != "fallback" {
		t.Errorf("DefaultProject() = %q, want %q", got, "fallback")
	}
}

func TestDefaultProject_EmptyWithNoDefault(t *testing.T) {
	f := testFactoryWithConfig(t, "server: https://example.com\n")

	got := DefaultProject(f, "")
	if got != "" {
		t.Errorf("DefaultProject() = %q, want %q", got, "")
	}
}

func TestDefaultProject_ConfigError(t *testing.T) {
	f := &Factory{
		ConfigPath: "/nonexistent/path/config.yaml",
		IOStreams:  &IOStreams{},
	}

	got := DefaultProject(f, "")
	if got != "" {
		t.Errorf("DefaultProject() = %q, want %q", got, "")
	}
}

// --- RequireProjectIdentifier tests ---

func TestRequireProjectIdentifier_EmptyNoDefault(t *testing.T) {
	f := testFactoryWithConfig(t, "server: https://example.com\napi_key: test\n")

	_, err := RequireProjectIdentifier(t.Context(), f, "")
	if err == nil {
		t.Fatal("expected error for empty project with no default")
	}
	if got := err.Error(); got != "project is required (use --project or set a default project)" {
		t.Errorf("error = %q, want project-required message", got)
	}
}

// --- HandleEmpty tests ---

func TestHandleEmpty_NonEmpty(t *testing.T) {
	spy := newSpyPrinter(output.FormatTable)

	items := []string{"a", "b"}
	if HandleEmpty(spy, items, "things") {
		t.Error("HandleEmpty() returned true for non-empty slice")
	}
	if len(spy.jsonCalls) > 0 || len(spy.warningCalls) > 0 {
		t.Error("HandleEmpty() should not produce output for non-empty slice")
	}
}

func TestHandleEmpty_EmptyJSON(t *testing.T) {
	spy := newSpyPrinter(output.FormatJSON)

	var items []string
	if !HandleEmpty(spy, items, "things") {
		t.Error("HandleEmpty() returned false for empty slice")
	}
	if len(spy.jsonCalls) != 1 {
		t.Fatalf("expected 1 JSON call, got %d", len(spy.jsonCalls))
	}
	if len(spy.warningCalls) > 0 {
		t.Error("HandleEmpty() should not emit warning for JSON format")
	}
}

func TestHandleEmpty_EmptyTable(t *testing.T) {
	spy := newSpyPrinter(output.FormatTable)

	var items []int
	if !HandleEmpty(spy, items, "widgets") {
		t.Error("HandleEmpty() returned false for empty slice")
	}
	if len(spy.warningCalls) != 1 {
		t.Fatalf("expected 1 warning call, got %d", len(spy.warningCalls))
	}
	if spy.warningCalls[0] != "No widgets found" {
		t.Errorf("warning = %q, want %q", spy.warningCalls[0], "No widgets found")
	}
	if len(spy.jsonCalls) > 0 {
		t.Error("HandleEmpty() should not emit JSON for table format")
	}
}

func TestHandleEmpty_EmptyCSV(t *testing.T) {
	spy := newSpyPrinter(output.FormatCSV)

	var items []int
	if !HandleEmpty(spy, items, "entries") {
		t.Error("HandleEmpty() returned false for empty slice")
	}
	if len(spy.warningCalls) != 1 {
		t.Fatalf("expected 1 warning call, got %d", len(spy.warningCalls))
	}
}

// --- WarnPagination tests ---

func TestWarnPagination_LimitZero(t *testing.T) {
	spy := newSpyPrinter(output.FormatTable)

	WarnPagination(spy, PaginationResult{Shown: 10, Total: 100, Limit: 0, Offset: 0, Noun: "items"})
	if len(spy.warningCalls) > 0 {
		t.Error("WarnPagination() should not warn when limit=0")
	}
}

func TestWarnPagination_AllShown(t *testing.T) {
	spy := newSpyPrinter(output.FormatTable)

	WarnPagination(spy, PaginationResult{Shown: 10, Total: 10, Limit: 25, Offset: 0, Noun: "items"})
	if len(spy.warningCalls) > 0 {
		t.Error("WarnPagination() should not warn when all results shown")
	}
}

func TestWarnPagination_MoreAvailable(t *testing.T) {
	spy := newSpyPrinter(output.FormatTable)

	WarnPagination(spy, PaginationResult{Shown: 25, Total: 100, Limit: 25, Offset: 0, Noun: "issues"})
	if len(spy.warningCalls) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(spy.warningCalls))
	}
	want := "Showing 25 of 100 issues. Use --offset to paginate."
	if spy.warningCalls[0] != want {
		t.Errorf("warning = %q, want %q", spy.warningCalls[0], want)
	}
}

func TestWarnPagination_JSONSuppressed(t *testing.T) {
	spy := newSpyPrinter(output.FormatJSON)

	WarnPagination(spy, PaginationResult{Shown: 25, Total: 100, Limit: 25, Offset: 0, Noun: "issues"})
	if len(spy.warningCalls) > 0 {
		t.Error("WarnPagination() should not warn for JSON format")
	}
}

func TestWarnPagination_WithOffset(t *testing.T) {
	spy := newSpyPrinter(output.FormatTable)

	WarnPagination(spy, PaginationResult{Shown: 25, Total: 100, Limit: 25, Offset: 50, Noun: "items"})
	if len(spy.warningCalls) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(spy.warningCalls))
	}
}

func TestWarnPagination_OffsetExhaustsResults(t *testing.T) {
	spy := newSpyPrinter(output.FormatTable)

	// limit + offset >= total means no more pages
	WarnPagination(spy, PaginationResult{Shown: 25, Total: 50, Limit: 25, Offset: 25, Noun: "items"})
	if len(spy.warningCalls) > 0 {
		t.Error("WarnPagination() should not warn when offset+limit covers total")
	}
}
