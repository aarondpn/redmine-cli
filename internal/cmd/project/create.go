package project

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func newCmdCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		name              string
		identifier        string
		description       string
		homepage          string
		public            bool
		parent            string
		inheritMembers    bool
		defaultAssignee   string
		trackers          []string
		enabledModules    []string
		issueCustomFields []string
		customFieldRaw    []string
		format            string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Args:    cobra.NoArgs,
		Aliases: []string{"new"},
		Short:   "Create a new project",
		Long:    "Create a new Redmine project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := validateEnabledModules(enabledModules); err != nil {
				return err
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			input := ops.CreateProjectInput{
				Name:       name,
				Identifier: identifier,
			}

			if description != "" {
				input.Description = description
			}
			if homepage != "" {
				input.Homepage = homepage
			}
			if cmd.Flags().Changed("public") {
				input.IsPublic = &public
			}
			if parent != "" {
				id, _, err := resolver.ResolveProject(ctx, client, parent)
				if err != nil {
					return fmt.Errorf("resolve --parent: %w", err)
				}
				input.ParentID = id
			}
			if cmd.Flags().Changed("inherit-members") {
				input.InheritMembers = inheritMembers
			}

			if defaultAssignee != "" {
				id, err := resolver.ResolveAssignee(ctx, client, defaultAssignee)
				if err != nil {
					return fmt.Errorf("resolve --default-assignee: %w", err)
				}
				input.DefaultAssignedToID = id
			}

			if len(trackers) > 0 {
				ids, err := resolver.ResolveTrackerNames(ctx, client, trackers)
				if err != nil {
					return fmt.Errorf("resolve --tracker: %w", err)
				}
				input.TrackerIDs = ids
			}

			if len(enabledModules) > 0 {
				input.EnabledModuleNames = enabledModules
			}

			if len(issueCustomFields) > 0 {
				ids, err := resolver.ResolveCustomFieldNames(ctx, client, issueCustomFields)
				if err != nil {
					return fmt.Errorf("resolve --issue-custom-field: %w", err)
				}
				input.IssueCustomFieldIDs = ids
			}

			if len(customFieldRaw) > 0 {
				values, err := parseCustomFieldValues(ctx, client, customFieldRaw)
				if err != nil {
					return err
				}
				input.CustomFieldValues = values
			}

			project, err := ops.CreateProject(ctx, client, input)
			if err != nil {
				return err
			}

			printer.Resource(project, fmt.Sprintf("Project %q created (ID: %d)", project.Name, project.ID))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Project name (required)")
	cmd.Flags().StringVar(&identifier, "identifier", "", "Project identifier (required)")
	cmd.Flags().StringVar(&description, "description", "", "Project description")
	cmd.Flags().StringVar(&homepage, "homepage", "", "Project homepage URL")
	cmd.Flags().BoolVar(&public, "public", false, "Make project public")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent project identifier, name, or numeric ID")
	cmd.Flags().BoolVar(&inheritMembers, "inherit-members", false, "Inherit members from the parent project")
	cmd.Flags().StringVar(&defaultAssignee, "default-assignee", "", "Default assignee for new issues (login, name, or numeric ID)")
	cmd.Flags().StringSliceVar(&trackers, "tracker", nil, "Tracker name or ID to enable (repeatable or comma-separated)")
	cmd.Flags().StringSliceVar(&enabledModules, "enable-module", nil,
		"Module to enable: boards, calendar, documents, files, gantt, issue_tracking, news, repository, time_tracking, wiki (repeatable or comma-separated)")
	cmd.Flags().StringSliceVar(&issueCustomFields, "issue-custom-field", nil, "Issue-level custom field name or ID to enable (repeatable or comma-separated)")
	cmd.Flags().StringArrayVar(&customFieldRaw, "custom-field", nil, "Project custom field value as name=value or id=value (repeatable)")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("identifier")

	_ = cmd.RegisterFlagCompletionFunc("parent", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("tracker", cmdutil.CompleteTrackers(f))
	_ = cmd.RegisterFlagCompletionFunc("enable-module", completeEnabledModules)

	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}
