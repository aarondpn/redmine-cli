package spec

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

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
