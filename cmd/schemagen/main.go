// Command schemagen writes the redmine-extension.yml JSON Schema, generated
// from the manifest structs in internal/extension/spec, to the embedded copy
// next to that package and to the published copy served by the docs site.
// Run it through `go generate ./internal/extension/spec/...`.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
)

// outputs are the schema destinations relative to the module root.
var outputs = []string{
	filepath.Join("internal", "extension", "spec", spec.SchemaFileName),
	filepath.Join("docs", "public", "schema", spec.SchemaFileName),
}

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	root, err := findModuleRoot(wd)
	if err != nil {
		log.Fatal(err)
	}
	if err := write(root); err != nil {
		log.Fatal(err)
	}
}

func write(root string) error {
	data, err := spec.GenerateSchema()
	if err != nil {
		return err
	}
	for _, rel := range outputs {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

func findModuleRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
