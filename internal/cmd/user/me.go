package user

import (
	"context"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/spf13/cobra"
)

func newCmdUserMe(f *cmdutil.Factory) *cobra.Command {
	var (
		format   string
		includes []string
	)

	cmd := &cobra.Command{
		Use:   "me",
		Args:  cobra.NoArgs,
		Short: "Show current authenticated user",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateUserIncludes(includes); err != nil {
				return err
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			stop := printer.Spinner("Fetching current user...")
			user, err := ops.GetCurrentUser(context.Background(), client, ops.GetCurrentUserInput{Includes: includes})
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
