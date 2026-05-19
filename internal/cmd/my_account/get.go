package myaccount

import (
	"context"
	"fmt"
	"strings"

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
			user, err := ops.GetMyAccount(context.Background(), client, ops.GetMyAccountInput{})
			stop()
			if err != nil {
				return err
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(user)
				return nil
			}

			printer.Detail(myAccountDetailRows(user))
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}

func myAccountDetailRows(user *models.User) []output.KeyValue {
	admin := "no"
	if user.Admin {
		admin = "yes"
	}
	status := userStatusName(user.Status)

	pairs := []output.KeyValue{
		{Key: "ID", Value: fmt.Sprintf("%d", user.ID)},
		{Key: "Login", Value: user.Login},
		{Key: "Name", Value: user.FirstName + " " + user.LastName},
		{Key: "Email", Value: user.Mail},
		{Key: "Admin", Value: admin},
		{Key: "Status", Value: status},
	}

	if user.MailNotification != "" {
		pairs = append(pairs, output.KeyValue{Key: "Mail Notification", Value: user.MailNotification})
	}
	if user.APIKey != "" {
		pairs = append(pairs, output.KeyValue{Key: "API Key", Value: user.APIKey})
	}
	if len(user.CustomFields) > 0 {
		pairs = append(pairs, output.KeyValue{Key: "Custom Fields", Value: formatCustomFieldValues(user.CustomFields)})
	}

	pairs = append(pairs,
		output.KeyValue{Key: "Created", Value: user.CreatedOn},
		output.KeyValue{Key: "Last Login", Value: user.LastLoginOn},
	)
	return pairs
}

func userStatusName(status int) string {
	switch status {
	case 1:
		return "active"
	case 2:
		return "registered"
	case 3:
		return "locked"
	default:
		return fmt.Sprintf("%d", status)
	}
}

func formatCustomFieldValues(items []models.CustomFieldValue) string {
	parts := make([]string, len(items))
	for i, cf := range items {
		parts[i] = fmt.Sprintf("%s: %v", cf.Name, cf.Value)
	}
	return strings.Join(parts, "; ")
}
