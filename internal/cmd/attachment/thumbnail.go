package attachment

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// thumbnailResult is the JSON payload emitted (with -o json) after a
// thumbnail has been saved to a file.
type thumbnailResult struct {
	ID    int    `json:"id"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
	Size  int    `json:"size"` // requested size in pixels; 0 = server default
}

func newCmdThumbnail(f *cmdutil.Factory) *cobra.Command {
	var (
		path    string
		size    int
		clobber bool
		format  string
	)

	cmd := &cobra.Command{
		Use:     "thumbnail <id>",
		Aliases: []string{"thumb"},
		Short:   "Download an attachment's thumbnail image",
		Long: "Download the thumbnail Redmine renders for an image attachment (or PDF, where the server\n" +
			"supports it) to --path, or to stdout with `--path -`.\n\n" +
			"--size is the edge length in pixels; Redmine rounds it up to a multiple of 50 and caps it at\n" +
			"800. Without it the server's default thumbnail size is used. Redmine answers \"not found\"\n" +
			"when the attachment cannot be thumbnailed (it is not an image, or the server has no\n" +
			"ImageMagick installed).\n\n" +
			"An existing file at --path is never overwritten unless --clobber is given.\n\n" +
			"Note: -o/--output controls the confirmation output (json prints a result object). It does\n" +
			"NOT change where the bytes go; use --path for that.",
		Example: `  # Save the default-size thumbnail
  redmine attachments thumbnail 42 --path thumb.png

  # Request a larger thumbnail
  redmine attachments thumbnail 42 --size 200 --path thumb.png

  # Replace an earlier thumbnail
  redmine attachments thumbnail 42 --path thumb.png --clobber

  # Stream to stdout
  redmine attachments thumbnail 42 --path - > thumb.png

  # Print a JSON result describing the saved file
  redmine attachments thumbnail 42 --path thumb.png -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid attachment ID: %s", args[0])
			}
			if id <= 0 {
				return fmt.Errorf("invalid attachment ID: %s", args[0])
			}
			if size < 0 {
				return fmt.Errorf("invalid --size %d: must be a positive number of pixels", size)
			}
			if path == "-" && f.JQ != nil {
				return fmt.Errorf("--jq is not supported with --path -: the thumbnail bytes are written to stdout")
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			ctx := cmdutil.Context(cmd)
			stop := printer.Spinner("Downloading thumbnail...")

			// Stream to stdout: stdout carries the raw bytes, so the
			// confirmation always goes to stderr and no JSON envelope is
			// emitted (it would corrupt the piped output).
			if path == "-" {
				n, err := client.Attachments.Thumbnail(ctx, id, size, f.IOStreams.Out)
				stop()
				if err != nil {
					return fmt.Errorf("failed to download thumbnail of attachment %d: %w", id, err)
				}
				fmt.Fprintf(f.IOStreams.ErrOut, "Downloaded thumbnail of attachment %d (%d bytes) to stdout\n", id, n)
				return nil
			}

			n, err := cmdutil.SaveStreamToFile(ctx, path, cmdutil.SaveOptions{Clobber: clobber}, func(ctx context.Context, w io.Writer) (int64, error) {
				return client.Attachments.Thumbnail(ctx, id, size, w)
			})
			stop()
			if err != nil {
				return fmt.Errorf("failed to download thumbnail of attachment %d: %w", id, err)
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(thumbnailResult{ID: id, Path: path, Bytes: n, Size: size})
				return nil
			}
			printer.Success(fmt.Sprintf("Saved thumbnail of attachment %d (%d bytes) to %s", id, n, path))
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Destination file path (use - for stdout) (required)")
	cmd.Flags().IntVar(&size, "size", 0, "Thumbnail edge length in pixels (Redmine rounds up to a multiple of 50, max 800)")
	cmd.Flags().BoolVar(&clobber, "clobber", false, "Overwrite an existing file at the destination")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.MarkFlagRequired("path")

	return cmd
}
