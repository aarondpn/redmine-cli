package group

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
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
			result, err := ops.ListGroups(context.Background(), client, ops.ListGroupsInput{
				Limit:  cmdutil.OpsLimit(limit),
				Offset: offset,
			})
			stop()
			if err != nil {
				return err
			}
			groups, total := result.Groups, result.TotalCount

			cmdutil.RenderCollection(printer, groups, []string{"ID", "Name"}, func(g models.Group, styled bool) []string {
				id := fmt.Sprintf("%d", g.ID)
				if styled {
					id = output.StyleID.Render(id)
				}
				return []string{id, g.Name}
			})

			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(groups), Total: total, Limit: limit, Offset: offset, Noun: "groups",
			})
			return nil
		},
	}

	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
