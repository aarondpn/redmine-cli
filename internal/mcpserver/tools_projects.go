package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
)

func registerProjectTools(s *mcp.Server, client *api.Client, opts Options) {
	registerToolSpec(s, client, opts, toolSpec[ops.ListProjectsInput, ops.ProjectsListResult]{
		Name:        "list_projects",
		Description: "List Redmine projects visible to the authenticated user.",
		Call:        ops.ListProjects,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.GetProjectInput, *models.Project]{
		Name:        "get_project",
		Description: "Fetch a single Redmine project by identifier or numeric ID.",
		Call:        ops.GetProject,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.ListProjectMembersInput, ops.ProjectMembersListResult]{
		Name:        "list_project_members",
		Description: "List memberships of a project (users and groups with roles).",
		Call:        ops.ListProjectMembers,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.CreateProjectInput, *models.Project]{
		Name:        "create_project",
		Description: "Create a new Redmine project. Requires --enable-writes.",
		Writes:      true,
		Call:        ops.CreateProject,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.UpdateProjectInput, ops.MessageResult]{
		Name:        "update_project",
		Description: "Update an existing Redmine project. Requires --enable-writes.",
		Writes:      true,
		Call:        ops.UpdateProject,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.DeleteProjectInput, ops.MessageResult]{
		Name:        "delete_project",
		Description: "Delete a Redmine project. Destructive. Requires --enable-writes.",
		Writes:      true,
		Call:        ops.DeleteProject,
	})
}
