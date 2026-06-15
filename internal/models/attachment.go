package models

// Attachment represents a file attached to a Redmine resource (issue, wiki
// page, ...). It is returned by the /attachments/:id.json endpoint and by the
// attachments[] array on issues and wiki pages (include=attachments).
type Attachment struct {
	ID           int    `json:"id"`
	Filename     string `json:"filename"`
	Filesize     int64  `json:"filesize"`
	ContentType  string `json:"content_type"`
	Description  string `json:"description"`
	ContentURL   string `json:"content_url"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	Author       IDName `json:"author"`
	CreatedOn    string `json:"created_on"`
}
