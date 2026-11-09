package attachment

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func TestRenderAttachmentDetail_Thumbnail(t *testing.T) {
	var buf bytes.Buffer
	p := output.NewStdPrinter(&buf, &buf, false, true, "table")
	renderAttachmentDetail(p, &models.Attachment{ID: 1, Filename: "a.png", ThumbnailURL: "http://x/thumb/1"})
	if !strings.Contains(buf.String(), "Thumbnail URL:") || !strings.Contains(buf.String(), "http://x/thumb/1") {
		t.Errorf("missing thumbnail:\n%s", buf.String())
	}
	buf.Reset()
	renderAttachmentDetail(p, &models.Attachment{ID: 1, Filename: "a.png"})
	if strings.Contains(buf.String(), "Thumbnail") {
		t.Errorf("unexpected thumbnail row:\n%s", buf.String())
	}
}
