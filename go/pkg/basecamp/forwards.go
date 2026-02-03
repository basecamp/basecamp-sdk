package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// ForwardListOptions specifies options for listing forwards.
type ForwardListOptions struct {
	// Limit is the maximum number of forwards to return.
	// If 0 (default), returns all forwards. Use a positive value to cap results.
	Limit int

	// Page, if non-zero, disables pagination and returns only the first page.
	// NOTE: The page number itself is not yet honored due to OpenAPI client
	// limitations. Use 0 to paginate through all results up to Limit.
	Page int
}

// ForwardReplyListOptions specifies options for listing forward replies.
type ForwardReplyListOptions struct {
	// Limit is the maximum number of replies to return.
	// If 0 (default), returns all replies. Use a positive value to cap results.
	Limit int

	// Page, if non-zero, disables pagination and returns only the first page.
	// NOTE: The page number itself is not yet honored due to OpenAPI client
	// limitations. Use 0 to paginate through all results up to Limit.
	Page int
}

// ForwardListResult contains the results from listing forwards.
type ForwardListResult struct {
	// Forwards is the list of forwards returned.
	Forwards []Forward
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// ForwardReplyListResult contains the results from listing forward replies.
type ForwardReplyListResult struct {
	// Replies is the list of forward replies returned.
	Replies []ForwardReply
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// Inbox represents a Basecamp email inbox (forwards tool).
type Inbox struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Title     string    `json:"title"`
	Type      string    `json:"type"`
	URL       string    `json:"url"`
	AppURL    string    `json:"app_url"`
	Bucket    *Bucket   `json:"bucket,omitempty"`
	Creator   *Person   `json:"creator,omitempty"`
}

// Forward represents a forwarded email in Basecamp.
type Forward struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Subject   string    `json:"subject"`
	Content   string    `json:"content"`
	From      string    `json:"from"`
	Type      string    `json:"type"`
	URL       string    `json:"url"`
	AppURL    string    `json:"app_url"`
	Parent    *Parent   `json:"parent,omitempty"`
	Bucket    *Bucket   `json:"bucket,omitempty"`
	Creator   *Person   `json:"creator,omitempty"`
}

// ForwardReply represents a reply to a forwarded email.
type ForwardReply struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Content   string    `json:"content"`
	Type      string    `json:"type"`
	URL       string    `json:"url"`
	AppURL    string    `json:"app_url"`
	Parent    *Parent   `json:"parent,omitempty"`
	Bucket    *Bucket   `json:"bucket,omitempty"`
	Creator   *Person   `json:"creator,omitempty"`
}

// CreateForwardReplyRequest specifies the parameters for creating a reply to a forward.
type CreateForwardReplyRequest struct {
	// Content is the reply body in HTML (required).
	Content string `json:"content"`
}

// ForwardsService handles email forward operations.
type ForwardsService struct {
	client *AccountClient
}

// NewForwardsService creates a new ForwardsService.
func NewForwardsService(client *AccountClient) *ForwardsService {
	return &ForwardsService{client: client}
}

// GetInbox returns an inbox by ID.
func (s *ForwardsService) GetInbox(ctx context.Context, inboxID int64) (result *Inbox, err error) {
	op := OperationInfo{
		Service: "Forwards", Operation: "GetInbox",
		ResourceType: "inbox", IsMutation: false,
		ResourceID: inboxID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetInboxWithResponse(ctx, s.client.accountID, inboxID)
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

	inbox := inboxFromGenerated(*resp.JSON200)
	return &inbox, nil
}

// List returns all forwards in an inbox.
//
// By default, returns all forwards (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of forwards to return (0 = all)
//   - Page: if non-zero, disables pagination and returns first page only
//
// The returned ForwardListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *ForwardsService) List(ctx context.Context, inboxID int64, opts *ForwardListOptions) (result *ForwardListResult, err error) {
	op := OperationInfo{
		Service: "Forwards", Operation: "List",
		ResourceType: "forward", IsMutation: false,
		ResourceID: inboxID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListForwardsWithResponse(ctx, s.client.accountID, inboxID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var forwards []Forward
	if resp.JSON200 != nil {
		for _, gf := range *resp.JSON200 {
			forwards = append(forwards, forwardFromGenerated(gf))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &ForwardListResult{Forwards: forwards, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = all (default for forwards), >0 = specific limit
	limit := 0 // default to all for forwards
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(forwards) >= limit {
		return &ForwardListResult{Forwards: forwards[:limit], Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, err := s.client.parent.FollowPagination(ctx, resp.HTTPResponse, len(forwards), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gf generated.Forward
		if err := json.Unmarshal(raw, &gf); err != nil {
			return nil, fmt.Errorf("failed to parse forward: %w", err)
		}
		forwards = append(forwards, forwardFromGenerated(gf))
	}

	return &ForwardListResult{Forwards: forwards, Meta: ListMeta{TotalCount: totalCount}}, nil
}

// Get returns a forward by ID.
func (s *ForwardsService) Get(ctx context.Context, forwardID int64) (result *Forward, err error) {
	op := OperationInfo{
		Service: "Forwards", Operation: "Get",
		ResourceType: "forward", IsMutation: false,
		ResourceID: forwardID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetForwardWithResponse(ctx, s.client.accountID, forwardID)
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

	forward := forwardFromGenerated(*resp.JSON200)
	return &forward, nil
}

// ListReplies returns all replies to a forward.
//
// By default, returns all replies (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of replies to return (0 = all)
//   - Page: if non-zero, disables pagination and returns first page only
//
// The returned ForwardReplyListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *ForwardsService) ListReplies(ctx context.Context, forwardID int64, opts *ForwardReplyListOptions) (result *ForwardReplyListResult, err error) {
	op := OperationInfo{
		Service: "Forwards", Operation: "ListReplies",
		ResourceType: "forward_reply", IsMutation: false,
		ResourceID: forwardID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListForwardRepliesWithResponse(ctx, s.client.accountID, forwardID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var replies []ForwardReply
	if resp.JSON200 != nil {
		for _, gr := range *resp.JSON200 {
			replies = append(replies, forwardReplyFromGenerated(gr))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &ForwardReplyListResult{Replies: replies, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = all (default for replies), >0 = specific limit
	limit := 0 // default to all for replies
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(replies) >= limit {
		return &ForwardReplyListResult{Replies: replies[:limit], Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, err := s.client.parent.FollowPagination(ctx, resp.HTTPResponse, len(replies), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gr generated.ForwardReply
		if err := json.Unmarshal(raw, &gr); err != nil {
			return nil, fmt.Errorf("failed to parse forward reply: %w", err)
		}
		replies = append(replies, forwardReplyFromGenerated(gr))
	}

	return &ForwardReplyListResult{Replies: replies, Meta: ListMeta{TotalCount: totalCount}}, nil
}

// GetReply returns a forward reply by ID.
func (s *ForwardsService) GetReply(ctx context.Context, forwardID, replyID int64) (result *ForwardReply, err error) {
	op := OperationInfo{
		Service: "Forwards", Operation: "GetReply",
		ResourceType: "forward_reply", IsMutation: false,
		ResourceID: replyID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetForwardReplyWithResponse(ctx, s.client.accountID, forwardID, replyID)
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

	reply := forwardReplyFromGenerated(*resp.JSON200)
	return &reply, nil
}

// CreateReply creates a new reply to a forwarded email.
// Returns the created reply.
func (s *ForwardsService) CreateReply(ctx context.Context, forwardID int64, req *CreateForwardReplyRequest) (result *ForwardReply, err error) {
	op := OperationInfo{
		Service: "Forwards", Operation: "CreateReply",
		ResourceType: "forward_reply", IsMutation: true,
		ResourceID: forwardID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.Content == "" {
		err = ErrUsage("reply content is required")
		return nil, err
	}

	body := generated.CreateForwardReplyJSONRequestBody{
		Content: req.Content,
	}

	resp, err := s.client.parent.gen.CreateForwardReplyWithResponse(ctx, s.client.accountID, forwardID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	reply := forwardReplyFromGenerated(*resp.JSON201)
	return &reply, nil
}

// inboxFromGenerated converts a generated Inbox to our clean type.
func inboxFromGenerated(gi generated.Inbox) Inbox {
	i := Inbox{
		Status:    gi.Status,
		CreatedAt: gi.CreatedAt,
		UpdatedAt: gi.UpdatedAt,
		Title:     gi.Title,
		Type:      gi.Type,
		URL:       gi.Url,
		AppURL:    gi.AppUrl,
	}

	if gi.Id != 0 {
		i.ID = gi.Id
	}

	if gi.Bucket.Id != 0 || gi.Bucket.Name != "" {
		i.Bucket = &Bucket{
			ID:   gi.Bucket.Id,
			Name: gi.Bucket.Name,
			Type: gi.Bucket.Type,
		}
	}

	if gi.Creator.Id != 0 || gi.Creator.Name != "" {
		i.Creator = &Person{
			ID:           gi.Creator.Id,
			Name:         gi.Creator.Name,
			EmailAddress: gi.Creator.EmailAddress,
			AvatarURL:    gi.Creator.AvatarUrl,
			Admin:        gi.Creator.Admin,
			Owner:        gi.Creator.Owner,
		}
	}

	return i
}

// forwardFromGenerated converts a generated Forward to our clean type.
func forwardFromGenerated(gf generated.Forward) Forward {
	f := Forward{
		Status:    gf.Status,
		CreatedAt: gf.CreatedAt,
		UpdatedAt: gf.UpdatedAt,
		Subject:   gf.Subject,
		Content:   gf.Content,
		From:      gf.From,
		Type:      gf.Type,
		URL:       gf.Url,
		AppURL:    gf.AppUrl,
	}

	if gf.Id != 0 {
		f.ID = gf.Id
	}

	if gf.Parent.Id != 0 || gf.Parent.Title != "" {
		f.Parent = &Parent{
			ID:     gf.Parent.Id,
			Title:  gf.Parent.Title,
			Type:   gf.Parent.Type,
			URL:    gf.Parent.Url,
			AppURL: gf.Parent.AppUrl,
		}
	}

	if gf.Bucket.Id != 0 || gf.Bucket.Name != "" {
		f.Bucket = &Bucket{
			ID:   gf.Bucket.Id,
			Name: gf.Bucket.Name,
			Type: gf.Bucket.Type,
		}
	}

	if gf.Creator.Id != 0 || gf.Creator.Name != "" {
		f.Creator = &Person{
			ID:           gf.Creator.Id,
			Name:         gf.Creator.Name,
			EmailAddress: gf.Creator.EmailAddress,
			AvatarURL:    gf.Creator.AvatarUrl,
			Admin:        gf.Creator.Admin,
			Owner:        gf.Creator.Owner,
		}
	}

	return f
}

// forwardReplyFromGenerated converts a generated ForwardReply to our clean type.
func forwardReplyFromGenerated(gr generated.ForwardReply) ForwardReply {
	r := ForwardReply{
		Status:    gr.Status,
		CreatedAt: gr.CreatedAt,
		UpdatedAt: gr.UpdatedAt,
		Content:   gr.Content,
		Type:      gr.Type,
		URL:       gr.Url,
		AppURL:    gr.AppUrl,
	}

	if gr.Id != 0 {
		r.ID = gr.Id
	}

	if gr.Parent.Id != 0 || gr.Parent.Title != "" {
		r.Parent = &Parent{
			ID:     gr.Parent.Id,
			Title:  gr.Parent.Title,
			Type:   gr.Parent.Type,
			URL:    gr.Parent.Url,
			AppURL: gr.Parent.AppUrl,
		}
	}

	if gr.Bucket.Id != 0 || gr.Bucket.Name != "" {
		r.Bucket = &Bucket{
			ID:   gr.Bucket.Id,
			Name: gr.Bucket.Name,
			Type: gr.Bucket.Type,
		}
	}

	if gr.Creator.Id != 0 || gr.Creator.Name != "" {
		r.Creator = &Person{
			ID:           gr.Creator.Id,
			Name:         gr.Creator.Name,
			EmailAddress: gr.Creator.EmailAddress,
			AvatarURL:    gr.Creator.AvatarUrl,
			Admin:        gr.Creator.Admin,
			Owner:        gr.Creator.Owner,
		}
	}

	return r
}
