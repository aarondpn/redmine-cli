package models

// ProjectFile represents a project-level file as returned by the Redmine
// /projects/:id/files.json endpoint.
type ProjectFile struct {
	ID          int     `json:"id"`
	Filename    string  `json:"filename"`
	Filesize    int64   `json:"filesize"`
	ContentType string  `json:"content_type,omitempty"`
	Description string  `json:"description,omitempty"`
	ContentURL  string  `json:"content_url,omitempty"`
	Author      IDName  `json:"author"`
	CreatedOn   string  `json:"created_on"`
	Version     *IDName `json:"version,omitempty"`
	Digest      string  `json:"digest,omitempty"`
	Downloads   int     `json:"downloads,omitempty"`
}

// ProjectFileCreate defines fields for uploading a file to a project. The
// Token is obtained from the /uploads.json endpoint.
type ProjectFileCreate struct {
	Token       string `json:"token"`
	Filename    string `json:"filename,omitempty"`
	VersionID   int    `json:"version_id,omitempty"`
	Description string `json:"description,omitempty"`
	ContentType string `json:"content_type,omitempty"`
}
