package group

import (
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
	"github.com/spf13/cobra"
)

func newCmdGroupUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		name           string
		userIDs        []int
		twofaRequired  bool
		customFieldRaw []string
	)

	cmd := &cobra.Command{
		Use:     "update <id-or-name>",
		Short:   "Update a group",
		Long:    "Update a group. Accepts a numeric ID or group name.",
		Aliases: []string{"edit"},
		Example: `  # Set a group custom field
  redmine groups update Backend --custom-field "Cost Center=CC-43"

  # Stop requiring two-factor authentication (Redmine 5.0+)
  redmine groups update Backend --twofa-required=false`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := cmdutil.Context(cmd)
			id, err := resolver.ResolveGroup(ctx, client, args[0])
			if err != nil {
				return err
			}

			printer := f.Printer("")

			input := ops.UpdateGroupInput{ID: id}
			if cmd.Flags().Changed("name") {
				input.Name = &name
			}
			if cmd.Flags().Changed("user-ids") {
				input.UserIDs = &userIDs
			}
			if cmd.Flags().Changed("twofa-required") {
				input.TwofaRequired = &twofaRequired
			}

			if len(customFieldRaw) > 0 {
				values, err := cmdutil.ParseCustomFieldValues(ctx, client, customFieldRaw, models.CustomizedTypeGroup)
				if err != nil {
					return err
				}
				input.CustomFieldValues = values
			}

			stop := printer.Spinner("Updating group...")
			_, err = ops.UpdateGroup(ctx, client, input)
			stop()
			if err != nil {
				return cmdutil.WithValidationHint(err, cmdutil.CustomFieldValidationHint(customFieldRaw))
			}

			printer.Action(output.ActionUpdated, "group", id, fmt.Sprintf("Updated group %d", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Group name")
	cmd.Flags().IntSliceVar(&userIDs, "user-ids", nil, "User IDs (replaces current members)")
	cmd.Flags().BoolVar(&twofaRequired, "twofa-required", false, "Require two-factor authentication for the group's members (Redmine 5.0+, older versions ignore it; write-only: groups get does not show it)")
	cmdutil.AddCustomFieldFlag(cmd, f, &customFieldRaw, "Group custom field", models.CustomizedTypeGroup)
	return cmd
}
