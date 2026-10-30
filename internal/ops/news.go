package ops

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// newsReadBackWindow is how many of the newest project news items are
// inspected to find the item that was just created.
const newsReadBackWindow = 25

// newsGetIncludes lists the include keys accepted by GET /news/:id.json.
var newsGetIncludes = map[string]struct{}{"attachments": {}, "comments": {}, "reactions": {}}

type ListNewsInput struct {
	ProjectID string `json:"project_id,omitempty" jsonschema:"Project identifier or numeric ID. Omit to list news across all visible projects."`
	Limit     int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset    int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type NewsListResult struct {
	News       []models.News `json:"news"`
	Count      int           `json:"count"`
	TotalCount int           `json:"total_count"`
}

type GetNewsInput struct {
	ID       int      `json:"id" jsonschema:"Numeric news ID (list_news to discover)."`
	Includes []string `json:"includes,omitempty" jsonschema:"Extra sections: attachments, comments, reactions (Redmine 7.1+)."`
}

type CreateNewsInput struct {
	ProjectID   string          `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Title       string          `json:"title" jsonschema:"News title (max 60 characters)."`
	Summary     string          `json:"summary,omitempty" jsonschema:"Short summary (max 255 characters)."`
	Description string          `json:"description" jsonschema:"News body (Textile or Markdown depending on the Redmine configuration)."`
	Uploads     []models.Upload `json:"-"`
}

type CreateNewsResult struct {
	ID       int      `json:"id,omitempty" jsonschema:"ID of the created news; omitted when it could not be determined."`
	Message  string   `json:"message"`
	Warnings []string `json:"-"`
}

// MCPMessage exposes the human-readable message used by the MCP wrapper.
func (r CreateNewsResult) MCPMessage() string {
	return MessageResult{Message: r.Message, Warnings: r.Warnings}.MCPMessage()
}

type UpdateNewsInput struct {
	ID          int             `json:"id" jsonschema:"Numeric news ID."`
	Title       *string         `json:"title,omitempty" jsonschema:"New title (max 60 characters)."`
	Summary     *string         `json:"summary,omitempty" jsonschema:"New summary (max 255 characters). Pass an empty string to clear it."`
	Description *string         `json:"description,omitempty" jsonschema:"New body."`
	Uploads     []models.Upload `json:"-"`
}

type DeleteNewsInput struct {
	ID int `json:"id" jsonschema:"Numeric news ID to delete. Destructive."`
}

//mcpgen:tool list_news
//mcpgen:description List news, for one project (project_id) or across all visible projects.
//mcpgen:category news
func ListNews(ctx context.Context, client *api.Client, input ListNewsInput) (NewsListResult, error) {
	items, total, err := client.News.List(ctx, input.ProjectID, ListLimit(input.Limit), input.Offset)
	if err != nil {
		return NewsListResult{}, err
	}
	return NewsListResult{News: items, Count: len(items), TotalCount: total}, nil
}

//mcpgen:tool get_news
//mcpgen:description Fetch a single news item by ID. Optional includes add attachments, comments and reactions (Redmine 7.1+).
//mcpgen:category news
func GetNews(ctx context.Context, client *api.Client, input GetNewsInput) (*models.News, error) {
	if input.ID <= 0 {
		return nil, errors.New("id must be a positive news ID")
	}
	for _, raw := range input.Includes {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		if _, ok := newsGetIncludes[v]; !ok {
			return nil, fmt.Errorf("unknown include %q (allowed: attachments, comments, reactions)", v)
		}
	}
	return client.News.Get(ctx, input.ID, input.Includes)
}

//mcpgen:tool create_news
//mcpgen:description Create a news item in a project. Redmine returns no body; the ID is looked up afterwards and omitted when it cannot be determined. Requires --enable-writes.
//mcpgen:category news
//mcpgen:writes
func CreateNews(ctx context.Context, client *api.Client, input CreateNewsInput) (CreateNewsResult, error) {
	if strings.TrimSpace(input.ProjectID) == "" {
		return CreateNewsResult{}, errors.New("project_id is required")
	}
	if strings.TrimSpace(input.Title) == "" {
		return CreateNewsResult{}, errors.New("title is required")
	}
	if strings.TrimSpace(input.Description) == "" {
		return CreateNewsResult{}, errors.New("description is required")
	}
	err := client.News.Create(ctx, input.ProjectID, models.NewsCreate{
		Title:       input.Title,
		Summary:     input.Summary,
		Description: input.Description,
		Uploads:     input.Uploads,
	})
	if err != nil {
		return CreateNewsResult{}, err
	}

	// Redmine answers 204 without a body, so look the new item up. This is
	// best effort: the news exists now, and an error here would invite a
	// retry that creates a duplicate.
	id, rbErr := findCreatedNews(ctx, client, input)
	if id > 0 {
		return CreateNewsResult{
			ID:      id,
			Message: fmt.Sprintf("Created news %q (ID: %d)", input.Title, id),
		}, nil
	}
	reason := "no matching news item found among the newest ones"
	if rbErr != nil {
		reason = rbErr.Error()
	}
	return CreateNewsResult{
		Message:  fmt.Sprintf("Created news %q", input.Title),
		Warnings: []string{"the new news ID could not be determined: " + reason},
	}, nil
}

// findCreatedNews returns the ID of the newest project news item that matches
// the created content and was authored by the current user, or 0.
func findCreatedNews(ctx context.Context, client *api.Client, input CreateNewsInput) (int, error) {
	me, err := client.Users.Current(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("could not look up the current user: %w", err)
	}
	items, _, err := client.News.List(ctx, input.ProjectID, newsReadBackWindow, 0)
	if err != nil {
		return 0, fmt.Errorf("could not list the project's news: %w", err)
	}
	for _, n := range items {
		if n.Title == input.Title && n.Summary == input.Summary && n.Description == input.Description && n.Author.ID == me.ID {
			return n.ID, nil
		}
	}
	return 0, nil
}

//mcpgen:tool update_news
//mcpgen:description Update a news item's title, summary or description. Requires --enable-writes.
//mcpgen:category news
//mcpgen:writes
func UpdateNews(ctx context.Context, client *api.Client, input UpdateNewsInput) (MessageResult, error) {
	if input.ID <= 0 {
		return MessageResult{}, errors.New("id must be a positive news ID")
	}
	if input.Title == nil && input.Summary == nil && input.Description == nil && len(input.Uploads) == 0 {
		return MessageResult{}, errors.New("nothing to update: provide title, summary, description and/or uploads")
	}
	if input.Title != nil && strings.TrimSpace(*input.Title) == "" {
		return MessageResult{}, errors.New("title must not be empty")
	}
	if input.Description != nil && strings.TrimSpace(*input.Description) == "" {
		return MessageResult{}, errors.New("description must not be empty")
	}
	err := client.News.Update(ctx, input.ID, models.NewsUpdate{
		Title:       input.Title,
		Summary:     input.Summary,
		Description: input.Description,
		Uploads:     input.Uploads,
	})
	if err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Updated news %d", input.ID)}, nil
}

//mcpgen:tool delete_news
//mcpgen:description Delete a news item. Destructive. Requires --enable-writes.
//mcpgen:category news
//mcpgen:writes
func DeleteNews(ctx context.Context, client *api.Client, input DeleteNewsInput) (MessageResult, error) {
	if input.ID <= 0 {
		return MessageResult{}, errors.New("id must be a positive news ID")
	}
	if err := client.News.Delete(ctx, input.ID); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Deleted news %d", input.ID)}, nil
}
