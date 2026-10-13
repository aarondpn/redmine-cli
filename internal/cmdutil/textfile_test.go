package cmdutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTextFile_PreservesContent(t *testing.T) {
	// CRLF line endings, a BOM-less UTF-8 payload, leading/trailing blank
	// lines and trailing spaces must all survive unchanged.
	content := "\n  indented\r\nline with trailing spaces   \n\nü ✓\n\n"
	path := filepath.Join(t.TempDir(), "body.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ReadTextFile(path, nil)
	if err != nil {
		t.Fatalf("ReadTextFile: %v", err)
	}
	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}

	got, err = ReadTextFile("-", strings.NewReader(content))
	if err != nil {
		t.Fatalf("ReadTextFile(-): %v", err)
	}
	if got != content {
		t.Errorf("stdin: got %q, want %q", got, content)
	}
}

func TestReadTextFile_Errors(t *testing.T) {
	if _, err := ReadTextFile("", nil); err == nil {
		t.Error("expected an error for an empty path")
	}
	if _, err := ReadTextFile(filepath.Join(t.TempDir(), "missing"), nil); err == nil {
		t.Error("expected an error for a missing file")
	}
	if _, err := ReadTextFile("-", strings.NewReader("\xff")); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Errorf("err = %v, want a UTF-8 validation error", err)
	}
}
