package user

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/output"
	"github.com/spf13/cobra"
)

func newCmdUserCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		login     string
		password  string
		firstname string
		lastname  string
		mail      string
		admin     bool
		format    string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new user",
		Aliases: []string{"new"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			stop := printer.Spinner("Creating user...")
			user, err := client.Users.Create(context.Background(), models.UserCreate{
				Login:     login,
				Password:  password,
				FirstName: firstname,
				LastName:  lastname,
				Mail:      mail,
				Admin:     admin,
			})
			stop()
			if err != nil {
				return err
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(user)
				return nil
			}

			printer.Success(fmt.Sprintf("Created user %q (ID: %d)", user.Login, user.ID))
			return nil
		},
	}

	cmd.Flags().StringVar(&login, "login", "", "Login name (required)")
	cmd.Flags().StringVar(&password, "password", "", "Password (required)")
	cmd.Flags().StringVar(&firstname, "firstname", "", "First name (required)")
	cmd.Flags().StringVar(&lastname, "lastname", "", "Last name (required)")
	cmd.Flags().StringVar(&mail, "mail", "", "Email address (required)")
	cmd.Flags().BoolVar(&admin, "admin", false, "Grant admin privileges")
	cmd.MarkFlagRequired("login")
	cmd.MarkFlagRequired("password")
	cmd.MarkFlagRequired("firstname")
	cmd.MarkFlagRequired("lastname")
	cmd.MarkFlagRequired("mail")
	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
