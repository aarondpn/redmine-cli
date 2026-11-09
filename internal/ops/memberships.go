package ops

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

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
	UserID    int    `json:"user_id,omitempty" jsonschema:"Numeric ID of the user or group to add as a member. Set either user_id or user_ids."`
	UserIDs   []int  `json:"user_ids,omitempty" jsonschema:"Numeric IDs of several users or groups to add with the same roles in one request, instead of user_id. Every principal is checked afterwards; the result is the first membership, list_memberships shows the others."`
	RoleIDs   []int  `json:"role_ids" jsonschema:"One or more role IDs to grant. Use list_roles to discover valid values."`
}

type UpdateMembershipInput struct {
	ID      int   `json:"id" jsonschema:"Numeric membership ID."`
	RoleIDs []int `json:"role_ids" jsonschema:"Replacement set of role IDs. Use list_roles to discover valid values."`
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
//mcpgen:description Add a user or group, or several at once with user_ids, to a project with the given roles. Requires --enable-writes.
//mcpgen:category memberships
//mcpgen:writes
func CreateMembership(ctx context.Context, client *api.Client, input CreateMembershipInput) (*models.Membership, error) {
	memberships, err := CreateMemberships(ctx, client, input)
	if err != nil {
		return nil, err
	}
	return &memberships[0], nil
}

// CreateMemberships adds the principal in input.UserID, or every principal in
// input.UserIDs, to the project and returns their memberships in input
// order. Several principals go out in one request as membership[user_ids];
// Redmine answers (and validates) only the first of them, so the project's
// memberships are read back and a principal without one is reported as an
// error.
func CreateMemberships(ctx context.Context, client *api.Client, input CreateMembershipInput) ([]models.Membership, error) {
	ids := input.UserIDs
	switch {
	case input.UserID != 0 && len(ids) > 0:
		return nil, fmt.Errorf("set either user_id or user_ids, not both")
	case input.UserID != 0:
		ids = []int{input.UserID}
	case len(ids) == 0:
		return nil, fmt.Errorf("user_id or user_ids is required")
	}
	if len(ids) == 1 {
		m, err := client.Memberships.Create(ctx, input.ProjectID, models.MembershipCreate{UserID: ids[0], RoleIDs: input.RoleIDs})
		if err != nil {
			return nil, err
		}
		return []models.Membership{*m}, nil
	}

	_, createErr := client.Memberships.Create(ctx, input.ProjectID, models.MembershipCreate{UserIDs: ids, RoleIDs: input.RoleIDs})
	var apiErr *api.APIError
	if createErr != nil && (!errors.As(createErr, &apiErr) || !apiErr.IsValidationError()) {
		return nil, createErr
	}
	// A 422 only describes the first principal; the others may have been
	// added anyway, so the read-back runs in both cases.
	existing, _, err := client.Memberships.List(ctx, input.ProjectID, 0, 0)
	if err != nil {
		if createErr != nil {
			return nil, createErr
		}
		return nil, fmt.Errorf("read back memberships of project %s: %w", input.ProjectID, err)
	}
	byPrincipal := make(map[int]models.Membership, len(existing))
	for _, m := range existing {
		if p := m.Principal(); p != nil {
			byPrincipal[p.ID] = m
		}
	}
	found := make([]models.Membership, 0, len(ids))
	var missing []string
	for _, id := range ids {
		if m, ok := byPrincipal[id]; ok {
			found = append(found, m)
		} else {
			missing = append(missing, strconv.Itoa(id))
		}
	}
	switch {
	case len(missing) > 0 && createErr != nil:
		// Keep the 422 (callers map it to validation_failed) and add the
		// principals it does not mention.
		combined := *apiErr
		combined.Errors = append(append([]string(nil), apiErr.Errors...),
			fmt.Sprintf("principal IDs %s were not added to project %s", strings.Join(missing, ", "), input.ProjectID))
		return nil, &combined
	case len(missing) > 0:
		return nil, fmt.Errorf("principal IDs %s were not added to project %s (Redmine reports errors only for the first principal of a request; add the missing ones one at a time to see why)", strings.Join(missing, ", "), input.ProjectID)
	case createErr != nil:
		return nil, createErr
	}
	return found, nil
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
