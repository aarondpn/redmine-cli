package ops

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

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
	ProjectID   string          `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Page        string          `json:"page" jsonschema:"Wiki page title (slug) to create or overwrite."`
	Text        string          `json:"text" jsonschema:"Page body (Textile or Markdown depending on the Redmine configuration)."`
	Title       string          `json:"title,omitempty" jsonschema:"Optional page title to store the page under instead of page; Redmine uses it as the URL slug."`
	ParentTitle string          `json:"parent_title,omitempty" jsonschema:"Title of an existing wiki page in the same project to nest this page under."`
	Comments    string          `json:"comments,omitempty" jsonschema:"Edit comment."`
	Uploads     []models.Upload `json:"-"`
}

type UpdateWikiPageInput struct {
	ProjectID   string          `json:"project_id" jsonschema:"Project identifier or numeric ID."`
	Page        string          `json:"page" jsonschema:"Wiki page title (slug) to update."`
	Text        *string         `json:"text,omitempty" jsonschema:"New page body, or new section content when section is set. When omitted, the current body is kept."`
	Title       *string         `json:"title,omitempty" jsonschema:"New page title. Renames the page; Redmine keeps a redirect from the old title. Requires the rename_wiki_pages permission."`
	ParentTitle *string         `json:"parent_title,omitempty" jsonschema:"Title of an existing wiki page in the same project to move this page under. Pass an empty string to move the page to the wiki root. Requires the rename_wiki_pages permission."`
	Comments    *string         `json:"comments,omitempty" jsonschema:"Edit comment."`
	Version     *int            `json:"version,omitempty" jsonschema:"Expected current page version. When set, Redmine returns 409 Conflict if the stored version does not match, giving optimistic-locking semantics."`
	Uploads     []models.Upload `json:"-"`
	Section     *int            `json:"section,omitempty" jsonschema:"Section number (1-based) to update instead of the full page."`
	SectionHash *string         `json:"section_hash,omitempty" jsonschema:"Hash of the original section content, used by Redmine for conflict detection on section edits."`
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
	page, err := client.Wikis.Create(ctx, input.ProjectID, input.Page, models.WikiPageCreate{
		Text:        input.Text,
		Title:       input.Title,
		ParentTitle: input.ParentTitle,
		Comments:    input.Comments,
		Uploads:     input.Uploads,
	})
	if err != nil {
		return nil, annotateWikiParentError(err, input.ParentTitle, input.Title)
	}
	// Creating over an existing page is an update as far as Redmine is
	// concerned, so the parent can be silently dropped here too. The returned
	// page is the stored one (the create response, or the api layer's
	// read-back after a 204 overwrite), so no further read-back is needed.
	// The result has no warnings channel; an inconclusive check returns the
	// page as stored, whose parent field shows what Redmine kept.
	if strings.TrimSpace(input.ParentTitle) != "" {
		if _, err := verifyWikiParent(ctx, client, input.ProjectID, page, input.ParentTitle, wikiParentCheck{
			otherChanges: true,
			droppedTitle: droppedWikiTitle(page, input.Title),
		}); err != nil {
			return nil, err
		}
	}
	return page, nil
}

//mcpgen:tool update_wiki_page
//mcpgen:description Update an existing wiki page. Requires --enable-writes.
//mcpgen:category wiki
//mcpgen:writes
func UpdateWikiPage(ctx context.Context, client *api.Client, input UpdateWikiPageInput) (MessageResult, error) {
	if input.Version != nil && *input.Version < 1 {
		return MessageResult{}, fmt.Errorf("version must be >= 1 when asserting optimistic concurrency")
	}
	if input.Section != nil {
		if *input.Section < 1 {
			return MessageResult{}, fmt.Errorf("section must be >= 1")
		}
		if input.Text == nil {
			return MessageResult{}, fmt.Errorf("section update requires text")
		}
	}
	if input.SectionHash != nil && input.Section == nil {
		return MessageResult{}, fmt.Errorf("section_hash requires section")
	}
	// A rename is dropped under the same rule as a parent change, so it never
	// counts as a change that may have landed without the parent.
	otherChanges := input.Text != nil || input.Comments != nil || len(input.Uploads) > 0
	if input.Text == nil {
		// Redmine requires the text on every update and answers 422 "Text
		// cannot be blank" without it, so a title, comment or parent change
		// alone would fail. Resend the current text unchanged.
		current, err := client.Wikis.Get(ctx, input.ProjectID, input.Page, nil)
		if err != nil {
			return MessageResult{}, fmt.Errorf("failed to fetch current text of wiki page %q: %w", input.Page, err)
		}
		// Redmine only checks the version when the text changes, which an
		// unchanged resend never does, so assert it here instead.
		if input.Version != nil && *input.Version != current.Version {
			return MessageResult{}, api.NewAPIError(http.StatusConflict, "",
				[]string{fmt.Sprintf("wiki page %q has been modified since version %d; it is now at version %d",
					input.Page, *input.Version, current.Version)})
		}
		// Pin the resend to the fetched version so an edit landing in
		// between makes Redmine answer 409 instead of being overwritten
		// with the stale text.
		v := current.Version
		input.Version = &v
		input.Text = &current.Text
	}
	if err := client.Wikis.Update(ctx, input.ProjectID, input.Page, models.WikiPageUpdate{
		Text:        input.Text,
		Title:       input.Title,
		ParentTitle: input.ParentTitle,
		Comments:    input.Comments,
		Version:     input.Version,
		Uploads:     input.Uploads,
		Section:     input.Section,
		SectionHash: input.SectionHash,
	}); err != nil {
		if input.ParentTitle != nil {
			title := ""
			if input.Title != nil {
				title = *input.Title
			}
			err = annotateWikiParentError(err, *input.ParentTitle, title)
		}
		return MessageResult{}, err
	}
	result := MessageResult{Message: fmt.Sprintf("Updated wiki page %s/%s", input.ProjectID, input.Page)}
	if input.ParentTitle != nil {
		// Redmine answers 204 No Content to the update, so read the page back
		// to confirm the parent change landed.
		page, droppedTitle, err := readBackWikiPage(ctx, client, input)
		if err != nil {
			// The update itself went through; only the check failed.
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("could not verify that wiki page %q's parent was changed: %v", input.Page, err))
			return result, nil
		}
		note, err := verifyWikiParent(ctx, client, input.ProjectID, page, *input.ParentTitle, wikiParentCheck{
			otherChanges: otherChanges,
			droppedTitle: droppedTitle,
		})
		if err != nil {
			return MessageResult{}, err
		}
		if note != "" {
			result.Warnings = append(result.Warnings, note)
		}
	}
	return result, nil
}

// readBackWikiPage fetches the page an update just saved. Redmine drops a
// rename under the same permission rule as a parent change, so with a rename
// in the request the page is looked up under its old title first: if it is
// still there, the rename was dropped and its title is returned. Otherwise
// (the old title now redirects, which the API cannot follow) the page moved
// and is read under its new title. Reading the new title first could match
// an unrelated page that already had that title.
func readBackWikiPage(ctx context.Context, client *api.Client, input UpdateWikiPageInput) (*models.WikiPage, string, error) {
	page, err := client.Wikis.Get(ctx, input.ProjectID, input.Page, nil)
	if input.Title == nil || *input.Title == "" {
		return page, "", err
	}
	if err == nil {
		return page, droppedWikiTitle(page, *input.Title), nil
	}
	page, err = client.Wikis.Get(ctx, input.ProjectID, *input.Title, nil)
	return page, "", err
}

// droppedWikiTitle returns title when page, read back after a rename to
// title, is not stored under it. A rename that only changes letter case is
// indistinguishable from no rename and counts as applied.
func droppedWikiTitle(page *models.WikiPage, title string) string {
	if title == "" || strings.EqualFold(page.Title, wikiTitleize(title)) {
		return ""
	}
	return title
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

// WikiParentNotAppliedError reports that Redmine saved a wiki page but left
// its parent unchanged.
//
// Redmine only applies parent_title to an existing page when the user has the
// rename_wiki_pages permission. Without it the field is dropped and the
// request still succeeds (204 No Content on update, or the stored page on a
// create that overwrote an existing page), so the only way to notice is to
// compare the stored parent with the requested one, which UpdateWikiPage and
// CreateWikiPage do whenever a parent was requested.
type WikiParentNotAppliedError struct {
	Page string
	// Requested and Current are parent page titles; empty means the page is
	// (or was meant to be) at the wiki root.
	Requested string
	Current   string
	// DroppedTitle is the new title the same request asked for, when Redmine
	// dropped that rename too (it needs the same permission).
	DroppedTitle string
	// OtherChanges reports whether the same request carried changes besides
	// the parent (text, comments, attachments). Redmine applies those even
	// when it drops the parent, so callers must not blindly resend them.
	OtherChanges bool
}

func (e *WikiParentNotAppliedError) Error() string {
	if details := e.Details(); len(details) > 0 {
		return e.Summary() + " " + strings.Join(details, " ")
	}
	return e.Summary()
}

// Summary is the one-paragraph explanation of what happened, without the
// follow-up facts returned by Details.
func (e *WikiParentNotAppliedError) Summary() string {
	return fmt.Sprintf("Wiki page %q: parent was not changed to %s; it is now %s. "+
		"Redmine silently ignores parent changes on existing pages unless you have the rename_wiki_pages permission.",
		e.Page, describeWikiParent(e.Requested), describeWikiParent(e.Current))
}

// Details lists the follow-up facts a caller needs to recover: a rename that
// was dropped as well and, when relevant, that the rest of the request may
// already have been applied.
func (e *WikiParentNotAppliedError) Details() []string {
	var details []string
	if e.DroppedTitle != "" {
		details = append(details, fmt.Sprintf("The rename to %q was not applied either; it needs the same permission.", e.DroppedTitle))
	}
	if e.OtherChanges {
		details = append(details, "The other changes in this request (text, comments, attachments) may already have been applied; do not resend them blindly.")
	}
	return details
}

// wikiParentCheck carries the request context verifyWikiParent reports.
type wikiParentCheck struct {
	otherChanges bool
	droppedTitle string
}

// verifyWikiParent compares page's stored parent with want ("" or blank for
// the wiki root). It returns *WikiParentNotAppliedError when Redmine kept a
// different parent, and an informational note when the outcome cannot be
// determined or a rename in the same request was dropped.
func verifyWikiParent(ctx context.Context, client *api.Client, projectID string, page *models.WikiPage, want string, check wikiParentCheck) (string, error) {
	got := ""
	if page.Parent != nil {
		got = page.Parent.Title
	}
	if wikiParentMatches(want, got) {
		if check.droppedTitle != "" {
			return fmt.Sprintf("Wiki page %q was not renamed to %q; renaming requires the rename_wiki_pages permission.",
				page.Title, check.droppedTitle), nil
		}
		return "", nil
	}
	// A dropped rename proves the parent was dropped too: both need the same
	// permission. Otherwise check what want resolves to before blaming it.
	if strings.TrimSpace(want) != "" && check.droppedTitle == "" {
		// Redmine follows rename redirects when it resolves parent_title, so a
		// parent named by an old title lands on the renamed page. Only blame
		// the permission when want names a different page or no page at all;
		// a redirect cannot be resolved through the API (Redmine answers it
		// with an HTML redirect), so anything else is inconclusive.
		resolved, err := client.Wikis.Get(ctx, projectID, want, nil)
		if err == nil && strings.EqualFold(resolved.Title, got) {
			return "", nil
		}
		var apiErr *api.APIError
		if err != nil && (!errors.As(err, &apiErr) || !apiErr.IsNotFound()) {
			return fmt.Sprintf("wiki page %q has parent %s, not %q; this is expected if %q is an old title of that page "+
				"(Redmine follows rename redirects), but could not be verified: %v",
				page.Title, describeWikiParent(got), want, want, err), nil
		}
	}
	return "", &WikiParentNotAppliedError{
		Page:         page.Title,
		Requested:    want,
		Current:      got,
		DroppedTitle: check.droppedTitle,
		OtherChanges: check.otherChanges,
	}
}

// wikiParentMatches reports whether the stored parent got is what Redmine
// resolves want to. Redmine treats a blank parent_title as "no parent" and
// otherwise looks the title up case-insensitively after titleizing it.
func wikiParentMatches(want, got string) bool {
	if strings.TrimSpace(want) == "" {
		return got == ""
	}
	return strings.EqualFold(wikiTitleize(want), got)
}

// annotateWikiParentError explains the bare 422 Redmine returns when it
// rejects a page attribute. Redmine validates parent_title (and title) on
// the page but renders the errors of the page content, so the response
// carries no message at all. title is the rename requested alongside, if any,
// which fails the same way when it is taken or invalid.
func annotateWikiParentError(err error, parent, title string) error {
	var apiErr *api.APIError
	if strings.TrimSpace(parent) == "" || !errors.As(err, &apiErr) || !apiErr.IsValidationError() || len(apiErr.Errors) > 0 {
		return err
	}
	msg := fmt.Sprintf("Parent page %q was rejected: it must be an existing page of the same wiki, and neither the page itself nor one of its descendants.", parent)
	if title != "" {
		msg = fmt.Sprintf("Redmine rejected the page without a reason: either parent page %q is not an existing page of the same wiki (or is the page itself or one of its descendants), or the new title %q is invalid or already taken.", parent, title)
	}
	apiErr.Errors = []string{msg}
	return err
}

var wikiTitleWhitespace = regexp.MustCompile(`\s+`)

// wikiTitleize mirrors Wiki.titleize in Redmine: whitespace runs become
// underscores and the characters ,./?;|: are removed. The first-letter
// upcase is skipped because callers compare case-insensitively.
func wikiTitleize(title string) string {
	title = wikiTitleWhitespace.ReplaceAllString(title, "_")
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(",./?;|:", r) {
			return -1
		}
		return r
	}, title)
}

func describeWikiParent(title string) string {
	if strings.TrimSpace(title) == "" {
		return "none (wiki root)"
	}
	return strconv.Quote(title)
}
