package models

// SearchResult represents a single result from the Redmine search API.
type SearchResult struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	Description string `json:"description"`
	DateTime    string `json:"datetime"`
}

// SearchResponse represents the full response from the Redmine search API.
type SearchResponse struct {
	Results    []SearchResult `json:"results"`
	TotalCount int            `json:"total_count"`
	Offset     int            `json:"offset"`
	Limit      int            `json:"limit"`
}
