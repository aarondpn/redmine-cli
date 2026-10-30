package group

import (
	"fmt"
	"slices"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
	"github.com/spf13/cobra"
)

func newCmdGroupGet(f *cmdutil.Factory) *cobra.Command {
	var (
		format             string
		includes           []string
		includeUsers       bool
		includeMemberships bool
	)

	cmd := &cobra.Command{
		Use:     "get <id-or-name>",
		Short:   "Show group details",
		Long:    "Show group details. Accepts a numeric ID or group name.",
		Aliases: []string{"show", "view"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateGetIncludes(includes); err != nil {
				return err
			}
			var legacy []string
			if includeUsers {
				legacy = append(legacy, "users")
			}
			if includeMemberships {
				legacy = append(legacy, "memberships")
			}
			includes = cmdutil.MergeIncludes(includes, legacy...)

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			id, err := resolver.ResolveGroup(cmdutil.Context(cmd), client, args[0])
			if err != nil {
				return err
			}

			printer := f.Printer(format)

			stop := printer.Spinner("Fetching group...")
			group, err := ops.GetGroup(cmdutil.Context(cmd), client, ops.GetGroupInput{
				ID:       id,
				Includes: includes,
			})
			stop()
			if err != nil {
				return err
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(group)
				return nil
			}

			details := []output.KeyValue{
				{Key: "ID", Value: fmt.Sprintf("%d", group.ID)},
				{Key: "Name", Value: group.Name},
			}

			if slices.Contains(includes, "users") && len(group.Users) > 0 {
				names := make([]string, len(group.Users))
				for i, u := range group.Users {
					names[i] = fmt.Sprintf("%s (ID: %d)", u.Name, u.ID)
				}
				details = append(details, output.KeyValue{Key: "Users", Value: strings.Join(names, ", ")})
			}

			if slices.Contains(includes, "memberships") && len(group.Memberships) > 0 {
				names := make([]string, len(group.Memberships))
				for i, m := range group.Memberships {
					names[i] = fmt.Sprintf("%s (ID: %d)", m.Project.Name, m.Project.ID)
					if len(m.Roles) > 0 {
						roleNames := make([]string, len(m.Roles))
						for j, r := range m.Roles {
							roleNames[j] = r.Name
						}
						names[i] += " [" + strings.Join(roleNames, ", ") + "]"
					}
				}
				details = append(details, output.KeyValue{Key: "Memberships", Value: strings.Join(names, "; ")})
			}

			printer.Detail(details)
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&includes, "include", nil, "Include related data: users, memberships (repeatable or comma-separated)")
	_ = cmd.RegisterFlagCompletionFunc("include", completeGetIncludes)
	cmd.Flags().BoolVar(&includeUsers, "include-users", false, "Include group members in output")
	cmd.Flags().BoolVar(&includeMemberships, "include-memberships", false, "Include project memberships in output")
	// DEPRECATED(remove-after: 2027-01-31): --include-users and --include-memberships aliases; delete both flags, the includeUsers/includeMemberships vars and the legacy merge in RunE
	cmdutil.DeprecateFlag(cmd, "include-users", "--include users", "2027-01-31")
	cmdutil.DeprecateFlag(cmd, "include-memberships", "--include memberships", "2027-01-31")
	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
