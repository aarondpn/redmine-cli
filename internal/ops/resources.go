package ops

import (
	"context"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func GetCurrentUserForResource(ctx context.Context, client *api.Client) (*models.User, error) {
	return client.Users.Current(ctx)
}

func GetUserForResource(ctx context.Context, client *api.Client, id int) (*models.User, error) {
	return client.Users.Get(ctx, id)
}

func GetWikiPageForResource(ctx context.Context, client *api.Client, projectID, page string) (*models.WikiPage, error) {
	return client.Wikis.Get(ctx, projectID, page, []string{"attachments"})
}
