package ops

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type ListWikiPagesInput struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Limit     int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset    int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type WikiPagesListResult struct {
	Pages      []models.WikiPageIndex `json:"pages"`
	Count      int                    `json:"count"`
	TotalCount int                    `json:"total_count"`
}

type GetWikiPageInput struct {
	ProjectID string   `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Page      string   `json:"page" jsonschema:"Wiki page title (slug)."`
	Includes  []string `json:"includes,omitempty" jsonschema:"Extra sections to include, e.g. 'attachments'."`
}

type CreateWikiPageInput struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Page      string `json:"page" jsonschema:"Wiki page title (slug) to create or overwrite."`
	Text      string `json:"text" jsonschema:"Page body (Textile or Markdown depending on the Redmine configuration)."`
	Title     string `json:"title,omitempty" jsonschema:"Optional display title; may differ from the slug."`
	Comments  string `json:"comments,omitempty" jsonschema:"Edit comment."`
}

type UpdateWikiPageInput struct {
	ProjectID string  `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Page      string  `json:"page" jsonschema:"Wiki page title (slug) to update."`
	Text      *string `json:"text,omitempty" jsonschema:"New page body."`
	Title     *string `json:"title,omitempty" jsonschema:"New display title."`
	Comments  *string `json:"comments,omitempty" jsonschema:"Edit comment."`
}

type DeleteWikiPageInput struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Page      string `json:"page" jsonschema:"Wiki page title (slug) to delete. Destructive."`
}

//mcpgen:tool list_wiki_pages
//mcpgen:description List wiki pages for a project.
//mcpgen:category wiki
func ListWikiPages(ctx context.Context, client *api.Client, input ListWikiPagesInput) (WikiPagesListResult, error) {
	pages, total, err := client.Wikis.List(ctx, input.ProjectID, ListLimit(input.Limit), input.Offset)
	if err != nil {
		return WikiPagesListResult{}, err
	}
	return WikiPagesListResult{Pages: pages, Count: len(pages), TotalCount: total}, nil
}

//mcpgen:tool get_wiki_page
//mcpgen:description Fetch a single wiki page.
//mcpgen:category wiki
func GetWikiPage(ctx context.Context, client *api.Client, input GetWikiPageInput) (*models.WikiPage, error) {
	return client.Wikis.Get(ctx, input.ProjectID, input.Page, input.Includes)
}

//mcpgen:tool create_wiki_page
//mcpgen:description Create (or overwrite) a wiki page. Requires --enable-writes.
//mcpgen:category wiki
//mcpgen:writes
func CreateWikiPage(ctx context.Context, client *api.Client, input CreateWikiPageInput) (*models.WikiPage, error) {
	return client.Wikis.Create(ctx, input.ProjectID, input.Page, models.WikiPageCreate{
		Text:     input.Text,
		Title:    input.Title,
		Comments: input.Comments,
	})
}

//mcpgen:tool update_wiki_page
//mcpgen:description Update an existing wiki page. Requires --enable-writes.
//mcpgen:category wiki
//mcpgen:writes
func UpdateWikiPage(ctx context.Context, client *api.Client, input UpdateWikiPageInput) (MessageResult, error) {
	if err := client.Wikis.Update(ctx, input.ProjectID, input.Page, models.WikiPageUpdate{
		Text:     input.Text,
		Title:    input.Title,
		Comments: input.Comments,
	}); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Updated wiki page %s/%s", input.ProjectID, input.Page)}, nil
}

//mcpgen:tool delete_wiki_page
//mcpgen:description Delete a wiki page. Destructive. Requires --enable-writes.
//mcpgen:category wiki
//mcpgen:writes
func DeleteWikiPage(ctx context.Context, client *api.Client, input DeleteWikiPageInput) (MessageResult, error) {
	if err := client.Wikis.Delete(ctx, input.ProjectID, input.Page); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Deleted wiki page %s/%s", input.ProjectID, input.Page)}, nil
}
