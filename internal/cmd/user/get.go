package user

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
	"github.com/spf13/cobra"
)

func newCmdUserGet(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "get <id>",
		Short:   "Show user details",
		Aliases: []string{"show"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid user ID: %s", args[0])
			}

			client, err := f.ApiClient()
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
