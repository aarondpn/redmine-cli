package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
)

// draft07 is the dialect of the generated schema. The YAML language server
// (and most editors) implement draft-07 fully but newer drafts only
// partially, so the generator emits draft-07 keywords only: no $defs,
// prefixItems, dependentRequired, unevaluated* or $dynamicRef.
const draft07 = "http://json-schema.org/draft-07/schema#"

// generateSchema builds the manifest JSON Schema from the spec structs. The
// output is deterministic so the freshness test can compare it byte for
// byte with the committed files.
func generateSchema() ([]byte, error) {
	s, err := buildManifestSchema()
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	// Re-encode for people reading the published file: dialect and identity
	// first, and '<' '>' '&' left unescaped in descriptions.
	return reindent(data, []string{"$schema", "$id", "title", "description"})
}

func buildManifestSchema() (*jsonschema.Schema, error) {
	s, err := jsonschema.For[spec.Manifest](nil)
	if err != nil {
		return nil, err
	}
	if err := applyExtras(reflect.TypeFor[spec.Manifest](), s); err != nil {
		return nil, err
	}
	walkSchemas(s, dropNull)
	if err := addConditionals(s); err != nil {
		return nil, err
	}
	s.Schema = draft07
	s.ID = spec.SchemaID
	s.Title = "redmine-cli extension manifest"
	s.Description = "Manifest (" + spec.FileName + ") of a redmine-cli extension. Each command becomes 'redmine <name> <command>' and, unless disabled, an MCP tool."
	return s, nil
}

// applyExtras copies the jsonschema_extras struct tags of t onto the
// inferred schema s, recursing through slices, pointers and structs.
func applyExtras(t reflect.Type, s *jsonschema.Schema) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Slice:
		if s.Items == nil {
			return nil
		}
		return applyExtras(t.Elem(), s.Items)
	case reflect.Struct:
	default:
		return nil
	}
	for _, f := range reflect.VisibleFields(t) {
		if !f.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			continue
		}
		ps := s.Properties[name]
		if ps == nil {
			return fmt.Errorf("%s.%s: no inferred schema for %q", t.Name(), f.Name, name)
		}
		if tag, ok := f.Tag.Lookup("jsonschema_extras"); ok {
			if err := applyExtrasTag(ps, tag); err != nil {
				return fmt.Errorf("%s.%s: %w", t.Name(), f.Name, err)
			}
		}
		if err := applyExtras(f.Type, ps); err != nil {
			return err
		}
	}
	return nil
}

func applyExtrasTag(s *jsonschema.Schema, tag string) error {
	for _, kv := range strings.Split(tag, ";") {
		key, val, ok := strings.Cut(kv, "=")
		if !ok || val == "" {
			return fmt.Errorf("malformed jsonschema_extras entry %q", kv)
		}
		switch key {
		case "enum":
			s.Enum = tagValues(val)
		case "examples":
			s.Examples = tagValues(val)
		case "default":
			raw, err := json.Marshal(tagValue(val))
			if err != nil {
				return err
			}
			s.Default = raw
		case "pattern":
			s.Pattern = val
		case "patternErrorMessage":
			// Understood by the YAML/JSON language servers, ignored by
			// validators; Parse uses it for the same purpose.
			setExtra(s, "patternErrorMessage", val)
		case "uniqueItems":
			b, err := strconv.ParseBool(val)
			if err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
			s.UniqueItems = b
		case "minItems", "maxLength", "minLength":
			n, err := strconv.Atoi(val)
			if err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
			switch key {
			case "minItems":
				s.MinItems = &n
			case "maxLength":
				s.MaxLength = &n
			case "minLength":
				s.MinLength = &n
			}
		case "itemTypes":
			if s.Items == nil {
				return fmt.Errorf("itemTypes on a non-array field")
			}
			s.Items.Type = ""
			s.Items.Types = strings.Split(val, "|")
		default:
			return fmt.Errorf("unknown jsonschema_extras key %q", key)
		}
	}
	return nil
}

func tagValues(val string) []any {
	parts := strings.Split(val, "|")
	out := make([]any, len(parts))
	for i, p := range parts {
		out[i] = tagValue(p)
	}
	return out
}

// tagValue keeps JSON literals typed (1, true) and treats everything else
// as a string.
func tagValue(s string) any {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}

func setExtra(s *jsonschema.Schema, key string, v any) {
	if s.Extra == nil {
		s.Extra = map[string]any{}
	}
	s.Extra[key] = v
}

// dropNull removes the "null" alternative For adds to pointers and slices.
// A manifest never uses null to mean "unset"; it leaves the key out.
func dropNull(s *jsonschema.Schema) {
	if s.Types == nil {
		return
	}
	types := slices.DeleteFunc(slices.Clone(s.Types), func(t string) bool { return t == "null" })
	if len(types) == 1 {
		s.Type, s.Types = types[0], nil
		return
	}
	s.Types = types
}

// walkSchemas calls fn for s and every subschema reachable through the
// keywords the generator uses.
func walkSchemas(s *jsonschema.Schema, fn func(*jsonschema.Schema)) {
	if s == nil {
		return
	}
	fn(s)
	for _, k := range slices.Sorted(maps.Keys(s.Properties)) {
		walkSchemas(s.Properties[k], fn)
	}
	walkSchemas(s.Items, fn)
	walkSchemas(s.AdditionalProperties, fn)
	walkSchemas(s.Not, fn)
	walkSchemas(s.If, fn)
	walkSchemas(s.Then, fn)
	walkSchemas(s.Else, fn)
	for _, list := range [][]*jsonschema.Schema{s.AllOf, s.AnyOf, s.OneOf} {
		for _, sub := range list {
			walkSchemas(sub, fn)
		}
	}
}

// addConditionals encodes the cross-field rules that JSON Schema can express
// (as draft-07 if/then) so editors flag them while typing; Parse enforces the
// same schema. Rules JSON Schema cannot express live in the spec package's
// semantic validation instead, never in both places.
//
// Each then-block carries its explanation in description, which `redmine
// extension check` prints and editors may show on hover. errorMessage is
// only set on the type/enum leaf that fails inside a then-block, the
// keywords the YAML language server documents errorMessage support for;
// conditional blocks themselves and oneOf get none.
func addConditionals(root *jsonschema.Schema) error {
	cmd := root.Properties["commands"].Items
	if cmd == nil {
		return fmt.Errorf("commands schema missing")
	}
	param := cmd.Properties["params"].Items
	out := cmd.Properties["output"]
	if param == nil || out == nil {
		return fmt.Errorf("params/output schema missing")
	}

	isRaw := hasConst("raw", true)
	cmd.AllOf = []*jsonschema.Schema{
		ifThen(isRaw, rule("Raw commands take no params (arguments are passed through).",
			"params", falseSchema())),
		ifThen(isRaw, rule("Raw commands take no output spec (stdout is passed through).",
			"output", falseSchema())),
		ifThen(isRaw, rule("Raw commands are never MCP tools.",
			"mcp", leaf(&jsonschema.Schema{Enum: []any{false}}, "Raw commands are never MCP tools."))),
		ifThen(hasConst("destructive", true), required(rule("Destructive commands must also set writes: true.",
			"writes", leaf(&jsonschema.Schema{Enum: []any{true}}, "Destructive commands must also set writes: true.")))),
	}

	notArray := &jsonschema.Schema{Not: hasConst("array", true)}
	param.AllOf = []*jsonschema.Schema{
		ifThen(
			&jsonschema.Schema{
				Required:   []string{"type"},
				Properties: map[string]*jsonschema.Schema{"type": {Enum: []any{string(spec.TypeNumber), string(spec.TypeBoolean), string(spec.TypeJSON)}}},
			},
			rule("enum is only allowed for string and integer parameters.", "enum", falseSchema()),
		),
		ifThen(hasConst("type", string(spec.TypeBoolean)), rule(
			"Boolean parameters cannot be arrays; use a single flag.",
			"array", leaf(&jsonschema.Schema{Enum: []any{false}}, "Boolean parameters cannot be arrays."))),
		ifThen(hasConst("required", true), rule(
			"A required parameter cannot have a default; drop required or the default.",
			"default", falseSchema())),
	}
	for _, t := range []spec.ParamType{spec.TypeString, spec.TypeInteger} {
		msg := fmt.Sprintf("enum values must be %s.", plural(t))
		param.AllOf = append(param.AllOf, ifThen(
			hasConst("type", string(t)),
			rule(msg, "enum", &jsonschema.Schema{Items: leaf(&jsonschema.Schema{Type: string(t)}, msg)}),
		))
	}
	for _, t := range []spec.ParamType{spec.TypeString, spec.TypeInteger, spec.TypeNumber, spec.TypeBoolean} {
		msg := fmt.Sprintf("default must be %s.", article(t))
		param.AllOf = append(param.AllOf, ifThen(
			allOf(hasConst("type", string(t)), notArray),
			rule(msg, "default", leaf(&jsonschema.Schema{Type: string(t)}, msg)),
		))
		if t == spec.TypeBoolean {
			continue // boolean arrays are rejected above
		}
		msg = fmt.Sprintf("default must be a list of %s.", plural(t))
		param.AllOf = append(param.AllOf, ifThen(
			allOf(hasConst("type", string(t)), hasConst("array", true)),
			rule(msg, "default", leaf(&jsonschema.Schema{Type: "array", Items: leaf(&jsonschema.Schema{Type: string(t)}, msg)}, msg)),
		))
	}
	param.AllOf = append(param.AllOf, ifThen(
		allOf(hasConst("type", string(spec.TypeJSON)), hasConst("array", true)),
		rule("default must be a list.", "default", leaf(&jsonschema.Schema{Type: "array"}, "default must be a list.")),
	))

	for _, field := range []string{"table", "detail"} {
		item := out.Properties[field].Items
		if item == nil {
			return fmt.Errorf("output.%s schema missing", field)
		}
		item.OneOf = []*jsonschema.Schema{
			{Required: []string{"path"}},
			{Required: []string{"jq"}},
		}
		item.Description = "Set exactly one of path or jq."
	}
	return nil
}

func ifThen(cond, then *jsonschema.Schema) *jsonschema.Schema {
	return &jsonschema.Schema{If: cond, Then: then}
}

// rule constrains one property and explains the rule in description.
func rule(description, prop string, s *jsonschema.Schema) *jsonschema.Schema {
	return &jsonschema.Schema{
		Description: description,
		Properties:  map[string]*jsonschema.Schema{prop: s},
	}
}

// required additionally requires the single property a rule constrains.
func required(s *jsonschema.Schema) *jsonschema.Schema {
	for k := range s.Properties {
		s.Required = append(s.Required, k)
	}
	return s
}

// leaf attaches errorMessage to a type or enum check.
func leaf(s *jsonschema.Schema, msg string) *jsonschema.Schema {
	setExtra(s, "errorMessage", msg)
	return s
}

// falseSchema is the schema `false`: the property must not be present.
func falseSchema() *jsonschema.Schema {
	return &jsonschema.Schema{Not: &jsonschema.Schema{}}
}

func allOf(list ...*jsonschema.Schema) *jsonschema.Schema {
	merged := &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{}}
	for _, s := range list {
		merged.Required = append(merged.Required, s.Required...)
		for k, v := range s.Properties {
			merged.Properties[k] = v
		}
		if s.Not != nil {
			merged.Not = s.Not
		}
	}
	return merged
}

func hasConst(prop string, v any) *jsonschema.Schema {
	return &jsonschema.Schema{
		Required:   []string{prop},
		Properties: map[string]*jsonschema.Schema{prop: {Const: &v}},
	}
}

func article(t spec.ParamType) string {
	if t == spec.TypeInteger {
		return "an integer"
	}
	return "a " + string(t)
}

func plural(t spec.ParamType) string {
	return string(t) + "s"
}
