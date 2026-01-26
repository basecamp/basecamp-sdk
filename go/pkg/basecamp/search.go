package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// SearchResult represents a single search result from the Basecamp API.
type SearchResult struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status"`
	Type             string    `json:"type"`
	URL              string    `json:"url"`
	AppURL           string    `json:"app_url"`
	BookmarkURL      string    `json:"bookmark_url"`
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
	Content          string    `json:"content,omitempty"`
	Description      string    `json:"description,omitempty"`
	Subject          string    `json:"subject,omitempty"`
}

// SearchMetadata represents metadata about available search scopes.
type SearchMetadata struct {
	Projects []SearchProject `json:"projects"`
}

// SearchProject represents a project available for search scope filtering.
type SearchProject struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// SearchOptions specifies optional parameters for search.
type SearchOptions struct {
	// Sort specifies the sort order: "created_at" or "updated_at" (default: relevance).
	Sort string
}

// SearchService handles search operations.
type SearchService struct {
	client *Client
}

// NewSearchService creates a new SearchService.
func NewSearchService(client *Client) *SearchService {
	return &SearchService{client: client}
}

// Search searches for content across the account.
// The query parameter is the search string.
// Returns a list of matching results.
func (s *SearchService) Search(ctx context.Context, query string, opts *SearchOptions) ([]SearchResult, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if query == "" {
		return nil, ErrUsage("search query is required")
	}

	// Build the query string
	params := url.Values{}
	params.Set("query", query)
	if opts != nil && opts.Sort != "" {
		params.Set("sort", opts.Sort)
	}

	path := fmt.Sprintf("/search.json?%s", params.Encode())
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	searchResults := make([]SearchResult, 0, len(results))
	for _, raw := range results {
		var sr SearchResult
		if err := json.Unmarshal(raw, &sr); err != nil {
			return nil, fmt.Errorf("failed to parse search result: %w", err)
		}
		searchResults = append(searchResults, sr)
	}

	return searchResults, nil
}

// Metadata returns metadata about available search scopes.
// This includes the list of projects available for filtering.
func (s *SearchService) Metadata(ctx context.Context) (*SearchMetadata, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.Get(ctx, "/searches/metadata.json")
	if err != nil {
		return nil, err
	}

	var metadata SearchMetadata
	if err := resp.UnmarshalData(&metadata); err != nil {
		return nil, fmt.Errorf("failed to parse search metadata: %w", err)
	}

	return &metadata, nil
}
