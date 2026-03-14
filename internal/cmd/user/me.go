package user

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
	"github.com/spf13/cobra"
)

func newCmdUserMe(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "me",
		Short: "Show current authenticated user",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			stop := printer.Spinner("Fetching current user...")
			user, err := client.Users.Current(context.Background())
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
