package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
)

func TestGenerateSchemaIsDeterministic(t *testing.T) {
	a, err := generateSchema()
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		b, err := generateSchema()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Fatal("GenerateSchema output differs between runs")
		}
	}
}

// mustGenerate returns the generated schema or fails the test.
func mustGenerate(t *testing.T) []byte {
	t.Helper()
	data, err := generateSchema()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSchemaHeader(t *testing.T) {
	data := mustGenerate(t)
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["$schema"] != draft07 {
		t.Errorf("$schema = %v, want %s", doc["$schema"], draft07)
	}
	if doc["$id"] != spec.SchemaID {
		t.Errorf("$id = %v, want %s", doc["$id"], spec.SchemaID)
	}
	if !strings.HasPrefix(string(data), "{\n  \"$schema\"") {
		t.Error("schema should start with $schema for readability")
	}
	if strings.Contains(string(data), "\\u00") {
		t.Error("schema contains HTML-escaped characters")
	}
}

// TestSchemaIsDraft07Only guards the YAML language server compatibility:
// it implements draft-07, so newer keywords must not appear anywhere.
func TestSchemaIsDraft07Only(t *testing.T) {
	forbidden := map[string]bool{
		"$defs": true, "$anchor": true, "$dynamicRef": true, "$dynamicAnchor": true,
		"$vocabulary": true, "prefixItems": true, "dependentRequired": true,
		"dependentSchemas": true, "unevaluatedItems": true, "unevaluatedProperties": true,
		"minContains": true, "maxContains": true, "$recursiveRef": true,
	}
	var doc any
	if err := json.Unmarshal(mustGenerate(t), &doc); err != nil {
		t.Fatal(err)
	}
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch t2 := v.(type) {
		case map[string]any:
			for k, child := range t2 {
				// Keys under "properties" are manifest field names, not keywords.
				if !strings.HasSuffix(path, "/properties") && forbidden[k] {
					t.Errorf("%s uses non-draft-07 keyword %q", path, k)
				}
				walk(path+"/"+k, child)
			}
		case []any:
			for _, child := range t2 {
				walk(path+"/[]", child)
			}
		}
	}
	walk("", doc)
}

func TestApplyExtrasTagErrors(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want string
	}{
		{"unknown key", "colour=red", "unknown jsonschema_extras key"},
		{"missing value", "enum", "malformed"},
		{"bad int", "minItems=x", "minItems"},
		{"bad bool", "uniqueItems=maybe", "uniqueItems"},
		{"itemTypes on scalar", "itemTypes=string", "non-array"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := applyExtrasTag(&jsonschema.Schema{}, tt.tag)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("applyExtrasTag(%q) = %v, want error containing %q", tt.tag, err, tt.want)
			}
		})
	}
}

func TestApplyExtrasTagTypedValues(t *testing.T) {
	s := &jsonschema.Schema{}
	if err := applyExtrasTag(s, "enum=1|two|true;default=false;examples=30s"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Enum, []any{float64(1), "two", true}) {
		t.Errorf("Enum = %#v", s.Enum)
	}
	if string(s.Default) != "false" {
		t.Errorf("Default = %s", s.Default)
	}
	if !reflect.DeepEqual(s.Examples, []any{"30s"}) {
		t.Errorf("Examples = %#v", s.Examples)
	}
}

func TestReindent(t *testing.T) {
	got, err := reindent([]byte(`{"b":1,"a":{"z":[1,{}],"y":[]},"$id":"<x>"}`), []string{"$id", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "$id": "<x>",
  "b": 1,
  "a": {
    "z": [
      1,
      {}
    ],
    "y": []
  }
}
`
	if string(got) != want {
		t.Errorf("reindent =\n%s\nwant\n%s", got, want)
	}
}

// TestErrorMessagesOnlyOnLeaves keeps errorMessage where the YAML language
// server is documented to use it (type and enum checks) and off if/then,
// oneOf and other combinators, where it would never be shown. Conditional
// rules explain themselves through the then-block description instead,
// which is also what `redmine extension check` prints.
func TestErrorMessagesOnlyOnLeaves(t *testing.T) {
	var doc any
	if err := json.Unmarshal(mustGenerate(t), &doc); err != nil {
		t.Fatal(err)
	}
	combinators := []string{"if", "then", "else", "oneOf", "anyOf", "allOf", "not"}
	thens := 0
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch node := v.(type) {
		case map[string]any:
			if _, ok := node["errorMessage"]; ok {
				_, hasType := node["type"]
				_, hasEnum := node["enum"]
				if !hasType && !hasEnum {
					t.Errorf("%s: errorMessage without type or enum", path)
				}
				for _, k := range combinators {
					if _, ok := node[k]; ok {
						t.Errorf("%s: errorMessage next to %s", path, k)
					}
				}
			}
			if then, ok := node["then"].(map[string]any); ok {
				thens++
				if d, _ := then["description"].(string); d == "" {
					t.Errorf("%s/then: conditional without description", path)
				}
			}
			for k, child := range node {
				walk(path+"/"+k, child)
			}
		case []any:
			for i, child := range node {
				walk(fmt.Sprintf("%s/%d", path, i), child)
			}
		}
	}
	walk("", doc)
	if thens == 0 {
		t.Fatal("no conditionals found")
	}
}
