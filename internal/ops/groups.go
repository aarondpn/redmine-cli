package ops

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type ListGroupsInput struct {
	Limit  int `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset int `json:"offset,omitempty" jsonschema:"Number of leading results to skip (pagination)."`
}

type GroupsListResult struct {
	Groups     []models.Group `json:"groups"`
	Count      int            `json:"count"`
	TotalCount int            `json:"total_count"`
}

type GetGroupInput struct {
	ID       int      `json:"id" jsonschema:"Numeric group ID."`
	Includes []string `json:"includes,omitempty" jsonschema:"Extra sections to include: 'users', 'memberships'."`
}

type CreateGroupInput struct {
	Name    string `json:"name" jsonschema:"Group name."`
	UserIDs []int  `json:"user_ids,omitempty" jsonschema:"Optional list of user IDs to add as group members."`
}

type UpdateGroupInput struct {
	ID      int     `json:"id" jsonschema:"Group ID to update."`
	Name    *string `json:"name,omitempty" jsonschema:"New group name."`
	UserIDs *[]int  `json:"user_ids,omitempty" jsonschema:"Replacement set of user IDs. Pass an empty list to remove all members."`
}

type DeleteGroupInput struct {
	ID int `json:"id" jsonschema:"Group ID to delete. Destructive."`
}

type GroupUserInput struct {
	GroupID int `json:"group_id" jsonschema:"Group ID."`
	UserID  int `json:"user_id" jsonschema:"User ID."`
}

//mcpgen:tool list_groups
//mcpgen:description List Redmine groups.
//mcpgen:category groups
func ListGroups(ctx context.Context, client *api.Client, input ListGroupsInput) (GroupsListResult, error) {
	groups, total, err := client.Groups.List(ctx, models.GroupFilter{
		Limit:  ListLimit(input.Limit),
		Offset: input.Offset,
	})
	if err != nil {
		return GroupsListResult{}, err
	}
	return GroupsListResult{Groups: groups, Count: len(groups), TotalCount: total}, nil
}

//mcpgen:tool get_group
//mcpgen:description Fetch a single Redmine group by ID.
//mcpgen:category groups
func GetGroup(ctx context.Context, client *api.Client, input GetGroupInput) (*models.Group, error) {
	return client.Groups.Get(ctx, input.ID, input.Includes)
}

//mcpgen:tool create_group
//mcpgen:description Create a new Redmine group. Requires --enable-writes.
//mcpgen:category groups
//mcpgen:writes
func CreateGroup(ctx context.Context, client *api.Client, input CreateGroupInput) (*models.Group, error) {
	return client.Groups.Create(ctx, models.GroupCreate{
		Name:    input.Name,
		UserIDs: input.UserIDs,
	})
}

//mcpgen:tool update_group
//mcpgen:description Update an existing Redmine group. Requires --enable-writes.
//mcpgen:category groups
//mcpgen:writes
func UpdateGroup(ctx context.Context, client *api.Client, input UpdateGroupInput) (MessageResult, error) {
	if err := client.Groups.Update(ctx, input.ID, models.GroupUpdate{
		Name:    input.Name,
		UserIDs: input.UserIDs,
	}); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Updated group %d", input.ID)}, nil
}

//mcpgen:tool delete_group
//mcpgen:description Delete a Redmine group. Destructive. Requires --enable-writes.
//mcpgen:category groups
//mcpgen:writes
func DeleteGroup(ctx context.Context, client *api.Client, input DeleteGroupInput) (MessageResult, error) {
	if err := client.Groups.Delete(ctx, input.ID); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Deleted group %d", input.ID)}, nil
}

//mcpgen:tool add_group_user
//mcpgen:description Add a user to a Redmine group. Requires --enable-writes.
//mcpgen:category groups
//mcpgen:writes
func AddGroupUser(ctx context.Context, client *api.Client, input GroupUserInput) (MessageResult, error) {
	if err := client.Groups.AddUser(ctx, input.GroupID, input.UserID); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Added user %d to group %d", input.UserID, input.GroupID)}, nil
}

//mcpgen:tool remove_group_user
//mcpgen:description Remove a user from a Redmine group. Requires --enable-writes.
//mcpgen:category groups
//mcpgen:writes
func RemoveGroupUser(ctx context.Context, client *api.Client, input GroupUserInput) (MessageResult, error) {
	if err := client.Groups.RemoveUser(ctx, input.GroupID, input.UserID); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Removed user %d from group %d", input.UserID, input.GroupID)}, nil
}
