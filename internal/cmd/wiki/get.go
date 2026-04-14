package wiki

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/output"
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
		Aliases: []string{"show"},
		Short:   "Get wiki page details",
		Long:    "Display detailed information about a Redmine wiki page.",
		Args:    cobra.ExactArgs(1),
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

			pageTitle := args[0]

			var page *models.WikiPage
			if version > 0 {
				page, err = client.Wikis.GetVersion(ctx, projectID, pageTitle, version)
			} else {
				page, err = client.Wikis.Get(ctx, projectID, pageTitle, include)
			}
			if err != nil {
				return err
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(page)
				return nil
			}

			pairs := []output.KeyValue{
				{Key: "Title", Value: output.StyleID.Render(page.Title)},
				{Key: "Version", Value: strconv.Itoa(page.Version)},
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

			if len(page.Attachments) > 0 {
				pairs = append(pairs, output.KeyValue{
					Key:   "Attachments",
					Value: fmt.Sprintf("%d file(s)", len(page.Attachments)),
				})
				for _, a := range page.Attachments {
					pairs = append(pairs, output.KeyValue{
						Key:   fmt.Sprintf("  %s", a.Filename),
						Value: fmt.Sprintf("%s (%d bytes)", a.Description, a.Filesize),
					})
				}
			}

			if page.Text != "" {
				pairs = append(pairs, output.KeyValue{Key: "Content", Value: page.Text})
			}

			printer.Detail(pairs)

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project identifier or ID (required if no default)")
	cmd.Flags().IntVar(&version, "version", 0, "Page version (0 for latest)")
	cmd.Flags().StringSliceVar(&include, "include", nil, "Include additional data (attachments)")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
