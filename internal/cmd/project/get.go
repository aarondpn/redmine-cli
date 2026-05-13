package project

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdGet(f *cmdutil.Factory) *cobra.Command {
	var (
		format   string
		includes []string
	)

	cmd := &cobra.Command{
		Use:     "get <identifier>",
		Aliases: []string{"show", "view"},
		Short:   "Get project details",
		Long:    "Display detailed information about a Redmine project.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateProjectIncludes(includes); err != nil {
				return err
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			project, err := ops.GetProject(context.Background(), client, ops.GetProjectInput{
				Identifier: args[0],
				Includes:   includes,
			})
			if err != nil {
				return err
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(project)
				return nil
			}

			pairs := []output.KeyValue{
				{Key: "ID", Value: output.StyleID.Render(strconv.Itoa(project.ID))},
				{Key: "Name", Value: project.Name},
				{Key: "Identifier", Value: project.Identifier},
				{Key: "Description", Value: project.Description},
				{Key: "Status", Value: projectStatusLabel(project.Status)},
				{Key: "Public", Value: formatBool(project.IsPublic)},
			}

			if project.Homepage != "" {
				pairs = append(pairs, output.KeyValue{Key: "Homepage", Value: project.Homepage})
			}

			if project.Parent != nil {
				pairs = append(pairs, output.KeyValue{
					Key:   "Parent",
					Value: fmt.Sprintf("%s (#%d)", project.Parent.Name, project.Parent.ID),
				})
			}

			if project.InheritMembers {
				pairs = append(pairs, output.KeyValue{Key: "Inherit Members", Value: formatBool(true)})
			}

			if project.DefaultAssignedTo != nil {
				pairs = append(pairs, output.KeyValue{
					Key:   "Default Assignee",
					Value: fmt.Sprintf("%s (#%d)", project.DefaultAssignedTo.Name, project.DefaultAssignedTo.ID),
				})
			}

			if project.DefaultVersion != nil {
				pairs = append(pairs, output.KeyValue{
					Key:   "Default Version",
					Value: fmt.Sprintf("%s (#%d)", project.DefaultVersion.Name, project.DefaultVersion.ID),
				})
			}

			if len(project.Trackers) > 0 {
				pairs = append(pairs, output.KeyValue{Key: "Trackers", Value: formatIDNameList(project.Trackers)})
			}
			if len(project.EnabledModules) > 0 {
				pairs = append(pairs, output.KeyValue{Key: "Enabled Modules", Value: formatModuleList(project.EnabledModules)})
			}
			if len(project.IssueCategories) > 0 {
				pairs = append(pairs, output.KeyValue{Key: "Issue Categories", Value: formatIDNameList(project.IssueCategories)})
			}
			if len(project.TimeEntryActivities) > 0 {
				pairs = append(pairs, output.KeyValue{Key: "Time Entry Activities", Value: formatActivities(project.TimeEntryActivities)})
			}
			if len(project.IssueCustomFields) > 0 {
				pairs = append(pairs, output.KeyValue{Key: "Issue Custom Fields", Value: formatIDNameList(project.IssueCustomFields)})
			}
			if len(project.CustomFields) > 0 {
				pairs = append(pairs, output.KeyValue{Key: "Custom Fields", Value: formatCustomFieldValues(project.CustomFields)})
			}

			pairs = append(pairs,
				output.KeyValue{Key: "Created", Value: project.CreatedOn},
				output.KeyValue{Key: "Updated", Value: project.UpdatedOn},
			)

			printer.Detail(pairs)
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)
	cmd.Flags().StringSliceVar(&includes, "include", nil,
		"Include related data: trackers, issue_categories, enabled_modules, time_entry_activities, issue_custom_fields (repeatable or comma-separated)")
	_ = cmd.RegisterFlagCompletionFunc("include", completeProjectIncludes)

	return cmd
}

func formatIDNameList(items []models.IDName) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = fmt.Sprintf("%s (#%d)", it.Name, it.ID)
	}
	return strings.Join(parts, ", ")
}

func formatModuleList(items []models.IDName) string {
	// Modules sometimes lack IDs in the response (older Redmine versions);
	// surface name-only when ID is zero so the output stays readable.
	parts := make([]string, len(items))
	for i, it := range items {
		if it.ID == 0 {
			parts[i] = it.Name
		} else {
			parts[i] = fmt.Sprintf("%s (#%d)", it.Name, it.ID)
		}
	}
	return strings.Join(parts, ", ")
}

func formatActivities(items []models.TimeEntryActivity) string {
	parts := make([]string, len(items))
	for i, a := range items {
		label := fmt.Sprintf("%s (#%d)", a.Name, a.ID)
		if a.IsDefault {
			label += " [default]"
		}
		parts[i] = label
	}
	return strings.Join(parts, ", ")
}

func formatCustomFieldValues(items []models.CustomFieldValue) string {
	parts := make([]string, len(items))
	for i, cf := range items {
		parts[i] = fmt.Sprintf("%s: %v", cf.Name, cf.Value)
	}
	return strings.Join(parts, "; ")
}
