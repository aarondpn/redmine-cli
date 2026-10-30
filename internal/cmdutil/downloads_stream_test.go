package cmdutil

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func writeString(s string) StreamFunc {
	return func(_ context.Context, w io.Writer) (int64, error) {
		n, err := io.WriteString(w, s)
		return int64(n), err
	}
}

func TestSaveStreamToFile_WritesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thumb.png")
	n, err := SaveStreamToFile(context.Background(), path, SaveOptions{}, writeString("pixels"))
	if err != nil || n != 6 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if got, _ := os.ReadFile(path); string(got) != "pixels" {
		t.Errorf("content = %q", got)
	}
}

func TestSaveStreamToFile_NoClobberAndClobber(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thumb.png")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := SaveStreamToFile(context.Background(), path, SaveOptions{}, writeString("new"))
	var exists *DestinationExistsError
	if !errors.As(err, &exists) {
		t.Fatalf("err = %v, want DestinationExistsError", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "old" {
		t.Errorf("existing file modified: %q", got)
	}
	if _, err := SaveStreamToFile(context.Background(), path, SaveOptions{Clobber: true}, writeString("new")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "new" {
		t.Errorf("content = %q, want new", got)
	}
}

func TestSaveStreamToFile_FailureLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "thumb.png")
	boom := errors.New("boom")
	_, err := SaveStreamToFile(context.Background(), path, SaveOptions{}, func(context.Context, io.Writer) (int64, error) {
		return 0, boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("leftover files: %v", entries)
	}
}
