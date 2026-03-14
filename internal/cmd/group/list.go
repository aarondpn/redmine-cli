package group

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/output"
	"github.com/spf13/cobra"
)

func newCmdGroupList(f *cmdutil.Factory) *cobra.Command {
	var (
		limit  int
		offset int
		format string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List groups",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			stop := printer.Spinner("Fetching groups...")
			groups, total, err := client.Groups.List(context.Background(), models.GroupFilter{
				Limit:  limit,
				Offset: offset,
			})
			stop()
			if err != nil {
				return err
			}

			switch printer.Format() {
			case output.FormatJSON:
				printer.JSON(groups)
			case output.FormatCSV:
				headers := []string{"ID", "Name"}
				rows := make([][]string, len(groups))
				for i, g := range groups {
					rows[i] = []string{
						fmt.Sprintf("%d", g.ID), g.Name,
					}
				}
				printer.CSV(headers, rows)
			default:
				headers := []string{"ID", "Name"}
				rows := make([][]string, len(groups))
				for i, g := range groups {
					rows[i] = []string{
						output.StyleID.Render(fmt.Sprintf("%d", g.ID)),
						g.Name,
					}
				}
				printer.Table(headers, rows)
				fmt.Fprintf(f.IOStreams.ErrOut, "\nShowing %d of %d groups\n", len(groups), total)
			}
			return nil
		},
	}

	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
