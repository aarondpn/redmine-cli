package project

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/output"
)

func newCmdMembers(f *cmdutil.Factory) *cobra.Command {
	var (
		limit  int
		format string
	)

	cmd := &cobra.Command{
		Use:   "members <identifier>",
		Short: "List project members",
		Long:  "List all members of a Redmine project.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			members, total, err := client.Projects.Members(context.Background(), args[0], limit)
			if err != nil {
				return err
			}

			if format == output.FormatJSON {
				printer.JSON(members)
				return nil
			}

			headers := []string{"ID", "User/Group", "Roles"}
			rows := make([][]string, 0, len(members))
			for _, m := range members {
				rows = append(rows, []string{
					output.StyleID.Render(strconv.Itoa(m.ID)),
					memberName(m),
					formatRoles(m.Roles),
				})
			}

			if format == output.FormatCSV {
				printer.CSV(headers, rows)
			} else {
				printer.Table(headers, rows)
				fmt.Fprintf(cmd.ErrOrStderr(), "Showing %d of %d members\n", len(members), total)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of results")
	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}

func memberName(m models.Membership) string {
	if m.User != nil {
		return m.User.Name
	}
	if m.Group != nil {
		return m.Group.Name + " (group)"
	}
	return "unknown"
}

func formatRoles(roles []models.IDName) string {
	names := make([]string, len(roles))
	for i, r := range roles {
		names[i] = r.Name
	}
	return strings.Join(names, ", ")
}
