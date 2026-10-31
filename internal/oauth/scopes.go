// Package oauth implements Redmine's OAuth2 authorization code flow with
// PKCE: the browser login, token refresh and revocation.
package oauth

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// readScopes are the permissions the CLI's read commands use. Redmine
// intersects a token's scopes with the user's role permissions, public ones
// included, so view_project, search_project and view_members must be listed.
var readScopes = []string{
	"view_project", "search_project", "view_members",
	"view_issues", "view_private_notes", "view_issue_watchers",
	"view_time_entries", "view_news", "view_files",
	"view_wiki_pages", "view_wiki_edits", "view_messages",
}

// writeScopes are the permissions the write commands use.
var writeScopes = []string{
	"add_project", "add_subprojects", "edit_project", "delete_project",
	"manage_members", "manage_versions", "manage_categories",
	"add_issues", "edit_issues", "edit_own_issues", "delete_issues",
	"manage_issue_relations", "manage_subtasks",
	"set_issues_private", "set_own_issues_private",
	"add_issue_notes", "edit_issue_notes", "edit_own_issue_notes", "set_notes_private",
	"add_issue_watchers", "delete_issue_watchers",
	"log_time", "edit_time_entries", "edit_own_time_entries", "log_time_for_other_users",
	"manage_news", "manage_files",
	"edit_wiki_pages", "delete_wiki_pages",
}

var scopeName = regexp.MustCompile(`^[a-z_]+$`)

// DefaultScopes returns the scopes a login requests: readScopes for a
// read-only profile, readScopes and writeScopes otherwise. Never "admin".
func DefaultScopes(readOnly bool) []string {
	scopes := slices.Clone(readScopes)
	if !readOnly {
		scopes = append(scopes, writeScopes...)
	}
	return scopes
}

// ParseScopes parses a --scopes value: names separated by commas and/or
// spaces; "default" expands to DefaultScopes(readOnly) (so "default,admin"
// adds admin). Duplicates are dropped, order kept. A name must match
// [a-z_]+; an empty result is an error.
func ParseScopes(s string, readOnly bool) ([]string, error) {
	var scopes []string
	add := func(name string) {
		if !slices.Contains(scopes, name) {
			scopes = append(scopes, name)
		}
	}
	for _, name := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }) {
		if name == "default" {
			for _, d := range DefaultScopes(readOnly) {
				add(d)
			}
			continue
		}
		if !scopeName.MatchString(name) {
			return nil, fmt.Errorf("invalid scope %q: use lower-case permission names such as view_issues", name)
		}
		add(name)
	}
	if len(scopes) == 0 {
		return nil, errors.New("no scopes given")
	}
	return scopes, nil
}
