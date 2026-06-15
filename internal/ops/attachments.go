package ops

import (
	"context"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type GetAttachmentInput struct {
	ID int `json:"id" jsonschema:"Numeric attachment ID. Discover IDs via get_issue with includes=[\"attachments\"]."`
}

//mcpgen:tool get_attachment
//mcpgen:description Fetch metadata for a Redmine attachment by ID: filename, size, content type, description, author, and download URL. Discover attachment IDs via get_issue with includes=["attachments"].
//mcpgen:category issues
func GetAttachment(ctx context.Context, client *api.Client, input GetAttachmentInput) (*models.Attachment, error) {
	return client.Attachments.Get(ctx, input.ID)
}
