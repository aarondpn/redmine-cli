package spec

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/jsonschema-go/jsonschema"
)

// schemaChecker localizes JSON Schema failures. jsonschema-go reports only
// the first failure and no instance location, so the checker validates the
// failing value's children against their own subschemas and reports at the
// deepest node that fails on its own. Node-level failures get readable
// messages for the keywords the manifest schema uses.
type schemaChecker struct {
	resolved map[*jsonschema.Schema]*jsonschema.Resolved
}

func newSchemaChecker() *schemaChecker {
	return &schemaChecker{resolved: map[*jsonschema.Schema]*jsonschema.Resolved{}}
}

// resolveError marks a schema that cannot be resolved: a bug in the schema,
// not in the validated value.
type resolveError struct{ err error }

func (e *resolveError) Error() string { return "resolve schema: " + e.err.Error() }
func (e *resolveError) Unwrap() error { return e.err }

func (c *schemaChecker) validate(s *jsonschema.Schema, v any) error {
	r, ok := c.resolved[s]
	if !ok {
		var err error
		// Resolve a copy: subschemas are resolved on their own and must not
		// share resolution state with the root.
		r, err = s.CloneSchemas().Resolve(nil)
		if err != nil {
			return &resolveError{err: err}
		}
		c.resolved[s] = r
	}
	return r.Validate(v)
}

// check validates v and records localized problems in is. It returns an
// error only when the schema itself is unusable.
func (c *schemaChecker) check(is *issues, ptr string, v any, s *jsonschema.Schema) error {
	verr := c.validate(s, v)
	if verr == nil {
		return nil
	}
	var rerr *resolveError
	if errors.As(verr, &rerr) {
		return verr
	}
	before := len(is.list)

	switch val := v.(type) {
	case map[string]any:
		if s.Properties != nil && typeAllows(s, "object") {
			keys := slices.SortedFunc(maps.Keys(val), func(a, b string) int {
				return cmp.Or(cmp.Compare(is.pos.line(pointer(ptr, a)), is.pos.line(pointer(ptr, b))), cmp.Compare(a, b))
			})
			for _, k := range keys {
				sub, ok := s.Properties[k]
				if !ok {
					if isFalseSchema(s.AdditionalProperties) {
						is.list = append(is.list, &Error{
							File:    is.file,
							Line:    is.pos.line(pointer(ptr, k)),
							Path:    displayPath(ptr),
							Message: fmt.Sprintf("unknown field %q", k),
						})
					}
					continue
				}
				if err := c.check(is, pointer(ptr, k), val[k], sub); err != nil {
					return err
				}
			}
		}
	case []any:
		if s.Items != nil && typeAllows(s, "array") {
			for i, item := range val {
				if err := c.check(is, pointer(ptr, i), item, s.Items); err != nil {
					return err
				}
			}
		}
	}

	for _, m := range c.nodeMessages(v, s) {
		at := ptr
		if m.key != "" {
			at = pointer(ptr, m.key)
		}
		is.addf(at, "%s", m.msg)
	}
	if len(is.list) == before {
		is.addf(ptr, "%s", cleanSchemaError(verr))
	}
	return nil
}

// nodeMessage is a problem with a value itself. key, when set, names the
// member of an object the problem is about (a conditional constraining one
// property), so it is reported at that member's line.
type nodeMessage struct {
	key string
	msg string
}

// nodeMessages explains why v itself (not its children) fails s.
func (c *schemaChecker) nodeMessages(v any, s *jsonschema.Schema) []nodeMessage {
	got := jsonTypeOf(v)
	if s.Type != "" || s.Types != nil {
		if !typeAllows(s, got) {
			return []nodeMessage{{msg: fmt.Sprintf("must be %s, got %s", describeTypes(s), got)}}
		}
	}
	var msgs []string
	var out []nodeMessage
	if s.Enum != nil && !slices.ContainsFunc(s.Enum, func(e any) bool { return jsonschema.Equal(e, v) }) {
		msgs = append(msgs, fmt.Sprintf("must be one of: %s (got %s)", formatList(s.Enum), formatValue(v)))
	}
	if s.Const != nil && !jsonschema.Equal(*s.Const, v) {
		msgs = append(msgs, fmt.Sprintf("must be %s", formatValue(*s.Const)))
	}
	switch val := v.(type) {
	case string:
		n := utf8.RuneCountInString(val)
		if s.MinLength != nil && n < *s.MinLength {
			if *s.MinLength == 1 {
				msgs = append(msgs, "must not be empty")
			} else {
				msgs = append(msgs, fmt.Sprintf("must be at least %d characters", *s.MinLength))
			}
		}
		if s.MaxLength != nil && n > *s.MaxLength {
			msgs = append(msgs, fmt.Sprintf("must be at most %d characters (got %d)", *s.MaxLength, n))
		}
		if s.Pattern != "" {
			if re, err := regexp.Compile(s.Pattern); err == nil && !re.MatchString(val) {
				if hint, ok := s.Extra["patternErrorMessage"].(string); ok {
					msgs = append(msgs, fmt.Sprintf("%q is invalid: %s", val, asClause(hint)))
				} else {
					msgs = append(msgs, fmt.Sprintf("%q does not match %s", val, s.Pattern))
				}
			}
		}
	case []any:
		if s.MinItems != nil && len(val) < *s.MinItems {
			if *s.MinItems == 1 {
				msgs = append(msgs, "must not be empty")
			} else {
				msgs = append(msgs, fmt.Sprintf("must have at least %d items", *s.MinItems))
			}
		}
		if s.UniqueItems {
			if dup, ok := firstDuplicate(val); ok {
				msgs = append(msgs, fmt.Sprintf("must not repeat a value (%s is listed twice)", formatValue(dup)))
			}
		}
	case map[string]any:
		for _, r := range s.Required {
			if _, ok := val[r]; !ok {
				msgs = append(msgs, fmt.Sprintf("missing required field %q", r))
			}
		}
	}
	if s.OneOf != nil {
		if err := c.validate(&jsonschema.Schema{OneOf: s.OneOf}, v); err != nil {
			if msg, ok := oneOfMessage(s.OneOf); ok {
				msgs = append(msgs, msg)
			} else {
				msgs = append(msgs, cleanSchemaError(err))
			}
		}
	}
	for _, m := range msgs {
		out = append(out, nodeMessage{msg: m})
	}
	for _, sub := range s.AllOf {
		err := c.validate(sub, v)
		if err == nil {
			continue
		}
		out = append(out, nodeMessage{key: constrainedKey(sub, v), msg: messageOf(sub, err)})
	}
	return out
}

// constrainedKey returns the single property an if/then rule constrains
// (through properties, required or not.required) when v has it, so the
// problem points at that line instead of the object.
func constrainedKey(s *jsonschema.Schema, v any) string {
	obj, ok := v.(map[string]any)
	if !ok || s.Then == nil {
		return ""
	}
	t := s.Then
	keys := slices.Collect(maps.Keys(t.Properties))
	keys = append(keys, t.Required...)
	if t.Not != nil {
		keys = append(keys, t.Not.Required...)
	}
	slices.Sort(keys)
	keys = slices.Compact(keys)
	if len(keys) != 1 {
		return ""
	}
	if _, ok := obj[keys[0]]; ok {
		return keys[0]
	}
	return ""
}

// messageOf explains a failed conditional with the description of its
// then-block, where the generator puts the rule in words.
func messageOf(s *jsonschema.Schema, err error) string {
	for _, cand := range []*jsonschema.Schema{s.Then, s} {
		if cand != nil && cand.Description != "" {
			return asClause(cand.Description)
		}
	}
	return cleanSchemaError(err)
}

// oneOfMessage phrases a oneOf of single required properties, the only
// kind the manifest schema uses: "set exactly one of path or jq".
func oneOfMessage(list []*jsonschema.Schema) (string, bool) {
	var names []string
	for _, alt := range list {
		if len(alt.Required) != 1 {
			return "", false
		}
		names = append(names, alt.Required[0])
	}
	return "set exactly one of " + strings.Join(names, " or "), true
}

// firstDuplicate returns the first value of list that equals an earlier
// one under JSON Schema equality (so 1 and 1.0 are the same value).
func firstDuplicate(list []any) (any, bool) {
	for i := range list {
		for j := range i {
			if jsonschema.Equal(list[i], list[j]) {
				return list[i], true
			}
		}
	}
	return nil, false
}

// asClause adapts an editor-facing sentence ("Set exactly one of path or
// jq.") to the CLI's error style: lowercase start, no trailing period.
func asClause(msg string) string {
	msg = strings.TrimSuffix(msg, ".")
	r, size := utf8.DecodeRuneInString(msg)
	if r == utf8.RuneError {
		return msg
	}
	return string(unicode.ToLower(r)) + msg[size:]
}

// cleanSchemaError strips jsonschema-go's "validating <schema>: " chain,
// which names schemas rather than locations and only adds noise.
func cleanSchemaError(err error) string {
	msg := err.Error()
	for strings.HasPrefix(msg, "validating ") {
		i := strings.Index(msg, ": ")
		if i < 0 {
			break
		}
		msg = msg[i+2:]
	}
	return msg
}

// isFalseSchema reports whether s is the schema `false` ({"not": {}}), as
// used for additionalProperties.
func isFalseSchema(s *jsonschema.Schema) bool {
	if s == nil || s.Not == nil {
		return false
	}
	data, err := s.Not.MarshalJSON()
	return err == nil && string(data) == "true"
}

// typeAllows reports whether s's type keyword admits a value of JSON type
// got. A schema without type admits everything; integral numbers satisfy
// "integer" and every integer satisfies "number".
func typeAllows(s *jsonschema.Schema, got string) bool {
	types := s.Types
	if s.Type != "" {
		types = []string{s.Type}
	}
	if types == nil {
		return true
	}
	for _, t := range types {
		if t == got || (t == "number" && got == "integer") {
			return true
		}
	}
	return false
}

// jsonTypeOf returns the JSON type of a decoded value; integral floats
// count as integers, as in JSON Schema.
func jsonTypeOf(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "integer"
	case float32:
		return jsonTypeOf(float64(t))
	case float64:
		if _, frac := math.Modf(t); frac == 0 && !math.IsInf(t, 0) {
			return "integer"
		}
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func describeTypes(s *jsonschema.Schema) string {
	types := s.Types
	if s.Type != "" {
		types = []string{s.Type}
	}
	parts := make([]string, len(types))
	for i, t := range types {
		switch t {
		case "integer", "object", "array":
			parts[i] = "an " + t
		default:
			parts[i] = "a " + t
		}
	}
	return strings.Join(parts, " or ")
}

func formatList(vals []any) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = formatValue(v)
	}
	return strings.Join(parts, ", ")
}

// formatValue renders scalars the way they are written in YAML.
func formatValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if _, frac := math.Modf(t); frac == 0 && math.Abs(t) < 1<<53 {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprint(t)
	case nil:
		return "null"
	default:
		return fmt.Sprint(t)
	}
}
