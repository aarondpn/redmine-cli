package user

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/spf13/cobra"
)

// allowedMailNotifications lists the Redmine mail notification preference
// values. The wiki documents only "only_my_events" and "none"; the full set
// comes from app/models/user.rb in Redmine.
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

func newCmdUserCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		login            string
		password         string
		firstname        string
		lastname         string
		mail             string
		admin            bool
		mailNotification string
		mustChangePasswd bool
		generatePassword bool
		authSourceID     int
		sendInformation  bool
		format           string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new user",
		Aliases: []string{"new"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateMailNotification(mailNotification); err != nil {
				return err
			}
			if password == "" && !generatePassword {
				return fmt.Errorf("either --password or --generate-password is required")
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			input := ops.CreateUserInput{
				Login:           login,
				Password:        password,
				FirstName:       firstname,
				LastName:        lastname,
				Mail:            mail,
				Admin:           admin,
				SendInformation: sendInformation,
			}
			if cmd.Flags().Changed("mail-notification") {
				v := mailNotification
				input.MailNotification = &v
			}
			if cmd.Flags().Changed("must-change-passwd") {
				v := mustChangePasswd
				input.MustChangePasswd = &v
			}
			if cmd.Flags().Changed("generate-password") {
				v := generatePassword
				input.GeneratePassword = &v
			}
			if cmd.Flags().Changed("auth-source-id") {
				v := authSourceID
				input.AuthSourceID = &v
			}

			stop := printer.Spinner("Creating user...")
			user, err := ops.CreateUser(context.Background(), client, input)
			stop()
			if err != nil {
				return err
			}

			printer.Resource(user, fmt.Sprintf("Created user %q (ID: %d)", user.Login, user.ID))
			return nil
		},
	}

	cmd.Flags().StringVar(&login, "login", "", "Login name (required)")
	cmd.Flags().StringVar(&password, "password", "", "Password (required unless --generate-password is set)")
	cmd.Flags().StringVar(&firstname, "firstname", "", "First name (required)")
	cmd.Flags().StringVar(&lastname, "lastname", "", "Last name (required)")
	cmd.Flags().StringVar(&mail, "mail", "", "Email address (required)")
	cmd.Flags().BoolVar(&admin, "admin", false, "Grant admin privileges")
	cmd.Flags().StringVar(&mailNotification, "mail-notification", "", "Email notification preference (all, only_my_events, only_assigned, only_owner, none)")
	cmd.Flags().BoolVar(&mustChangePasswd, "must-change-passwd", false, "Force the user to change their password on next login")
	cmd.Flags().BoolVar(&generatePassword, "generate-password", false, "Let the server generate a random password")
	cmd.Flags().IntVar(&authSourceID, "auth-source-id", 0, "Numeric authentication source ID for external auth")
	cmd.Flags().BoolVar(&sendInformation, "send-information", false, "Email the account info to the new user")
	cmd.MarkFlagRequired("login")
	cmd.MarkFlagRequired("firstname")
	cmd.MarkFlagRequired("lastname")
	cmd.MarkFlagRequired("mail")
	_ = cmd.RegisterFlagCompletionFunc("mail-notification", completeMailNotification)
	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
