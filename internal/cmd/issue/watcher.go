package issue

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

// NewCmdWatchers creates the issues watchers command group.
func NewCmdWatchers(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "watchers",
		Aliases: []string{"watcher"},
		Short:   "Manage issue watchers",
		Long:    "List, add, or remove watchers on an issue.",
	}
	cmd.AddCommand(newCmdWatcherList(f))
	cmd.AddCommand(newCmdWatcherAdd(f))
	cmd.AddCommand(newCmdWatcherRemove(f))
	return cmd
}

func newCmdWatcherList(f *cmdutil.Factory) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:     "list <issue-id>",
		Aliases: []string{"ls"},
		Short:   "List watchers of an issue",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)
			stop := printer.Spinner("Fetching watchers...")
			result, err := ops.ListIssueWatchers(context.Background(), client, ops.ListIssueWatchersInput{ID: id})
			stop()
			if err != nil {
				return fmt.Errorf("failed to list watchers for issue #%d: %w", id, err)
			}
			if cmdutil.HandleEmpty(printer, result.Watchers, "watchers") {
				return nil
			}
			cmdutil.RenderCollection(printer, result.Watchers, []string{"ID", "Name"}, func(w models.IDName, styled bool) []string {
				id := strconv.Itoa(w.ID)
				if styled {
					id = output.StyleID.Render(id)
				}
				return []string{id, w.Name}
			})
			return nil
		},
	}
	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}

func newCmdWatcherAdd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <issue-id> <user>",
		Short: "Add a watcher to an issue",
		Long:  "Add a user as a watcher. The user can be a numeric ID, login, name, or 'me'.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			issueID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			ctx := context.Background()
			userID, err := resolver.ResolveAssignee(ctx, client, args[1])
			if err != nil {
				return fmt.Errorf("resolving user: %w", err)
			}
			printer := f.Printer("")
			stop := printer.Spinner("Adding watcher...")
			_, err = ops.AddIssueWatcher(ctx, client, ops.AddIssueWatcherInput{ID: issueID, UserID: userID})
			stop()
			if err != nil {
				return fmt.Errorf("failed to add watcher: %w", err)
			}
			printer.Action(output.ActionWatched, "issue", issueID,
				fmt.Sprintf("Added user %d as watcher of issue #%d", userID, issueID))
			return nil
		},
	}
	_ = cmd.RegisterFlagCompletionFunc("user", cmdutil.CompleteUsers(f))
	return cmd
}

func newCmdWatcherRemove(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <issue-id> <user>",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove a watcher from an issue",
		Long:    "Remove a user as a watcher. The user can be a numeric ID, login, name, or 'me'.",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			issueID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			ctx := context.Background()
			userID, err := resolver.ResolveAssignee(ctx, client, args[1])
			if err != nil {
				return fmt.Errorf("resolving user: %w", err)
			}
			printer := f.Printer("")
			stop := printer.Spinner("Removing watcher...")
			_, err = ops.RemoveIssueWatcher(ctx, client, ops.RemoveIssueWatcherInput{ID: issueID, UserID: userID})
			stop()
			if err != nil {
				return fmt.Errorf("failed to remove watcher: %w", err)
			}
			printer.Action(output.ActionUnwatched, "issue", issueID,
				fmt.Sprintf("Removed user %d as watcher of issue #%d", userID, issueID))
			return nil
		},
	}
	return cmd
}
