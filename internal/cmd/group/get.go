package group

import (
	"context"
	"fmt"
	"strings"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
	"github.com/aarondpn/redmine-cli/internal/resolver"
	"github.com/spf13/cobra"
)

func newCmdGroupGet(f *cmdutil.Factory) *cobra.Command {
	var (
		format             string
		includeUsers       bool
		includeMemberships bool
	)

	cmd := &cobra.Command{
		Use:     "get <id-or-name>",
		Short:   "Show group details",
		Long:    "Show group details. Accepts a numeric ID or group name.",
		Aliases: []string{"show", "view"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			id, err := resolver.ResolveGroup(context.Background(), client, args[0])
			if err != nil {
				return err
			}

			printer := f.Printer(format)

			var includes []string
			if includeUsers {
				includes = append(includes, "users")
			}
			if includeMemberships {
				includes = append(includes, "memberships")
			}

			stop := printer.Spinner("Fetching group...")
			group, err := client.Groups.Get(context.Background(), id, includes)
			stop()
			if err != nil {
				return err
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(group)
				return nil
			}

			details := []output.KeyValue{
				{Key: "ID", Value: fmt.Sprintf("%d", group.ID)},
				{Key: "Name", Value: group.Name},
			}

			if includeUsers && len(group.Users) > 0 {
				names := make([]string, len(group.Users))
				for i, u := range group.Users {
					names[i] = fmt.Sprintf("%s (ID: %d)", u.Name, u.ID)
				}
				details = append(details, output.KeyValue{Key: "Users", Value: strings.Join(names, ", ")})
			}

			if includeMemberships && len(group.Memberships) > 0 {
				names := make([]string, len(group.Memberships))
				for i, m := range group.Memberships {
					names[i] = fmt.Sprintf("%s (ID: %d)", m.Name, m.ID)
				}
				details = append(details, output.KeyValue{Key: "Memberships", Value: strings.Join(names, ", ")})
			}

			printer.Detail(details)
			return nil
		},
	}

	cmd.Flags().BoolVar(&includeUsers, "include-users", false, "Include group members in output")
	cmd.Flags().BoolVar(&includeMemberships, "include-memberships", false, "Include project memberships in output")
	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
