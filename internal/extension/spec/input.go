package spec

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

// ToolName derives the MCP tool name of a command: <ext>_<command> with
// dashes replaced by underscores (MCP clients restrict tool names to
// [a-zA-Z0-9_-] and some reject dashes in practice).
func ToolName(ext, command string) string {
	return strings.ReplaceAll(ext+"_"+command, "-", "_")
}

// FlagName derives the CLI flag of a param: the name with '_' replaced by
// '-', following the CLI's flag style.
func FlagName(param string) string {
	return strings.ReplaceAll(param, "_", "-")
}

// InputSchema returns the JSON Schema of a command's input object, used as
// the MCP tool input_schema and to validate input on both surfaces. It
// carries no $schema, so it is read as 2020-12, which is what MCP expects;
// it only uses keywords that mean the same in draft-07.
func InputSchema(c *Command) *jsonschema.Schema {
	s := &jsonschema.Schema{
		Type:                 "object",
		Properties:           map[string]*jsonschema.Schema{},
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}
	for i := range c.Params {
		p := &c.Params[i]
		s.Properties[p.Name] = paramSchema(p)
		s.PropertyOrder = append(s.PropertyOrder, p.Name)
		if p.Required {
			s.Required = append(s.Required, p.Name)
		}
	}
	return s
}

func paramSchema(p *Param) *jsonschema.Schema {
	item := &jsonschema.Schema{}
	switch p.Type {
	case TypeJSON:
		// Any JSON value: leave type open.
	default:
		item.Type = string(p.Type)
	}
	if len(p.Enum) > 0 {
		item.Enum = slices.Clone(p.Enum)
	}
	s := item
	if p.Array {
		s = &jsonschema.Schema{Type: "array", Items: item}
		if p.Required {
			// The CLI cannot express an empty required list either.
			one := 1
			s.MinItems = &one
		}
	}
	s.Description = p.Description
	if s.Description == "" && p.Type == TypeJSON {
		// A schema without type and description marshals as `true`, which
		// some MCP clients reject as a property schema.
		s.Description = "Any JSON value."
	}
	if p.Default != nil {
		if raw, err := json.Marshal(p.Default); err == nil {
			s.Default = raw
		}
	}
	return s
}

// ValidateInput checks an input object against InputSchema(c). Problems are
// returned as a *ValidationError naming the offending keys.
func ValidateInput(c *Command, input map[string]any) error {
	is := &issues{}
	if err := newSchemaChecker().check(is, "", toAnyMap(input), InputSchema(c)); err != nil {
		return err
	}
	return is.err()
}

// PrepareInput turns an MCP tool call's arguments into the object sent to
// the extension: null values count as absent (clients send them for unset
// optionals), the rest is validated, numbers are normalized to the param
// type (integers as int64), and defaults are applied.
func PrepareInput(c *Command, args map[string]any) (map[string]any, error) {
	in := make(map[string]any, len(args))
	for k, v := range args {
		if v != nil {
			in[k] = v
		}
	}
	if err := ValidateInput(c, in); err != nil {
		return nil, err
	}
	for k, v := range in {
		p := c.Param(k)
		if p == nil {
			continue
		}
		in[k] = canonical(p, v)
	}
	return ApplyDefaults(c, in), nil
}

// ApplyDefaults returns a copy of input with the default of every absent
// param filled in. Absent params without a default stay absent.
func ApplyDefaults(c *Command, input map[string]any) map[string]any {
	out := maps.Clone(input)
	if out == nil {
		out = map[string]any{}
	}
	for i := range c.Params {
		p := &c.Params[i]
		if _, ok := out[p.Name]; ok || p.Default == nil {
			continue
		}
		out[p.Name] = deepCopy(p.Default)
	}
	return out
}

// canonical converts an already validated value to the Go type the CLI
// would produce for p (int64 for integers, float64 for numbers).
func canonical(p *Param, v any) any {
	if p.Array {
		list, ok := v.([]any)
		if !ok {
			return v
		}
		out := make([]any, len(list))
		for i, item := range list {
			out[i] = canonicalScalar(p.Type, item)
		}
		return out
	}
	return canonicalScalar(p.Type, v)
}

func canonicalScalar(t ParamType, v any) any {
	if c, ok := coerceScalar(t, v); ok {
		return c
	}
	return v
}

// coerceScalar converts v to the canonical Go value of type t (string,
// int64, float64, bool; json values unchanged) and reports whether v is a
// value of that type.
func coerceScalar(t ParamType, v any) (any, bool) {
	switch t {
	case TypeString:
		s, ok := v.(string)
		return s, ok
	case TypeBoolean:
		b, ok := v.(bool)
		return b, ok
	case TypeInteger:
		switch n := v.(type) {
		case int:
			return int64(n), true
		case int64:
			return n, true
		case json.Number:
			i, err := n.Int64()
			return i, err == nil
		case float64:
			if _, frac := math.Modf(n); frac == 0 && math.Abs(n) <= 1<<53 {
				return int64(n), true
			}
		}
		return nil, false
	case TypeNumber:
		switch n := v.(type) {
		case int:
			return float64(n), true
		case int64:
			return float64(n), true
		case float64:
			return n, true
		case json.Number:
			f, err := n.Float64()
			return f, err == nil
		}
		return nil, false
	case TypeJSON:
		return v, true
	}
	return nil, false
}

// checkValue validates v against p's type, array-ness and enum. The error
// reads as a predicate ("must be ...") so callers can prefix the subject.
func checkValue(p *Param, v any) error {
	if !p.Array {
		return checkScalar(p, v)
	}
	list, ok := v.([]any)
	if !ok {
		return fmt.Errorf("must be a list of %s, got %s", plural(p.Type), jsonTypeOf(v))
	}
	for i, item := range list {
		if err := checkScalar(p, item); err != nil {
			return fmt.Errorf("item %d %w", i+1, err)
		}
	}
	return nil
}

func checkScalar(p *Param, v any) error {
	c, ok := coerceScalar(p.Type, v)
	if !ok {
		return fmt.Errorf("must be %s, got %s", article(p.Type), jsonTypeOf(v))
	}
	if len(p.Enum) > 0 && !slices.ContainsFunc(p.Enum, func(e any) bool { return jsonschema.Equal(e, c) }) {
		return fmt.Errorf("must be one of: %s (got %s)", formatList(p.Enum), formatValue(v))
	}
	return nil
}

// toAnyMap widens typed slices (e.g. []string from a caller) to []any so
// the schema checker sees plain JSON shapes.
func toAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = widen(v)
	}
	return out
}

func widen(v any) any {
	switch t := v.(type) {
	case []string:
		return toAnySlice(t)
	case []int64:
		return toAnySlice(t)
	case []float64:
		return toAnySlice(t)
	case []bool:
		return toAnySlice(t)
	case []int:
		return toAnySlice(t)
	}
	return v
}

func toAnySlice[T any](in []T) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

func deepCopy(v any) any {
	switch t := v.(type) {
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = deepCopy(t[i])
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = deepCopy(val)
		}
		return out
	default:
		return v
	}
}
