package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
)

const (
	mimeJSON     = "application/json"
	mimeMarkdown = "text/markdown"
)

// registerResources wires the read-only URI templates. Resources are exposed
// regardless of EnableWrites because they do not mutate state.
func registerResources(s *mcp.Server, client *api.Client) {
	s.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: tmplIssue,
		Name:        "Redmine issue",
		Description: "A Redmine issue including journals, attachments, relations, children, and watchers.",
		MIMEType:    mimeJSON,
	}, handleIssueResource(client))

	s.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: tmplProject,
		Name:        "Redmine project",
		Description: "A Redmine project including trackers, categories, and enabled modules.",
		MIMEType:    mimeJSON,
	}, handleProjectResource(client))

	s.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: tmplUser,
		Name:        "Redmine user",
		Description: "A Redmine user. Use 'me' as the id to fetch the authenticated user.",
		MIMEType:    mimeJSON,
	}, handleUserResource(client))

	s.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: tmplTimeEntry,
		Name:        "Redmine time entry",
		Description: "A single Redmine time entry.",
		MIMEType:    mimeJSON,
	}, handleTimeEntryResource(client))

	s.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: tmplWiki,
		Name:        "Redmine wiki page",
		Description: "A Redmine wiki page rendered as Markdown.",
		MIMEType:    mimeMarkdown,
	}, handleWikiResource(client))

	s.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: tmplVersion,
		Name:        "Redmine version",
		Description: "A Redmine version (milestone).",
		MIMEType:    mimeJSON,
	}, handleVersionResource(client))
}

func handleIssueResource(client *api.Client) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		_, parts, err := parseRedmineURI(req.Params.URI)
		if err != nil {
			return nil, err
		}
		seg, err := expectSingleSegment(parts, "issue")
		if err != nil {
			return nil, err
		}
		id, err := parseIntID(seg)
		if err != nil {
			return nil, err
		}
		issue, err := client.Issues.Get(ctx, id, []string{"journals", "attachments", "relations", "children", "watchers"})
		if err != nil {
			return nil, resourceError(req.Params.URI, err)
		}
		return jsonContent(req.Params.URI, issue)
	}
}

func handleProjectResource(client *api.Client) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		_, parts, err := parseRedmineURI(req.Params.URI)
		if err != nil {
			return nil, err
		}
		ident, err := expectSingleSegment(parts, "project")
		if err != nil {
			return nil, err
		}
		project, err := client.Projects.Get(ctx, ident, []string{"trackers", "issue_categories", "enabled_modules"})
		if err != nil {
			return nil, resourceError(req.Params.URI, err)
		}
		return jsonContent(req.Params.URI, project)
	}
}

func handleUserResource(client *api.Client) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		_, parts, err := parseRedmineURI(req.Params.URI)
		if err != nil {
			return nil, err
		}
		seg, err := expectSingleSegment(parts, "user")
		if err != nil {
			return nil, err
		}
		if seg == "me" {
			user, err := client.Users.Current(ctx)
			if err != nil {
				return nil, resourceError(req.Params.URI, err)
			}
			return jsonContent(req.Params.URI, user)
		}
		id, err := parseIntID(seg)
		if err != nil {
			return nil, err
		}
		user, err := client.Users.Get(ctx, id)
		if err != nil {
			return nil, resourceError(req.Params.URI, err)
		}
		return jsonContent(req.Params.URI, user)
	}
}

func handleTimeEntryResource(client *api.Client) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		_, parts, err := parseRedmineURI(req.Params.URI)
		if err != nil {
			return nil, err
		}
		seg, err := expectSingleSegment(parts, "time-entry")
		if err != nil {
			return nil, err
		}
		id, err := parseIntID(seg)
		if err != nil {
			return nil, err
		}
		entry, err := client.TimeEntries.Get(ctx, id)
		if err != nil {
			return nil, resourceError(req.Params.URI, err)
		}
		return jsonContent(req.Params.URI, entry)
	}
}

func handleWikiResource(client *api.Client) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		_, parts, err := parseRedmineURI(req.Params.URI)
		if err != nil {
			return nil, err
		}
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("wiki URI must match %s", tmplWiki)
		}
		page, err := client.Wikis.Get(ctx, parts[0], parts[1], []string{"attachments"})
		if err != nil {
			return nil, resourceError(req.Params.URI, err)
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      req.Params.URI,
				MIMEType: mimeMarkdown,
				Text:     page.Text,
			}},
		}, nil
	}
}

func handleVersionResource(client *api.Client) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		_, parts, err := parseRedmineURI(req.Params.URI)
		if err != nil {
			return nil, err
		}
		seg, err := expectSingleSegment(parts, "version")
		if err != nil {
			return nil, err
		}
		id, err := parseIntID(seg)
		if err != nil {
			return nil, err
		}
		version, err := client.Versions.Get(ctx, id)
		if err != nil {
			return nil, resourceError(req.Params.URI, err)
		}
		return jsonContent(req.Params.URI, version)
	}
}

// jsonContent marshals v into a single JSON ResourceContents entry.
func jsonContent(uri string, v any) (*mcp.ReadResourceResult, error) {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding resource %s: %w", uri, err)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: mimeJSON,
			Text:     string(body),
		}},
	}, nil
}

// resourceError maps Redmine API errors to MCP-style resource errors so that
// a missing issue surfaces as ResourceNotFoundError rather than an opaque HTTP
// 404 message.
func resourceError(uri string, err error) error {
	var ae *api.APIError
	if errors.As(err, &ae) && ae.IsNotFound() {
		return mcp.ResourceNotFoundError(uri)
	}
	return fmt.Errorf("%s: %s", uri, describeAPIError(err))
}
