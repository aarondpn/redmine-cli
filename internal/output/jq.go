package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/itchyny/gojq"
)

// jqBufferLimit is how much filtered output Evaluate holds back before it
// starts streaming. Below the limit a runtime error leaves stdout empty;
// above it, memory stays bounded (e.g. `--jq 'repeat(1)' | head`) at the
// cost of partial output when a later result fails.
const jqBufferLimit = 1 << 20

// JQFilter is a compiled --jq expression that is applied to every JSON
// document a command would otherwise print. String results are printed raw
// (without quotes), every other value is printed as JSON, and each result
// goes on its own line.
type JQFilter struct {
	code *gojq.Code
}

// JQRuntimeError reports that a --jq expression failed while filtering the
// output of a command that itself completed successfully. Callers must not
// treat it as a failure of the underlying operation (e.g. a create that
// already happened).
type JQRuntimeError struct {
	Err error
}

func (e *JQRuntimeError) Error() string {
	return "command succeeded, but --jq failed: " + e.Err.Error()
}

func (e *JQRuntimeError) Unwrap() error { return e.Err }

// JQCompileError reports a --jq expression that is empty or does not parse
// or compile. It is raised before the command runs.
type JQCompileError struct {
	Err error
}

func (e *JQCompileError) Error() string {
	return "invalid --jq expression: " + e.Err.Error()
}

func (e *JQCompileError) Unwrap() error { return e.Err }

// CompileJQ parses and compiles a jq expression. Compiling up front lets
// callers reject a malformed expression before any API request is made.
// Failures are returned as *JQCompileError.
func CompileJQ(expr string) (*JQFilter, error) {
	if strings.TrimSpace(expr) == "" {
		return nil, &JQCompileError{Err: errors.New("expression is empty")}
	}
	query, err := gojq.Parse(expr)
	if err != nil {
		return nil, &JQCompileError{Err: err}
	}
	code, err := gojq.Compile(query, gojq.WithEnvironLoader(os.Environ))
	if err != nil {
		return nil, &JQCompileError{Err: err}
	}
	return &JQFilter{code: code}, nil
}

// Evaluate runs the filter against the JSON encoding of v and writes the
// results to w. The input is exactly the JSON that RenderJSON would print, so
// expressions can be written against the documented output shape.
//
// When indent is true, object and array results are pretty-printed;
// otherwise they are written compactly, one per line. Results are buffered
// so a runtime error leaves w untouched, unless the output grows past
// jqBufferLimit, in which case it is streamed. Filter failures are returned
// as *JQRuntimeError.
func (q *JQFilter) Evaluate(w io.Writer, v any, indent bool) error {
	data, err := json.Marshal(normalizeJSONValue(v))
	if err != nil {
		return &JQRuntimeError{Err: err}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var input any
	if err := dec.Decode(&input); err != nil {
		return &JQRuntimeError{Err: err}
	}

	out := &jqWriter{w: w}
	iter := q.code.Run(input)
	for {
		result, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := result.(error); isErr {
			var halt *gojq.HaltError
			if errors.As(err, &halt) && halt.Value() == nil {
				break
			}
			return &JQRuntimeError{Err: err}
		}
		if err := out.writeResult(result, indent); err != nil {
			return err
		}
	}
	return out.flush()
}

// jqWriter buffers filter results up to jqBufferLimit and streams them once
// the limit is exceeded.
type jqWriter struct {
	w         io.Writer
	buf       bytes.Buffer
	streaming bool
}

func (o *jqWriter) writeResult(v any, indent bool) error {
	if s, ok := v.(string); ok {
		o.buf.WriteString(s)
		o.buf.WriteByte('\n')
		return o.maybeStream()
	}
	data, err := gojq.Marshal(v)
	if err != nil {
		return &JQRuntimeError{Err: err}
	}
	wrote := false
	if indent {
		switch v.(type) {
		case map[string]any, []any:
			if err := json.Indent(&o.buf, data, "", "  "); err != nil {
				return &JQRuntimeError{Err: err}
			}
			wrote = true
		}
	}
	if !wrote {
		o.buf.Write(data)
	}
	o.buf.WriteByte('\n')
	return o.maybeStream()
}

func (o *jqWriter) maybeStream() error {
	if !o.streaming && o.buf.Len() <= jqBufferLimit {
		return nil
	}
	o.streaming = true
	return o.flush()
}

func (o *jqWriter) flush() error {
	_, err := o.buf.WriteTo(o.w)
	return err
}
