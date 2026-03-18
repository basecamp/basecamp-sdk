package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
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

// SearchListResult contains the results from searching.
type SearchListResult struct {
	// Results is the list of search results returned.
	Results []SearchResult
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// SearchOptions specifies optional parameters for search.
type SearchOptions struct {
	// Sort specifies the sort order: "best_match" or "created_at" (default: created_at desc).
	// "best_match" returns results ranked by relevance with a recency boost.
	Sort string

	// Limit is the maximum number of results to return.
	// If 0 (default), returns all results.
	Limit int

	// Page, if positive, disables pagination and returns only the first page.
	Page int
}

// SearchService handles search operations.
type SearchService struct {
	client *AccountClient
}

// NewSearchService creates a new SearchService.
func NewSearchService(client *AccountClient) *SearchService {
	return &SearchService{client: client}
}

// Search searches for content across the account.
// The query parameter is the search string.
//
// Pagination options:
//   - Limit: maximum number of results to return (0 = all)
//   - Page: if positive, disables pagination and returns first page only
//
// The returned SearchListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *SearchService) Search(ctx context.Context, query string, opts *SearchOptions) (result *SearchListResult, err error) {
	op := OperationInfo{
		Service: "Search", Operation: "Search",
		ResourceType: "search", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if query == "" {
		err = ErrUsage("search query is required")
		return nil, err
	}

	params := &generated.SearchParams{
		Q: query,
	}
	if opts != nil && opts.Sort != "" {
		params.Sort = opts.Sort
	}

	resp, err := s.client.parent.gen.SearchWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var searchResults []SearchResult
	if resp.JSON200 != nil {
		for _, gsr := range *resp.JSON200 {
			searchResults = append(searchResults, searchResultFromGenerated(gsr))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &SearchListResult{Results: searchResults, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = all (default for search)
	limit := 0
	if opts != nil {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(searchResults) >= limit {
		return &SearchListResult{Results: searchResults[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(searchResults), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(searchResults), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gsr generated.SearchResult
		if err := json.Unmarshal(raw, &gsr); err != nil {
			return nil, fmt.Errorf("failed to parse search result: %w", err)
		}
		searchResults = append(searchResults, searchResultFromGenerated(gsr))
	}

	return &SearchListResult{Results: searchResults, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Metadata returns metadata about available search scopes.
// This includes the list of projects available for filtering.
func (s *SearchService) Metadata(ctx context.Context) (result *SearchMetadata, err error) {
	op := OperationInfo{
		Service: "Search", Operation: "Metadata",
		ResourceType: "search", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetSearchMetadataWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	// Convert metadata
	metadata := &SearchMetadata{
		Projects: make([]SearchProject, 0, len(resp.JSON200.Projects)),
	}
	for _, gsp := range resp.JSON200.Projects {
		metadata.Projects = append(metadata.Projects, SearchProject{
			ID:   derefInt64(gsp.Id),
			Name: gsp.Name,
		})
	}

	return metadata, nil
}

// searchResultFromGenerated converts a generated SearchResult to our clean SearchResult type.
func searchResultFromGenerated(gsr generated.SearchResult) SearchResult {
	sr := SearchResult{
		Status:           gsr.Status,
		VisibleToClients: gsr.VisibleToClients,
		CreatedAt:        gsr.CreatedAt,
		UpdatedAt:        gsr.UpdatedAt,
		Title:            gsr.Title,
		InheritsStatus:   gsr.InheritsStatus,
		Type:             gsr.Type,
		URL:              gsr.Url,
		AppURL:           gsr.AppUrl,
		BookmarkURL:      gsr.BookmarkUrl,
		Content:          gsr.Content,
		Description:      gsr.Description,
		Subject:          gsr.Subject,
	}

	if gsr.Id != 0 {
		sr.ID = gsr.Id
	}

	// Convert nested types
	if gsr.Parent.Id != 0 || gsr.Parent.Title != "" {
		sr.Parent = &Parent{
			ID:     gsr.Parent.Id,
			Title:  gsr.Parent.Title,
			Type:   gsr.Parent.Type,
			URL:    gsr.Parent.Url,
			AppURL: gsr.Parent.AppUrl,
		}
	}

	if gsr.Bucket.Id != 0 || gsr.Bucket.Name != "" {
		sr.Bucket = &Bucket{
			ID:   gsr.Bucket.Id,
			Name: gsr.Bucket.Name,
			Type: gsr.Bucket.Type,
		}
	}

	if gsr.Creator.Id != 0 || gsr.Creator.Name != "" {
		sr.Creator = &Person{
			ID:           gsr.Creator.Id,
			Name:         gsr.Creator.Name,
			EmailAddress: gsr.Creator.EmailAddress,
			AvatarURL:    gsr.Creator.AvatarUrl,
			Admin:        gsr.Creator.Admin,
			Owner:        gsr.Creator.Owner,
		}
	}

	return sr
}
