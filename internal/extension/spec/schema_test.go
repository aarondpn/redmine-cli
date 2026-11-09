package spec

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
)

func TestSchemaFilesAreFresh(t *testing.T) {
	want, err := GenerateSchema()
	if err != nil {
		t.Fatalf("GenerateSchema: %v", err)
	}
	if !bytes.Equal(SchemaJSON(), want) {
		t.Errorf("embedded %s is stale; run `go generate ./internal/extension/spec/...`", SchemaFileName)
	}
	published, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "public", "schema", SchemaFileName))
	if err != nil {
		t.Fatalf("read published schema: %v", err)
	}
	if !bytes.Equal(published, want) {
		t.Errorf("docs/public/schema/%s is stale; run `go generate ./internal/extension/spec/...`", SchemaFileName)
	}
}

func TestGenerateSchemaIsDeterministic(t *testing.T) {
	a, err := GenerateSchema()
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		b, err := GenerateSchema()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Fatal("GenerateSchema output differs between runs")
		}
	}
}

func TestSchemaHeader(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(SchemaJSON(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["$schema"] != draft07 {
		t.Errorf("$schema = %v, want %s", doc["$schema"], draft07)
	}
	if doc["$id"] != SchemaID {
		t.Errorf("$id = %v, want %s", doc["$id"], SchemaID)
	}
	if !strings.HasPrefix(string(SchemaJSON()), "{\n  \"$schema\"") {
		t.Error("schema should start with $schema for readability")
	}
	if strings.Contains(string(SchemaJSON()), "\\u00") {
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
	if err := json.Unmarshal(SchemaJSON(), &doc); err != nil {
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

func TestSchemaResolvesAsDraft07(t *testing.T) {
	s, err := manifestSchema()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Resolve(nil); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
}

func TestSchemaAcceptsExampleManifest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", FileName))
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	s, err := manifestSchema()
	if err != nil {
		t.Fatal(err)
	}
	rs, err := s.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rs.Validate(doc); err != nil {
		t.Fatalf("example manifest fails the raw schema: %v", err)
	}
}

// TestStructTagsAgree keeps YAML marshalling (used for scaffolding) and the
// JSON-based schema/decoding on the same field names.
func TestStructTagsAgree(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeFor[Manifest](), reflect.TypeFor[Command](), reflect.TypeFor[Param](),
		reflect.TypeFor[Output](), reflect.TypeFor[Column](), reflect.TypeFor[DetailField](),
	} {
		for _, f := range reflect.VisibleFields(typ) {
			if !f.IsExported() {
				continue
			}
			j, y := f.Tag.Get("json"), f.Tag.Get("yaml")
			if j != y {
				t.Errorf("%s.%s: json tag %q != yaml tag %q", typ.Name(), f.Name, j, y)
			}
			if _, ok := f.Tag.Lookup("jsonschema"); !ok {
				t.Errorf("%s.%s: missing jsonschema description", typ.Name(), f.Name)
			}
		}
	}
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
