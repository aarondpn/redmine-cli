package user

import (
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
	"github.com/spf13/cobra"
)

func newCmdUserUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		firstname        string
		lastname         string
		mail             string
		admin            bool
		status           int
		mailNotification string
		mustChangePasswd bool
		generatePassword bool
		authSourceID     int
		passwordStdin    bool
		language         string
		groups           []string
		notifiedProjects []string
		prefRaw          []string
		customFieldRaw   []string
	)

	cmd := &cobra.Command{
		Use:   "update <id-or-name>",
		Short: "Update a user",
		Long: "Update a user. Accepts a numeric ID, login, full name, or 'me'.\n\n" +
			"--password-stdin sets a new password without putting it on the command line: in a terminal it " +
			"prompts twice with hidden input, otherwise it reads standard input (one trailing newline is removed).\n\n" +
			"--group and --notified-project replace the current set; pass \"\" to clear it. --pref sets user " +
			"preferences such as hide_mail, time_zone or no_self_notified.",
		Aliases: []string{"edit"},
		Example: `  # Set a multi-value custom field
  redmine users update jdoe --custom-field Skills=Go --custom-field Skills=SQL

  # Set a new password (prompted, or piped in from a secret store)
  redmine users update jdoe --password-stdin
  pass show redmine/jdoe | redmine users update jdoe --password-stdin

  # Put a user into two groups and notify them only about one project
  redmine users update jdoe --group Developers --group QA --mail-notification selected --notified-project web

  # Change language and preferences
  redmine users update jdoe --language de --pref time_zone=Berlin --pref no_self_notified=1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("mail-notification") {
				if err := ValidateMailNotification(mailNotification); err != nil {
					return err
				}
			}
			prefs, err := ParsePrefs(prefRaw)
			if err != nil {
				return err
			}
			// The password is read before any request, so a prompt is not
			// mixed with the spinner and a bad input costs no round trip.
			var password string
			if passwordStdin {
				if password, err = cmdutil.ReadNewPassword(f, "password-stdin"); err != nil {
					return err
				}
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := cmdutil.Context(cmd)
			id, err := resolver.ResolveUser(ctx, client, args[0])
			if err != nil {
				return err
			}

			printer := f.Printer("")

			input := ops.UpdateUserInput{ID: id, Password: password, Pref: prefs}
			if cmd.Flags().Changed("firstname") {
				input.FirstName = &firstname
			}
			if cmd.Flags().Changed("lastname") {
				input.LastName = &lastname
			}
			if cmd.Flags().Changed("mail") {
				input.Mail = &mail
			}
			if cmd.Flags().Changed("admin") {
				input.Admin = &admin
			}
			if cmd.Flags().Changed("status") {
				input.Status = &status
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
			if cmd.Flags().Changed("language") {
				v := language
				input.Language = &v
			}
			if cmd.Flags().Changed("group") {
				ids, err := resolver.ResolveGroupNames(ctx, client, groups)
				if err != nil {
					return fmt.Errorf("resolve --group: %w", err)
				}
				if ids == nil {
					ids = []int{}
				}
				input.GroupIDs = &ids
			}
			if cmd.Flags().Changed("notified-project") {
				ids, err := ResolveNotifiedProjects(ctx, client, notifiedProjects)
				if err != nil {
					return err
				}
				input.NotifiedProjectIDs = &ids
			}

			if len(customFieldRaw) > 0 {
				values, err := cmdutil.ParseCustomFieldValues(ctx, client, customFieldRaw, models.CustomizedTypeUser)
				if err != nil {
					return err
				}
				input.CustomFieldValues = values
			}

			stop := printer.Spinner("Updating user...")
			_, err = ops.UpdateUser(ctx, client, input)
			stop()
			if err != nil {
				return cmdutil.WithValidationHint(err, cmdutil.CustomFieldValidationHint(customFieldRaw))
			}

			printer.Action(output.ActionUpdated, "user", id, fmt.Sprintf("Updated user %d", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&firstname, "firstname", "", "First name")
	cmd.Flags().StringVar(&lastname, "lastname", "", "Last name")
	cmd.Flags().StringVar(&mail, "mail", "", "Email address")
	cmd.Flags().BoolVar(&admin, "admin", false, "Admin privileges")
	cmd.Flags().IntVar(&status, "status", 0, "User status (1=active, 2=registered, 3=locked)")
	AddMailNotificationFlag(cmd, &mailNotification, true)
	AddNotifiedProjectFlag(cmd, f, &notifiedProjects)
	AddLanguageFlag(cmd, &language)
	cmd.Flags().StringArrayVar(&groups, "group", nil, "Group name or ID the user belongs to (repeatable, replaces the user's groups, \"\" removes the user from all groups)")
	cmd.Flags().BoolVar(&mustChangePasswd, "must-change-passwd", false, "Force the user to change their password on next login")
	cmd.Flags().BoolVar(&generatePassword, "generate-password", false, "Let the server generate a random password")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "Set a new password read from standard input (prompted twice with hidden input in a terminal)")
	cmd.Flags().IntVar(&authSourceID, "auth-source-id", 0, "Numeric authentication source ID for external auth")
	AddPrefFlag(cmd, &prefRaw)
	cmdutil.AddCustomFieldFlag(cmd, f, &customFieldRaw, "User custom field", models.CustomizedTypeUser)
	cmd.MarkFlagsMutuallyExclusive("password-stdin", "generate-password")
	_ = cmd.RegisterFlagCompletionFunc("group", cmdutil.CompleteGroups(f))
	return cmd
}
