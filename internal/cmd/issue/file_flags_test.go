package issue

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// longText exercises the content the file flags exist for: multi-line
// Textile with quotes, shell metacharacters, non-ASCII text and a trailing
// newline, all of which must reach Redmine byte-for-byte.
const longText = "h2. Summary\n\n* item with \"quotes\" and 'apostrophes'\n* $(not a subshell) `nor this`\n\nUmlaute: äöü — ✓\n"

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.textile")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCmdIssueUpdate_DescriptionFile(t *testing.T) {
	fake := &issueStatusServer{}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(NewCmdUpdate(f))
	cmd.SetArgs([]string{"7", "--description-file", writeTempFile(t, longText)})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := fake.lastIssue(t)["description"]; got != longText {
		t.Errorf("description = %q, want %q", got, longText)
	}
}

func TestCmdIssueUpdate_NoteFileFromStdin(t *testing.T) {
	fake := &issueStatusServer{}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	f.IOStreams.In.(*bytes.Buffer).WriteString(longText)
	cmd := quiet(NewCmdUpdate(f))
	cmd.SetArgs([]string{"7", "--note-file", "-", "--private-notes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	issue := fake.lastIssue(t)
	if issue["notes"] != longText {
		t.Errorf("notes = %q, want %q", issue["notes"], longText)
	}
	if issue["private_notes"] != true {
		t.Errorf("private_notes = %v, want true (--note-file satisfies --private-notes)", issue["private_notes"])
	}
}

func TestCmdIssueUpdate_FileFlagErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"description mutually exclusive", []string{"7", "--description", "x", "--description-file", "f"}, "[description description-file] were all set"},
		{"note mutually exclusive", []string{"7", "--note", "x", "--note-file", "f"}, "[note note-file] were all set"},
		{"two stdin readers", []string{"7", "--description-file", "-", "--note-file", "-"}, "cannot all read from standard input"},
		{"missing file", []string{"7", "--note-file", filepath.Join(t.TempDir(), "missing.txt")}, "--note-file:"},
		{"invalid utf-8", []string{"7", "--description-file", writeTempFile(t, "\xff\xfe")}, "not valid UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &issueStatusServer{}
			srv := fake.start(t)

			f := testutil.NewFactory(t, srv.URL)
			cmd := quiet(NewCmdUpdate(f))
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.want)
			}
			if len(fake.puts) != 0 {
				t.Errorf("no update should be sent, got %v", fake.puts)
			}
		})
	}
}

func TestCmdIssueComment_MessageFile(t *testing.T) {
	fake := &issueStatusServer{}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(NewCmdComment(f))
	cmd.SetArgs([]string{"7", "--message-file", writeTempFile(t, longText)})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := fake.lastIssue(t)["notes"]; got != longText {
		t.Errorf("notes = %q, want %q", got, longText)
	}
}

func TestCmdIssueComment_MessageFileShorthandStdin(t *testing.T) {
	fake := &issueStatusServer{}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	f.IOStreams.In.(*bytes.Buffer).WriteString(longText)
	cmd := quiet(NewCmdComment(f))
	cmd.SetArgs([]string{"7", "-F", "-"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := fake.lastIssue(t)["notes"]; got != longText {
		t.Errorf("notes = %q, want %q", got, longText)
	}
}

func TestCmdIssueComment_MessageFileErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"mutually exclusive", []string{"7", "-m", "x", "-F", "f"}, "[message message-file] were all set"},
		{"empty file", []string{"7", "--message-file", writeTempFile(t, "")}, "--message-file is empty"},
		{"neither flag", []string{"7"}, "--message or --message-file is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &issueStatusServer{}
			srv := fake.start(t)

			f := testutil.NewFactory(t, srv.URL)
			cmd := quiet(NewCmdComment(f))
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.want)
			}
			if len(fake.puts) != 0 {
				t.Errorf("no update should be sent, got %v", fake.puts)
			}
		})
	}
}

func TestCmdIssueClose_NoteFile(t *testing.T) {
	fake := &issueStatusServer{issueBody: issue7Closed}
	srv := fake.start(t)

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(NewCmdClose(f))
	cmd.SetArgs([]string{"7", "--note-file", writeTempFile(t, longText)})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := fake.lastIssue(t)["notes"]; got != longText {
		t.Errorf("notes = %q, want %q", got, longText)
	}
}

func TestCmdIssueCreate_DescriptionFile(t *testing.T) {
	var posted map[string]any
	srv := newCreateIssueServer(t, &posted)
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	f.IOStreams.In.(*bytes.Buffer).WriteString(longText)
	cmd := quiet(NewCmdCreate(f))
	cmd.SetArgs([]string{"--project", "17", "--subject", "test", "--description-file", "-", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	issue, _ := posted["issue"].(map[string]any)
	if issue["description"] != longText {
		t.Errorf("description = %q, want %q", issue["description"], longText)
	}
}

func TestCmdIssueCreate_DescriptionFlagsMutuallyExclusive(t *testing.T) {
	var posted map[string]any
	srv := newCreateIssueServer(t, &posted)
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := quiet(NewCmdCreate(f))
	cmd.SetArgs([]string{"--project", "17", "--subject", "test", "--description", "x", "--description-file", "-"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "[description description-file] were all set") {
		t.Fatalf("err = %v, want a mutual-exclusion error", err)
	}
	if posted != nil {
		t.Errorf("no issue should be created, got %v", posted)
	}
}
