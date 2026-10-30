package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// NewsService handles news-related API calls.
type NewsService struct {
	client *Client
}

// List retrieves news, newest first. With an empty projectID it lists the
// news of all visible projects. If limit is 0, all news are fetched.
func (s *NewsService) List(ctx context.Context, projectID string, limit, offset int) ([]models.News, int, error) {
	path := "/news.json"
	if projectID != "" {
		seg, err := projectSegment(projectID)
		if err != nil {
			return nil, 0, err
		}
		path = fmt.Sprintf("/projects/%s/news.json", seg)
	}
	var params url.Values
	if offset > 0 {
		params = url.Values{}
		params.Set("offset", strconv.Itoa(offset))
	}
	return FetchAll[models.News](ctx, s.client, path, params, "news", limit)
}

// Get retrieves a single news item. Valid includes are attachments, comments
// and reactions (Redmine 7.1+).
func (s *NewsService) Get(ctx context.Context, id int, includes []string) (*models.News, error) {
	var params url.Values
	if len(includes) > 0 {
		params = url.Values{}
		params.Set("include", joinStrings(includes, ","))
	}
	var resp struct {
		News models.News `json:"news"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/news/%d.json", id), params, &resp); err != nil {
		return nil, err
	}
	return &resp.News, nil
}

// Create creates a news item in a project. Redmine answers 204 No Content, so
// the new ID is not returned.
func (s *NewsService) Create(ctx context.Context, projectID string, n models.NewsCreate) error {
	seg, err := projectSegment(projectID)
	if err != nil {
		return err
	}
	body := map[string]interface{}{"news": n}
	return s.client.Post(ctx, fmt.Sprintf("/projects/%s/news.json", seg), body, nil)
}

// Update updates a news item.
func (s *NewsService) Update(ctx context.Context, id int, u models.NewsUpdate) error {
	body := map[string]interface{}{"news": u}
	return s.client.Put(ctx, fmt.Sprintf("/news/%d.json", id), body)
}

// Delete deletes a news item.
func (s *NewsService) Delete(ctx context.Context, id int) error {
	return s.client.Delete(ctx, fmt.Sprintf("/news/%d.json", id))
}
