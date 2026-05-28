package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// DefaultBoostLimit is the default number of boosts to return when no limit is specified.
// Matches the API's maxPageSize of 50 (from x-basecamp-pagination in the spec).
const DefaultBoostLimit = 50

// Boost represents a Basecamp boost (emoji reaction) on a recording.
type Boost struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	Booster   *Person   `json:"booster,omitempty"`
	Recording *Parent   `json:"recording,omitempty"`
}

// BoostListOptions specifies options for listing boosts.
type BoostListOptions struct {
	// Limit is the maximum number of boosts to return.
	// If 0, uses DefaultBoostLimit (50). Use -1 for unlimited.
	Limit int

	// Page, if positive, disables automatic pagination and returns only the first page.
	// NOTE: The page number itself is not honored; setting Page=2 does NOT fetch page 2.
	Page int
}

// BoostListResult contains the results from listing boosts.
type BoostListResult struct {
	// Boosts is the list of boosts returned.
	Boosts []Boost
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// BoostsService handles boost operations.
type BoostsService struct {
	client *AccountClient
}

// NewBoostsService creates a new BoostsService.
func NewBoostsService(client *AccountClient) *BoostsService {
	return &BoostsService{client: client}
}

// ListRecording returns boosts on a recording.
//
// By default, returns up to 50 boosts. Use Limit: -1 for unlimited.
//
// Pagination options:
//   - Limit: maximum number of boosts to return (0 = 50, -1 = unlimited)
//   - Page: if positive, disables pagination and returns first page only
//
// The returned BoostListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *BoostsService) ListRecording(ctx context.Context, recordingID int64, opts *BoostListOptions) (result *BoostListResult, err error) {
	op := OperationInfo{
		Service: "Boosts", Operation: "ListRecording",
		ResourceType: "boost", IsMutation: false,
		ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListRecordingBoostsWithResponse(ctx, s.client.accountID, recordingID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var boosts []Boost
	if resp.JSON200 != nil {
		for _, gb := range *resp.JSON200 {
			boosts = append(boosts, boostFromGenerated(gb))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &BoostListResult{Boosts: boosts, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = default (50), -1 = unlimited, >0 = specific limit
	limit := DefaultBoostLimit
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0 // unlimited
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	// Check if we already have enough items
	if limit > 0 && len(boosts) >= limit {
		return &BoostListResult{Boosts: boosts[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(boosts), limit)}}, nil
	}

	// Follow pagination via Link headers
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(boosts), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gb generated.Boost
		if err := json.Unmarshal(raw, &gb); err != nil {
			return nil, fmt.Errorf("failed to parse boost: %w", err)
		}
		boosts = append(boosts, boostFromGenerated(gb))
	}

	return &BoostListResult{Boosts: boosts, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// ListEvent returns boosts on a specific event within a recording.
//
// By default, returns up to 50 boosts. Use Limit: -1 for unlimited.
//
// Pagination options:
//   - Limit: maximum number of boosts to return (0 = 50, -1 = unlimited)
//   - Page: if positive, disables pagination and returns first page only
//
// The returned BoostListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *BoostsService) ListEvent(ctx context.Context, recordingID, eventID int64, opts *BoostListOptions) (result *BoostListResult, err error) {
	op := OperationInfo{
		Service: "Boosts", Operation: "ListEvent",
		ResourceType: "boost", IsMutation: false,
		ResourceID: eventID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListEventBoostsWithResponse(ctx, s.client.accountID, recordingID, eventID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var boosts []Boost
	if resp.JSON200 != nil {
		for _, gb := range *resp.JSON200 {
			boosts = append(boosts, boostFromGenerated(gb))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &BoostListResult{Boosts: boosts, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = default (50), -1 = unlimited, >0 = specific limit
	limit := DefaultBoostLimit
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0 // unlimited
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	// Check if we already have enough items
	if limit > 0 && len(boosts) >= limit {
		return &BoostListResult{Boosts: boosts[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(boosts), limit)}}, nil
	}

	// Follow pagination via Link headers
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(boosts), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gb generated.Boost
		if err := json.Unmarshal(raw, &gb); err != nil {
			return nil, fmt.Errorf("failed to parse boost: %w", err)
		}
		boosts = append(boosts, boostFromGenerated(gb))
	}

	return &BoostListResult{Boosts: boosts, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Get returns a boost by ID.
func (s *BoostsService) Get(ctx context.Context, boostID int64) (result *Boost, err error) {
	op := OperationInfo{
		Service: "Boosts", Operation: "Get",
		ResourceType: "boost", IsMutation: false,
		ResourceID: boostID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetBoostWithResponse(ctx, s.client.accountID, boostID)
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

	boost := boostFromGenerated(*resp.JSON200)
	return &boost, nil
}

// CreateRecording creates a boost on a recording.
// content is the emoji content for the boost.
// Returns the created boost.
func (s *BoostsService) CreateRecording(ctx context.Context, recordingID int64, content string) (result *Boost, err error) {
	op := OperationInfo{
		Service: "Boosts", Operation: "CreateRecording",
		ResourceType: "boost", IsMutation: true,
		ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if content == "" {
		err = ErrUsage("boost content is required")
		return nil, err
	}

	body := generated.CreateRecordingBoostJSONRequestBody{
		Content: content,
	}

	resp, err := s.client.parent.gen.CreateRecordingBoostWithResponse(ctx, s.client.accountID, recordingID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	boost := boostFromGenerated(*resp.JSON201)
	return &boost, nil
}

// CreateEvent creates a boost on a specific event within a recording.
// content is the emoji content for the boost.
// Returns the created boost.
func (s *BoostsService) CreateEvent(ctx context.Context, recordingID, eventID int64, content string) (result *Boost, err error) {
	op := OperationInfo{
		Service: "Boosts", Operation: "CreateEvent",
		ResourceType: "boost", IsMutation: true,
		ResourceID: eventID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if content == "" {
		err = ErrUsage("boost content is required")
		return nil, err
	}

	body := generated.CreateEventBoostJSONRequestBody{
		Content: content,
	}

	resp, err := s.client.parent.gen.CreateEventBoostWithResponse(ctx, s.client.accountID, recordingID, eventID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	boost := boostFromGenerated(*resp.JSON201)
	return &boost, nil
}

// Delete deletes a boost.
func (s *BoostsService) Delete(ctx context.Context, boostID int64) (err error) {
	op := OperationInfo{
		Service: "Boosts", Operation: "Delete",
		ResourceType: "boost", IsMutation: true,
		ResourceID: boostID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DeleteBoostWithResponse(ctx, s.client.accountID, boostID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// boostFromGenerated converts a generated Boost to our clean Boost type.
func boostFromGenerated(gb generated.Boost) Boost {
	b := Boost{
		Content:   gb.Content,
		CreatedAt: gb.CreatedAt,
	}

	b.ID = gb.Id

	if gb.Booster.Id != 0 || gb.Booster.Name != "" {
		booster := personFromGenerated(gb.Booster)
		b.Booster = &booster
	}

	if gb.Recording.Id != 0 || gb.Recording.Title != "" {
		b.Recording = &Parent{
			ID:     gb.Recording.Id,
			Title:  gb.Recording.Title,
			Type:   gb.Recording.Type,
			URL:    gb.Recording.Url,
			AppURL: gb.Recording.AppUrl,
		}
	}

	return b
}
