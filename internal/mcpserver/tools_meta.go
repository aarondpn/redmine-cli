package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
)

func registerMetaTools(s *mcp.Server, client *api.Client, opts Options) {
	registerToolSpec(s, client, opts, toolSpec[ops.ListVersionsInput, ops.VersionsListResult]{
		Name:        "list_versions",
		Description: "List versions (milestones) for a project.",
		Call:        ops.ListVersions,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.GetVersionInput, *models.Version]{
		Name:        "get_version",
		Description: "Fetch a single version (milestone) by ID.",
		Call:        ops.GetVersion,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.CreateVersionInput, *models.Version]{
		Name:        "create_version",
		Description: "Create a project version (milestone). Requires --enable-writes.",
		Writes:      true,
		Call:        ops.CreateVersion,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.UpdateVersionInput, ops.MessageResult]{
		Name:        "update_version",
		Description: "Update an existing version (milestone). Requires --enable-writes.",
		Writes:      true,
		Call:        ops.UpdateVersion,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.DeleteVersionInput, ops.MessageResult]{
		Name:        "delete_version",
		Description: "Delete a version (milestone). Destructive. Requires --enable-writes.",
		Writes:      true,
		Call:        ops.DeleteVersion,
	})
	registerToolSpec(s, client, opts, toolSpec[struct{}, ops.TrackersListResult]{
		Name:        "list_trackers",
		Description: "List all trackers (Bug, Feature, ...) configured in this Redmine instance.",
		Call:        ops.ListTrackers,
	})
	registerToolSpec(s, client, opts, toolSpec[struct{}, ops.StatusesListResult]{
		Name:        "list_statuses",
		Description: "List all issue statuses configured in this Redmine instance.",
		Call:        ops.ListStatuses,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.ListCategoriesInput, ops.CategoriesListResult]{
		Name:        "list_categories",
		Description: "List issue categories for a project.",
		Call:        ops.ListCategories,
	})
}
