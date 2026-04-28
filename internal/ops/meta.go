package ops

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type ListVersionsInput struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Limit     int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset    int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type VersionsListResult struct {
	Versions   []models.Version `json:"versions"`
	Count      int              `json:"count"`
	TotalCount int              `json:"total_count"`
}

type GetVersionInput struct {
	ID int `json:"id" jsonschema:"Numeric version (milestone) ID."`
}

type CreateVersionInput struct {
	ProjectID     string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Name          string `json:"name" jsonschema:"Version name."`
	Status        string `json:"status,omitempty" jsonschema:"Version status: open, locked, or closed."`
	Sharing       string `json:"sharing,omitempty" jsonschema:"Version sharing: none, descendants, hierarchy, tree, or system."`
	DueDate       string `json:"due_date,omitempty" jsonschema:"Due date (YYYY-MM-DD)."`
	Description   string `json:"description,omitempty" jsonschema:"Version description."`
	WikiPageTitle string `json:"wiki_page_title,omitempty" jsonschema:"Associated wiki page title."`
}

type UpdateVersionInput struct {
	ID            int     `json:"id" jsonschema:"Numeric version (milestone) ID."`
	Name          *string `json:"name,omitempty" jsonschema:"New version name."`
	Status        *string `json:"status,omitempty" jsonschema:"New status: open, locked, or closed."`
	Sharing       *string `json:"sharing,omitempty" jsonschema:"New sharing: none, descendants, hierarchy, tree, or system."`
	DueDate       *string `json:"due_date,omitempty" jsonschema:"New due date (YYYY-MM-DD)."`
	Description   *string `json:"description,omitempty" jsonschema:"New description."`
	WikiPageTitle *string `json:"wiki_page_title,omitempty" jsonschema:"New associated wiki page title."`
}

type DeleteVersionInput struct {
	ID int `json:"id" jsonschema:"Numeric version (milestone) ID to delete. Destructive."`
}

type ListCategoriesInput struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
}

type CategoriesListResult struct {
	Categories []models.IssueCategory `json:"issue_categories"`
	Count      int                    `json:"count"`
	TotalCount int                    `json:"total_count"`
}

type TrackersListResult struct {
	Trackers []models.Tracker `json:"trackers"`
	Count    int              `json:"count"`
}

type StatusesListResult struct {
	Statuses []models.IssueStatus `json:"issue_statuses"`
	Count    int                  `json:"count"`
}

func ListVersions(ctx context.Context, client *api.Client, input ListVersionsInput) (VersionsListResult, error) {
	versions, total, err := client.Versions.List(ctx, input.ProjectID, ListLimit(input.Limit), input.Offset)
	if err != nil {
		return VersionsListResult{}, err
	}
	return VersionsListResult{Versions: versions, Count: len(versions), TotalCount: total}, nil
}

func GetVersion(ctx context.Context, client *api.Client, input GetVersionInput) (*models.Version, error) {
	return client.Versions.Get(ctx, input.ID)
}

func CreateVersion(ctx context.Context, client *api.Client, input CreateVersionInput) (*models.Version, error) {
	return client.Versions.Create(ctx, input.ProjectID, models.VersionCreate{
		Name:          input.Name,
		Status:        input.Status,
		Sharing:       input.Sharing,
		DueDate:       input.DueDate,
		Description:   input.Description,
		WikiPageTitle: input.WikiPageTitle,
	})
}

func UpdateVersion(ctx context.Context, client *api.Client, input UpdateVersionInput) (MessageResult, error) {
	err := client.Versions.Update(ctx, input.ID, models.VersionUpdate{
		Name:          input.Name,
		Status:        input.Status,
		Sharing:       input.Sharing,
		DueDate:       input.DueDate,
		Description:   input.Description,
		WikiPageTitle: input.WikiPageTitle,
	})
	if err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Updated version %d", input.ID)}, nil
}

func DeleteVersion(ctx context.Context, client *api.Client, input DeleteVersionInput) (MessageResult, error) {
	if err := client.Versions.Delete(ctx, input.ID); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Deleted version %d", input.ID)}, nil
}

func ListTrackers(ctx context.Context, client *api.Client, _ struct{}) (TrackersListResult, error) {
	trackers, err := client.Trackers.List(ctx)
	if err != nil {
		return TrackersListResult{}, err
	}
	return TrackersListResult{Trackers: trackers, Count: len(trackers)}, nil
}

func ListStatuses(ctx context.Context, client *api.Client, _ struct{}) (StatusesListResult, error) {
	statuses, err := client.Statuses.List(ctx)
	if err != nil {
		return StatusesListResult{}, err
	}
	return StatusesListResult{Statuses: statuses, Count: len(statuses)}, nil
}

func ListCategories(ctx context.Context, client *api.Client, input ListCategoriesInput) (CategoriesListResult, error) {
	categories, total, err := client.Categories.List(ctx, input.ProjectID)
	if err != nil {
		return CategoriesListResult{}, err
	}
	return CategoriesListResult{Categories: categories, Count: len(categories), TotalCount: total}, nil
}

func GetVersionForResource(ctx context.Context, client *api.Client, id int) (*models.Version, error) {
	return client.Versions.Get(ctx, id)
}
