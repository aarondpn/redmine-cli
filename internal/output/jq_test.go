package output

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func evalJQ(t *testing.T, expr string, v any, indent bool) (string, error) {
	t.Helper()
	q, err := CompileJQ(expr)
	if err != nil {
		t.Fatalf("CompileJQ(%q): %v", expr, err)
	}
	var buf bytes.Buffer
	err = q.Evaluate(&buf, v, indent)
	return buf.String(), err
}

type jqTestIssue struct {
	ID      int           `json:"id"`
	Subject string        `json:"subject"`
	Status  jqTestStatus  `json:"status"`
	Tags    []string      `json:"tags"`
	Parent  *jqTestStatus `json:"parent,omitempty"`
}

type jqTestStatus struct {
	Name string `json:"name"`
}

func TestJQFilter_Results(t *testing.T) {
	issue := jqTestIssue{ID: 7, Subject: "Crash <on> save", Status: jqTestStatus{Name: "New"}}
	list := []jqTestIssue{
		{ID: 1, Subject: "one", Status: jqTestStatus{Name: "New"}},
		{ID: 2, Subject: "two", Status: jqTestStatus{Name: "Closed"}},
	}

	tests := []struct {
		name  string
		expr  string
		input any
		want  string
	}{
		{"string is printed raw", ".status.name", issue, "New\n"},
		{"string keeps special characters", ".subject", issue, "Crash <on> save\n"},
		{"integer", ".id", issue, "7\n"},
		{"float", "1.5", issue, "1.5\n"},
		{"boolean", ".id == 7", issue, "true\n"},
		{"null", ".parent", issue, "null\n"},
		{"object is compact JSON", "{id, subject}", issue, `{"id":7,"subject":"Crash <on> save"}` + "\n"},
		{"array is compact JSON", "[.[].id]", list, "[1,2]\n"},
		{"multiple results one per line", ".[].id", list, "1\n2\n"},
		{"multiple string results", ".[] | .status.name", list, "New\nClosed\n"},
		{"multiple object results", ".[] | {id}", list, "{\"id\":1}\n{\"id\":2}\n"},
		{"no results", ".[] | select(.id > 10)", list, ""},
		{"nil slice is an empty array", ".", []jqTestIssue(nil), "[]\n"},
		{"nil slice length", "length", []jqTestIssue(nil), "0\n"},
		{"nil struct slice field encodes as in RenderJSON", ".tags", issue, "null\n"},
		{"large integers are preserved", ".id", map[string]any{"id": int64(9007199254740993)}, "9007199254740993\n"},
		{"halt stops without error", "1, halt, 2", issue, "1\n"},
		{"action envelope", ".ok", ActionEnvelope{Ok: true, Action: ActionDeleted, Resource: "issue", ID: 3}, "true\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evalJQ(t, tt.expr, tt.input, false)
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Evaluate(%q) = %q, want %q", tt.expr, got, tt.want)
			}
		})
	}
}

func TestJQFilter_IndentPrettyPrintsContainersOnly(t *testing.T) {
	got, err := evalJQ(t, "{id}, .status.name, .id", jqTestIssue{ID: 7, Status: jqTestStatus{Name: "New"}}, true)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	want := "{\n  \"id\": 7\n}\nNew\n7\n"
	if got != want {
		t.Fatalf("Evaluate = %q, want %q", got, want)
	}
}

func TestCompileJQ_InvalidExpression(t *testing.T) {
	for _, expr := range []string{".[", "foo(", ".a | undefined_function_xyz", "", "  "} {
		t.Run(expr, func(t *testing.T) {
			_, err := CompileJQ(expr)
			var compileErr *JQCompileError
			if !errors.As(err, &compileErr) {
				t.Fatalf("CompileJQ(%q) = %v, want *JQCompileError", expr, err)
			}
			if !strings.Contains(err.Error(), "invalid --jq expression") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestJQFilter_RuntimeErrorWritesNothing(t *testing.T) {
	// The first result is valid; the error on the second must still leave
	// the writer untouched so stdout stays empty on failure.
	got, err := evalJQ(t, ".id, .[0]", jqTestIssue{ID: 7}, false)
	var jqErr *JQRuntimeError
	if !errors.As(err, &jqErr) {
		t.Fatalf("expected *JQRuntimeError, got %T: %v", err, err)
	}
	if !strings.HasPrefix(err.Error(), "command succeeded, but --jq failed: ") {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("expected no output on runtime error, got %q", got)
	}
}

func TestJQFilter_LargeOutputIsStreamed(t *testing.T) {
	// Past jqBufferLimit the output is flushed, so a later runtime error
	// leaves partial output behind instead of holding everything in memory.
	q, err := CompileJQ(`(range(0; 400000) | "line"), error("late failure")`)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err = q.Evaluate(&buf, map[string]any{}, false)
	if err == nil || !strings.Contains(err.Error(), "late failure") {
		t.Fatalf("expected late runtime error, got %v", err)
	}
	if buf.Len() <= jqBufferLimit {
		t.Fatalf("expected streamed partial output above %d bytes, got %d", jqBufferLimit, buf.Len())
	}
}

// limitedWriter accepts n bytes and then fails, like a closed pipe.
type limitedWriter struct{ n int }

func (w *limitedWriter) Write(p []byte) (int, error) {
	if len(p) > w.n {
		return 0, io.ErrClosedPipe
	}
	w.n -= len(p)
	return len(p), nil
}

func TestJQFilter_InfiniteOutputStopsOnWriteError(t *testing.T) {
	// `--jq 'repeat(1)' | head` must terminate with bounded memory once the
	// reader goes away rather than buffering forever.
	q, err := CompileJQ("repeat(1)")
	if err != nil {
		t.Fatal(err)
	}
	err = q.Evaluate(&limitedWriter{n: 4 * jqBufferLimit}, map[string]any{}, false)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("expected write error, got %v", err)
	}
}

func TestJQFilter_HaltErrorIsAnError(t *testing.T) {
	_, err := evalJQ(t, `"boom" | halt_error`, map[string]any{}, false)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected halt_error to surface, got %v", err)
	}
}

func TestStdPrinter_JQFiltersJSONAndEnvelopes(t *testing.T) {
	q, err := CompileJQ(".id")
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	p := NewStdPrinter(&out, &errOut, false, true, FormatJSON)
	var jqErr error
	p.SetJQ(q, func(err error) { jqErr = err })

	p.JSON(jqTestIssue{ID: 7})
	p.Resource(jqTestIssue{ID: 8}, "ignored")
	p.Action(ActionDeleted, "issue", 9, "Deleted issue #9")
	p.Outcome(false, ActionUpdated, "cli", "1.0.0", "Already up to date")

	if jqErr != nil {
		t.Fatalf("unexpected jq error: %v", jqErr)
	}
	// Resource bodies and action envelopes are filtered alike; the string
	// ID of the last envelope is printed raw.
	if got, want := out.String(), "7\n8\n9\n1.0.0\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if errOut.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", errOut.String())
	}
}

func TestStdPrinter_JQRuntimeErrorReported(t *testing.T) {
	q, err := CompileJQ(".[0]")
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	p := NewStdPrinter(&out, &errOut, false, true, FormatJSON)
	var jqErr error
	p.SetJQ(q, func(err error) { jqErr = err })

	p.JSON(map[string]any{"id": 1})

	if jqErr == nil {
		t.Fatal("expected jq runtime error to be reported")
	}
	if out.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", out.String())
	}
}
