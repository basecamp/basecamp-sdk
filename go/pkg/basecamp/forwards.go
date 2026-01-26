package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
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
	client *Client
}

// NewForwardsService creates a new ForwardsService.
func NewForwardsService(client *Client) *ForwardsService {
	return &ForwardsService{client: client}
}

// GetInbox returns an inbox by ID.
// bucketID is the project ID, inboxID is the inbox ID.
func (s *ForwardsService) GetInbox(ctx context.Context, bucketID, inboxID int64) (*Inbox, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/inboxes/%d.json", bucketID, inboxID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var inbox Inbox
	if err := resp.UnmarshalData(&inbox); err != nil {
		return nil, fmt.Errorf("failed to parse inbox: %w", err)
	}

	return &inbox, nil
}

// List returns all forwards in an inbox.
// bucketID is the project ID, inboxID is the inbox ID.
func (s *ForwardsService) List(ctx context.Context, bucketID, inboxID int64) ([]Forward, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/inboxes/%d/forwards.json", bucketID, inboxID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	forwards := make([]Forward, 0, len(results))
	for _, raw := range results {
		var f Forward
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("failed to parse forward: %w", err)
		}
		forwards = append(forwards, f)
	}

	return forwards, nil
}

// Get returns a forward by ID.
// bucketID is the project ID, forwardID is the forward ID.
func (s *ForwardsService) Get(ctx context.Context, bucketID, forwardID int64) (*Forward, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/inbox_forwards/%d.json", bucketID, forwardID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var forward Forward
	if err := resp.UnmarshalData(&forward); err != nil {
		return nil, fmt.Errorf("failed to parse forward: %w", err)
	}

	return &forward, nil
}

// ListReplies returns all replies to a forward.
// bucketID is the project ID, forwardID is the forward ID.
func (s *ForwardsService) ListReplies(ctx context.Context, bucketID, forwardID int64) ([]ForwardReply, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/inbox_forwards/%d/replies.json", bucketID, forwardID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	replies := make([]ForwardReply, 0, len(results))
	for _, raw := range results {
		var r ForwardReply
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("failed to parse forward reply: %w", err)
		}
		replies = append(replies, r)
	}

	return replies, nil
}

// GetReply returns a forward reply by ID.
// bucketID is the project ID, forwardID is the forward ID, replyID is the reply ID.
func (s *ForwardsService) GetReply(ctx context.Context, bucketID, forwardID, replyID int64) (*ForwardReply, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/inbox_forwards/%d/replies/%d.json", bucketID, forwardID, replyID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var reply ForwardReply
	if err := resp.UnmarshalData(&reply); err != nil {
		return nil, fmt.Errorf("failed to parse forward reply: %w", err)
	}

	return &reply, nil
}

// CreateReply creates a new reply to a forwarded email.
// bucketID is the project ID, forwardID is the forward ID.
// Returns the created reply.
func (s *ForwardsService) CreateReply(ctx context.Context, bucketID, forwardID int64, req *CreateForwardReplyRequest) (*ForwardReply, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Content == "" {
		return nil, ErrUsage("reply content is required")
	}

	path := fmt.Sprintf("/buckets/%d/inbox_forwards/%d/replies.json", bucketID, forwardID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var reply ForwardReply
	if err := resp.UnmarshalData(&reply); err != nil {
		return nil, fmt.Errorf("failed to parse forward reply: %w", err)
	}

	return &reply, nil
}
