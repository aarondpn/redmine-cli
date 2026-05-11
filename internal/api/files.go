package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// FileService handles project file API calls.
type FileService struct {
	client *Client
}

// List retrieves the file list for a project. The Redmine endpoint does not
// honour pagination parameters, so all files are returned in a single
// response. limit/offset are still accepted for client-side trimming via
// FetchAll's fallback path.
func (s *FileService) List(ctx context.Context, projectID string, limit, offset int) ([]models.ProjectFile, int, error) {
	path := fmt.Sprintf("/projects/%s/files.json", url.PathEscape(projectID))
	var params url.Values
	if offset > 0 {
		params = url.Values{}
		params.Set("offset", strconv.Itoa(offset))
	}
	return FetchAll[models.ProjectFile](ctx, s.client, path, params, "files", limit)
}

// Create uploads a file to a project using a previously obtained upload
// token. Redmine's POST /projects/:id/files.json returns 204 No Content on
// success and does not echo the created resource.
func (s *FileService) Create(ctx context.Context, projectID string, file models.ProjectFileCreate) error {
	body := map[string]interface{}{"file": file}
	path := fmt.Sprintf("/projects/%s/files.json", url.PathEscape(projectID))
	return s.client.Post(ctx, path, body, nil)
}
