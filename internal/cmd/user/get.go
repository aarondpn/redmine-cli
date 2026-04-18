package user

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
	"github.com/aarondpn/redmine-cli/internal/resolver"
	"github.com/spf13/cobra"
)

func newCmdUserGet(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "get <id-or-name>",
		Short:   "Show user details",
		Long:    "Show user details. Accepts a numeric ID, login, full name, or 'me'.",
		Aliases: []string{"show", "view"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			id, err := resolver.ResolveUser(context.Background(), client, args[0])
			if err != nil {
				return err
			}

			printer := f.Printer(format)

			stop := printer.Spinner("Fetching user...")
			user, err := client.Users.Get(context.Background(), id)
			stop()
			if err != nil {
				return err
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(user)
				return nil
			}

			admin := "no"
			if user.Admin {
				admin = "yes"
			}

			printer.Detail([]output.KeyValue{
				{Key: "ID", Value: fmt.Sprintf("%d", user.ID)},
				{Key: "Login", Value: user.Login},
				{Key: "Name", Value: user.FirstName + " " + user.LastName},
				{Key: "Email", Value: user.Mail},
				{Key: "Admin", Value: admin},
				{Key: "Status", Value: userStatusName(user.Status)},
				{Key: "Created", Value: user.CreatedOn},
				{Key: "Last Login", Value: user.LastLoginOn},
			})
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
