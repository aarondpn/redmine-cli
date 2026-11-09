package membership

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func newCmdMembershipCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		project string
		users   []string
		userID  int
		groupID int
		roleIDs []int
		roles   []string
		format  string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Args:    cobra.NoArgs,
		Aliases: []string{"add", "new"},
		Short:   "Add a member to a project",
		Long: "Create a new membership, adding a user or group to a project with specified roles.\n\n" +
			"Repeat --user to add several users with the same roles in one request. Redmine adds all of them or none: " +
			"if one user is invalid or already a member, the request fails and nobody is added; add the users one at a time " +
			"to see which one fails.\n\n" +
			"With --user, -o json always prints an array of memberships (one per user, in flag order), even for a single user; " +
			"--user-id and --group-id print a single membership object.",
		Example: `  # Add a user with roles
  redmine memberships create --project myproject --user-id 5 --role-ids 1,2

  # Add several users by login, name or ID with the same roles
  redmine memberships create --project myproject --user jdoe --user "Ann Lee" --user 7 --roles Developer

  # Add a user with roles resolved by name
  redmine memberships create --project myproject --user-id 5 --roles Manager,Developer

  # Add a group with a role
  redmine memberships create --project myproject --group-id 10 --role-ids 3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			hasUser := cmd.Flags().Changed("user-id") || cmd.Flags().Changed("user")
			hasGroup := cmd.Flags().Changed("group-id")
			if !hasUser && !hasGroup {
				return fmt.Errorf("either --user, --user-id or --group-id is required")
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			project, err = cmdutil.RequireProjectIdentifier(cmdutil.Context(cmd), f, project)
			if err != nil {
				return err
			}
			resolvedRoleIDs, err := resolveRoleIDs(
				cmdutil.Context(cmd),
				client,
				roleIDs,
				roles,
				cmd.Flags().Changed("role-ids"),
				cmd.Flags().Changed("roles"),
			)
			if err != nil {
				return err
			}

			input := ops.CreateMembershipInput{ProjectID: project, RoleIDs: resolvedRoleIDs}
			switch {
			case hasGroup:
				input.UserID = groupID
			case len(users) > 0:
				ids, err := resolveUsers(cmd, client, users)
				if err != nil {
					return err
				}
				input.UserIDs = ids
			default:
				input.UserID = userID
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Creating membership...")
			memberships, err := ops.CreateMemberships(cmdutil.Context(cmd), client, input)
			stop()
			if err != nil {
				return err
			}

			// --user always yields an array, so scripts get one shape no
			// matter how many users they pass.
			var result any = memberships[0]
			if len(users) > 0 {
				result = memberships
			}
			if len(memberships) == 1 {
				m := memberships[0]
				printer.Resource(result, fmt.Sprintf("Created membership (ID: %d) for %s in project %s", m.ID, memberName(m), m.Project.Name))
				return nil
			}
			names := make([]string, len(memberships))
			for i, m := range memberships {
				names[i] = fmt.Sprintf("%s (membership %d)", memberName(m), m.ID)
			}
			printer.Resource(result, fmt.Sprintf("Added %d members to project %s: %s", len(memberships), memberships[0].Project.Name, strings.Join(names, ", ")))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID (required)")
	cmd.Flags().StringArrayVar(&users, "user", nil, "User to add: ID, login, name, or 'me' (repeatable to add several users with the same roles; -o json prints an array)")
	cmd.Flags().IntVar(&userID, "user-id", 0, "User ID to add")
	cmd.Flags().IntVar(&groupID, "group-id", 0, "Group ID to add")
	cmd.Flags().IntSliceVar(&roleIDs, "role-ids", nil, "Role IDs to assign")
	cmd.Flags().StringSliceVar(&roles, "roles", nil, "Role names or IDs to assign (repeatable or comma-separated)")
	cmd.MarkFlagsMutuallyExclusive("user", "user-id", "group-id")
	cmd.MarkFlagsMutuallyExclusive("role-ids", "roles")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("roles", cmdutil.CompleteRoles(f))
	_ = cmd.RegisterFlagCompletionFunc("user", cmdutil.CompleteUsers(f))

	return cmd
}

// resolveUsers resolves the --user values to user IDs. Numeric IDs and "me"
// need no lookup; names and logins use the admin-only users list.
func resolveUsers(cmd *cobra.Command, client *api.Client, inputs []string) ([]int, error) {
	ids := make([]int, 0, len(inputs))
	for _, in := range inputs {
		id, err := resolver.ResolveUser(cmdutil.Context(cmd), client, in)
		if err != nil {
			return nil, fmt.Errorf("resolve --user %q: %w", in, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
