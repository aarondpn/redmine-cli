package user

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/resolver"
	"github.com/spf13/cobra"
)

func newCmdUserUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		firstname string
		lastname  string
		mail      string
		admin     bool
		status    int
	)

	cmd := &cobra.Command{
		Use:     "update <id-or-name>",
		Short:   "Update a user",
		Long:    "Update a user. Accepts a numeric ID, login, full name, or 'me'.",
		Aliases: []string{"edit"},
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

			printer := f.Printer("")

			update := models.UserUpdate{}
			if cmd.Flags().Changed("firstname") {
				update.FirstName = &firstname
			}
			if cmd.Flags().Changed("lastname") {
				update.LastName = &lastname
			}
			if cmd.Flags().Changed("mail") {
				update.Mail = &mail
			}
			if cmd.Flags().Changed("admin") {
				update.Admin = &admin
			}
			if cmd.Flags().Changed("status") {
				update.Status = &status
			}

			stop := printer.Spinner("Updating user...")
			err = client.Users.Update(context.Background(), id, update)
			stop()
			if err != nil {
				return err
			}

			printer.Success(fmt.Sprintf("Updated user %d", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&firstname, "firstname", "", "First name")
	cmd.Flags().StringVar(&lastname, "lastname", "", "Last name")
	cmd.Flags().StringVar(&mail, "mail", "", "Email address")
	cmd.Flags().BoolVar(&admin, "admin", false, "Admin privileges")
	cmd.Flags().IntVar(&status, "status", 0, "User status (1=active, 2=registered, 3=locked)")
	return cmd
}
