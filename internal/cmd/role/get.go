package role

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func newCmdRoleGet(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "get <id-or-name>",
		Aliases: []string{"show", "view"},
		Short:   "Show role details",
		Long:    "Show role details. Accepts a numeric ID or role name.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			id, err := resolver.ResolveRole(context.Background(), client, args[0])
			if err != nil {
				return err
			}

			printer := f.Printer(format)

			stop := printer.Spinner("Fetching role...")
			role, err := ops.GetRole(context.Background(), client, ops.GetRoleInput{ID: id})
			stop()
			if err != nil {
				return err
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(role)
				return nil
			}

			details := []output.KeyValue{
				{Key: "ID", Value: fmt.Sprintf("%d", role.ID)},
				{Key: "Name", Value: role.Name},
			}
			if role.Assignable != nil {
				details = append(details, output.KeyValue{Key: "Assignable", Value: optionalBoolLabel(role.Assignable)})
			}
			if builtin := optionalBoolLabel(builtinPtr(*role)); builtin != "" {
				details = append(details, output.KeyValue{Key: "Builtin", Value: builtin})
			}
			if role.IssuesVisibility != "" {
				details = append(details, output.KeyValue{Key: "Issues Visibility", Value: role.IssuesVisibility})
			}
			if role.TimeEntriesVisibility != "" {
				details = append(details, output.KeyValue{Key: "Time Entries Visibility", Value: role.TimeEntriesVisibility})
			}
			if role.UsersVisibility != "" {
				details = append(details, output.KeyValue{Key: "Users Visibility", Value: role.UsersVisibility})
			}
			if len(role.Permissions) > 0 {
				details = append(details, output.KeyValue{Key: "Permissions", Value: strings.Join(role.Permissions, ", ")})
			}

			printer.Detail(details)
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
