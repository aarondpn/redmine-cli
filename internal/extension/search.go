package extension

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/extension/spec"
	"github.com/aarondpn/redmine-cli/v2/internal/ghrelease"
)

// MaxSearchResults is the largest page the GitHub search API returns.
const MaxSearchResults = 100

// SearchResult is one extension repository found on GitHub.
type SearchResult struct {
	Repo        string `json:"repo"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Stars       int    `json:"stars"`
	URL         string `json:"url"`
	Installed   bool   `json:"installed"`
}

type searchResponse struct {
	Items []struct {
		FullName    string `json:"full_name"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Stars       int    `json:"stargazers_count"`
		HTMLURL     string `json:"html_url"`
		Owner       struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"items"`
}

// Search looks up repositories with the extension topic on GitHub, most
// starred first, and marks the ones that are installed. query narrows the
// search (GitHub search syntax); limit is capped at MaxSearchResults.
func (m *Manager) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 || limit > MaxSearchResults {
		limit = MaxSearchResults
	}
	q := strings.TrimSpace(strings.TrimSpace(query) + " topic:" + Topic)
	params := url.Values{
		"q":        {q},
		"sort":     {"stars"},
		"order":    {"desc"},
		"per_page": {strconv.Itoa(limit)},
	}
	gh := m.github()
	var resp searchResponse
	if err := gh.GetJSON(ctx, gh.APIURL("/search/repositories")+"?"+params.Encode(), ghrelease.MaxMetadataBytes, &resp); err != nil {
		return nil, fmt.Errorf("GitHub search failed: %w", err)
	}

	installed := map[string]bool{}
	if exts, err := m.List(); err == nil {
		for _, e := range exts {
			if repo := e.Repo(); repo != "" {
				installed[strings.ToLower(repo)] = true
			}
		}
	}
	results := make([]SearchResult, 0, len(resp.Items))
	for _, item := range resp.Items {
		name, ok := strings.CutPrefix(item.Name, Prefix)
		if !ok || spec.ValidateExtensionName(name) != nil {
			// Tagged with the topic but not installable under its name.
			continue
		}
		results = append(results, SearchResult{
			Repo:        item.FullName,
			Name:        name,
			Description: item.Description,
			Stars:       item.Stars,
			URL:         item.HTMLURL,
			Installed:   installed[strings.ToLower(item.FullName)],
		})
	}
	return results, nil
}
