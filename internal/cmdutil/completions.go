package cmdutil

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

const completionTimeout = 3 * time.Second

// completionClient returns an API client and a timeout context for use in
// completion functions, after applying the global flags given on the
// command line being completed (see applyCompletionFlags). Returns nil if
// the client cannot be initialized (e.g., no config, no server URL).
func completionClient(f *Factory, cmd *cobra.Command) (*api.Client, context.Context, context.CancelFunc) {
	if !applyCompletionFlags(f, cmd) {
		return nil, nil, nil
	}
	client, err := f.ApiClient()
	if err != nil {
		return nil, nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), completionTimeout)
	return client, ctx, cancel
}

// filterCompletions returns the items whose value starts with toComplete
// (case-insensitively), each made safe with safeCompletion.
// applyCompletionFlags applies the root command's global flags (--config,
// --profile, --server, ...) to f, since cobra does not run the root
// command's PersistentPreRunE for `redmine __complete`. It reports false
// when completion must not load the configuration: the flags are invalid,
// or --api-key-file - would read the key from the terminal the user is
// typing in.
func applyCompletionFlags(f *Factory, cmd *cobra.Command) bool {
	if err := ApplyGlobalFlags(f, cmd); err != nil {
		return false
	}
	return f.APIKeyFile != "-"
}

func filterCompletions(items []string, toComplete string) []string {
	prefix := strings.ToLower(toComplete)
	var filtered []string
	for _, item := range items {
		item, ok := safeCompletion(item)
		if !ok {
			continue
		}
		// Match against the value part (before any \t description).
		val, _, _ := strings.Cut(item, "\t")
		if strings.HasPrefix(strings.ToLower(val), prefix) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// safeCompletion prepares a "value\tdescription" completion candidate built
// from server data for the shell. The completion protocol has one candidate
// per line, so a value containing a line break is dropped (ok is false)
// rather than split into several candidates, and runs of whitespace in the
// description, including tabs and line breaks, become single spaces. Other
// control characters, which the shell would print verbatim when listing
// candidates, are neutralized with output.SanitizeTerminal.
func safeCompletion(item string) (string, bool) {
	val, desc, hasDesc := strings.Cut(item, "\t")
	if strings.ContainsAny(val, "\n\r") {
		return "", false
	}
	val = output.SanitizeTerminal(val)
	if !hasDesc {
		return val, true
	}
	return val + "\t" + output.SanitizeTerminal(strings.Join(strings.Fields(desc), " ")), true
}

// CompleteOutputFormat provides static completions for the --output flag.
func CompleteOutputFormat(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return filterCompletions([]string{
		"table\tDefault tabular output",
		"json\tJSON output",
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
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f, cmd)
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
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f, cmd)
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
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f, cmd)
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

// CompleteCustomFields returns a completion function for the custom-fields
// command argument. Hits the admin-only /custom_fields.json endpoint, so it
// silently returns no candidates when the authenticated user lacks access.
func CompleteCustomFields(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f, cmd)
		if client == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		defer cancel()

		fields, err := client.CustomFields.List(ctx)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		items := make([]string, 0, len(fields))
		for _, fld := range fields {
			items = append(items, fmt.Sprintf("%s\t%s/%s", fld.Name, fld.CustomizedType, fld.FieldFormat))
		}
		return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteCustomFieldValues returns a completion function for --custom-field.
// Before the '=' it offers "Name=" for the custom fields of customizedType;
// after "Name=" (or "<id>=") of a list field it offers the possible values.
// Hits the admin-only /custom_fields.json endpoint, so it silently returns no
// candidates when the authenticated user lacks access.
func CompleteCustomFieldValues(f *Factory, customizedType string) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f, cmd)
		if client == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		defer cancel()

		fields, err := client.CustomFields.List(ctx)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		fields = slices.DeleteFunc(fields, func(fld models.CustomField) bool { return fld.CustomizedType != customizedType })

		key, _, hasValue := strings.Cut(toComplete, "=")
		if hasValue {
			var items []string
			for _, fld := range fields {
				if !strings.EqualFold(fld.Name, key) && strconv.Itoa(fld.ID) != key {
					continue
				}
				for _, pv := range fld.PossibleValues {
					items = append(items, key+"="+pv.Value)
				}
			}
			return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoFileComp
		}

		items := make([]string, 0, len(fields))
		for _, fld := range fields {
			desc := fld.FieldFormat
			if fld.Multiple {
				desc += ", multiple"
			}
			if fld.IsRequired {
				desc += ", required"
			}
			items = append(items, fmt.Sprintf("%s=\t%s", fld.Name, desc))
		}
		return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteRoles returns a completion function for role-related flags.
func CompleteRoles(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f, cmd)
		if client == nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		defer cancel()

		roles, err := client.Roles.List(ctx)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		items := make([]string, 0, len(roles))
		for _, r := range roles {
			if r.Assignable != nil && !r.IsAssignable() {
				continue
			}
			items = append(items, fmt.Sprintf("%s\tID %d", r.Name, r.ID))
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
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f, cmd)
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
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f, cmd)
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
// "me" is always offered because the resolver handles it without listing
// users; the rest of the list comes from /users.json, which requires admin.
// Non-admin sessions degrade gracefully to just the "me" candidate.
func CompleteUsers(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		items := []string{"me\tCurrent user"}

		client, ctx, cancel := completionClient(f, cmd)
		if client == nil {
			return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoFileComp
		}
		defer cancel()

		users, _, err := client.Users.List(ctx, models.UserFilter{Limit: 100})
		if err != nil {
			return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoFileComp
		}

		for _, u := range users {
			name := strings.TrimSpace(u.FirstName + " " + u.LastName)
			if u.Login != "" {
				items = append(items, fmt.Sprintf("%s\t%s", u.Login, name))
			}
		}
		return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

// CompleteAssignees returns a completion function for --assignee flags.
// "me" and "none" are always offered. When the project is known (--project,
// the default project, or the issue in the first positional argument) the
// candidates are all of the project's members, users and groups alike, from
// the memberships endpoint, which non-admins can read. Otherwise it falls
// back to CompleteUsers.
func CompleteAssignees(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	completeUsers := CompleteUsers(f)
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		none := "none\tNo assignee"

		if items, ok := completeProjectMembers(f, cmd, args); ok {
			items = append([]string{"me\tCurrent user", none}, items...)
			return filterCompletions(items, toComplete), cobra.ShellCompDirectiveNoFileComp
		}

		items, directive := completeUsers(cmd, args, toComplete)
		return append(filterCompletions([]string{none}, toComplete), items...), directive
	}
}

// completeProjectMembers lists every user and group that is a member of the
// command's project as completion candidates. The project comes from
// --project (or the default project) on commands with that flag, and from
// the issue named by the first positional argument otherwise (issues update
// / assign). ok is false when no project is known or a request fails.
func completeProjectMembers(f *Factory, cmd *cobra.Command, args []string) ([]string, bool) {
	var project string
	issueID := 0
	if cmd.Flags().Lookup("project") != nil {
		project = resolveProjectForCompletion(f, cmd)
	} else if len(args) > 0 {
		issueID, _ = strconv.Atoi(args[0])
	}
	if project == "" && issueID <= 0 {
		return nil, false
	}

	client, ctx, cancel := completionClient(f, cmd)
	if client == nil {
		return nil, false
	}
	defer cancel()

	if project == "" {
		issue, err := client.Issues.Get(ctx, issueID, nil)
		if err != nil {
			return nil, false
		}
		project = strconv.Itoa(issue.Project.ID)
	}

	memberships, _, err := client.Memberships.List(ctx, project, 0, 0)
	if err != nil {
		return nil, false
	}
	items := make([]string, 0, len(memberships))
	for _, m := range memberships {
		switch {
		case m.User != nil:
			items = append(items, fmt.Sprintf("%s\tuser, ID %d", m.User.Name, m.User.ID))
		case m.Group != nil:
			items = append(items, fmt.Sprintf("%s\tgroup, ID %d", m.Group.Name, m.Group.ID))
		}
	}
	return items, true
}

// CompleteGroups returns a completion function for the --group flag.
// Returns empty completions gracefully if the user lacks admin privileges.
func CompleteGroups(f *Factory) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		client, ctx, cancel := completionClient(f, cmd)
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
	if project == "" && applyCompletionFlags(f, cmd) {
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
		client, ctx, cancel := completionClient(f, cmd)
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
		client, ctx, cancel := completionClient(f, cmd)
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
