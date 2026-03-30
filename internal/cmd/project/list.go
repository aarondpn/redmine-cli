package project

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
)

func newCmdList(f *cmdutil.Factory) *cobra.Command {
	var (
		limit  int
		offset int
		format string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List projects",
		Long:    "List all accessible Redmine projects.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			projects, total, err := client.Projects.List(context.Background(), nil, limit, offset)
			if err != nil {
				return err
			}

			headers := []string{"ID", "Identifier", "Name", "Status", "Public"}

			switch printer.Format() {
			case output.FormatJSON:
				printer.JSON(projects)
			case output.FormatCSV:
				rows := make([][]string, 0, len(projects))
				for _, p := range projects {
					rows = append(rows, []string{
						strconv.Itoa(p.ID),
						p.Identifier,
						p.Name,
						projectStatusLabel(p.Status),
						formatBool(p.IsPublic),
					})
				}
				printer.CSV(headers, rows)
			default:
				rows := make([][]string, 0, len(projects))
				for _, p := range projects {
					rows = append(rows, []string{
						output.StyleID.Render(strconv.Itoa(p.ID)),
						p.Identifier,
						p.Name,
						projectStatusLabel(p.Status),
						formatBool(p.IsPublic),
					})
				}
				printer.Table(headers, rows)
			}

			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(projects), Total: total, Limit: limit, Offset: offset, Noun: "projects",
			})

			return nil
		},
	}

	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}

func projectStatusLabel(status int) string {
	switch status {
	case 1:
		return "active"
	case 5:
		return "archived"
	case 9:
		return "closed"
	default:
		return strconv.Itoa(status)
	}
}

func formatBool(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
