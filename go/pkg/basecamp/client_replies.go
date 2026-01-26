package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ClientReply represents a reply to a client correspondence or approval.
type ClientReply struct {
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
	Content          string    `json:"content"`
}

// ClientRepliesService handles client reply operations.
type ClientRepliesService struct {
	client *Client
}

// NewClientRepliesService creates a new ClientRepliesService.
func NewClientRepliesService(client *Client) *ClientRepliesService {
	return &ClientRepliesService{client: client}
}

// List returns all replies for a client recording (correspondence or approval).
// bucketID is the project ID, recordingID is the parent correspondence/approval ID.
func (s *ClientRepliesService) List(ctx context.Context, bucketID, recordingID int64) ([]ClientReply, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/client/recordings/%d/replies.json", bucketID, recordingID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	replies := make([]ClientReply, 0, len(results))
	for _, raw := range results {
		var r ClientReply
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("failed to parse client reply: %w", err)
		}
		replies = append(replies, r)
	}

	return replies, nil
}

// Get returns a specific client reply.
// bucketID is the project ID, recordingID is the parent correspondence/approval ID,
// replyID is the client reply ID.
func (s *ClientRepliesService) Get(ctx context.Context, bucketID, recordingID, replyID int64) (*ClientReply, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/client/recordings/%d/replies/%d.json", bucketID, recordingID, replyID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var reply ClientReply
	if err := resp.UnmarshalData(&reply); err != nil {
		return nil, fmt.Errorf("failed to parse client reply: %w", err)
	}

	return &reply, nil
}
