package cmdutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// SaveAttachmentToDir streams an attachment into dir using its real filename
// and returns the path written and the number of bytes streamed. The directory
// is created if it does not exist. The server-supplied filename is reduced to
// its base component so a malicious or malformed name (e.g. one containing
// "../") cannot escape dir.
func SaveAttachmentToDir(ctx context.Context, client *api.Client, att *models.Attachment, dir string) (string, int64, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, fmt.Errorf("creating directory %s: %w", dir, err)
	}
	name := filepath.Base(att.Filename)
	// filepath.Base can only yield ".", "..", the separator, or "" for a
	// degenerate name; all of them would write outside dir (or to dir itself),
	// so fall back to a synthetic, contained name.
	if name == "." || name == ".." || name == string(os.PathSeparator) || name == "" {
		name = fmt.Sprintf("attachment-%d", att.ID)
	}
	path := filepath.Join(dir, name)
	n, err := SaveAttachmentToFile(ctx, client, att, path)
	return path, n, err
}

// SaveAttachmentToFile streams an attachment to the exact path given and
// returns the number of bytes written. The parent directory must already
// exist. The file is created (or truncated) with 0o644 permissions.
func SaveAttachmentToFile(ctx context.Context, client *api.Client, att *models.Attachment, path string) (int64, error) {
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("creating %s: %w", path, err)
	}

	n, err := client.Attachments.Download(ctx, att, f)
	if closeErr := f.Close(); closeErr != nil && err == nil {
		err = fmt.Errorf("closing %s: %w", path, closeErr)
	}
	if err != nil {
		// Best-effort cleanup of the partial file so a failed download does
		// not leave a truncated artifact behind.
		_ = os.Remove(path)
		return n, err
	}
	return n, nil
}
