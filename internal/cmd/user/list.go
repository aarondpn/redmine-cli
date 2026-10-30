package user

import (
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
	"github.com/spf13/cobra"
)

func newCmdUserList(f *cmdutil.Factory) *cobra.Command {
	var (
		status   string
		name     string
		group    string
		limit    int
		offset   int
		format   string
		includes []string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Args:    cobra.NoArgs,
		Short:   "List users",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateUserListIncludes(includes); err != nil {
				return err
			}
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			var groupID int
			if group != "" {
				groupID, err = resolver.ResolveGroup(cmdutil.Context(cmd), client, group)
				if err != nil {
					return err
				}
			}

			stop := printer.Spinner("Fetching users...")
			result, err := ops.ListUsers(cmdutil.Context(cmd), client, ops.ListUsersInput{
				Status:   status,
				Name:     name,
				GroupID:  groupID,
				Limit:    cmdutil.OpsLimit(limit),
				Offset:   offset,
				Includes: includes,
			})
			stop()
			if err != nil {
				return err
			}
			users, total := result.Users, result.TotalCount

			cmdutil.RenderCollection(printer, users, []string{"ID", "Login", "Name", "Email", "Admin", "Status"},
				func(u models.User) []string {
					admin := ""
					if u.Admin != nil && *u.Admin {
						admin = "yes"
					}
					return []string{
						output.StyleID.Render(fmt.Sprintf("%d", u.ID)),
						u.Login,
						u.FirstName + " " + u.LastName,
						u.Mail,
						admin,
						UserStatusLabel(u.Status),
					}
				},
			)

			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(users), Total: total, Limit: limit, Offset: offset, Noun: "users",
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by status (active, registered, locked)")
	cmd.Flags().StringVar(&name, "name", "", "Filter by name")
	cmd.Flags().StringVar(&group, "group", "", "Filter by group name or ID")
	cmd.Flags().StringSliceVar(&includes, "include", nil,
		"Include related data: auth_source (Redmine 6.0+, admin only; repeatable or comma-separated)")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("include", completeUserListIncludes)

	_ = cmd.RegisterFlagCompletionFunc("group", cmdutil.CompleteGroups(f))
	_ = cmd.RegisterFlagCompletionFunc("status", cmdutil.CompleteUserStatus)

	return cmd
}

// UserStatusLabel renders a possibly-absent status. Redmine omits status in
// several responses (users/show for non-admins, users/index before 6.0,
// my/account), so nil is shown as "unknown" rather than a fake 0.
func UserStatusLabel(status *int) string {
	if status == nil {
		return "unknown"
	}
	return UserStatusName(*status)
}

// AdminLabel renders a possibly-absent admin flag as yes/no/unknown.
func AdminLabel(admin *bool) string {
	switch {
	case admin == nil:
		return "unknown"
	case *admin:
		return "yes"
	default:
		return "no"
	}
}

// UserStatusName renders a numeric Redmine user status as a human label.
func UserStatusName(status int) string {
	switch status {
	case 1:
		return "active"
	case 2:
		return "registered"
	case 3:
		return "locked"
	default:
		return fmt.Sprintf("%d", status)
	}
}
