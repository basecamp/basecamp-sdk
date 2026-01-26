package basecamp

import (
	"context"
	"fmt"
	"time"
)

// MessageBoard represents a Basecamp message board in a project.
type MessageBoard struct {
	ID            int64     `json:"id"`
	Status        string    `json:"status"`
	Title         string    `json:"title"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Type          string    `json:"type"`
	URL           string    `json:"url"`
	AppURL        string    `json:"app_url"`
	MessagesCount int       `json:"messages_count"`
	MessagesURL   string    `json:"messages_url"`
	Bucket        *Bucket   `json:"bucket,omitempty"`
	Creator       *Person   `json:"creator,omitempty"`
}

// MessageBoardsService handles message board operations.
type MessageBoardsService struct {
	client *Client
}

// NewMessageBoardsService creates a new MessageBoardsService.
func NewMessageBoardsService(client *Client) *MessageBoardsService {
	return &MessageBoardsService{client: client}
}

// Get returns a message board by ID.
// bucketID is the project ID, boardID is the message board ID.
func (s *MessageBoardsService) Get(ctx context.Context, bucketID, boardID int64) (*MessageBoard, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/message_boards/%d.json", bucketID, boardID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var board MessageBoard
	if err := resp.UnmarshalData(&board); err != nil {
		return nil, fmt.Errorf("failed to parse message board: %w", err)
	}

	return &board, nil
}
