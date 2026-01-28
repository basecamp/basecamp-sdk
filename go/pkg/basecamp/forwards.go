package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

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
// bucketID is the project ID, inboxID is the inbox ID.
func (s *ForwardsService) GetInbox(ctx context.Context, bucketID, inboxID int64) (result *Inbox, err error) {
	op := OperationInfo{
		Service: "Forwards", Operation: "GetInbox",
		ResourceType: "inbox", IsMutation: false,
		BucketID: bucketID, ResourceID: inboxID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.GetInboxWithResponse(ctx, s.client.accountID, bucketID, inboxID)
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

	inbox := inboxFromGenerated(resp.JSON200.Inbox)
	return &inbox, nil
}

// List returns all forwards in an inbox.
// bucketID is the project ID, inboxID is the inbox ID.
func (s *ForwardsService) List(ctx context.Context, bucketID, inboxID int64) (result []Forward, err error) {
	op := OperationInfo{
		Service: "Forwards", Operation: "List",
		ResourceType: "forward", IsMutation: false,
		BucketID: bucketID, ResourceID: inboxID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.ListForwardsWithResponse(ctx, s.client.accountID, bucketID, inboxID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	forwards := make([]Forward, 0, len(*resp.JSON200))
	for _, gf := range *resp.JSON200 {
		forwards = append(forwards, forwardFromGenerated(gf))
	}

	return forwards, nil
}

// Get returns a forward by ID.
// bucketID is the project ID, forwardID is the forward ID.
func (s *ForwardsService) Get(ctx context.Context, bucketID, forwardID int64) (result *Forward, err error) {
	op := OperationInfo{
		Service: "Forwards", Operation: "Get",
		ResourceType: "forward", IsMutation: false,
		BucketID: bucketID, ResourceID: forwardID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.GetForwardWithResponse(ctx, s.client.accountID, bucketID, forwardID)
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

	forward := forwardFromGenerated(resp.JSON200.Forward)
	return &forward, nil
}

// ListReplies returns all replies to a forward.
// bucketID is the project ID, forwardID is the forward ID.
func (s *ForwardsService) ListReplies(ctx context.Context, bucketID, forwardID int64) (result []ForwardReply, err error) {
	op := OperationInfo{
		Service: "Forwards", Operation: "ListReplies",
		ResourceType: "forward_reply", IsMutation: false,
		BucketID: bucketID, ResourceID: forwardID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.ListForwardRepliesWithResponse(ctx, s.client.accountID, bucketID, forwardID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	replies := make([]ForwardReply, 0, len(*resp.JSON200))
	for _, gr := range *resp.JSON200 {
		replies = append(replies, forwardReplyFromGenerated(gr))
	}

	return replies, nil
}

// GetReply returns a forward reply by ID.
// bucketID is the project ID, forwardID is the forward ID, replyID is the reply ID.
func (s *ForwardsService) GetReply(ctx context.Context, bucketID, forwardID, replyID int64) (result *ForwardReply, err error) {
	op := OperationInfo{
		Service: "Forwards", Operation: "GetReply",
		ResourceType: "forward_reply", IsMutation: false,
		BucketID: bucketID, ResourceID: replyID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.GetForwardReplyWithResponse(ctx, s.client.accountID, bucketID, forwardID, replyID)
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

	reply := forwardReplyFromGenerated(resp.JSON200.Reply)
	return &reply, nil
}

// CreateReply creates a new reply to a forwarded email.
// bucketID is the project ID, forwardID is the forward ID.
// Returns the created reply.
func (s *ForwardsService) CreateReply(ctx context.Context, bucketID, forwardID int64, req *CreateForwardReplyRequest) (result *ForwardReply, err error) {
	op := OperationInfo{
		Service: "Forwards", Operation: "CreateReply",
		ResourceType: "forward_reply", IsMutation: true,
		BucketID: bucketID, ResourceID: forwardID,
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

	resp, err := s.client.gen.CreateForwardReplyWithResponse(ctx, s.client.accountID, bucketID, forwardID, body)
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

	reply := forwardReplyFromGenerated(resp.JSON200.Reply)
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

	if gi.Id != nil {
		i.ID = *gi.Id
	}

	if gi.Bucket.Id != nil || gi.Bucket.Name != "" {
		i.Bucket = &Bucket{
			ID:   derefInt64(gi.Bucket.Id),
			Name: gi.Bucket.Name,
			Type: gi.Bucket.Type,
		}
	}

	if gi.Creator.Id != nil || gi.Creator.Name != "" {
		i.Creator = &Person{
			ID:           derefInt64(gi.Creator.Id),
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

	if gf.Id != nil {
		f.ID = *gf.Id
	}

	if gf.Parent.Id != nil || gf.Parent.Title != "" {
		f.Parent = &Parent{
			ID:     derefInt64(gf.Parent.Id),
			Title:  gf.Parent.Title,
			Type:   gf.Parent.Type,
			URL:    gf.Parent.Url,
			AppURL: gf.Parent.AppUrl,
		}
	}

	if gf.Bucket.Id != nil || gf.Bucket.Name != "" {
		f.Bucket = &Bucket{
			ID:   derefInt64(gf.Bucket.Id),
			Name: gf.Bucket.Name,
			Type: gf.Bucket.Type,
		}
	}

	if gf.Creator.Id != nil || gf.Creator.Name != "" {
		f.Creator = &Person{
			ID:           derefInt64(gf.Creator.Id),
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

	if gr.Id != nil {
		r.ID = *gr.Id
	}

	if gr.Parent.Id != nil || gr.Parent.Title != "" {
		r.Parent = &Parent{
			ID:     derefInt64(gr.Parent.Id),
			Title:  gr.Parent.Title,
			Type:   gr.Parent.Type,
			URL:    gr.Parent.Url,
			AppURL: gr.Parent.AppUrl,
		}
	}

	if gr.Bucket.Id != nil || gr.Bucket.Name != "" {
		r.Bucket = &Bucket{
			ID:   derefInt64(gr.Bucket.Id),
			Name: gr.Bucket.Name,
			Type: gr.Bucket.Type,
		}
	}

	if gr.Creator.Id != nil || gr.Creator.Name != "" {
		r.Creator = &Person{
			ID:           derefInt64(gr.Creator.Id),
			Name:         gr.Creator.Name,
			EmailAddress: gr.Creator.EmailAddress,
			AvatarURL:    gr.Creator.AvatarUrl,
			Admin:        gr.Creator.Admin,
			Owner:        gr.Creator.Owner,
		}
	}

	return r
}
