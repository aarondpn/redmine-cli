package ops

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type ListProjectsInput struct {
	Includes []string `json:"includes,omitempty" jsonschema:"Extra sections to include: trackers, issue_categories, enabled_modules, time_entry_activities, issue_custom_fields."`
	Limit    int      `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset   int      `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type ProjectsListResult struct {
	Projects   []models.Project `json:"projects"`
	Count      int              `json:"count"`
	TotalCount int              `json:"total_count"`
}

type GetProjectInput struct {
	Identifier string   `json:"identifier" jsonschema:"Project identifier (slug) or numeric ID."`
	Includes   []string `json:"includes,omitempty" jsonschema:"Extra sections to include: trackers, issue_categories, enabled_modules, time_entry_activities, issue_custom_fields."`
}

type CreateProjectInput struct {
	Name                string            `json:"name" jsonschema:"Human-readable project name."`
	Identifier          string            `json:"identifier" jsonschema:"URL-safe project identifier (slug)."`
	Description         string            `json:"description,omitempty" jsonschema:"Project description."`
	Homepage            string            `json:"homepage,omitempty" jsonschema:"Project homepage URL."`
	IsPublic            *bool             `json:"is_public,omitempty" jsonschema:"Mark the project as public."`
	ParentID            int               `json:"parent_id,omitempty" jsonschema:"Parent project numeric ID."`
	InheritMembers      bool              `json:"inherit_members,omitempty" jsonschema:"Inherit members from the parent project."`
	DefaultAssignedToID int               `json:"default_assigned_to_id,omitempty" jsonschema:"Numeric user ID set as the default assignee for new issues."`
	DefaultVersionID    int               `json:"default_version_id,omitempty" jsonschema:"Numeric version ID set as the default version for new issues. Only effective on update; versions cannot be referenced at project creation."`
	TrackerIDs          []int             `json:"tracker_ids,omitempty" jsonschema:"Numeric IDs of trackers enabled for this project. Replaces the current set when provided."`
	EnabledModuleNames  []string          `json:"enabled_module_names,omitempty" jsonschema:"Module names enabled for this project (boards, calendar, documents, files, gantt, issue_tracking, news, repository, time_tracking, wiki). Replaces the current set when provided."`
	IssueCustomFieldIDs []int             `json:"issue_custom_field_ids,omitempty" jsonschema:"Numeric IDs of issue-level custom fields enabled for this project."`
	CustomFieldValues   map[string]string `json:"custom_field_values,omitempty" jsonschema:"Values for project-level custom fields, keyed by custom field ID as a string."`
}

type UpdateProjectInput struct {
	Identifier          string            `json:"identifier" jsonschema:"Project identifier or numeric ID to update."`
	Name                *string           `json:"name,omitempty" jsonschema:"New project name."`
	Description         *string           `json:"description,omitempty" jsonschema:"New project description."`
	Homepage            *string           `json:"homepage,omitempty" jsonschema:"New project homepage URL."`
	IsPublic            *bool             `json:"is_public,omitempty" jsonschema:"Toggle public visibility."`
	ParentID            *int              `json:"parent_id,omitempty" jsonschema:"New parent project numeric ID. 0 detaches the project from its parent."`
	InheritMembers      *bool             `json:"inherit_members,omitempty" jsonschema:"Toggle inheriting members from the parent project."`
	DefaultAssignedToID *int              `json:"default_assigned_to_id,omitempty" jsonschema:"Numeric user ID set as the default assignee for new issues. 0 clears the default."`
	DefaultVersionID    *int              `json:"default_version_id,omitempty" jsonschema:"Numeric version ID set as the default version for new issues. 0 clears the default."`
	TrackerIDs          []int             `json:"tracker_ids,omitempty" jsonschema:"Replace the enabled tracker set with the given IDs."`
	EnabledModuleNames  []string          `json:"enabled_module_names,omitempty" jsonschema:"Replace the enabled module set with the given names."`
	IssueCustomFieldIDs []int             `json:"issue_custom_field_ids,omitempty" jsonschema:"Replace the enabled issue custom-field set with the given IDs."`
	CustomFieldValues   map[string]string `json:"custom_field_values,omitempty" jsonschema:"Values for project-level custom fields, keyed by custom field ID as a string."`
}

type DeleteProjectInput struct {
	Identifier string `json:"identifier" jsonschema:"Project identifier or numeric ID to delete. Destructive."`
}

type ArchiveProjectInput struct {
	Identifier string `json:"identifier" jsonschema:"Project identifier or numeric ID to archive. Requires Redmine 5.0+."`
}

type UnarchiveProjectInput struct {
	Identifier string `json:"identifier" jsonschema:"Project identifier or numeric ID to unarchive. Requires Redmine 5.0+."`
}

type ListProjectMembersInput struct {
	Identifier string `json:"identifier" jsonschema:"Project identifier or numeric ID."`
	Limit      int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset     int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type ProjectMembersListResult struct {
	Members    []models.Membership `json:"members"`
	Count      int                 `json:"count"`
	TotalCount int                 `json:"total_count"`
}

//mcpgen:tool list_projects
//mcpgen:description List Redmine projects.
//mcpgen:category projects
func ListProjects(ctx context.Context, client *api.Client, input ListProjectsInput) (ProjectsListResult, error) {
	projects, total, err := client.Projects.List(ctx, input.Includes, ListLimit(input.Limit), input.Offset)
	if err != nil {
		return ProjectsListResult{}, err
	}
	return ProjectsListResult{Projects: projects, Count: len(projects), TotalCount: total}, nil
}

//mcpgen:tool get_project
//mcpgen:description Fetch a single Redmine project by identifier or ID.
//mcpgen:category projects
func GetProject(ctx context.Context, client *api.Client, input GetProjectInput) (*models.Project, error) {
	return client.Projects.Get(ctx, input.Identifier, input.Includes)
}

//mcpgen:tool create_project
//mcpgen:description Create a new Redmine project. Requires --enable-writes.
//mcpgen:category projects
//mcpgen:writes
func CreateProject(ctx context.Context, client *api.Client, input CreateProjectInput) (*models.Project, error) {
	return client.Projects.Create(ctx, models.ProjectCreate{
		Name:                input.Name,
		Identifier:          input.Identifier,
		Description:         input.Description,
		Homepage:            input.Homepage,
		IsPublic:            input.IsPublic,
		ParentID:            input.ParentID,
		InheritMembers:      input.InheritMembers,
		DefaultAssignedToID: input.DefaultAssignedToID,
		DefaultVersionID:    input.DefaultVersionID,
		TrackerIDs:          input.TrackerIDs,
		EnabledModuleNames:  input.EnabledModuleNames,
		IssueCustomFieldIDs: input.IssueCustomFieldIDs,
		CustomFieldValues:   input.CustomFieldValues,
	})
}

//mcpgen:tool update_project
//mcpgen:description Update an existing Redmine project. Requires --enable-writes.
//mcpgen:category projects
//mcpgen:writes
func UpdateProject(ctx context.Context, client *api.Client, input UpdateProjectInput) (MessageResult, error) {
	err := client.Projects.Update(ctx, input.Identifier, models.ProjectUpdate{
		Name:                input.Name,
		Description:         input.Description,
		Homepage:            input.Homepage,
		IsPublic:            input.IsPublic,
		ParentID:            input.ParentID,
		InheritMembers:      input.InheritMembers,
		DefaultAssignedToID: input.DefaultAssignedToID,
		DefaultVersionID:    input.DefaultVersionID,
		TrackerIDs:          input.TrackerIDs,
		EnabledModuleNames:  input.EnabledModuleNames,
		IssueCustomFieldIDs: input.IssueCustomFieldIDs,
		CustomFieldValues:   input.CustomFieldValues,
	})
	if err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Updated project %s", input.Identifier)}, nil
}

//mcpgen:tool delete_project
//mcpgen:description Delete a Redmine project. Destructive. Requires --enable-writes.
//mcpgen:category projects
//mcpgen:writes
func DeleteProject(ctx context.Context, client *api.Client, input DeleteProjectInput) (MessageResult, error) {
	if err := client.Projects.Delete(ctx, input.Identifier); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Deleted project %s", input.Identifier)}, nil
}

//mcpgen:tool archive_project
//mcpgen:description Archive a Redmine project. Hides it from default listings. Requires Redmine 5.0+ and --enable-writes.
//mcpgen:category projects
//mcpgen:writes
func ArchiveProject(ctx context.Context, client *api.Client, input ArchiveProjectInput) (MessageResult, error) {
	if err := client.Projects.Archive(ctx, input.Identifier); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Archived project %s", input.Identifier)}, nil
}

//mcpgen:tool unarchive_project
//mcpgen:description Unarchive a Redmine project. Requires Redmine 5.0+ and --enable-writes.
//mcpgen:category projects
//mcpgen:writes
func UnarchiveProject(ctx context.Context, client *api.Client, input UnarchiveProjectInput) (MessageResult, error) {
	if err := client.Projects.Unarchive(ctx, input.Identifier); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Unarchived project %s", input.Identifier)}, nil
}

//mcpgen:tool list_project_members
//mcpgen:description List members for a Redmine project.
//mcpgen:category projects
func ListProjectMembers(ctx context.Context, client *api.Client, input ListProjectMembersInput) (ProjectMembersListResult, error) {
	members, total, err := client.Projects.Members(ctx, input.Identifier, ListLimit(input.Limit), input.Offset)
	if err != nil {
		return ProjectMembersListResult{}, err
	}
	return ProjectMembersListResult{Members: members, Count: len(members), TotalCount: total}, nil
}

func GetProjectForResource(ctx context.Context, client *api.Client, identifier string) (*models.Project, error) {
	return client.Projects.Get(ctx, identifier, []string{"trackers", "issue_categories", "enabled_modules"})
}
