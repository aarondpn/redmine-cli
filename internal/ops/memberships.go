package ops

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type ListMembershipsInput struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Limit     int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset    int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type MembershipsListResult struct {
	Memberships []models.Membership `json:"memberships"`
	Count       int                 `json:"count"`
	TotalCount  int                 `json:"total_count"`
}

type GetMembershipInput struct {
	ID int `json:"id" jsonschema:"Numeric membership ID."`
}

type CreateMembershipInput struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	UserID    int    `json:"user_id" jsonschema:"Numeric user ID to add as a member."`
	RoleIDs   []int  `json:"role_ids" jsonschema:"One or more role IDs to grant."`
}

type UpdateMembershipInput struct {
	ID      int   `json:"id" jsonschema:"Numeric membership ID."`
	RoleIDs []int `json:"role_ids" jsonschema:"Replacement set of role IDs."`
}

type DeleteMembershipInput struct {
	ID int `json:"id" jsonschema:"Numeric membership ID to delete. Destructive."`
}

//mcpgen:tool list_memberships
//mcpgen:description List memberships for a project.
//mcpgen:category memberships
func ListMemberships(ctx context.Context, client *api.Client, input ListMembershipsInput) (MembershipsListResult, error) {
	members, total, err := client.Memberships.List(ctx, input.ProjectID, ListLimit(input.Limit), input.Offset)
	if err != nil {
		return MembershipsListResult{}, err
	}
	return MembershipsListResult{Memberships: members, Count: len(members), TotalCount: total}, nil
}

//mcpgen:tool get_membership
//mcpgen:description Fetch a single project membership by ID.
//mcpgen:category memberships
func GetMembership(ctx context.Context, client *api.Client, input GetMembershipInput) (*models.Membership, error) {
	return client.Memberships.Get(ctx, input.ID)
}

//mcpgen:tool create_membership
//mcpgen:description Add a user to a project with the given roles. Requires --enable-writes.
//mcpgen:category memberships
//mcpgen:writes
func CreateMembership(ctx context.Context, client *api.Client, input CreateMembershipInput) (*models.Membership, error) {
	return client.Memberships.Create(ctx, input.ProjectID, models.MembershipCreate{
		UserID:  input.UserID,
		RoleIDs: input.RoleIDs,
	})
}

//mcpgen:tool update_membership
//mcpgen:description Replace the roles on a project membership. Requires --enable-writes.
//mcpgen:category memberships
//mcpgen:writes
func UpdateMembership(ctx context.Context, client *api.Client, input UpdateMembershipInput) (MessageResult, error) {
	if err := client.Memberships.Update(ctx, input.ID, models.MembershipUpdate{RoleIDs: input.RoleIDs}); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Updated membership %d", input.ID)}, nil
}

//mcpgen:tool delete_membership
//mcpgen:description Remove a project membership. Destructive. Requires --enable-writes.
//mcpgen:category memberships
//mcpgen:writes
func DeleteMembership(ctx context.Context, client *api.Client, input DeleteMembershipInput) (MessageResult, error) {
	if err := client.Memberships.Delete(ctx, input.ID); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Deleted membership %d", input.ID)}, nil
}
