package project

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

func newCmdUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		name              string
		description       string
		homepage          string
		public            bool
		parentID          int
		inheritMembers    bool
		defaultAssignee   string
		defaultVersion    string
		trackers          []string
		enabledModules    []string
		issueCustomFields []string
		customFieldRaw    []string
	)

	cmd := &cobra.Command{
		Use:               "update <identifier>",
		Aliases:           []string{"edit"},
		Short:             "Update a project",
		Long:              "Update an existing Redmine project.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: cmdutil.CompleteProjects(f),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := validateEnabledModules(enabledModules); err != nil {
				return err
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer("")
			identifier := args[0]

			input := ops.UpdateProjectInput{Identifier: identifier}

			if cmd.Flags().Changed("name") {
				input.Name = &name
			}
			if cmd.Flags().Changed("description") {
				input.Description = &description
			}
			if cmd.Flags().Changed("homepage") {
				input.Homepage = &homepage
			}
			if cmd.Flags().Changed("public") {
				input.IsPublic = &public
			}
			if cmd.Flags().Changed("parent") {
				input.ParentID = &parentID
			}
			if cmd.Flags().Changed("inherit-members") {
				input.InheritMembers = &inheritMembers
			}

			if cmd.Flags().Changed("default-assignee") {
				id, err := resolveOptionalUserID(ctx, client, defaultAssignee)
				if err != nil {
					return err
				}
				input.DefaultAssignedToID = &id
			}

			if cmd.Flags().Changed("default-version") {
				id, err := resolveOptionalVersionID(ctx, client, defaultVersion, identifier)
				if err != nil {
					return err
				}
				input.DefaultVersionID = &id
			}

			if cmd.Flags().Changed("tracker") {
				ids, err := resolver.ResolveTrackerNames(ctx, client, trackers)
				if err != nil {
					return fmt.Errorf("resolve --tracker: %w", err)
				}
				input.TrackerIDs = ids
			}

			if cmd.Flags().Changed("enable-module") {
				input.EnabledModuleNames = enabledModules
			}

			if cmd.Flags().Changed("issue-custom-field") {
				ids, err := resolver.ResolveCustomFieldNames(ctx, client, issueCustomFields)
				if err != nil {
					return fmt.Errorf("resolve --issue-custom-field: %w", err)
				}
				input.IssueCustomFieldIDs = ids
			}

			if cmd.Flags().Changed("custom-field") {
				values, err := parseCustomFieldValues(ctx, client, customFieldRaw)
				if err != nil {
					return err
				}
				input.CustomFieldValues = values
			}

			if _, err := ops.UpdateProject(ctx, client, input); err != nil {
				return err
			}

			printer.Action(output.ActionUpdated, "project", identifier, fmt.Sprintf("Project %q updated", identifier))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Project name")
	cmd.Flags().StringVar(&description, "description", "", "Project description")
	cmd.Flags().StringVar(&homepage, "homepage", "", "Project homepage URL")
	cmd.Flags().BoolVar(&public, "public", false, "Set public visibility")
	cmd.Flags().IntVar(&parentID, "parent", 0, "Parent project ID (0 detaches)")
	cmd.Flags().BoolVar(&inheritMembers, "inherit-members", false, "Toggle inheriting members from the parent project")
	cmd.Flags().StringVar(&defaultAssignee, "default-assignee", "", "Default assignee for new issues (login, name, or numeric ID). Pass empty to attempt to clear; some Redmine versions ignore the clear and treat 0 as 'no change'.")
	cmd.Flags().StringVar(&defaultVersion, "default-version", "", "Default version for new issues (name or numeric ID). Pass empty to attempt to clear; some Redmine versions ignore the clear and treat 0 as 'no change'.")
	cmd.Flags().StringSliceVar(&trackers, "tracker", nil, "Tracker name or ID to enable (replaces current set)")
	cmd.Flags().StringSliceVar(&enabledModules, "enable-module", nil,
		"Module name to enable (replaces current set): boards, calendar, documents, files, gantt, issue_tracking, news, repository, time_tracking, wiki")
	cmd.Flags().StringSliceVar(&issueCustomFields, "issue-custom-field", nil, "Issue-level custom field name or ID to enable (replaces current set)")
	cmd.Flags().StringArrayVar(&customFieldRaw, "custom-field", nil, "Project custom field value as name=value or id=value (repeatable)")

	_ = cmd.RegisterFlagCompletionFunc("tracker", cmdutil.CompleteTrackers(f))
	_ = cmd.RegisterFlagCompletionFunc("enable-module", completeEnabledModules)

	return cmd
}

// resolveOptionalUserID returns 0 for an empty input (signals "clear" to
// Redmine) and otherwise resolves the input via ResolveAssignee.
func resolveOptionalUserID(ctx context.Context, client *api.Client, input string) (int, error) {
	if input == "" {
		return 0, nil
	}
	id, err := resolver.ResolveAssignee(ctx, client, input)
	if err != nil {
		return 0, fmt.Errorf("resolve --default-assignee: %w", err)
	}
	return id, nil
}

// resolveOptionalVersionID returns 0 for an empty input (signals "clear" to
// Redmine) and otherwise resolves the input via ResolveVersion.
func resolveOptionalVersionID(ctx context.Context, client *api.Client, input, projectIdentifier string) (int, error) {
	if input == "" {
		return 0, nil
	}
	id, err := resolver.ResolveVersion(ctx, client, input, projectIdentifier)
	if err != nil {
		return 0, fmt.Errorf("resolve --default-version: %w", err)
	}
	return id, nil
}
