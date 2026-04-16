package output

import (
	"fmt"
	"io"
)

// Printer handles all output rendering.
type Printer interface {
	Table(headers []string, rows [][]string)
	Detail(pairs []KeyValue)
	JSON(v interface{})
	CSV(headers []string, rows [][]string)
	Success(msg string)
	Error(msg string)
	Warning(msg string)
	// Outcome emits a completed command result. In JSON mode this writes a
	// stable envelope to stdout. In other modes it writes the human message to
	// stderr as either success or warning text.
	Outcome(ok bool, action, resource string, id any, humanMsg string)
	// Resource emits a resource body in JSON mode and a human success message
	// in other formats.
	Resource(v any, humanMsg string)
	// Action emits a completed-mutation result. In JSON mode this writes a
	// compact action envelope to stdout. In any other mode it writes the
	// human message to stderr via Success.
	Action(action, resource string, id any, humanMsg string)
	Spinner(msg string) func()
	Format() string
}

// KeyValue is a label-value pair for detail views.
type KeyValue struct {
	Key   string
	Value string
}

// StdPrinter is the standard implementation of Printer.
type StdPrinter struct {
	out     io.Writer
	errOut  io.Writer
	isTTY   bool
	noColor bool
	format  string
}

// NewStdPrinter creates a new StdPrinter.
func NewStdPrinter(out, errOut io.Writer, isTTY, noColor bool, format string) *StdPrinter {
	if format == "" {
		format = FormatTable
	}
	// When not a TTY, default to plain output
	if !isTTY && !noColor {
		noColor = true
	}
	return &StdPrinter{
		out:     out,
		errOut:  errOut,
		isTTY:   isTTY,
		noColor: noColor,
		format:  format,
	}
}

func (p *StdPrinter) Format() string {
	return p.format
}

func (p *StdPrinter) Table(headers []string, rows [][]string) {
	RenderTable(p.out, headers, rows, p.noColor)
}

func (p *StdPrinter) Detail(pairs []KeyValue) {
	maxKeyLen := 0
	for _, kv := range pairs {
		if len(kv.Key) > maxKeyLen {
			maxKeyLen = len(kv.Key)
		}
	}
	for _, kv := range pairs {
		if p.noColor {
			fmt.Fprintf(p.out, "%-*s  %s\n", maxKeyLen, kv.Key+":", kv.Value)
		} else {
			fmt.Fprintf(p.out, "%s  %s\n",
				StyleLabel.Render(fmt.Sprintf("%-*s", maxKeyLen, kv.Key+":")),
				kv.Value,
			)
		}
	}
}

func (p *StdPrinter) JSON(v interface{}) {
	RenderJSON(p.out, v)
}

func (p *StdPrinter) CSV(headers []string, rows [][]string) {
	RenderCSV(p.out, headers, rows)
}

func (p *StdPrinter) Success(msg string) {
	if p.noColor {
		fmt.Fprintf(p.errOut, "OK: %s\n", msg)
	} else {
		fmt.Fprintln(p.errOut, StyleSuccess.Render("✓ "+msg))
	}
}

func (p *StdPrinter) Error(msg string) {
	if p.noColor {
		fmt.Fprintf(p.errOut, "ERROR: %s\n", msg)
	} else {
		fmt.Fprintln(p.errOut, StyleError.Render("✗ "+msg))
	}
}

func (p *StdPrinter) Warning(msg string) {
	if p.noColor {
		fmt.Fprintf(p.errOut, "WARNING: %s\n", msg)
	} else {
		fmt.Fprintln(p.errOut, StyleWarning.Render("! "+msg))
	}
}

func (p *StdPrinter) Outcome(ok bool, action, resource string, id any, humanMsg string) {
	if p.format == FormatJSON {
		_ = RenderActionJSON(p.out, ActionEnvelope{
			Ok:       ok,
			Action:   action,
			Resource: resource,
			ID:       id,
			Message:  humanMsg,
		})
		return
	}
	if ok {
		p.Success(humanMsg)
		return
	}
	p.Warning(humanMsg)
}

func (p *StdPrinter) Resource(v any, humanMsg string) {
	if p.format == FormatJSON {
		p.JSON(v)
		return
	}
	p.Success(humanMsg)
}

func (p *StdPrinter) Action(action, resource string, id any, humanMsg string) {
	p.Outcome(true, action, resource, id, humanMsg)
}

func (p *StdPrinter) Spinner(msg string) func() {
	return StartSpinner(msg, p.isTTY && !p.noColor)
}
