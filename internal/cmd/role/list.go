package role

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdRoleList(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "list",
		Args:    cobra.NoArgs,
		Aliases: []string{"ls"},
		Short:   "List all roles",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			stop := printer.Spinner("Fetching roles...")
			result, err := ops.ListRoles(context.Background(), client, struct{}{})
			stop()
			if err != nil {
				return err
			}

			cmdutil.RenderCollection(printer, result.Roles, []string{"ID", "Name", "Assignable", "Builtin"}, func(r models.Role, styled bool) []string {
				id := fmt.Sprintf("%d", r.ID)
				if styled {
					id = output.StyleID.Render(id)
				}
				return []string{id, r.Name, optionalBoolLabel(r.Assignable), optionalBoolLabel(builtinPtr(r))}
			})
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}

func optionalBoolLabel(v *bool) string {
	if v == nil {
		return ""
	}
	if *v {
		return "yes"
	}
	return "no"
}

func builtinPtr(r models.Role) *bool {
	if r.Builtin != nil {
		if r.IsBuiltin != nil && *r.IsBuiltin && !*r.Builtin {
			value := true
			return &value
		}
		return r.Builtin
	}
	return r.IsBuiltin
}
