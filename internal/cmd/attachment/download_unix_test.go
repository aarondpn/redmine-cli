//go:build unix

package attachment

import (
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// `--path /dev/null` worked before downloads went through a temporary file
// and must keep working without --clobber.
func TestCmdAttachmentDownload_ToDevNull(t *testing.T) {
	srv := attachmentServer(t)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdDownload(f)
	cmd.SetArgs([]string{"55", "--path", "/dev/null"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "to /dev/null") {
		t.Errorf("stderr = %q, want the confirmation for /dev/null", stderr)
	}
}
