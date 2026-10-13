package models

import "strings"

// Collection fields on response models (Issue.Journals, Project.Trackers,
// User.Memberships, ...) are tagged `omitzero` rather than `omitempty`, and
// the difference is load-bearing for JSON output:
//
//   - nil slice: the collection was not fetched (no include, or Redmine chose
//     not to send it, e.g. watchers without the view permission). The key is
//     omitted, because emitting [] would claim "there are none".
//   - non-nil empty slice: the collection was fetched and is empty. The key is
//     emitted as [], so consumers can iterate it (`jq '.journals[]'`) without
//     guarding against a missing key.
//
// encoding/json decodes a JSON `[]` into a non-nil empty slice, so every
// collection Redmine sends as an empty array round-trips as [] for free. The
// helpers below cover the cases where Redmine omits a key even though it was
// requested and is known to be empty.
//
// Request payload types (IssueCreate, ProjectUpdate, ...) are separate structs
// and keep `omitempty`; an empty slice there would change server state.

// NormalizeCollections marks collections that Redmine always computes for an
// issue response but omits from the body when empty. custom_fields is part of
// every issue response. includes holds the include= values sent with the
// request; pass only includes the endpoint honours (GET /issues/:id.json drops
// "children" for leaf issues and "relations" when there are none, whereas the
// list endpoint ignores both).
func (i *Issue) NormalizeCollections(includes []string) {
	i.CustomFields = emptyIfNil(i.CustomFields)
	if hasInclude(includes, "children") {
		i.Children = emptyIfNil(i.Children)
	}
	if hasInclude(includes, "relations") {
		i.Relations = emptyIfNil(i.Relations)
	}
}

// NormalizeCollections marks custom_fields as fetched. Redmine renders it on
// every project response but omits the key when the project has no visible
// custom field values. Include-driven collections (trackers, enabled_modules,
// ...) are always sent as arrays when requested and need no help.
func (p *Project) NormalizeCollections() {
	p.CustomFields = emptyIfNil(p.CustomFields)
}

// NormalizeCollections marks custom_fields as fetched. Redmine renders it on
// every user response but omits the key when the user has no visible custom
// field values. memberships and groups are left alone: Redmine sends them as
// arrays when included and visible, and omits them only when the caller lacks
// permission, in which case [] would be wrong.
func (u *User) NormalizeCollections() {
	u.CustomFields = emptyIfNil(u.CustomFields)
}

func emptyIfNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// hasInclude reports whether name is among includes. Entries may themselves
// be comma-joined ("children,relations"), as the api layer joins them into a
// single include= value and Redmine splits and trims that value.
func hasInclude(includes []string, name string) bool {
	for _, inc := range includes {
		for part := range strings.SplitSeq(inc, ",") {
			if strings.TrimSpace(part) == name {
				return true
			}
		}
	}
	return false
}
