package ops

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type PrioritiesListResult struct {
	Priorities []models.Enumeration `json:"issue_priorities"`
	Count      int                  `json:"count"`
}

type ListTimeEntryActivitiesInput struct {
	ProjectID string `json:"project_id,omitempty" jsonschema:"Project identifier or numeric ID. When set, returns the activities available in that project (only active ones, project-specific overrides included); Redmine reports only id and name then, so active is always true and is_default is derived from the default shared activity of the same name."`
}

type TimeEntryActivitiesListResult struct {
	Activities []models.Enumeration `json:"time_entry_activities"`
	Count      int                  `json:"count"`
}

type DocumentCategoriesListResult struct {
	DocumentCategories []models.Enumeration `json:"document_categories"`
	Count              int                  `json:"count"`
}

//mcpgen:tool list_priorities
//mcpgen:description List issue priorities with their IDs, default flag and active state. Use the IDs for priority_id.
//mcpgen:category meta
func ListPriorities(ctx context.Context, client *api.Client, _ struct{}) (PrioritiesListResult, error) {
	items, err := client.Enumerations.IssuePriorities(ctx)
	if err != nil {
		return PrioritiesListResult{}, err
	}
	return PrioritiesListResult{Priorities: items, Count: len(items)}, nil
}

//mcpgen:tool list_time_entry_activities
//mcpgen:description List time entry activities with their IDs. Pass project_id to get the activities available in that project, including project-specific overrides whose IDs differ from the global ones. Use the IDs for activity_id.
//mcpgen:category time
func ListTimeEntryActivities(ctx context.Context, client *api.Client, input ListTimeEntryActivitiesInput) (TimeEntryActivitiesListResult, error) {
	if input.ProjectID == "" {
		items, err := client.Enumerations.TimeEntryActivities(ctx)
		if err != nil {
			return TimeEntryActivitiesListResult{}, err
		}
		return TimeEntryActivitiesListResult{Activities: items, Count: len(items)}, nil
	}
	project, err := client.Projects.Get(ctx, input.ProjectID, []string{"time_entry_activities"})
	if err != nil {
		return TimeEntryActivitiesListResult{}, err
	}
	if project.TimeEntryActivities == nil {
		return TimeEntryActivitiesListResult{}, fmt.Errorf("Redmine returned no time entry activities for project %q (it needs the view_time_entries permission there)", input.ProjectID)
	}
	// Redmine only reports {id, name} here and only lists active activities.
	// Fill in the flags it omits so they do not decode as misleading false values.
	defaults := map[string]bool{}
	if shared, sharedErr := client.Enumerations.TimeEntryActivities(ctx); sharedErr == nil {
		for _, a := range shared {
			if a.IsDefault {
				defaults[a.Name] = true
			}
		}
	}
	items := make([]models.Enumeration, len(project.TimeEntryActivities))
	for i, a := range project.TimeEntryActivities {
		a.Active = true
		a.IsDefault = defaults[a.Name]
		items[i] = a
	}
	return TimeEntryActivitiesListResult{Activities: items, Count: len(items)}, nil
}

// ListDocumentCategories lists document categories. It is CLI-only (no MCP tool).
func ListDocumentCategories(ctx context.Context, client *api.Client, _ struct{}) (DocumentCategoriesListResult, error) {
	items, err := client.Enumerations.DocumentCategories(ctx)
	if err != nil {
		return DocumentCategoriesListResult{}, err
	}
	return DocumentCategoriesListResult{DocumentCategories: items, Count: len(items)}, nil
}
