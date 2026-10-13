package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/itchyny/gojq"
)

// JQFilter is a compiled --jq expression that is applied to every JSON
// document a command would otherwise print. The semantics follow gh's --jq:
// string results are printed raw (without quotes), every other value is
// printed as JSON, and each result goes on its own line.
type JQFilter struct {
	code *gojq.Code
}

// CompileJQ parses and compiles a jq expression. Compiling up front lets
// callers reject a malformed expression before any API request is made.
func CompileJQ(expr string) (*JQFilter, error) {
	query, err := gojq.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid --jq expression: %w", err)
	}
	code, err := gojq.Compile(query, gojq.WithEnvironLoader(os.Environ))
	if err != nil {
		return nil, fmt.Errorf("invalid --jq expression: %w", err)
	}
	return &JQFilter{code: code}, nil
}

// Evaluate runs the filter against the JSON encoding of v and writes the
// results to w. The input is exactly the JSON that RenderJSON would print, so
// expressions can be written against the documented output shape.
//
// When indent is true, object and array results are pretty-printed;
// otherwise they are written compactly, one per line. Results are buffered
// so a runtime error leaves w untouched.
func (q *JQFilter) Evaluate(w io.Writer, v any, indent bool) error {
	data, err := json.Marshal(normalizeJSONValue(v))
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var input any
	if err := dec.Decode(&input); err != nil {
		return err
	}

	var buf bytes.Buffer
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
			return fmt.Errorf("--jq: %w", err)
		}
		if err := writeJQResult(&buf, result, indent); err != nil {
			return err
		}
	}
	_, err = buf.WriteTo(w)
	return err
}

func writeJQResult(buf *bytes.Buffer, v any, indent bool) error {
	if s, ok := v.(string); ok {
		buf.WriteString(s)
		buf.WriteByte('\n')
		return nil
	}
	data, err := gojq.Marshal(v)
	if err != nil {
		return err
	}
	if indent {
		switch v.(type) {
		case map[string]any, []any:
			if err := json.Indent(buf, data, "", "  "); err != nil {
				return err
			}
			buf.WriteByte('\n')
			return nil
		}
	}
	buf.Write(data)
	buf.WriteByte('\n')
	return nil
}
