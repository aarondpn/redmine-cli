package user

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
	"github.com/spf13/cobra"
)

func newCmdUserList(f *cmdutil.Factory) *cobra.Command {
	var (
		status string
		name   string
		group  string
		limit  int
		offset int
		format string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List users",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			var groupID int
			if group != "" {
				groupID, err = resolver.ResolveGroup(context.Background(), client, group)
				if err != nil {
					return err
				}
			}

			stop := printer.Spinner("Fetching users...")
			users, total, err := client.Users.List(context.Background(), models.UserFilter{
				Status:  status,
				Name:    name,
				GroupID: groupID,
				Limit:   limit,
				Offset:  offset,
			})
			stop()
			if err != nil {
				return err
			}

			cmdutil.RenderCollection(printer, users, []string{"ID", "Login", "Name", "Email", "Admin", "Status"},
				func(u models.User, styled bool) []string {
					id := fmt.Sprintf("%d", u.ID)
					admin := fmt.Sprintf("%t", u.Admin)
					if styled {
						id = output.StyleID.Render(id)
						admin = ""
						if u.Admin {
							admin = "yes"
						}
					}
					return []string{
						id,
						u.Login,
						u.FirstName + " " + u.LastName,
						u.Mail,
						admin,
						userStatusName(u.Status),
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
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("group", cmdutil.CompleteGroups(f))
	_ = cmd.RegisterFlagCompletionFunc("status", cmdutil.CompleteUserStatus)

	return cmd
}

func userStatusName(status int) string {
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
