package wiki

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdGet(f *cmdutil.Factory) *cobra.Command {
	var (
		project string
		version int
		include []string
		format  string
	)

	cmd := &cobra.Command{
		Use:     "get <page>",
		Aliases: []string{"show", "view"},
		Short:   "Get wiki page details",
		Long:    "Display detailed information about a Redmine wiki page.",
		Example: `  # View a wiki page
  redmine wiki get WikiStart --project myproject

  # View a specific version
  redmine wiki get WikiStart --project myproject --version 3

  # Include attachments
  redmine wiki get WikiStart --project myproject --include attachments`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateGetIncludes(include); err != nil {
				return err
			}

			ctx := cmdutil.Context(cmd)

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			projectID, err := cmdutil.RequireProjectIdentifier(ctx, f, project)
			if err != nil {
				return err
			}

			pageTitle := args[0]

			stop := printer.Spinner("Fetching wiki page...")
			getInput := ops.GetWikiPageInput{
				ProjectID: projectID,
				Page:      pageTitle,
				Includes:  cmdutil.MergeIncludes(include),
			}
			if version > 0 {
				// With --include attachments, a versioned fetch lists the
				// page's current attachments, not those of that version.
				getInput.Version = &version
			}
			page, err := ops.GetWikiPage(ctx, client, getInput)
			stop()
			if err != nil {
				return fmt.Errorf("failed to get wiki page %q: %w", pageTitle, err)
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(page)
				return nil
			}

			pairs := []output.KeyValue{
				{Key: "Title", Value: output.StyleID.Render(page.Title)},
				{Key: "Version", Value: strconv.Itoa(page.Version)},
			}

			if page.Project != nil {
				pairs = append(pairs, output.KeyValue{Key: "Project", Value: page.Project.Name})
			}
			if page.Author != nil {
				pairs = append(pairs, output.KeyValue{Key: "Author", Value: page.Author.Name})
			}
			if page.Comments != "" {
				pairs = append(pairs, output.KeyValue{Key: "Comments", Value: page.Comments})
			}
			if page.Parent != nil {
				pairs = append(pairs, output.KeyValue{Key: "Parent", Value: page.Parent.Title})
			}

			pairs = append(pairs,
				output.KeyValue{Key: "Created", Value: page.CreatedOn},
				output.KeyValue{Key: "Updated", Value: page.UpdatedOn},
			)

			if page.Text != "" {
				pairs = append(pairs, output.KeyValue{Key: "Content", Value: page.Text})
			}

			printer.Detail(pairs)

			if len(page.Attachments) > 0 {
				fmt.Fprintln(f.IOStreams.Out)
				fmt.Fprintln(f.IOStreams.Out, "Attachments:")
				rows := make([][]string, len(page.Attachments))
				for i, a := range page.Attachments {
					rows[i] = []string{
						strconv.Itoa(a.ID),
						a.Filename,
						strconv.FormatInt(a.Filesize, 10),
						a.ContentType,
					}
				}
				printer.Table([]string{"ID", "Filename", "Size", "Content-Type"}, rows)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project identifier or ID (required if no default)")
	cmd.Flags().IntVar(&version, "version", 0, "Page version (0 for latest)")
	cmd.Flags().StringSliceVar(&include, "include", nil, "Include related data: attachments (repeatable or comma-separated)")
	_ = cmd.RegisterFlagCompletionFunc("include", completeGetIncludes)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
