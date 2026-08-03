package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Draft is an unpublished message, document, upload, or client
// approval/correspondence — a flat, purpose-built envelope, not the shared
// recording projection.
type Draft struct {
	ID     int64  `json:"id"`
	AppURL string `json:"app_url"`
	Title  string `json:"title"`
	// Type is the short recordable name: message, document, upload,
	// client_approval, or client_correspondence.
	Type   string      `json:"type"`
	Bucket DraftBucket `json:"bucket"`
	// Parent is the recording the draft is filed under; nil for drafts filed
	// directly under their bucket (the key is always present on the wire).
	Parent  *DraftParent `json:"parent"`
	Excerpt string       `json:"excerpt"`
	// CreatedAt/UpdatedAt are RFC3339 timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// ScheduledPostingAt is nil unless the draft is scheduled to publish later.
	ScheduledPostingAt *time.Time `json:"scheduled_posting_at"`
}

// DraftBucket is the project a draft lives in.
type DraftBucket struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	AppURL string `json:"app_url"`
}

// DraftParent is the parent recording a draft is filed under.
type DraftParent struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	AppURL string `json:"app_url"`
}

// DraftListResult contains a page (or all pages) of drafts plus metadata.
type DraftListResult struct {
	Drafts []Draft
	Meta   ListMeta
}

// DraftsService lists the current user's unpublished drafts.
type DraftsService struct {
	client *AccountClient
}

// NewDraftsService creates a new DraftsService.
func NewDraftsService(client *AccountClient) *DraftsService {
	return &DraftsService{client: client}
}

// List returns the current user's drafts across active projects, most recently
// updated first (capped at 250 server-side, like /my/assignments). Pass a
// positive page to return only that page; page 0 follows the Link header.
func (s *DraftsService) List(ctx context.Context, page int32) (result *DraftListResult, err error) {
	op := OperationInfo{
		Service: "Drafts", Operation: "List",
		ResourceType: "draft", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.ListMyDraftsParams
	if page > 0 {
		params = &generated.ListMyDraftsParams{Page: &page}
	}
	resp, err := s.client.parent.gen.ListMyDraftsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var drafts []Draft
	if resp.JSON200 != nil {
		for _, gd := range *resp.JSON200 {
			drafts = append(drafts, draftFromGenerated(gd))
		}
	}
	totalCount := parseTotalCount(resp.HTTPResponse)
	if page > 0 {
		return &DraftListResult{Drafts: drafts, Meta: ListMeta{TotalCount: totalCount, Truncated: hasNextPage(resp.HTTPResponse)}}, nil
	}

	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(drafts), 0)
	if err != nil {
		return nil, err
	}
	for _, raw := range rawMore {
		var gd generated.Draft
		if err := json.Unmarshal(raw, &gd); err != nil {
			return nil, fmt.Errorf("failed to parse draft: %w", err)
		}
		drafts = append(drafts, draftFromGenerated(gd))
	}
	return &DraftListResult{Drafts: drafts, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

func draftFromGenerated(gd generated.Draft) Draft {
	d := Draft{
		ID:                 gd.Id,
		AppURL:             gd.AppUrl,
		Title:              gd.Title,
		Type:               gd.Type,
		Bucket:             DraftBucket{ID: gd.Bucket.Id, Name: gd.Bucket.Name, AppURL: gd.Bucket.AppUrl},
		Excerpt:            gd.Excerpt,
		CreatedAt:          gd.CreatedAt,
		UpdatedAt:          gd.UpdatedAt,
		ScheduledPostingAt: gd.ScheduledPostingAt,
	}
	if p := gd.Parent; p != nil {
		d.Parent = &DraftParent{ID: p.Id, Title: p.Title, AppURL: p.AppUrl}
	}
	return d
}
