package cmdutil

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Descriptions pair with paths by position; fewer descriptions than paths
// leave the rest undescribed, and an empty description means none.
func TestUploadAttachments_DescriptionsByPosition(t *testing.T) {
	ts, _ := newUploadServer(t, []string{"t1", "t2", "t3"})
	c := newClientFor(ts)

	p1 := writeTempFile(t, "one.txt", []byte("1"))
	p2 := writeTempFile(t, "two.txt", []byte("2"))
	p3 := writeTempFile(t, "three.txt", []byte("3"))

	ups, err := UploadAttachments(context.Background(), c, []string{p1, p2, p3}, "", "second")
	if err != nil {
		t.Fatalf("UploadAttachments: %v", err)
	}
	got := []string{ups[0].Description, ups[1].Description, ups[2].Description}
	if got[0] != "" || got[1] != "second" || got[2] != "" {
		t.Fatalf("descriptions = %q, want [\"\" \"second\" \"\"]", got)
	}
}

// More descriptions than paths fails before anything is uploaded.
func TestUploadAttachments_TooManyDescriptions(t *testing.T) {
	ts, recs := newUploadServer(t, nil)
	c := newClientFor(ts)

	p := writeTempFile(t, "one.txt", []byte("1"))
	_, err := UploadAttachments(context.Background(), c, []string{p}, "a", "b")
	if err == nil || !strings.Contains(err.Error(), "--attach-description given 2 time(s) but --attach only 1:") {
		t.Fatalf("err = %v", err)
	}
	if len(*recs) != 0 {
		t.Fatalf("uploaded %d files, want none", len(*recs))
	}
}

func TestCheckAttachDescriptions(t *testing.T) {
	if err := CheckAttachDescriptions([]string{"a", "b"}, []string{"x"}); err != nil {
		t.Fatalf("fewer descriptions: %v", err)
	}
	if err := CheckAttachDescriptions(nil, nil); err != nil {
		t.Fatalf("none: %v", err)
	}
	if err := CheckAttachDescriptions(nil, []string{"x"}); err == nil {
		t.Fatal("a description without --attach must fail")
	}
}

func TestAddAttachFlags_Repeatable(t *testing.T) {
	var paths, descs []string
	cmd := &cobra.Command{Use: "x", RunE: func(*cobra.Command, []string) error { return nil }}
	AddAttachFlags(cmd, &paths, &descs)
	cmd.SetArgs([]string{"--attach", "a,b.txt", "--attach-description", "x, y", "--attach", "c"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "a,b.txt" || len(descs) != 1 || descs[0] != "x, y" {
		t.Fatalf("paths = %q descs = %q (commas must not split values)", paths, descs)
	}
}
