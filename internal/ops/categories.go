package ops

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type GetIssueCategoryInput struct {
	ID int `json:"id" jsonschema:"Numeric issue category ID (list_categories to discover)."`
}

type CreateIssueCategoryInput struct {
	ProjectID    string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Name         string `json:"name" jsonschema:"Category name (max 60 characters, unique within the project)."`
	AssignedToID int    `json:"assigned_to_id,omitempty" jsonschema:"Default assignee for new issues in this category: user or group ID."`
}

type UpdateIssueCategoryInput struct {
	ID           int     `json:"id" jsonschema:"Numeric issue category ID."`
	Name         *string `json:"name,omitempty" jsonschema:"New category name."`
	AssignedToID *int    `json:"assigned_to_id,omitempty" jsonschema:"New default assignee ID (user or group); 0 clears it."`
}

type DeleteIssueCategoryInput struct {
	ID           int `json:"id" jsonschema:"Numeric issue category ID to delete. Destructive."`
	ReassignToID int `json:"reassign_to_id,omitempty" jsonschema:"Move the category's issues to this category of the same project instead of leaving them uncategorized."`
}

//mcpgen:tool get_issue_category
//mcpgen:description Fetch a single issue category by ID.
//mcpgen:category meta
func GetIssueCategory(ctx context.Context, client *api.Client, input GetIssueCategoryInput) (*models.IssueCategory, error) {
	if input.ID <= 0 {
		return nil, errors.New("id must be a positive issue category ID")
	}
	return client.Categories.Get(ctx, input.ID)
}

//mcpgen:tool create_issue_category
//mcpgen:description Create an issue category in a project. Requires --enable-writes.
//mcpgen:category meta
//mcpgen:writes
func CreateIssueCategory(ctx context.Context, client *api.Client, input CreateIssueCategoryInput) (*models.IssueCategory, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, errors.New("name is required")
	}
	if strings.TrimSpace(input.ProjectID) == "" {
		return nil, errors.New("project_id is required")
	}
	if input.AssignedToID < 0 {
		return nil, fmt.Errorf("invalid assigned_to_id %d: must be a positive user or group ID", input.AssignedToID)
	}
	return client.Categories.Create(ctx, input.ProjectID, models.IssueCategoryCreate{
		Name:         input.Name,
		AssignedToID: input.AssignedToID,
	})
}

//mcpgen:tool update_issue_category
//mcpgen:description Update an issue category's name or default assignee (assigned_to_id 0 clears it). Requires --enable-writes.
//mcpgen:category meta
//mcpgen:writes
func UpdateIssueCategory(ctx context.Context, client *api.Client, input UpdateIssueCategoryInput) (MessageResult, error) {
	if input.ID <= 0 {
		return MessageResult{}, errors.New("id must be a positive issue category ID")
	}
	if input.Name == nil && input.AssignedToID == nil {
		return MessageResult{}, errors.New("nothing to update: provide name and/or assigned_to_id")
	}
	if input.Name != nil && strings.TrimSpace(*input.Name) == "" {
		return MessageResult{}, errors.New("name must not be empty")
	}
	if input.AssignedToID != nil && *input.AssignedToID < 0 {
		return MessageResult{}, fmt.Errorf("invalid assigned_to_id %d: use 0 to clear the default assignee", *input.AssignedToID)
	}
	err := client.Categories.Update(ctx, input.ID, models.IssueCategoryUpdate{
		Name:         input.Name,
		AssignedToID: input.AssignedToID,
	})
	if err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Updated issue category %d", input.ID)}, nil
}

//mcpgen:tool delete_issue_category
//mcpgen:description Delete an issue category. Its issues lose their category unless reassign_to_id names another category of the same project. Destructive. Requires --enable-writes.
//mcpgen:category meta
//mcpgen:writes
func DeleteIssueCategory(ctx context.Context, client *api.Client, input DeleteIssueCategoryInput) (MessageResult, error) {
	if input.ID <= 0 {
		return MessageResult{}, errors.New("id must be a positive issue category ID")
	}
	if input.ReassignToID < 0 {
		return MessageResult{}, fmt.Errorf("invalid reassign_to_id %d: must be a positive issue category ID", input.ReassignToID)
	}
	if input.ReassignToID == input.ID {
		return MessageResult{}, errors.New("reassign_to_id must differ from the category being deleted")
	}
	if input.ReassignToID > 0 {
		// Redmine silently ignores a reassign target that is unknown or lives in
		// another project and leaves the issues uncategorized, so check it here.
		src, err := client.Categories.Get(ctx, input.ID)
		if err != nil {
			return MessageResult{}, err
		}
		dst, err := client.Categories.Get(ctx, input.ReassignToID)
		if err != nil {
			var apiErr *api.APIError
			if errors.As(err, &apiErr) && apiErr.IsNotFound() {
				return MessageResult{}, fmt.Errorf("reassign target category %d not found", input.ReassignToID)
			}
			return MessageResult{}, fmt.Errorf("reassign_to_id %d: %w", input.ReassignToID, err)
		}
		if src.Project.ID != dst.Project.ID {
			return MessageResult{}, fmt.Errorf("reassign_to_id %d belongs to project %q, not %q: Redmine would leave the issues uncategorized", input.ReassignToID, dst.Project.Name, src.Project.Name)
		}
	}
	if err := client.Categories.Delete(ctx, input.ID, input.ReassignToID); err != nil {
		return MessageResult{}, err
	}
	msg := fmt.Sprintf("Deleted issue category %d", input.ID)
	if input.ReassignToID > 0 {
		msg += fmt.Sprintf(", issues moved to category %d", input.ReassignToID)
	}
	return MessageResult{Message: msg}, nil
}
