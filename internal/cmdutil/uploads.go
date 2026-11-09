package cmdutil

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// AddAttachFlags registers the repeatable --attach and --attach-description
// flags. Descriptions pair with attachments by position: the first
// --attach-description describes the first --attach, and so on. Pass both
// slices to UploadAttachments.
func AddAttachFlags(cmd *cobra.Command, paths, descriptions *[]string) {
	cmd.Flags().StringArrayVar(paths, "attach", nil, "Path to file to attach (repeatable)")
	cmd.Flags().StringArrayVar(descriptions, "attach-description", nil,
		"Description of the file given by the --attach at the same position (repeatable; pass \"\" to skip a file)")
}

// CheckAttachDescriptions rejects more --attach-description values than
// --attach values. Call it before any request so nothing is uploaded or
// changed when the flags do not pair up.
func CheckAttachDescriptions(paths, descriptions []string) error {
	if len(descriptions) > len(paths) {
		return fmt.Errorf("--attach-description given %d time(s) but --attach only %d: each description pairs with the --attach at the same position",
			len(descriptions), len(paths))
	}
	return nil
}

// UploadAttachments uploads each given file path and returns Upload references
// suitable for inclusion in an issue, wiki or news create/update payload.
// descriptions[i], when present and non-empty, becomes the description of
// paths[i]; see AddAttachFlags.
func UploadAttachments(ctx context.Context, client *api.Client, paths []string, descriptions ...string) ([]models.Upload, error) {
	if err := CheckAttachDescriptions(paths, descriptions); err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil
	}
	uploads := make([]models.Upload, 0, len(paths))
	for i, p := range paths {
		up, err := uploadOne(ctx, client, p)
		if err != nil {
			return nil, fmt.Errorf("uploading %s: %w", p, err)
		}
		if i < len(descriptions) {
			up.Description = descriptions[i]
		}
		uploads = append(uploads, up)
	}
	return uploads, nil
}

func uploadOne(ctx context.Context, client *api.Client, path string) (models.Upload, error) {
	f, err := os.Open(path)
	if err != nil {
		return models.Upload{}, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return models.Upload{}, err
	}

	ct := DetectContentType(f, path)
	if _, err := f.Seek(0, 0); err != nil {
		return models.Upload{}, err
	}

	name := filepath.Base(path)
	token, err := client.Attachments.Upload(ctx, name, f, info.Size())
	if err != nil {
		return models.Upload{}, err
	}

	return models.Upload{
		Token:       token,
		Filename:    name,
		ContentType: ct,
	}, nil
}

// DetectContentType resolves a MIME type from the file extension, falling back
// to sniffing the first 512 bytes. The file position is left unspecified; the
// caller must seek before reading the file for upload.
func DetectContentType(f *os.File, path string) string {
	if ct := mime.TypeByExtension(filepath.Ext(path)); ct != "" {
		return ct
	}
	var sniff [512]byte
	n, _ := f.Read(sniff[:])
	if n == 0 {
		return ""
	}
	return http.DetectContentType(sniff[:n])
}
