package ops

import (
	"context"
	"fmt"
	"slices"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// IssueRelationTypes are the valid relation_type values accepted by Redmine.
// Reference: https://www.redmine.org/projects/redmine/wiki/Rest_IssueRelations
var IssueRelationTypes = []string{
	"relates",
	"duplicates",
	"duplicated",
	"blocks",
	"blocked",
	"precedes",
	"follows",
	"copied_to",
	"copied_from",
}

// IssueRelationTypesSupportingDelay lists relation types that accept a delay
// value. Redmine rejects delay on any other type.
var IssueRelationTypesSupportingDelay = []string{"precedes", "follows"}

type ListIssueRelationsInput struct {
	IssueID int `json:"issue_id" jsonschema:"Issue ID to list relations for."`
}

type IssueRelationsResult struct {
	Relations []models.IssueRelation `json:"relations"`
	Count     int                    `json:"count"`
}

type GetIssueRelationInput struct {
	ID int `json:"id" jsonschema:"Relation ID."`
}

type CreateIssueRelationInput struct {
	IssueID      int    `json:"issue_id" jsonschema:"Source issue ID."`
	IssueToID    int    `json:"issue_to_id" jsonschema:"Target issue ID."`
	RelationType string `json:"relation_type,omitempty" jsonschema:"Relation type: relates, duplicates, duplicated, blocks, blocked, precedes, follows, copied_to, copied_from. Defaults to 'relates'."`
	Delay        *int   `json:"delay,omitempty" jsonschema:"Number of days delay. Only meaningful for relation types 'precedes' and 'follows'."`
}

type DeleteIssueRelationInput struct {
	ID int `json:"id" jsonschema:"Relation ID to delete."`
}

func validateRelationType(rt string) error {
	if rt == "" || slices.Contains(IssueRelationTypes, rt) {
		return nil
	}
	return fmt.Errorf("invalid relation_type %q (valid: %v)", rt, IssueRelationTypes)
}

//mcpgen:tool list_issue_relations
//mcpgen:description List relations attached to an issue.
//mcpgen:category issues
func ListIssueRelations(ctx context.Context, client *api.Client, input ListIssueRelationsInput) (IssueRelationsResult, error) {
	relations, err := client.Relations.ListByIssue(ctx, input.IssueID)
	if err != nil {
		return IssueRelationsResult{}, err
	}
	return IssueRelationsResult{Relations: relations, Count: len(relations)}, nil
}

//mcpgen:tool get_issue_relation
//mcpgen:description Fetch a single issue relation by its relation ID.
//mcpgen:category issues
func GetIssueRelation(ctx context.Context, client *api.Client, input GetIssueRelationInput) (*models.IssueRelation, error) {
	return client.Relations.Get(ctx, input.ID)
}

//mcpgen:tool create_issue_relation
//mcpgen:description Create a relation between two issues. Requires --enable-writes.
//mcpgen:category issues
//mcpgen:writes
func CreateIssueRelation(ctx context.Context, client *api.Client, input CreateIssueRelationInput) (*models.IssueRelation, error) {
	if input.IssueID <= 0 {
		return nil, fmt.Errorf("issue_id must be a positive issue ID")
	}
	if input.IssueToID <= 0 {
		return nil, fmt.Errorf("issue_to_id must be a positive issue ID")
	}
	if input.IssueID == input.IssueToID {
		return nil, fmt.Errorf("cannot relate an issue to itself")
	}
	if err := validateRelationType(input.RelationType); err != nil {
		return nil, err
	}
	if input.Delay != nil && input.RelationType != "" && !slices.Contains(IssueRelationTypesSupportingDelay, input.RelationType) {
		return nil, fmt.Errorf("delay is only valid for relation types %v", IssueRelationTypesSupportingDelay)
	}
	payload := models.IssueRelationCreate{
		IssueToID:    input.IssueToID,
		RelationType: input.RelationType,
		Delay:        input.Delay,
	}
	return client.Relations.Create(ctx, input.IssueID, payload)
}

//mcpgen:tool delete_issue_relation
//mcpgen:description Delete an issue relation by its relation ID. Requires --enable-writes.
//mcpgen:category issues
//mcpgen:writes
func DeleteIssueRelation(ctx context.Context, client *api.Client, input DeleteIssueRelationInput) (MessageResult, error) {
	if input.ID <= 0 {
		return MessageResult{}, fmt.Errorf("id must be a positive relation ID")
	}
	if err := client.Relations.Delete(ctx, input.ID); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Deleted relation #%d", input.ID)}, nil
}
