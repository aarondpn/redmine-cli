package ops

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type UpdateAttachmentInput struct {
	ID          int     `json:"id" jsonschema:"Numeric attachment ID."`
	Filename    *string `json:"filename,omitempty" jsonschema:"New filename."`
	Description *string `json:"description,omitempty" jsonschema:"New description; an empty string clears it."`
	ContentType *string `json:"content_type,omitempty" jsonschema:"New MIME content type, e.g. image/png."`
}

type DeleteAttachmentInput struct {
	ID int `json:"id" jsonschema:"Numeric attachment ID to delete. Destructive."`
}

//mcpgen:tool update_attachment
//mcpgen:description Rename an attachment or change its description or content type. Requires --enable-writes.
//mcpgen:category issues
//mcpgen:writes
func UpdateAttachment(ctx context.Context, client *api.Client, input UpdateAttachmentInput) (MessageResult, error) {
	if input.ID <= 0 {
		return MessageResult{}, errors.New("id must be a positive attachment ID")
	}
	if input.Filename == nil && input.Description == nil && input.ContentType == nil {
		return MessageResult{}, errors.New("nothing to update: provide filename, description and/or content_type")
	}
	if input.Filename != nil && strings.TrimSpace(*input.Filename) == "" {
		return MessageResult{}, errors.New("filename must not be empty")
	}
	err := client.Attachments.Update(ctx, input.ID, models.AttachmentUpdate{
		Filename:    input.Filename,
		Description: input.Description,
		ContentType: input.ContentType,
	})
	if err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Updated attachment %d", input.ID)}, nil
}

//mcpgen:tool delete_attachment
//mcpgen:description Delete an attachment. Destructive. Requires --enable-writes.
//mcpgen:category issues
//mcpgen:writes
func DeleteAttachment(ctx context.Context, client *api.Client, input DeleteAttachmentInput) (MessageResult, error) {
	if input.ID <= 0 {
		return MessageResult{}, errors.New("id must be a positive attachment ID")
	}
	if err := client.Attachments.Delete(ctx, input.ID); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Deleted attachment %d", input.ID)}, nil
}

type GetAttachmentInput struct {
	ID int `json:"id" jsonschema:"Numeric attachment ID. Discover IDs via get_issue with includes=[\"attachments\"]."`
}

//mcpgen:tool get_attachment
//mcpgen:description Fetch metadata for a Redmine attachment by ID: filename, size, content type, description, author, and download URL. Discover attachment IDs via get_issue with includes=["attachments"].
//mcpgen:category issues
func GetAttachment(ctx context.Context, client *api.Client, input GetAttachmentInput) (*models.Attachment, error) {
	return client.Attachments.Get(ctx, input.ID)
}
