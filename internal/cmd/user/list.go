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
		status      string
		name        string
		group       string
		sort        string
		extraFilter []string
		limit       int
		offset      int
		format      string
		includes    []string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Args:  cobra.NoArgs,
		Short: "List users",
		Long: `List users.

Redmine lists only active users unless --status is given. --status all lists
every status on every supported Redmine version: it sends status= (empty),
and retries with status=* when Redmine 6.0+ rejects the empty value. A raw
--status '*' needs Redmine 5.1+.

Redmine's generic f[]/op[]/v[] filters (via --filter) need Redmine 5.1+.`,
		Example: `  # Every user, whatever the status
  redmine users list --status all

  # Most recently created users first
  redmine users list --sort created_on:desc --limit 10

  # Raw filter escape hatch (Redmine 5.1+): any status, login contains "smith"
  redmine users list --filter 'f[]=status' --filter 'op[status]=*' \
    --filter 'f[]=login' --filter 'op[login]=~' --filter 'v[login][]=smith'`,
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateUserListIncludes(includes); err != nil {
				return err
			}
			extra, err := cmdutil.ParseFilterFlag(extraFilter)
			if err != nil {
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
				Status:      status,
				Name:        name,
				GroupID:     groupID,
				Sort:        sort,
				ExtraParams: extra,
				Limit:       cmdutil.OpsLimit(limit),
				Offset:      offset,
				Includes:    includes,
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

	cmd.Flags().StringVar(&status, "status", "", "Filter by status: active, registered, locked, all, or a numeric status (default: Redmine lists active users only)")
	cmd.Flags().StringVar(&name, "name", "", "Filter by name")
	cmd.Flags().StringVar(&group, "group", "", "Filter by group name or ID")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort field (e.g., created_on:desc)")
	cmdutil.AddFilterFlag(cmd, &extraFilter, "'f[]=status'", "'op[status]=*'")
	cmd.Flags().StringSliceVar(&includes, "include", nil,
		"Include related data: auth_source (Redmine 6.0+, admin only; repeatable or comma-separated)")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("include", completeUserListIncludes)

	_ = cmd.RegisterFlagCompletionFunc("group", cmdutil.CompleteGroups(f))
	_ = cmd.RegisterFlagCompletionFunc("status", cmdutil.CompleteUserListStatus)

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
