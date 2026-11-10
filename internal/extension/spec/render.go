package spec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/itchyny/gojq"

	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// View is how a command result is shown in the CLI table format.
type View int

const (
	// ViewJSON: no matching output spec; print the result as pretty JSON.
	ViewJSON View = iota
	// ViewTable: an array result rendered with output.table.
	ViewTable
	// ViewDetail: an object result rendered with output.detail.
	ViewDetail
)

// Rendered is the evaluated output spec, ready for output.Printer.Table or
// output.Printer.Detail. Values are raw extension data; the printer
// sanitizes them for the terminal.
type Rendered struct {
	View    View
	Headers []string
	Rows    [][]string
	Pairs   []output.KeyValue
}

const (
	// renderTimeout bounds jq evaluation when the caller's context has no
	// deadline: manifest expressions come from third parties and jq can
	// loop forever.
	renderTimeout = 5 * time.Second
	// maxCellResults caps how many jq results are joined into one cell.
	maxCellResults = 1000
)

// Render evaluates o against result: arrays use the table spec, objects the
// detail spec. Anything else, or a missing spec, yields ViewJSON. result may
// be any JSON-marshalable value; it is normalized like decoded JSON (with
// json.Number) first, so large IDs keep their exact digits.
func Render(ctx context.Context, o *Output, result any) (*Rendered, error) {
	if o == nil || (len(o.Table) == 0 && len(o.Detail) == 0) {
		return &Rendered{View: ViewJSON}, nil
	}
	v, err := normalizeResult(result)
	if err != nil {
		return nil, err
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, renderTimeout)
		defer cancel()
	}
	switch val := v.(type) {
	case []any:
		if len(o.Table) == 0 {
			break
		}
		headers, rows, err := renderTable(ctx, o.Table, val)
		if err != nil {
			return nil, err
		}
		return &Rendered{View: ViewTable, Headers: headers, Rows: rows}, nil
	case map[string]any:
		if len(o.Detail) == 0 {
			break
		}
		pairs, err := renderDetail(ctx, o.Detail, val)
		if err != nil {
			return nil, err
		}
		return &Rendered{View: ViewDetail, Pairs: pairs}, nil
	}
	return &Rendered{View: ViewJSON}, nil
}

func renderTable(ctx context.Context, cols []Column, items []any) ([]string, [][]string, error) {
	sels := make([]selector, len(cols))
	headers := make([]string, len(cols))
	for i, c := range cols {
		s, err := newSelector(c.Path, c.JQ)
		if err != nil {
			return nil, nil, fmt.Errorf("column %q: %w", c.Header, err)
		}
		sels[i] = s
		headers[i] = c.Header
	}
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		row := make([]string, len(cols))
		for i, s := range sels {
			cell, err := s.eval(ctx, item)
			if err != nil {
				return nil, nil, fmt.Errorf("column %q: %w", cols[i].Header, err)
			}
			row[i] = cell
		}
		rows = append(rows, row)
	}
	return headers, rows, nil
}

func renderDetail(ctx context.Context, fields []DetailField, obj map[string]any) ([]output.KeyValue, error) {
	pairs := make([]output.KeyValue, 0, len(fields))
	for _, f := range fields {
		s, err := newSelector(f.Path, f.JQ)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", f.Key, err)
		}
		val, err := s.eval(ctx, obj)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", f.Key, err)
		}
		pairs = append(pairs, output.KeyValue{Key: f.Key, Value: val})
	}
	return pairs, nil
}

// selector is a compiled path or jq expression.
type selector struct {
	path []string
	code *gojq.Code
}

func newSelector(path, jq string) (selector, error) {
	switch {
	case jq != "" && path != "":
		return selector{}, errors.New("set exactly one of path or jq")
	case jq != "":
		code, err := compileJQ(jq)
		if err != nil {
			return selector{}, fmt.Errorf("invalid jq expression: %w", err)
		}
		return selector{code: code}, nil
	default:
		segs, err := splitPath(path)
		if err != nil {
			return selector{}, err
		}
		return selector{path: segs}, nil
	}
}

func (s selector) eval(ctx context.Context, v any) (string, error) {
	if s.code == nil {
		return formatCell(lookupPath(v, s.path)), nil
	}
	iter := s.code.RunWithContext(ctx, v)
	var parts []string
	for len(parts) < maxCellResults {
		r, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := r.(error); isErr {
			var halt *gojq.HaltError
			if errors.As(err, &halt) && halt.ExitCode() == 0 {
				break
			}
			return "", fmt.Errorf("jq: %w", err)
		}
		if r == nil {
			continue
		}
		parts = append(parts, formatCell(r))
	}
	return strings.Join(parts, ", "), nil
}

// compileJQ compiles a manifest jq expression. Unlike --jq it never gets
// the process environment: the expression comes from a third party and the
// rendered cell is shown to the user.
func compileJQ(expr string) (*gojq.Code, error) {
	if strings.TrimSpace(expr) == "" {
		return nil, errors.New("expression is empty")
	}
	q, err := gojq.Parse(expr)
	if err != nil {
		return nil, err
	}
	return gojq.Compile(q)
}

// splitPath parses dot notation ("author.login", "items.0.name").
func splitPath(path string) ([]string, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("path must not be empty")
	}
	segs := strings.Split(path, ".")
	for _, s := range segs {
		if s == "" {
			return nil, fmt.Errorf("path %q has an empty segment", path)
		}
	}
	return segs, nil
}

// lookupPath follows segs through objects (by key) and arrays (by numeric
// index). A missing step yields nil, rendered as an empty cell.
func lookupPath(v any, segs []string) any {
	for _, seg := range segs {
		switch t := v.(type) {
		case map[string]any:
			v = t[seg]
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(t) {
				return nil
			}
			v = t[i]
		default:
			return nil
		}
	}
	return v
}

// formatCell renders a value for a table cell: scalars as text, lists of
// scalars joined with ", ", anything else as compact JSON.
func formatCell(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	case bool:
		return strconv.FormatBool(t)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case *big.Int:
		return t.String()
	case []any:
		parts := make([]string, len(t))
		for i, item := range t {
			switch item.(type) {
			case map[string]any, []any:
				return compactJSON(t)
			}
			parts[i] = formatCell(item)
		}
		return strings.Join(parts, ", ")
	default:
		return compactJSON(t)
	}
}

func compactJSON(v any) string {
	data, err := gojq.Marshal(v)
	if err != nil {
		data, err = json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
	}
	return string(data)
}

// normalizeResult re-decodes v as generic JSON with json.Number, the shape
// both the path lookup and gojq expect.
func normalizeResult(v any) (any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var out any
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	return out, nil
}
