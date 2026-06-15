package attachment

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// downloadResult is the JSON payload emitted (with -o json) after a successful
// download to a file or directory.
type downloadResult struct {
	ID          int    `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type,omitempty"`
	Path        string `json:"path"`
	Bytes       int64  `json:"bytes"`
}

func newCmdDownload(f *cmdutil.Factory) *cobra.Command {
	var (
		dir    string
		path   string
		format string
	)

	cmd := &cobra.Command{
		Use:     "download <id>",
		Aliases: []string{"dl", "get-file"},
		Short:   "Download an attachment file",
		Long: "Download the raw bytes of an attachment, authenticating with the active\n" +
			"profile's server and API key (no manual key handling, no curl).\n\n" +
			"By default the file is saved in the current directory under its real\n" +
			"filename. Use --dir to choose a directory, --path to choose an exact file\n" +
			"path, or `--path -` to stream the bytes to stdout for piping. The file is\n" +
			"streamed to disk, never buffered entirely in memory.\n\n" +
			"Note: for this command -o/--output controls the confirmation output\n" +
			"(json prints a result object; otherwise a human-readable line is written to\n" +
			"stderr). It does NOT change where the bytes go; use --path/--dir for that.",
		Example: `  # Save to ./<filename> in the current directory
  redmine attachments download 42

  # Save into a directory, keeping the real filename
  redmine attachments download 42 --dir ./downloads

  # Save to an explicit path
  redmine attachments download 42 --path ./logo.png

  # Stream to stdout (pipe into another tool)
  redmine attachments download 42 --path - > logo.png

  # Print a JSON result describing the saved file
  redmine attachments download 42 --dir ./downloads -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid attachment ID: %s", args[0])
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			ctx := context.Background()
			stop := printer.Spinner("Downloading attachment...")
			att, err := ops.GetAttachment(ctx, client, ops.GetAttachmentInput{ID: id})
			if err != nil {
				stop()
				return fmt.Errorf("failed to get attachment %d: %w", id, err)
			}

			// Stream to stdout: stdout carries the raw bytes, so the
			// confirmation always goes to stderr and no JSON envelope is
			// emitted (it would corrupt the piped output).
			if path == "-" {
				n, err := client.Attachments.Download(ctx, att, f.IOStreams.Out)
				stop()
				if err != nil {
					return fmt.Errorf("failed to download attachment %d: %w", id, err)
				}
				fmt.Fprintf(f.IOStreams.ErrOut, "Downloaded %q (%d bytes) to stdout\n", att.Filename, n)
				return nil
			}

			var (
				dest  string
				bytes int64
			)
			switch {
			case path != "":
				dest = path
				bytes, err = cmdutil.SaveAttachmentToFile(ctx, client, att, path)
			case dir != "":
				dest, bytes, err = cmdutil.SaveAttachmentToDir(ctx, client, att, dir)
			default:
				dest, bytes, err = cmdutil.SaveAttachmentToDir(ctx, client, att, ".")
			}
			stop()
			if err != nil {
				return fmt.Errorf("failed to download attachment %d: %w", id, err)
			}

			result := downloadResult{
				ID:          att.ID,
				Filename:    att.Filename,
				ContentType: att.ContentType,
				Path:        dest,
				Bytes:       bytes,
			}
			if printer.Format() == output.FormatJSON {
				printer.JSON(result)
				return nil
			}
			printer.Success(fmt.Sprintf("Downloaded %q (%d bytes) to %s", att.Filename, bytes, dest))
			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Directory to save into, using the attachment's real filename")
	cmd.Flags().StringVar(&path, "path", "", "Exact destination file path (use - for stdout)")
	cmd.MarkFlagsMutuallyExclusive("dir", "path")
	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}
