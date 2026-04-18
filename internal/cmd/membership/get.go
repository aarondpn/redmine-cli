package membership

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdMembershipGet(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "get <id>",
		Aliases: []string{"show", "view"},
		Short:   "Get membership details",
		Long:    "Display detailed information about a specific membership.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("membership ID must be a number: %s", args[0])
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching membership...")
			m, err := client.Memberships.Get(context.Background(), id)
			stop()
			if err != nil {
				return fmt.Errorf("failed to get membership %d: %w", id, err)
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(m)
				return nil
			}

			pairs := []output.KeyValue{
				{Key: "ID", Value: output.StyleID.Render(fmt.Sprintf("%d", m.ID))},
				{Key: "Project", Value: m.Project.Name},
				{Key: "User/Group", Value: memberName(*m)},
				{Key: "Roles", Value: formatRoles(m.Roles)},
			}
			printer.Detail(pairs)
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
