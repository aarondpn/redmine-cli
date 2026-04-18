package time

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdTimeGet(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <id>",
		Aliases: []string{"show", "view"},
		Short:   "Show a time entry",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid time entry ID: %s", args[0])
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			entry, err := client.TimeEntries.Get(context.Background(), id)
			if err != nil {
				return err
			}

			printer := f.Printer("")

			if printer.Format() == output.FormatJSON {
				printer.JSON(entry)
				return nil
			}

			issueRef := "N/A"
			if entry.Issue != nil {
				issueRef = fmt.Sprintf("#%d", entry.Issue.ID)
			}

			printer.Detail([]output.KeyValue{
				{Key: "ID", Value: output.StyleID.Render(strconv.Itoa(entry.ID))},
				{Key: "Project", Value: entry.Project.Name},
				{Key: "Issue", Value: issueRef},
				{Key: "User", Value: entry.User.Name},
				{Key: "Activity", Value: entry.Activity.Name},
				{Key: "Hours", Value: fmt.Sprintf("%.2f", entry.Hours)},
				{Key: "Comments", Value: entry.Comments},
				{Key: "Date", Value: entry.SpentOn},
				{Key: "Created", Value: entry.CreatedOn},
				{Key: "Updated", Value: entry.UpdatedOn},
			})

			return nil
		},
	}

	return cmd
}
