package user

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
	"github.com/spf13/cobra"
)

func newCmdUserGet(f *cmdutil.Factory) *cobra.Command {
	var (
		format   string
		includes []string
	)

	cmd := &cobra.Command{
		Use:     "get <id-or-name>",
		Short:   "Show user details",
		Long:    "Show user details. Accepts a numeric ID, login, full name, or 'me'.",
		Aliases: []string{"show", "view"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateUserIncludes(includes); err != nil {
				return err
			}

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
			user, err := ops.GetUser(context.Background(), client, ops.GetUserInput{ID: id, Includes: includes})
			stop()
			if err != nil {
				return err
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(user)
				return nil
			}

			printer.Detail(userDetailRows(user))
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)
	cmd.Flags().StringSliceVar(&includes, "include", nil,
		"Include related data: memberships, groups (repeatable or comma-separated)")
	_ = cmd.RegisterFlagCompletionFunc("include", completeUserIncludes)
	return cmd
}

// userDetailRows builds the key/value rows shown by `users get` and
// `users me` in non-JSON output. Memberships, Groups, and Custom Fields rows
// only appear when populated so simple users don't get noisy detail blocks.
func userDetailRows(user *models.User) []output.KeyValue {
	admin := "no"
	if user.Admin {
		admin = "yes"
	}

	pairs := []output.KeyValue{
		{Key: "ID", Value: fmt.Sprintf("%d", user.ID)},
		{Key: "Login", Value: user.Login},
		{Key: "Name", Value: user.FirstName + " " + user.LastName},
		{Key: "Email", Value: user.Mail},
		{Key: "Admin", Value: admin},
		{Key: "Status", Value: userStatusName(user.Status)},
	}

	if user.MailNotification != "" {
		pairs = append(pairs, output.KeyValue{Key: "Mail Notification", Value: user.MailNotification})
	}
	if len(user.Memberships) > 0 {
		pairs = append(pairs, output.KeyValue{Key: "Memberships", Value: formatUserMembershipList(user.Memberships)})
	}
	if len(user.Groups) > 0 {
		pairs = append(pairs, output.KeyValue{Key: "Groups", Value: formatIDNameList(user.Groups)})
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
