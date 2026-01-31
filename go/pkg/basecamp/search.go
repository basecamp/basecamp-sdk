package basecamp

import (
	"context"
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

// SearchOptions specifies optional parameters for search.
type SearchOptions struct {
	// Sort specifies the sort order: "created_at" or "updated_at" (default: relevance).
	Sort string
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
// Returns a list of matching results.
func (s *SearchService) Search(ctx context.Context, query string, opts *SearchOptions) (result []SearchResult, err error) {
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
		Query: query,
	}
	if opts != nil && opts.Sort != "" {
		params.Sort = opts.Sort
	}

	resp, err := s.client.parent.gen.SearchWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	searchResults := make([]SearchResult, 0, len(*resp.JSON200))
	for _, gsr := range *resp.JSON200 {
		searchResults = append(searchResults, searchResultFromGenerated(gsr))
	}

	return searchResults, nil
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
	if err = checkResponse(resp.HTTPResponse); err != nil {
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

	if gsr.Id != nil {
		sr.ID = *gsr.Id
	}

	// Convert nested types
	if gsr.Parent.Id != nil || gsr.Parent.Title != "" {
		sr.Parent = &Parent{
			ID:     derefInt64(gsr.Parent.Id),
			Title:  gsr.Parent.Title,
			Type:   gsr.Parent.Type,
			URL:    gsr.Parent.Url,
			AppURL: gsr.Parent.AppUrl,
		}
	}

	if gsr.Bucket.Id != nil || gsr.Bucket.Name != "" {
		sr.Bucket = &Bucket{
			ID:   derefInt64(gsr.Bucket.Id),
			Name: gsr.Bucket.Name,
			Type: gsr.Bucket.Type,
		}
	}

	if gsr.Creator.Id != nil || gsr.Creator.Name != "" {
		sr.Creator = &Person{
			ID:           derefInt64(gsr.Creator.Id),
			Name:         gsr.Creator.Name,
			EmailAddress: gsr.Creator.EmailAddress,
			AvatarURL:    gsr.Creator.AvatarUrl,
			Admin:        gsr.Creator.Admin,
			Owner:        gsr.Creator.Owner,
		}
	}

	return sr
}
