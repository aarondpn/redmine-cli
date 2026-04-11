package models

// WikiPage represents a Redmine wiki page.
type WikiPage struct {
	Title       string         `json:"title"`
	Text        string         `json:"text"`
	Comments    string         `json:"comments,omitempty"`
	Version     int            `json:"version"`
	Author      *IDName        `json:"author,omitempty"`
	UpdatedOn   string         `json:"updated_on"`
	CreatedOn   string         `json:"created_on"`
	Parent      *WikiPageTitle `json:"parent,omitempty"`
	Attachments []Attachment   `json:"attachments,omitempty"`
}

// WikiPageTitle is a minimal reference used for parent pages.
type WikiPageTitle struct {
	Title string `json:"title"`
}

// Attachment represents a file attached to a wiki page.
type Attachment struct {
	ID          int    `json:"id"`
	Filename    string `json:"filename"`
	Filesize    int64  `json:"filesize"`
	ContentType string `json:"content_type"`
	Description string `json:"description"`
	ContentURL  string `json:"content_url"`
	Author      IDName `json:"author"`
	CreatedOn   string `json:"created_on"`
}

// WikiPageIndex represents a wiki page entry in the index listing.
type WikiPageIndex struct {
	Title     string `json:"title"`
	UpdatedOn string `json:"updated_on"`
}

// WikiPageCreate defines fields for creating a wiki page.
type WikiPageCreate struct {
	Text     string `json:"text"`
	Comments string `json:"comments,omitempty"`
	Title    string `json:"title,omitempty"`
}

// WikiPageUpdate defines fields for updating a wiki page.
type WikiPageUpdate struct {
	Text     *string `json:"text,omitempty"`
	Comments *string `json:"comments,omitempty"`
	Title    *string `json:"title,omitempty"`
}
