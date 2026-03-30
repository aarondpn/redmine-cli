package membership

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/output"
)

func newCmdMembershipCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		project string
		userID  int
		groupID int
		roleIDs []int
		format  string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"add", "new"},
		Short:   "Add a member to a project",
		Long:    "Create a new membership, adding a user or group to a project with specified roles.",
		Example: `  # Add a user with roles
  redmine memberships create --project myproject --user-id 5 --role-ids 1,2

  # Add a group with a role
  redmine memberships create --project myproject --group-id 10 --role-ids 3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			hasUser := cmd.Flags().Changed("user-id")
			hasGroup := cmd.Flags().Changed("group-id")
			if !hasUser && !hasGroup {
				return fmt.Errorf("either --user-id or --group-id is required")
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			project, err = cmdutil.RequireProjectIdentifier(context.Background(), f, project)
			if err != nil {
				return err
			}

			memberID := userID
			if hasGroup {
				memberID = groupID
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Creating membership...")
			m, err := client.Memberships.Create(context.Background(), project, models.MembershipCreate{
				UserID:  memberID,
				RoleIDs: roleIDs,
			})
			stop()
			if err != nil {
				return err
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(m)
				return nil
			}

			printer.Success(fmt.Sprintf("Created membership (ID: %d) for %s in project %s", m.ID, memberName(*m), m.Project.Name))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID (required)")
	cmd.Flags().IntVar(&userID, "user-id", 0, "User ID to add")
	cmd.Flags().IntVar(&groupID, "group-id", 0, "Group ID to add")
	cmd.Flags().IntSliceVar(&roleIDs, "role-ids", nil, "Role IDs to assign (required)")
	cmd.MarkFlagRequired("role-ids")
	cmd.MarkFlagsMutuallyExclusive("user-id", "group-id")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
