package update

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// --- Windows-style replacement of the running binary ---

func TestSwapMovingAside(t *testing.T) {
	dir := t.TempDir()
	execPath := filepath.Join(dir, "redmine.exe")
	tmpPath := filepath.Join(dir, ".redmine-update-1")
	writeFile(t, execPath, "old")
	writeFile(t, execPath+".old", "from a previous update")
	writeFile(t, tmpPath, "new")

	if err := swapMovingAside(tmpPath, execPath); err != nil {
		t.Fatalf("swapMovingAside: %v", err)
	}
	if got := readFile(t, execPath); got != "new" {
		t.Errorf("exec content = %q, want %q", got, "new")
	}
	if got := readFile(t, execPath+".old"); got != "old" {
		t.Errorf(".old content = %q, want %q", got, "old")
	}
	if _, err := os.Stat(tmpPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("temp file still present: %v", err)
	}
}

func TestSwapMovingAside_RestoresOnFailure(t *testing.T) {
	dir := t.TempDir()
	execPath := filepath.Join(dir, "redmine.exe")
	writeFile(t, execPath, "old")

	// A missing temp file makes the second rename fail.
	err := swapMovingAside(filepath.Join(dir, "missing"), execPath)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if got := readFile(t, execPath); got != "old" {
		t.Errorf("exec content = %q, want the restored %q", got, "old")
	}
}

func TestReplaceBinary_MovesAsideWhenRequired(t *testing.T) {
	orig := moveRunningBinaryAside
	t.Cleanup(func() { moveRunningBinaryAside = orig })
	moveRunningBinaryAside = true

	dir := t.TempDir()
	execPath := filepath.Join(dir, "redmine.exe")
	writeFile(t, execPath, "old")

	if err := replaceBinary(execPath, []byte("new")); err != nil {
		t.Fatalf("replaceBinary: %v", err)
	}
	if got := readFile(t, execPath); got != "new" {
		t.Errorf("exec content = %q, want %q", got, "new")
	}
	if got := readFile(t, execPath+".old"); got != "old" {
		t.Errorf(".old content = %q, want %q", got, "old")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
