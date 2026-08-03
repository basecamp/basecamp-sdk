package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Bookmark is a personal link between the current user and a single recording.
// Bookmarks are visible only to their creator; the wrapped recording is the
// shared recording projection (parent is optional there).
type Bookmark struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Recording Recording `json:"recording"`
}

// BookmarkListResult contains a page (or all pages) of bookmarks plus metadata.
type BookmarkListResult struct {
	Bookmarks []Bookmark
	Meta      ListMeta
}

// BookmarksService handles the current user's personal bookmarks.
type BookmarksService struct {
	client *AccountClient
}

// NewBookmarksService creates a new BookmarksService.
func NewBookmarksService(client *AccountClient) *BookmarksService {
	return &BookmarksService{client: client}
}

// List returns the current user's bookmarks, most recently bookmarked first.
// Pass a positive page to return only that page; page 0 follows the Link
// header across all pages.
func (s *BookmarksService) List(ctx context.Context, page int32) (result *BookmarkListResult, err error) {
	op := OperationInfo{
		Service: "Bookmarks", Operation: "List",
		ResourceType: "bookmark", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.ListMyBookmarksParams
	if page > 0 {
		params = &generated.ListMyBookmarksParams{Page: &page}
	}
	resp, err := s.client.parent.gen.ListMyBookmarksWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var bookmarks []Bookmark
	if resp.JSON200 != nil {
		for _, gb := range *resp.JSON200 {
			bookmarks = append(bookmarks, bookmarkFromGenerated(gb))
		}
	}
	totalCount := parseTotalCount(resp.HTTPResponse)
	if page > 0 {
		return &BookmarkListResult{Bookmarks: bookmarks, Meta: ListMeta{TotalCount: totalCount, Truncated: hasNextPage(resp.HTTPResponse)}}, nil
	}

	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(bookmarks), 0)
	if err != nil {
		return nil, err
	}
	for _, raw := range rawMore {
		var gb generated.Bookmark
		if err := json.Unmarshal(raw, &gb); err != nil {
			return nil, fmt.Errorf("failed to parse bookmark: %w", err)
		}
		bookmarks = append(bookmarks, bookmarkFromGenerated(gb))
	}
	return &BookmarkListResult{Bookmarks: bookmarks, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Get reports whether the current user has bookmarked the recording.
func (s *BookmarksService) Get(ctx context.Context, recordingID int64) (result bool, err error) {
	op := OperationInfo{
		Service: "Bookmarks", Operation: "Get",
		ResourceType: "bookmark", IsMutation: false,
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

	resp, err := s.client.parent.gen.GetBookmarkWithResponse(ctx, s.client.accountID, recordingID)
	if err != nil {
		return false, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return false, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return false, err
	}
	return resp.JSON200.Bookmarked, nil
}

// Create bookmarks the recording for the current user. Idempotent:
// re-bookmarking returns the existing bookmark, never a duplicate.
func (s *BookmarksService) Create(ctx context.Context, recordingID int64) (result *Bookmark, err error) {
	op := OperationInfo{
		Service: "Bookmarks", Operation: "Create",
		ResourceType: "bookmark", IsMutation: true,
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

	resp, err := s.client.parent.gen.CreateBookmarkWithResponse(ctx, s.client.accountID, recordingID)
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

	bookmark := bookmarkFromGenerated(*resp.JSON201)
	return &bookmark, nil
}

// Delete removes the current user's bookmark from the recording. Idempotent:
// deleting an absent bookmark also succeeds (204 either way).
func (s *BookmarksService) Delete(ctx context.Context, recordingID int64) (err error) {
	op := OperationInfo{
		Service: "Bookmarks", Operation: "Delete",
		ResourceType: "bookmark", IsMutation: true,
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

	resp, err := s.client.parent.gen.DeleteBookmarkWithResponse(ctx, s.client.accountID, recordingID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

func bookmarkFromGenerated(gb generated.Bookmark) Bookmark {
	return Bookmark{
		ID:        gb.Id,
		CreatedAt: gb.CreatedAt,
		UpdatedAt: gb.UpdatedAt,
		Recording: recordingFromGenerated(gb.Recording),
	}
}
