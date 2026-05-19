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
)

// NewCmdRelations creates the issues relations command group.
func NewCmdRelations(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "relations",
		Aliases: []string{"relation"},
		Short:   "Manage issue relations",
		Long: "List, create, and delete relations between issues. " +
			"Relation types: relates, duplicates, duplicated, blocks, blocked, " +
			"precedes, follows, copied_to, copied_from.",
	}
	cmd.AddCommand(newCmdRelationList(f))
	cmd.AddCommand(newCmdRelationAdd(f))
	cmd.AddCommand(newCmdRelationRemove(f))
	return cmd
}

func newCmdRelationList(f *cmdutil.Factory) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:     "list <issue-id>",
		Aliases: []string{"ls"},
		Short:   "List relations of an issue",
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
			stop := printer.Spinner("Fetching relations...")
			result, err := ops.ListIssueRelations(context.Background(), client, ops.ListIssueRelationsInput{IssueID: id})
			stop()
			if err != nil {
				return fmt.Errorf("failed to list relations for issue #%d: %w", id, err)
			}
			if cmdutil.HandleEmpty(printer, result.Relations, "relations") {
				return nil
			}
			cmdutil.RenderCollection(printer, result.Relations,
				[]string{"ID", "Issue", "Related Issue", "Type", "Delay"},
				func(r models.IssueRelation, styled bool) []string {
					id := strconv.Itoa(r.ID)
					from := fmt.Sprintf("#%d", r.IssueID)
					to := fmt.Sprintf("#%d", r.IssueToID)
					if styled {
						id = output.StyleID.Render(id)
						from = output.StyleID.Render(from)
						to = output.StyleID.Render(to)
					} else {
						from = strconv.Itoa(r.IssueID)
						to = strconv.Itoa(r.IssueToID)
					}
					return []string{id, from, to, r.RelationType, formatDelay(r.Delay)}
				})
			return nil
		},
	}
	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}

func newCmdRelationAdd(f *cmdutil.Factory) *cobra.Command {
	var (
		toID    int
		relType string
		delay   int
		format  string
	)
	cmd := &cobra.Command{
		Use:   "add <issue-id>",
		Short: "Create a relation from an issue to another",
		Long: "Create a relation between two issues. --type controls the kind of " +
			"relation; --delay (days) is only valid for 'precedes' and 'follows'.",
		Example: `  # Mark issue 123 as blocking issue 124
  redmine issues relations add 123 --to 124 --type blocks

  # 123 precedes 124 by 5 days
  redmine issues relations add 123 --to 124 --type precedes --delay 5`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			issueID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}
			if toID <= 0 {
				return fmt.Errorf("--to is required")
			}
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			input := ops.CreateIssueRelationInput{
				IssueID:      issueID,
				IssueToID:    toID,
				RelationType: relType,
			}
			if cmd.Flags().Changed("delay") {
				input.Delay = &delay
			}
			printer := f.Printer(format)
			stop := printer.Spinner("Creating relation...")
			rel, err := ops.CreateIssueRelation(context.Background(), client, input)
			stop()
			if err != nil {
				return fmt.Errorf("failed to create relation: %w", err)
			}
			if printer.Format() == output.FormatJSON {
				printer.JSON(rel)
				return nil
			}
			printer.Action(output.ActionRelated, "issue", issueID,
				fmt.Sprintf("Created relation #%d: #%d %s #%d", rel.ID, rel.IssueID, rel.RelationType, rel.IssueToID))
			return nil
		},
	}
	cmd.Flags().IntVar(&toID, "to", 0, "Target issue ID (required)")
	cmd.Flags().StringVar(&relType, "type", "relates", "Relation type (relates|duplicates|duplicated|blocks|blocked|precedes|follows|copied_to|copied_from)")
	cmd.Flags().IntVar(&delay, "delay", 0, "Number of days delay (only valid for precedes/follows)")
	cmdutil.AddOutputFlag(cmd, &format)
	_ = cmd.MarkFlagRequired("to")
	_ = cmd.RegisterFlagCompletionFunc("type", completeRelationTypes)
	return cmd
}

func newCmdRelationRemove(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <relation-id>",
		Aliases: []string{"rm", "delete"},
		Short:   "Delete a relation by its relation ID",
		Long: "Delete an issue relation by its relation ID. List relations with " +
			"`redmine issues relations list <issue-id>` to discover the ID.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			relID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid relation ID: %s", args[0])
			}
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer("")
			stop := printer.Spinner("Deleting relation...")
			_, err = ops.DeleteIssueRelation(context.Background(), client, ops.DeleteIssueRelationInput{ID: relID})
			stop()
			if err != nil {
				return fmt.Errorf("failed to delete relation #%d: %w", relID, err)
			}
			printer.Action(output.ActionUnrelated, "relation", relID,
				fmt.Sprintf("Deleted relation #%d", relID))
			return nil
		},
	}
	return cmd
}

func completeRelationTypes(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return ops.IssueRelationTypes, cobra.ShellCompDirectiveNoFileComp
}

func formatDelay(d *int) string {
	if d == nil {
		return ""
	}
	return strconv.Itoa(*d)
}
