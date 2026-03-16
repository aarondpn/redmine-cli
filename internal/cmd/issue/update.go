package issue

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
)

// NewCmdUpdate creates the issues update command.
func NewCmdUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		subject     string
		description string
		tracker     int
		status      int
		priority    int
		assignee    int
		doneRatio   int
		note        string
	)

	cmd := &cobra.Command{
		Use:     "update <id>",
		Aliases: []string{"edit"},
		Short:   "Update an issue",
		Long:    "Update fields on an existing issue.",
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

			update := models.IssueUpdate{}

			if cmd.Flags().Changed("subject") {
				update.Subject = &subject
			}
			if cmd.Flags().Changed("description") {
				update.Description = &description
			}
			if cmd.Flags().Changed("tracker") {
				update.TrackerID = &tracker
			}
			if cmd.Flags().Changed("status") {
				update.StatusID = &status
			}
			if cmd.Flags().Changed("priority") {
				update.PriorityID = &priority
			}
			if cmd.Flags().Changed("assignee") {
				update.AssignedToID = &assignee
			}
			if cmd.Flags().Changed("done-ratio") {
				update.DoneRatio = &doneRatio
			}
			if cmd.Flags().Changed("note") {
				update.Notes = &note
			}

			printer := f.Printer("")
			stop := printer.Spinner("Updating issue...")
			err = client.Issues.Update(context.Background(), id, update)
			stop()
			if err != nil {
				return fmt.Errorf("failed to update issue %s: %w", fmt.Sprintf("#%d", id), err)
			}

			printer.Success(fmt.Sprintf("Updated issue %s", fmt.Sprintf("#%d", id)))
			return nil
		},
	}

	cmd.Flags().StringVar(&subject, "subject", "", "Issue subject")
	cmd.Flags().StringVar(&description, "description", "", "Issue description")
	cmd.Flags().IntVar(&tracker, "tracker", 0, "Tracker ID")
	cmd.Flags().IntVar(&status, "status", 0, "Status ID")
	cmd.Flags().IntVar(&priority, "priority", 0, "Priority ID")
	cmd.Flags().IntVar(&assignee, "assignee", 0, "Assignee user ID")
	cmd.Flags().IntVar(&doneRatio, "done-ratio", 0, "Done ratio (0-100)")
	cmd.Flags().StringVar(&note, "note", "", "Add a note to the issue")

	return cmd
}
