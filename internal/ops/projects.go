package ops

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type ListProjectsInput struct {
	Includes []string `json:"includes,omitempty" jsonschema:"Extra sections to include: trackers, issue_categories, enabled_modules."`
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
	Includes   []string `json:"includes,omitempty" jsonschema:"Extra sections to include: trackers, issue_categories, enabled_modules."`
}

type CreateProjectInput struct {
	Name           string `json:"name" jsonschema:"Human-readable project name."`
	Identifier     string `json:"identifier" jsonschema:"URL-safe project identifier (slug)."`
	Description    string `json:"description,omitempty" jsonschema:"Project description."`
	IsPublic       *bool  `json:"is_public,omitempty" jsonschema:"Mark the project as public."`
	ParentID       int    `json:"parent_id,omitempty" jsonschema:"Parent project numeric ID."`
	InheritMembers bool   `json:"inherit_members,omitempty" jsonschema:"Inherit members from the parent project."`
}

type UpdateProjectInput struct {
	Identifier  string  `json:"identifier" jsonschema:"Project identifier or numeric ID to update."`
	Name        *string `json:"name,omitempty" jsonschema:"New project name."`
	Description *string `json:"description,omitempty" jsonschema:"New project description."`
	IsPublic    *bool   `json:"is_public,omitempty" jsonschema:"Toggle public visibility."`
}

type DeleteProjectInput struct {
	Identifier string `json:"identifier" jsonschema:"Project identifier or numeric ID to delete. Destructive."`
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

func ListProjects(ctx context.Context, client *api.Client, input ListProjectsInput) (ProjectsListResult, error) {
	projects, total, err := client.Projects.List(ctx, input.Includes, ListLimit(input.Limit), input.Offset)
	if err != nil {
		return ProjectsListResult{}, err
	}
	return ProjectsListResult{Projects: projects, Count: len(projects), TotalCount: total}, nil
}

func GetProject(ctx context.Context, client *api.Client, input GetProjectInput) (*models.Project, error) {
	return client.Projects.Get(ctx, input.Identifier, input.Includes)
}

func CreateProject(ctx context.Context, client *api.Client, input CreateProjectInput) (*models.Project, error) {
	return client.Projects.Create(ctx, models.ProjectCreate{
		Name:           input.Name,
		Identifier:     input.Identifier,
		Description:    input.Description,
		IsPublic:       input.IsPublic,
		ParentID:       input.ParentID,
		InheritMembers: input.InheritMembers,
	})
}

func UpdateProject(ctx context.Context, client *api.Client, input UpdateProjectInput) (MessageResult, error) {
	err := client.Projects.Update(ctx, input.Identifier, models.ProjectUpdate{
		Name:        input.Name,
		Description: input.Description,
		IsPublic:    input.IsPublic,
	})
	if err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Updated project %s", input.Identifier)}, nil
}

func DeleteProject(ctx context.Context, client *api.Client, input DeleteProjectInput) (MessageResult, error) {
	if err := client.Projects.Delete(ctx, input.Identifier); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Deleted project %s", input.Identifier)}, nil
}

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
