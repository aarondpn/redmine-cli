package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
)

const (
	mimeJSON = "application/json"
	// mimeWikiText is used for wiki page bodies. Redmine's default text
	// formatter is Textile and CommonMark is opt-in per instance, so the
	// server cannot safely advertise a specific markup dialect; text/plain
	// keeps hosts from mis-rendering Textile as Markdown.
	mimeWikiText = "text/plain"
)

type resourceTemplateDefinition struct {
	Template *mcp.ResourceTemplate
	Read     func(context.Context, *api.Client, string) (*mcp.ReadResourceResult, error)
}

// registerResources wires the read-only URI templates. Resources are exposed
// regardless of EnableWrites because they do not mutate state.
func registerResources(s *mcp.Server, client *api.Client) {
	for _, def := range resourceTemplateDefinitions() {
		definition := def
		s.AddResourceTemplate(definition.Template, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return definition.Read(ctx, client, req.Params.URI)
		})
	}
}

func resourceTemplateDefinitions() []resourceTemplateDefinition {
	return []resourceTemplateDefinition{
		{
			Template: &mcp.ResourceTemplate{
				URITemplate: tmplIssue,
				Name:        "Redmine issue",
				Description: "A Redmine issue including journals, attachments, relations, children, and watchers.",
				MIMEType:    mimeJSON,
			},
			Read: readIssueResource,
		},
		{
			Template: &mcp.ResourceTemplate{
				URITemplate: tmplProject,
				Name:        "Redmine project",
				Description: "A Redmine project including trackers, categories, and enabled modules.",
				MIMEType:    mimeJSON,
			},
			Read: readProjectResource,
		},
		{
			Template: &mcp.ResourceTemplate{
				URITemplate: tmplUser,
				Name:        "Redmine user",
				Description: "A Redmine user. Use 'me' as the id to fetch the authenticated user.",
				MIMEType:    mimeJSON,
			},
			Read: readUserResource,
		},
		{
			Template: &mcp.ResourceTemplate{
				URITemplate: tmplTimeEntry,
				Name:        "Redmine time entry",
				Description: "A single Redmine time entry.",
				MIMEType:    mimeJSON,
			},
			Read: readTimeEntryResource,
		},
		{
			Template: &mcp.ResourceTemplate{
				URITemplate: tmplWiki,
				Name:        "Redmine wiki page",
				Description: "A Redmine wiki page body. Markup is whatever the Redmine instance is configured to use (Textile by default, CommonMark optional).",
				MIMEType:    mimeWikiText,
			},
			Read: readWikiResource,
		},
		{
			Template: &mcp.ResourceTemplate{
				URITemplate: tmplVersion,
				Name:        "Redmine version",
				Description: "A Redmine version (milestone).",
				MIMEType:    mimeJSON,
			},
			Read: readVersionResource,
		},
	}
}

func readIssueResource(ctx context.Context, client *api.Client, uri string) (*mcp.ReadResourceResult, error) {
	_, parts, err := parseRedmineURI(uri)
	if err != nil {
		return nil, err
	}
	segment, err := expectSingleSegment(parts, "issue")
	if err != nil {
		return nil, err
	}
	id, err := parseIntID(segment)
	if err != nil {
		return nil, err
	}
	issue, err := ops.GetIssueForResource(ctx, client, id)
	if err != nil {
		return nil, resourceError(uri, err)
	}
	return jsonContent(uri, issue)
}

func readProjectResource(ctx context.Context, client *api.Client, uri string) (*mcp.ReadResourceResult, error) {
	_, parts, err := parseRedmineURI(uri)
	if err != nil {
		return nil, err
	}
	identifier, err := expectSingleSegment(parts, "project")
	if err != nil {
		return nil, err
	}
	project, err := ops.GetProjectForResource(ctx, client, identifier)
	if err != nil {
		return nil, resourceError(uri, err)
	}
	return jsonContent(uri, project)
}

func readUserResource(ctx context.Context, client *api.Client, uri string) (*mcp.ReadResourceResult, error) {
	_, parts, err := parseRedmineURI(uri)
	if err != nil {
		return nil, err
	}
	segment, err := expectSingleSegment(parts, "user")
	if err != nil {
		return nil, err
	}
	if segment == "me" {
		user, err := ops.GetCurrentUserForResource(ctx, client)
		if err != nil {
			return nil, resourceError(uri, err)
		}
		return jsonContent(uri, user)
	}
	id, err := parseIntID(segment)
	if err != nil {
		return nil, err
	}
	user, err := ops.GetUserForResource(ctx, client, id)
	if err != nil {
		return nil, resourceError(uri, err)
	}
	return jsonContent(uri, user)
}

func readTimeEntryResource(ctx context.Context, client *api.Client, uri string) (*mcp.ReadResourceResult, error) {
	_, parts, err := parseRedmineURI(uri)
	if err != nil {
		return nil, err
	}
	segment, err := expectSingleSegment(parts, "time-entry")
	if err != nil {
		return nil, err
	}
	id, err := parseIntID(segment)
	if err != nil {
		return nil, err
	}
	entry, err := ops.GetTimeEntryForResource(ctx, client, id)
	if err != nil {
		return nil, resourceError(uri, err)
	}
	return jsonContent(uri, entry)
}

func readWikiResource(ctx context.Context, client *api.Client, uri string) (*mcp.ReadResourceResult, error) {
	_, parts, err := parseRedmineURI(uri)
	if err != nil {
		return nil, err
	}
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("wiki URI must match %s", tmplWiki)
	}
	page, err := ops.GetWikiPageForResource(ctx, client, parts[0], parts[1])
	if err != nil {
		return nil, resourceError(uri, err)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: mimeWikiText,
			Text:     page.Text,
		}},
	}, nil
}

func readVersionResource(ctx context.Context, client *api.Client, uri string) (*mcp.ReadResourceResult, error) {
	_, parts, err := parseRedmineURI(uri)
	if err != nil {
		return nil, err
	}
	segment, err := expectSingleSegment(parts, "version")
	if err != nil {
		return nil, err
	}
	id, err := parseIntID(segment)
	if err != nil {
		return nil, err
	}
	version, err := ops.GetVersionForResource(ctx, client, id)
	if err != nil {
		return nil, resourceError(uri, err)
	}
	return jsonContent(uri, version)
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
