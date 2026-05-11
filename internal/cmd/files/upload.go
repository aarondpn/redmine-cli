package files

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func newCmdUpload(f *cmdutil.Factory) *cobra.Command {
	var (
		project     string
		filename    string
		version     string
		description string
		contentType string
		format      string
	)

	cmd := &cobra.Command{
		Use:     "upload <path>",
		Aliases: []string{"add"},
		Short:   "Upload a file to a project",
		Long:    "Upload a file from disk and attach it to a Redmine project. Optionally pin it to a version (milestone).",
		Example: `  # Upload a release artifact
  redmine files upload ./release.tar.gz --project myproject

  # Attach the upload to a version (milestone) and add a description
  redmine files upload ./changelog.md --project myproject \
    --version 1.2.0 --description "Release notes"

  # Override the displayed filename
  redmine files upload ./build.zip --project myproject --filename "build-1.2.0.zip"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			projectID, err := cmdutil.RequireProjectIdentifier(ctx, f, project)
			if err != nil {
				return err
			}

			path := args[0]
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("opening %s: %w", path, err)
			}
			defer file.Close()

			info, err := file.Stat()
			if err != nil {
				return fmt.Errorf("stat %s: %w", path, err)
			}

			displayName := filename
			if displayName == "" {
				displayName = filepath.Base(path)
			}

			ct := contentType
			if ct == "" {
				ct = detectContentType(file, path)
				if _, err := file.Seek(0, 0); err != nil {
					return fmt.Errorf("rewinding %s: %w", path, err)
				}
			}

			var versionID int
			if version != "" {
				versionID, err = resolver.ResolveVersion(ctx, client, version, projectID)
				if err != nil {
					return err
				}
			}

			stop := printer.Spinner("Uploading file...")
			token, err := client.Attachments.Upload(ctx, displayName, file, info.Size())
			if err != nil {
				stop()
				return fmt.Errorf("failed to upload %s: %w", path, err)
			}

			if _, err := ops.UploadProjectFile(ctx, client, ops.UploadProjectFileInput{
				ProjectID:   projectID,
				Token:       token,
				Filename:    displayName,
				VersionID:   versionID,
				Description: description,
				ContentType: ct,
			}); err != nil {
				stop()
				return fmt.Errorf("failed to attach %s to project: %w", displayName, err)
			}
			stop()

			printer.Action(output.ActionUploaded, "project_file", displayName,
				fmt.Sprintf("Uploaded %s to project %s", displayName, projectID))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project identifier or ID (required if no default)")
	cmd.Flags().StringVar(&filename, "filename", "", "Override the displayed filename (defaults to the file's base name)")
	cmd.Flags().StringVar(&version, "version", "", "Attach to a version: name or numeric ID")
	cmd.Flags().StringVar(&description, "description", "", "Optional description")
	cmd.Flags().StringVar(&contentType, "content-type", "", "Override the detected MIME type")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("version", cmdutil.CompleteVersions(f))

	return cmd
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
