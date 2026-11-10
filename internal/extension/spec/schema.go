package spec

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
)

// embeddedSchema is the generated manifest schema (see cmd/schemagen),
// byte for byte the file published at SchemaID.
//
//go:embed redmine-extension.v1.json
var embeddedSchema []byte

// manifestSchema is the parsed embedded schema used for validation.
var manifestSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	var s jsonschema.Schema
	if err := json.Unmarshal(embeddedSchema, &s); err != nil {
		return nil, fmt.Errorf("embedded manifest schema: %w", err)
	}
	return &s, nil
})
