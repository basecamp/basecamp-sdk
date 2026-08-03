package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// ClientApprovalListOptions specifies options for listing client approvals.
type ClientApprovalListOptions struct {
	// Limit is the maximum number of client approvals to return.
	// If 0, returns all. Use -1 for unlimited (same as 0).
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination.
	Page int

	// Sort field: "created_at" or "updated_at".
	Sort string

	// Direction: "asc" or "desc".
	Direction string
}

// ClientApproval represents a Basecamp client approval request.
type ClientApproval struct {
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
	// ContentAttachments holds structured metadata for the downloadable files
	// embedded in the rich text Content. @required — the API always sends this
	// array (empty when the content has no inline files). No omitempty, so on
	// marshal a non-nil slice emits its elements ([] when empty) and a nil
	// slice emits null; the key is never
	// dropped. Decode distinguishes a server-sent [] (non-nil) from nil. See
	// RichTextAttachment.
	ContentAttachments []RichTextAttachment     `json:"content_attachments"`
	Subject            string                   `json:"subject"`
	DueOn              *string                  `json:"due_on,omitempty"`
	RepliesCount       int                      `json:"replies_count"`
	RepliesURL         string                   `json:"replies_url"`
	ApprovalStatus     string                   `json:"approval_status"`
	Approver           *Person                  `json:"approver,omitempty"`
	Responses          []ClientApprovalResponse `json:"responses,omitempty"`
}

// ClientApprovalResponse represents a response to a client approval.
type ClientApprovalResponse struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status"`
	Type             string    `json:"type"`
	AppURL           string    `json:"app_url"`
	BookmarkURL      string    `json:"bookmark_url"`
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
	Content          string    `json:"content"`
	Approved         bool      `json:"approved"`
}

// ClientApprovalListResult contains the results from listing client approvals.
type ClientApprovalListResult struct {
	// Approvals is the list of client approvals returned.
	Approvals []ClientApproval
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// ClientApprovalsService handles client approval operations.
type ClientApprovalsService struct {
	client *AccountClient
}

// NewClientApprovalsService creates a new ClientApprovalsService.
func NewClientApprovalsService(client *AccountClient) *ClientApprovalsService {
	return &ClientApprovalsService{client: client}
}

// List returns all client approvals in a project.
//
// Pagination options:
//   - Limit: maximum number of client approvals to return (0 = all, -1 = unlimited)
//   - Page: if positive, fetches only that page and disables auto-pagination
//
// The returned ClientApprovalListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *ClientApprovalsService) List(ctx context.Context, bucketID int64, opts *ClientApprovalListOptions) (result *ClientApprovalListResult, err error) {
	op := OperationInfo{
		Service: "ClientApprovals", Operation: "List",
		ResourceType: "client_approval", IsMutation: false,
		ProjectID: bucketID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.ListClientApprovalsParams
	if opts != nil && (opts.Sort != "" || opts.Direction != "" || opts.Page > 0) {
		params = &generated.ListClientApprovalsParams{
			Sort:      omitzero(opts.Sort),
			Direction: omitzero(opts.Direction),
		}
		if opts.Page > 0 {
			var page *int32
			if page, err = pageParam(opts.Page); err != nil {
				return nil, err
			}
			params.Page = page
		}
	}
	resp, err := s.client.parent.gen.ListClientApprovalsWithResponse(ctx, s.client.accountID, bucketID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var approvals []ClientApproval
	if resp.JSON200 != nil {
		for _, ga := range *resp.JSON200 {
			approvals = append(approvals, clientApprovalFromGenerated(ga))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &ClientApprovalListResult{Approvals: approvals, Meta: ListMeta{TotalCount: totalCount, Truncated: hasNextPage(resp.HTTPResponse)}}, nil
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
	if limit > 0 && len(approvals) >= limit {
		return &ClientApprovalListResult{Approvals: approvals[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(approvals), limit)}}, nil
	}

	// Follow pagination via Link headers
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(approvals), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var ga generated.ClientApproval
		if err := json.Unmarshal(raw, &ga); err != nil {
			return nil, fmt.Errorf("failed to parse client approval: %w", err)
		}
		approvals = append(approvals, clientApprovalFromGenerated(ga))
	}

	return &ClientApprovalListResult{Approvals: approvals, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Get returns a client approval by ID.
func (s *ClientApprovalsService) Get(ctx context.Context, approvalID int64) (result *ClientApproval, err error) {
	op := OperationInfo{
		Service: "ClientApprovals", Operation: "Get",
		ResourceType: "client_approval", IsMutation: false,
		ResourceID: approvalID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetClientApprovalWithResponse(ctx, s.client.accountID, approvalID)
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

	approval := clientApprovalFromGenerated(*resp.JSON200)
	return &approval, nil
}

// clientApprovalFromGenerated converts a generated ClientApproval to our clean type.
func clientApprovalFromGenerated(ga generated.ClientApproval) ClientApproval {
	a := ClientApproval{
		Status:           ga.Status,
		VisibleToClients: ga.VisibleToClients,
		CreatedAt:        ga.CreatedAt,
		UpdatedAt:        ga.UpdatedAt,
		Title:            ga.Title,
		InheritsStatus:   ga.InheritsStatus,
		Type:             ga.Type,
		URL:              ga.Url,
		AppURL:           ga.AppUrl,
		BookmarkURL:      deref(ga.BookmarkUrl),
		SubscriptionURL:  deref(ga.SubscriptionUrl),
		Content:          deref(ga.Content),
		Subject:          deref(ga.Subject),
		RepliesCount:     int(deref(ga.RepliesCount)),
		RepliesURL:       deref(ga.RepliesUrl),
		ApprovalStatus:   deref(ga.ApprovalStatus),
	}

	if ga.Id != 0 {
		a.ID = ga.Id
	}

	// Presence is the pointer; a present zero date is a real value.
	if ga.DueOn != nil {
		dueOn := ga.DueOn.String()
		a.DueOn = &dueOn
	}

	if ga.Parent.Id != 0 || ga.Parent.Title != "" {
		a.Parent = &Parent{
			ID:     ga.Parent.Id,
			Title:  ga.Parent.Title,
			Type:   ga.Parent.Type,
			URL:    ga.Parent.Url,
			AppURL: ga.Parent.AppUrl,
		}
	}

	if ga.Bucket.Id != 0 || ga.Bucket.Name != "" {
		a.Bucket = &Bucket{
			ID:   ga.Bucket.Id,
			Name: ga.Bucket.Name,
			Type: ga.Bucket.Type,
		}
	}

	if ga.Creator.Id != 0 || ga.Creator.Name != "" {
		creator := personFromGenerated(ga.Creator)
		a.Creator = &creator
	}

	if ga.Approver != nil {
		approver := personFromGenerated(*ga.Approver)
		a.Approver = &approver
	}

	// Convert responses
	if len(ga.Responses) > 0 {
		a.Responses = make([]ClientApprovalResponse, 0, len(ga.Responses))
		for _, gr := range ga.Responses {
			resp := ClientApprovalResponse{
				Status:           deref(gr.Status),
				VisibleToClients: deref(gr.VisibleToClients),
				CreatedAt:        deref(gr.CreatedAt),
				UpdatedAt:        deref(gr.UpdatedAt),
				Title:            deref(gr.Title),
				InheritsStatus:   deref(gr.InheritsStatus),
				Type:             deref(gr.Type),
				AppURL:           deref(gr.AppUrl),
				BookmarkURL:      deref(gr.BookmarkUrl),
				Content:          deref(gr.Content),
				Approved:         deref(gr.Approved),
			}
			if gr.Id != nil {
				resp.ID = *gr.Id
			}
			if gr.Parent != nil {
				resp.Parent = &Parent{
					ID:     gr.Parent.Id,
					Title:  gr.Parent.Title,
					Type:   gr.Parent.Type,
					URL:    gr.Parent.Url,
					AppURL: gr.Parent.AppUrl,
				}
			}
			if gr.Bucket != nil {
				resp.Bucket = &Bucket{
					ID:   gr.Bucket.Id,
					Name: gr.Bucket.Name,
					Type: gr.Bucket.Type,
				}
			}
			if gr.Creator != nil {
				respCreator := personFromGenerated(*gr.Creator)
				resp.Creator = &respCreator
			}
			a.Responses = append(a.Responses, resp)
		}
	}

	a.ContentAttachments = richTextAttachmentsFromGenerated(ga.ContentAttachments)

	return a
}
