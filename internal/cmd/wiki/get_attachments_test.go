package wiki

import (
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// The attachment IDs are what wiki update --remove-attachment takes, so the
// table output must show them, with and without --version.
func TestGet_AttachmentsTable(t *testing.T) {
	const page = `{"wiki_page":{"title":"MyPage","text":"body","version":2,"attachments":[{"id":42,"filename":"spec.pdf","filesize":1234,"content_type":"application/pdf"}]}}`
	for _, args := range [][]string{
		{"--include", "attachments"},
		{"--include", "attachments", "--version", "1"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			srv := newWikiGetServer(t, page)
			defer srv.Close()

			f := testutil.NewFactory(t, srv.URL)
			cmd := newCmdGet(f)
			cmd.SetArgs(append([]string{"MyPage", "--project", "proj", "--output", "table"}, args...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			stdout := testutil.Stdout(f)
			for _, want := range []string{"Attachments:", "ID", "Content-Type", "42", "spec.pdf", "1234", "application/pdf"} {
				if !strings.Contains(stdout, want) {
					t.Errorf("output missing %q:\n%s", want, stdout)
				}
			}
		})
	}
}
