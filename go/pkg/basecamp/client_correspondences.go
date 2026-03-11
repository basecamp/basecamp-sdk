package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// ClientCorrespondenceListOptions specifies options for listing client correspondences.
type ClientCorrespondenceListOptions struct {
	// Limit is the maximum number of client correspondences to return.
	// If 0, returns all. Use -1 for unlimited (same as 0).
	Limit int

	// Page, if positive, disables pagination and returns only the first page.
	Page int
}

// ClientCorrespondence represents a Basecamp client correspondence (message to clients).
type ClientCorrespondence struct {
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
	SubscriptionURL  string    `json:"subscription_url"`
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
	Content          string    `json:"content"`
	Subject          string    `json:"subject"`
	RepliesCount     int       `json:"replies_count"`
	RepliesURL       string    `json:"replies_url"`
}

// ClientCorrespondenceListResult contains the results from listing client correspondences.
type ClientCorrespondenceListResult struct {
	// Correspondences is the list of client correspondences returned.
	Correspondences []ClientCorrespondence
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// ClientCorrespondencesService handles client correspondence operations.
type ClientCorrespondencesService struct {
	client *AccountClient
}

// NewClientCorrespondencesService creates a new ClientCorrespondencesService.
func NewClientCorrespondencesService(client *AccountClient) *ClientCorrespondencesService {
	return &ClientCorrespondencesService{client: client}
}

// List returns all client correspondences in a project.
//
// Pagination options:
//   - Limit: maximum number of client correspondences to return (0 = all, -1 = unlimited)
//   - Page: if positive, disables pagination and returns first page only
//
// The returned ClientCorrespondenceListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *ClientCorrespondencesService) List(ctx context.Context, opts *ClientCorrespondenceListOptions) (result *ClientCorrespondenceListResult, err error) {
	op := OperationInfo{
		Service: "ClientCorrespondences", Operation: "List",
		ResourceType: "client_correspondence", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListClientCorrespondencesWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var correspondences []ClientCorrespondence
	if resp.JSON200 != nil {
		for _, gc := range *resp.JSON200 {
			correspondences = append(correspondences, clientCorrespondenceFromGenerated(gc))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &ClientCorrespondenceListResult{Correspondences: correspondences, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = all (no limit)
	limit := 0
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0 // unlimited
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	// Check if we already have enough items
	if limit > 0 && len(correspondences) >= limit {
		return &ClientCorrespondenceListResult{Correspondences: correspondences[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(correspondences), limit)}}, nil
	}

	// Follow pagination via Link headers
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(correspondences), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gc generated.ClientCorrespondence
		if err := json.Unmarshal(raw, &gc); err != nil {
			return nil, fmt.Errorf("failed to parse client correspondence: %w", err)
		}
		correspondences = append(correspondences, clientCorrespondenceFromGenerated(gc))
	}

	return &ClientCorrespondenceListResult{Correspondences: correspondences, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Get returns a client correspondence by ID.
func (s *ClientCorrespondencesService) Get(ctx context.Context, correspondenceID int64) (result *ClientCorrespondence, err error) {
	op := OperationInfo{
		Service: "ClientCorrespondences", Operation: "Get",
		ResourceType: "client_correspondence", IsMutation: false,
		ResourceID: correspondenceID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetClientCorrespondenceWithResponse(ctx, s.client.accountID, correspondenceID)
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

	correspondence := clientCorrespondenceFromGenerated(*resp.JSON200)
	return &correspondence, nil
}

// clientCorrespondenceFromGenerated converts a generated ClientCorrespondence to our clean type.
func clientCorrespondenceFromGenerated(gc generated.ClientCorrespondence) ClientCorrespondence {
	c := ClientCorrespondence{
		Status:           gc.Status,
		VisibleToClients: gc.VisibleToClients,
		CreatedAt:        gc.CreatedAt,
		UpdatedAt:        gc.UpdatedAt,
		Title:            gc.Title,
		InheritsStatus:   gc.InheritsStatus,
		Type:             gc.Type,
		URL:              gc.Url,
		AppURL:           gc.AppUrl,
		BookmarkURL:      gc.BookmarkUrl,
		SubscriptionURL:  gc.SubscriptionUrl,
		Content:          gc.Content,
		Subject:          gc.Subject,
		RepliesCount:     int(gc.RepliesCount),
		RepliesURL:       gc.RepliesUrl,
	}

	if gc.Id != 0 {
		c.ID = gc.Id
	}

	if gc.Parent.Id != 0 || gc.Parent.Title != "" {
		c.Parent = &Parent{
			ID:     gc.Parent.Id,
			Title:  gc.Parent.Title,
			Type:   gc.Parent.Type,
			URL:    gc.Parent.Url,
			AppURL: gc.Parent.AppUrl,
		}
	}

	if gc.Bucket.Id != 0 || gc.Bucket.Name != "" {
		c.Bucket = &Bucket{
			ID:   gc.Bucket.Id,
			Name: gc.Bucket.Name,
			Type: gc.Bucket.Type,
		}
	}

	if gc.Creator.Id != 0 || gc.Creator.Name != "" {
		c.Creator = &Person{
			ID:           gc.Creator.Id,
			Name:         gc.Creator.Name,
			EmailAddress: gc.Creator.EmailAddress,
			AvatarURL:    gc.Creator.AvatarUrl,
			Admin:        gc.Creator.Admin,
			Owner:        gc.Creator.Owner,
		}
	}

	return c
}
