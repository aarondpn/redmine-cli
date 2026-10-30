package models

// News represents a Redmine news item (GET /news.json,
// GET /news/:id.json).
type News struct {
	ID          int    `json:"id"`
	Project     IDName `json:"project"`
	Author      IDName `json:"author"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	CreatedOn   string `json:"created_on"`
	// Attachments (include=attachments), Comments (include=comments) and
	// Reactions (include=reactions) are only present when requested.
	Attachments []Attachment  `json:"attachments,omitzero"`
	Comments    []NewsComment `json:"comments,omitzero"`
	// Reactions (include=reactions) needs Redmine 7.1+.
	Reactions []Reaction `json:"reactions,omitzero"`
}

// NewsComment is a comment on a news item (include=comments). Redmine does
// not accept news comments over the REST API; they are read-only.
type NewsComment struct {
	ID      int    `json:"id"`
	Author  IDName `json:"author"`
	Content string `json:"content"`
	// Reactions (include=reactions) needs Redmine 7.1+.
	Reactions []Reaction `json:"reactions,omitzero"`
}

// NewsCreate is the payload for POST /projects/:id/news.json.
type NewsCreate struct {
	Title       string   `json:"title"`
	Summary     string   `json:"summary,omitempty"`
	Description string   `json:"description"`
	Uploads     []Upload `json:"uploads,omitempty"`
}

// NewsUpdate is the payload for PUT /news/:id.json. Nil fields are left
// unchanged.
type NewsUpdate struct {
	Title       *string  `json:"title,omitempty"`
	Summary     *string  `json:"summary,omitempty"`
	Description *string  `json:"description,omitempty"`
	Uploads     []Upload `json:"uploads,omitempty"`
}
