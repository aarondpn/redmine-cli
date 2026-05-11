package ops

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type ListProjectFilesInput struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Limit     int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset    int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type ProjectFilesListResult struct {
	Files      []models.ProjectFile `json:"files"`
	Count      int                  `json:"count"`
	TotalCount int                  `json:"total_count"`
}

type UploadProjectFileInput struct {
	ProjectID   string `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Token       string `json:"token" jsonschema:"Upload token returned by the /uploads.json endpoint."`
	Filename    string `json:"filename,omitempty" jsonschema:"File name shown in Redmine. Defaults to the uploaded file's name."`
	VersionID   int    `json:"version_id,omitempty" jsonschema:"Optional version (milestone) to attach the file to."`
	Description string `json:"description,omitempty" jsonschema:"Optional human-readable description."`
	ContentType string `json:"content_type,omitempty" jsonschema:"Optional MIME type override."`
}

//mcpgen:tool list_project_files
//mcpgen:description List files attached to a project.
//mcpgen:category projects
func ListProjectFiles(ctx context.Context, client *api.Client, input ListProjectFilesInput) (ProjectFilesListResult, error) {
	files, total, err := client.Files.List(ctx, input.ProjectID, ListLimit(input.Limit), input.Offset)
	if err != nil {
		return ProjectFilesListResult{}, err
	}
	return ProjectFilesListResult{Files: files, Count: len(files), TotalCount: total}, nil
}

// UploadProjectFile attaches a previously uploaded file (via the /uploads.json
// flow) to a project. The Redmine API returns no body on success; callers can
// re-list the project's files to surface the resulting record.
func UploadProjectFile(ctx context.Context, client *api.Client, input UploadProjectFileInput) (MessageResult, error) {
	if input.Token == "" {
		return MessageResult{}, fmt.Errorf("token is required")
	}
	if err := client.Files.Create(ctx, input.ProjectID, models.ProjectFileCreate{
		Token:       input.Token,
		Filename:    input.Filename,
		VersionID:   input.VersionID,
		Description: input.Description,
		ContentType: input.ContentType,
	}); err != nil {
		return MessageResult{}, err
	}
	name := input.Filename
	if name == "" {
		name = "file"
	}
	return MessageResult{Message: fmt.Sprintf("Uploaded %s to project %s", name, input.ProjectID)}, nil
}
