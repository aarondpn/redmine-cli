package cmdutil

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

const completionTimeout = 3 * time.Second

// completionClient returns an API client and a timeout context for use in
// completion functions. Returns nil if the client cannot be initialized
// (e.g., no config, no server URL).
func completionClient(f *Factory) (*api.Client, context.Context, context.CancelFunc) {
	client, err := f.ApiClient()
	if err != nil {
		return nil, nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
	return client, ctx, cancel
}

func filterCompletions(items []string, toComplete string) []string {
	if toComplete == "" {
		return items
	}
	prefix := strings.ToLower(toComplete)
	var filtered []string
	for _, item := range items {
		// Match against the value part (before any \t description).
		val := item
		if idx := strings.Index(item, "\t"); idx >= 0 {
			val = item[:idx]
		}
		if strings.HasPrefix(strings.ToLower(val), prefix) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// CompleteOutputFormat provides static completions for the --output flag.
func CompleteOutputFormat(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return filterCompletions([]string{
		"table\tDefault tabular output",
		"json\tJSON output",
		"csv\tCSV output",
	}, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// CompleteVersionStatus provides static completions for version --status flag.
func CompleteVersionStatus(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return filterCompletions([]string{
		"open",
		"locked",
		"closed",
	}, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// CompleteVersionSharing provides static completions for version --sharing flag.
func CompleteVersionSharing(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return filterCompletions([]string{
		"none",
		"descendants",
		"hierarchy",
		"tree",
		"system",
	}, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// CompleteUserStatus provides static completions for user --status flag.
func CompleteUserStatus(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return filterCompletions([]string{
		"active",
		"registered",
		"locked",
	}, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// CompleteProjects returns a completion function for the --project flag.
func CompleteProjects(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f)
		if client == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		defer cancel()

		projects, _, err := client.Projects.List(ctx, nil, 0, 0)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		items := make([]string, 0, len(projects))
		for _, p := range projects {
			items = append(items, fmt.Sprintf("%s\t%s", p.Identifier, p.Name))
		}
		return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteTrackers returns a completion function for the --tracker flag.
func CompleteTrackers(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f)
		if client == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		defer cancel()

		trackers, err := client.Trackers.List(ctx)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		items := make([]string, 0, len(trackers))
		for _, t := range trackers {
			items = append(items, t.Name)
		}
		return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteStatuses returns a completion function for the --status flag.
func CompleteStatuses(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f)
		if client == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		defer cancel()

		statuses, err := client.Statuses.List(ctx)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		items := make([]string, 0, len(statuses))
		for _, s := range statuses {
			items = append(items, s.Name)
		}
		return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteIssueListStatus returns a completion function for the --status flag
// on the issues list command, which also accepts special values.
func CompleteIssueListStatus(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		static := []string{
			"open\tShow open issues (default)",
			"closed\tShow closed issues",
			"*\tShow all issues",
		}
		// Fetch real status names from the API (already filtered by toComplete).
		dynamic, _ := CompleteStatuses(f)(cmd, args, toComplete)
		all := append(static, dynamic...)
		// Only filter the static entries; dynamic ones are already filtered.
		if toComplete == "" {
			return all, cobra.ShellCompDirectiveNoFileComp
		}
		return filterCompletions(all, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// CompletePriorities returns a completion function for the --priority flag.
func CompletePriorities(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f)
		if client == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		defer cancel()

		priorities, err := client.Enumerations.IssuePriorities(ctx)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		items := make([]string, 0, len(priorities))
		for _, p := range priorities {
			items = append(items, p.Name)
		}
		return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteActivities returns a completion function for the --activity flag.
func CompleteActivities(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f)
		if client == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		defer cancel()

		activities, err := client.Enumerations.TimeEntryActivities(ctx)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		items := make([]string, 0, len(activities))
		for _, a := range activities {
			items = append(items, a.Name)
		}
		return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteUsers returns a completion function for user-related flags.
// Returns empty completions gracefully if the user lacks admin privileges.
func CompleteUsers(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f)
		if client == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		defer cancel()

		users, _, err := client.Users.List(ctx, models.UserFilter{Limit: 100})
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		items := make([]string, 0, len(users)+1)
		items = append(items, "me\tCurrent user")
		for _, u := range users {
			name := strings.TrimSpace(u.FirstName + " " + u.LastName)
			if u.Login != "" {
				items = append(items, fmt.Sprintf("%s\t%s", u.Login, name))
			}
		}
		return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteGroups returns a completion function for the --group flag.
// Returns empty completions gracefully if the user lacks admin privileges.
func CompleteGroups(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f)
		if client == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		defer cancel()

		groups, _, err := client.Groups.List(ctx, models.GroupFilter{Limit: 100})
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		items := make([]string, 0, len(groups))
		for _, g := range groups {
			items = append(items, g.Name)
		}
		return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// resolveProjectForCompletion reads the --project flag from the command,
// falls back to the default project from config, and returns the identifier.
func resolveProjectForCompletion(f *Factory, cmd *cobra.Command) string {
	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		if cfg, err := f.Config(); err == nil && cfg.DefaultProject != "" {
			project = cfg.DefaultProject
		}
	}
	if project == "" {
		return ""
	}
	// Use the project value as-is for the API call (identifier or ID both work).
	return project
}

// CompleteCategories returns a completion function for the --category flag.
// Requires --project to be set (or a default project in config).
func CompleteCategories(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f)
		if client == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		defer cancel()

		project := resolveProjectForCompletion(f, cmd)
		if project == "" {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		categories, _, err := client.Categories.List(ctx, project)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		items := make([]string, 0, len(categories))
		for _, c := range categories {
			items = append(items, c.Name)
		}
		return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteVersions returns a completion function for the --version flag
// that shows all versions including closed ones.
// Requires --project to be set (or a default project in config).
func CompleteVersions(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeVersions(f, false)
}

// CompleteOpenVersions returns a completion function for the --version flag
// that only shows open and locked versions (excludes closed).
// Useful for create/update commands where closed versions aren't valid targets.
func CompleteOpenVersions(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeVersions(f, true)
}

func completeVersions(f *Factory, excludeClosed bool) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f)
		if client == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		defer cancel()

		project := resolveProjectForCompletion(f, cmd)
		if project == "" {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		versions, _, err := client.Versions.List(ctx, project, 0, 0)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		items := make([]string, 0, len(versions))
		for _, v := range versions {
			if excludeClosed && v.Status == "closed" {
				continue
			}
			items = append(items, fmt.Sprintf("%s\t%s", v.Name, v.Status))
		}
		return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}
