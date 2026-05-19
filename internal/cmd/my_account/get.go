package myaccount

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/cmd/user"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/spf13/cobra"
)

func newCmdMyAccountGet(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "get",
		Short:   "Show your own account details",
		Long:    "Show the authenticated user's account, including api_key and custom_fields.",
		Aliases: []string{"show", "view"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			stop := printer.Spinner("Fetching account...")
			u, err := ops.GetMyAccount(context.Background(), client, ops.GetMyAccountInput{})
			stop()
			if err != nil {
				return err
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(u)
				return nil
			}

			printer.Detail(myAccountDetailRows(u))
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}

func myAccountDetailRows(u *models.User) []output.KeyValue {
	admin := "no"
	if u.Admin {
		admin = "yes"
	}

	pairs := []output.KeyValue{
		{Key: "ID", Value: fmt.Sprintf("%d", u.ID)},
		{Key: "Login", Value: u.Login},
		{Key: "Name", Value: u.FirstName + " " + u.LastName},
		{Key: "Email", Value: u.Mail},
		{Key: "Admin", Value: admin},
		{Key: "Status", Value: user.UserStatusName(u.Status)},
	}

	if u.MailNotification != "" {
		pairs = append(pairs, output.KeyValue{Key: "Mail Notification", Value: u.MailNotification})
	}
	if u.APIKey != "" {
		pairs = append(pairs, output.KeyValue{Key: "API Key", Value: u.APIKey})
	}
	if len(u.CustomFields) > 0 {
		pairs = append(pairs, output.KeyValue{Key: "Custom Fields", Value: user.FormatCustomFieldValues(u.CustomFields)})
	}

	pairs = append(pairs,
		output.KeyValue{Key: "Created", Value: u.CreatedOn},
		output.KeyValue{Key: "Last Login", Value: u.LastLoginOn},
	)
	return pairs
}
