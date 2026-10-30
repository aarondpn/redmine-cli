package models

// WikiPage represents a Redmine wiki page.
//
// Project is only sent by Redmine 7.0+ (#43569), so it stays a pointer and
// callers must treat it as optional.
type WikiPage struct {
	Title       string         `json:"title"`
	Text        string         `json:"text"`
	Comments    string         `json:"comments,omitempty"`
	Version     int            `json:"version"`
	Author      *IDName        `json:"author,omitempty"`
	Project     *IDName        `json:"project,omitempty"`
	UpdatedOn   string         `json:"updated_on"`
	CreatedOn   string         `json:"created_on"`
	Parent      *WikiPageTitle `json:"parent,omitempty"`
	Attachments []Attachment   `json:"attachments,omitzero"`
}

// WikiPageTitle is a minimal reference used for parent pages.
type WikiPageTitle struct {
	Title string `json:"title"`
}

// WikiPageIndex represents a wiki page entry in the index listing.
//
// Parent is omitted for top-level pages. Project is only sent by Redmine
// 7.0+ (#43569), like on WikiPage.
type WikiPageIndex struct {
	Title     string         `json:"title"`
	Parent    *WikiPageTitle `json:"parent,omitempty"`
	Version   int            `json:"version"`
	Project   *IDName        `json:"project,omitempty"`
	CreatedOn string         `json:"created_on"`
	UpdatedOn string         `json:"updated_on"`
}

// WikiPageCreate defines fields for creating a wiki page.
//
// ParentTitle names the page to nest the new page under. Redmine resolves it
// with the same lookup as page URLs (spaces become underscores, matching is
// case-insensitive) and rejects unknown titles with a validation error.
type WikiPageCreate struct {
	Text        string   `json:"text"`
	Comments    string   `json:"comments,omitempty"`
	Title       string   `json:"title,omitempty"`
	ParentTitle string   `json:"parent_title,omitempty"`
	Uploads     []Upload `json:"uploads,omitempty"`
}

// WikiPageUpdate defines fields for updating a wiki page.
//
// Version, when non-nil, asserts the page version the client believes is
// current. Redmine compares it against the stored version and answers with
// 409 Conflict if the page has moved on, giving callers optimistic-locking
// semantics for the update.
//
// ParentTitle, when non-nil, moves the page under the named parent. A pointer
// to the empty string is sent as `"parent_title":""`, which Redmine treats as
// "no parent" and moves the page to the wiki root.
type WikiPageUpdate struct {
	Text        *string  `json:"text,omitempty"`
	Comments    *string  `json:"comments,omitempty"`
	Title       *string  `json:"title,omitempty"`
	ParentTitle *string  `json:"parent_title,omitempty"`
	Version     *int     `json:"version,omitempty"`
	Uploads     []Upload `json:"uploads,omitempty"`

	// Section and SectionHash select a single section to replace. Redmine
	// reads these from the top level of the request params (see
	// WikiController#update), NOT from inside the wiki_page object, so they
	// are excluded from the wiki_page JSON here and added top-level by
	// WikiService.Update. Sending them nested makes Redmine ignore them and
	// overwrite the entire page.
	Section     *int    `json:"-"`
	SectionHash *string `json:"-"`
}
