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

	out, err := Generate(repoRoot)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	toolsPath := filepath.Join(repoRoot, generatedToolsOut)
	gotTools, err := os.ReadFile(toolsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", toolsPath, err)
	}
	if string(gotTools) != string(out.ToolsGo) {
		t.Fatalf("%s is stale; run `go generate ./...`", generatedToolsOut)
	}

	docsPath := filepath.Join(repoRoot, generatedDocsOut)
	gotDocs, err := os.ReadFile(docsPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", docsPath, err)
	}
	if string(gotDocs) != string(out.DocsMD) {
		t.Fatalf("%s is stale; run `go generate ./...`", generatedDocsOut)
	}
}
