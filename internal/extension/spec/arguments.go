package spec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// DecodeArguments reads an input object given as JSON text: the arguments
// of an MCP tool call, or `extension check --input`. Numbers are kept as
// json.Number so large integers keep their exact digits; see
// NormalizeArguments. Empty input and null are an empty object.
func DecodeArguments(raw []byte) (map[string]any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return map[string]any{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if dec.More() {
		return nil, errors.New("invalid JSON: unexpected data after the object")
	}
	if v == nil {
		return map[string]any{}, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("must be a JSON object")
	}
	return m, nil
}

// NormalizeArguments converts the json.Number values DecodeArguments keeps
// into int64 (when the number is an integer in range) or float64, the types
// the input validation knows, for every argument except json params. An
// integer outside the int64 range becomes a float64, which PrepareInput
// then rejects for integer params instead of forwarding a rounded value. Those
// keep json.Number, as the CLI does for a json flag, so an arbitrary JSON
// value reaches the extension with its exact digits on both surfaces.
func NormalizeArguments(c *Command, args map[string]any) map[string]any {
	out := make(map[string]any, len(args))
	for k, v := range args {
		if p := c.Param(k); p != nil && p.Type == TypeJSON {
			out[k] = v
			continue
		}
		out[k] = normalizeArgument(v)
	}
	return out
}

func normalizeArgument(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
		return t
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = normalizeArgument(item)
		}
		return out
	}
	return v
}
