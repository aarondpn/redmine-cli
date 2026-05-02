package mcpgen

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGeneratedOutputsAreUpToDate(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))

	want, err := Generate(repoRoot)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	toolsPath := filepath.Join(repoRoot, generatedToolsOut)
	got, err := os.ReadFile(toolsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", toolsPath, err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s is stale; run `go generate ./...`", generatedToolsOut)
	}
}
