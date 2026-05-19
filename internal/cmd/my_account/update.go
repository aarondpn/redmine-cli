package myaccount

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/spf13/cobra"
)

var allowedMailNotifications = map[string]struct{}{
	"all":            {},
	"only_my_events": {},
	"only_assigned":  {},
	"only_owner":     {},
	"none":           {},
}

func validateMailNotification(value string) error {
	if value == "" {
		return nil
	}
	if _, ok := allowedMailNotifications[value]; !ok {
		return fmt.Errorf("invalid --mail-notification %q (allowed: %s)", value, sortedMailNotificationKeys())
	}
	return nil
}

func sortedMailNotificationKeys() string {
	keys := make([]string, 0, len(allowedMailNotifications))
	for k := range allowedMailNotifications {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

func completeMailNotification(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return strings.Split(sortedMailNotificationKeys(), ", "), cobra.ShellCompDirectiveNoFileComp
}

func newCmdMyAccountUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		firstname        string
		lastname         string
		mail             string
		mailNotification string
	)

	cmd := &cobra.Command{
		Use:     "update",
		Short:   "Update your own account",
		Long:    "Update the authenticated user's own account. Works without admin privileges.",
		Aliases: []string{"edit"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("mail-notification") {
				if err := validateMailNotification(mailNotification); err != nil {
					return err
				}
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer("")

			input := ops.UpdateMyAccountInput{}
			if cmd.Flags().Changed("firstname") {
				v := firstname
				input.FirstName = &v
			}
			if cmd.Flags().Changed("lastname") {
				v := lastname
				input.LastName = &v
			}
			if cmd.Flags().Changed("mail") {
				v := mail
				input.Mail = &v
			}
			if cmd.Flags().Changed("mail-notification") {
				v := mailNotification
				input.MailNotification = &v
			}

			stop := printer.Spinner("Updating account...")
			_, err = ops.UpdateMyAccount(context.Background(), client, input)
			stop()
			if err != nil {
				return err
			}

			printer.Action(output.ActionUpdated, "my-account", 0, "Updated my account")
			return nil
		},
	}

	cmd.Flags().StringVar(&firstname, "firstname", "", "First name")
	cmd.Flags().StringVar(&lastname, "lastname", "", "Last name")
	cmd.Flags().StringVar(&mail, "mail", "", "Email address")
	cmd.Flags().StringVar(&mailNotification, "mail-notification", "", "Email notification preference (all, only_my_events, only_assigned, only_owner, none)")
	_ = cmd.RegisterFlagCompletionFunc("mail-notification", completeMailNotification)
	return cmd
}
