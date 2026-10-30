package group

import (
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/spf13/cobra"
)

func newCmdGroupCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		name           string
		userIDs        []int
		format         string
		customFieldRaw []string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Args:    cobra.NoArgs,
		Short:   "Create a new group",
		Aliases: []string{"new"},
		Example: `  # Create a group with a custom field
  redmine groups create --name Backend --custom-field "Cost Center=CC-42"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			input := ops.CreateGroupInput{Name: name, UserIDs: userIDs}
			if len(customFieldRaw) > 0 {
				values, err := cmdutil.ParseCustomFieldValues(cmdutil.Context(cmd), client, customFieldRaw, models.CustomizedTypeGroup)
				if err != nil {
					return err
				}
				input.CustomFieldValues = values
			}

			stop := printer.Spinner("Creating group...")
			group, err := ops.CreateGroup(cmdutil.Context(cmd), client, input)
			stop()
			if err != nil {
				return cmdutil.WithValidationHint(err, cmdutil.CustomFieldValidationHint(customFieldRaw))
			}

			printer.Resource(group, fmt.Sprintf("Created group %q (ID: %d)", group.Name, group.ID))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Group name (required)")
	cmd.Flags().IntSliceVar(&userIDs, "user-ids", nil, "User IDs to add to the group")
	cmdutil.AddCustomFieldFlag(cmd, f, &customFieldRaw, "Group custom field", models.CustomizedTypeGroup)
	cmd.MarkFlagRequired("name")
	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
