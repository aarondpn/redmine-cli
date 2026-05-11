package files

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdList(f *cmdutil.Factory) *cobra.Command {
	var (
		project string
		limit   int
		offset  int
		format  string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List project files",
		Long: "List files attached to a Redmine project.\n\n" +
			"The Redmine endpoint returns the full file list in a single response and " +
			"ignores server-side pagination, so --limit and --offset are applied client-side.",
		Example: `  # List files in a project
  redmine files list --project myproject

  # JSON output
  redmine files list --project myproject -o json

  # Paginate
  redmine files list --project myproject --limit 10 --offset 20`,
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

			stop := printer.Spinner("Fetching project files...")
			result, err := ops.ListProjectFiles(ctx, client, ops.ListProjectFilesInput{
				ProjectID: projectID,
				Limit:     cmdutil.OpsLimit(limit),
				Offset:    offset,
			})
			stop()
			if err != nil {
				return fmt.Errorf("failed to list project files: %w", err)
			}
			fileList, total := result.Files, result.TotalCount

			if cmdutil.HandleEmpty(printer, fileList, "files") {
				return nil
			}

			cmdutil.RenderCollection(
				printer, fileList,
				[]string{"ID", "Filename", "Size", "Version", "Author", "Created"},
				func(file models.ProjectFile, styled bool) []string {
					id := strconv.Itoa(file.ID)
					if styled {
						id = output.StyleID.Render(id)
					}
					version := ""
					if file.Version != nil {
						version = file.Version.Name
					}
					return []string{
						id,
						file.Filename,
						strconv.FormatInt(file.Filesize, 10),
						version,
						file.Author.Name,
						file.CreatedOn,
					}
				},
			)

			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(fileList), Total: total, Limit: limit, Offset: offset, Noun: "files",
			})

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project identifier or ID (required if no default)")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
