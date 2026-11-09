package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
)

func TestWriteCreatesBothSchemas(t *testing.T) {
	root := t.TempDir()
	if err := write(root); err != nil {
		t.Fatalf("write: %v", err)
	}
	want, err := spec.GenerateSchema()
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range outputs {
		got, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from spec.GenerateSchema()", rel)
		}
	}
}

// TestCommittedSchemasAreFresh fails when the checked-in schema files lag
// behind the manifest structs.
func TestCommittedSchemasAreFresh(t *testing.T) {
	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	want, err := spec.GenerateSchema()
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range outputs {
		got, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is stale; run `go generate ./internal/extension/spec/...`", rel)
		}
	}
}

func TestFindModuleRoot(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := findModuleRoot(nested)
	if err != nil || got != dir {
		t.Fatalf("findModuleRoot = %q, %v; want %q", got, err, dir)
	}
}
