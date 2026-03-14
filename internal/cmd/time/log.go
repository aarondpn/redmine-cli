package time

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
)

func newCmdTimeLog(f *cmdutil.Factory) *cobra.Command {
	var (
		issue    int
		project  string
		hours    float64
		activity int
		date     string
		comment  string
	)

	cmd := &cobra.Command{
		Use:     "log",
		Aliases: []string{"add", "create"},
		Short:   "Log a time entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			if project == "" {
				cfg, cfgErr := f.Config()
				if cfgErr == nil && cfg.DefaultProject != "" {
					project = cfg.DefaultProject
				}
			}

			if date == "" {
				date = time.Now().Format("2006-01-02")
			}

			entry := models.TimeEntryCreate{
				IssueID:    issue,
				ProjectID:  project,
				Hours:      hours,
				ActivityID: activity,
				SpentOn:    date,
				Comments:   comment,
			}

			created, err := client.TimeEntries.Create(context.Background(), entry)
			if err != nil {
				return fmt.Errorf("%s", cmdutil.FormatError(err))
			}

			printer := f.Printer("")
			printer.Success(fmt.Sprintf("Time entry #%s created (%.2f hours on %s)",
				strconv.Itoa(created.ID), created.Hours, created.SpentOn))

			return nil
		},
	}

	cmd.Flags().IntVar(&issue, "issue", 0, "Issue ID")
	cmd.Flags().StringVar(&project, "project", "", "Project identifier")
	cmd.Flags().Float64Var(&hours, "hours", 0, "Hours spent (required)")
	cmd.Flags().IntVar(&activity, "activity", 0, "Activity ID")
	cmd.Flags().StringVar(&date, "date", "", "Date (YYYY-MM-DD, default today)")
	cmd.Flags().StringVar(&comment, "comment", "", "Comment")

	_ = cmd.MarkFlagRequired("hours")

	return cmd
}
