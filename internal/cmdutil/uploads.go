package cmdutil

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aarondpn/redmine-cli/internal/api"
	"github.com/aarondpn/redmine-cli/internal/models"
)

// UploadAttachments uploads each given file path and returns Upload references
// suitable for inclusion in an issue create/update payload.
func UploadAttachments(ctx context.Context, client *api.Client, paths []string) ([]models.Upload, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	uploads := make([]models.Upload, 0, len(paths))
	for _, p := range paths {
		up, err := uploadOne(ctx, client, p)
		if err != nil {
			return nil, fmt.Errorf("uploading %s: %w", p, err)
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

	ct := detectContentType(f, path)
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

// detectContentType resolves a MIME type from the file extension, falling back
// to sniffing the first 512 bytes. The file position is left unspecified; the
// caller must seek before reading the file for upload.
func detectContentType(f *os.File, path string) string {
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
