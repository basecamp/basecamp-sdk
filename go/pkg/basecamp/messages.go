package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Message represents a Basecamp message on a message board.
type Message struct {
	ID        int64        `json:"id"`
	Status    string       `json:"status"`
	Subject   string       `json:"subject"`
	Content   string       `json:"content"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	Type      string       `json:"type"`
	URL       string       `json:"url"`
	AppURL    string       `json:"app_url"`
	Parent    *Parent      `json:"parent,omitempty"`
	Bucket    *Bucket      `json:"bucket,omitempty"`
	Creator   *Person      `json:"creator,omitempty"`
	Category  *MessageType `json:"category,omitempty"`
}

// CreateMessageRequest specifies the parameters for creating a message.
type CreateMessageRequest struct {
	// Subject is the message title (required).
	Subject string `json:"subject"`
	// Content is the message body in HTML (optional).
	Content string `json:"content,omitempty"`
	// Status is either "drafted" or "active" (optional, defaults to active).
	Status string `json:"status,omitempty"`
	// CategoryID is the message type ID (optional).
	CategoryID int64 `json:"category_id,omitempty"`
}

// UpdateMessageRequest specifies the parameters for updating a message.
type UpdateMessageRequest struct {
	// Subject is the message title (optional).
	Subject string `json:"subject,omitempty"`
	// Content is the message body in HTML (optional).
	Content string `json:"content,omitempty"`
	// Status is either "drafted" or "active" (optional).
	Status string `json:"status,omitempty"`
	// CategoryID is the message type ID (optional).
	CategoryID int64 `json:"category_id,omitempty"`
}

// MessagesService handles message operations.
type MessagesService struct {
	client *Client
}

// NewMessagesService creates a new MessagesService.
func NewMessagesService(client *Client) *MessagesService {
	return &MessagesService{client: client}
}

// List returns all messages on a message board.
// bucketID is the project ID, boardID is the message board ID.
func (s *MessagesService) List(ctx context.Context, bucketID, boardID int64) ([]Message, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/message_boards/%d/messages.json", bucketID, boardID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	messages := make([]Message, 0, len(results))
	for _, raw := range results {
		var m Message
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("failed to parse message: %w", err)
		}
		messages = append(messages, m)
	}

	return messages, nil
}

// Get returns a message by ID.
// bucketID is the project ID, messageID is the message ID.
func (s *MessagesService) Get(ctx context.Context, bucketID, messageID int64) (*Message, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/messages/%d.json", bucketID, messageID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var message Message
	if err := resp.UnmarshalData(&message); err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}

	return &message, nil
}

// Create creates a new message on a message board.
// bucketID is the project ID, boardID is the message board ID.
// Returns the created message.
func (s *MessagesService) Create(ctx context.Context, bucketID, boardID int64, req *CreateMessageRequest) (*Message, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Subject == "" {
		return nil, ErrUsage("message subject is required")
	}

	path := fmt.Sprintf("/buckets/%d/message_boards/%d/messages.json", bucketID, boardID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var message Message
	if err := resp.UnmarshalData(&message); err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}

	return &message, nil
}

// Update updates an existing message.
// bucketID is the project ID, messageID is the message ID.
// Returns the updated message.
func (s *MessagesService) Update(ctx context.Context, bucketID, messageID int64, req *UpdateMessageRequest) (*Message, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil {
		return nil, ErrUsage("update request is required")
	}

	path := fmt.Sprintf("/buckets/%d/messages/%d.json", bucketID, messageID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var message Message
	if err := resp.UnmarshalData(&message); err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}

	return &message, nil
}

// Pin pins a message to the top of the message board.
// bucketID is the project ID, messageID is the message ID.
func (s *MessagesService) Pin(ctx context.Context, bucketID, messageID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/pin.json", bucketID, messageID)
	_, err := s.client.Post(ctx, path, nil)
	return err
}

// Unpin unpins a message from the top of the message board.
// bucketID is the project ID, messageID is the message ID.
func (s *MessagesService) Unpin(ctx context.Context, bucketID, messageID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/pin.json", bucketID, messageID)
	_, err := s.client.Delete(ctx, path)
	return err
}

// Trash moves a message to the trash.
// bucketID is the project ID, messageID is the message ID.
// Trashed messages can be recovered from the trash.
func (s *MessagesService) Trash(ctx context.Context, bucketID, messageID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/status/trashed.json", bucketID, messageID)
	_, err := s.client.Put(ctx, path, nil)
	return err
}

// Archive moves a message to the archive.
// bucketID is the project ID, messageID is the message ID.
// Archived messages can be unarchived.
func (s *MessagesService) Archive(ctx context.Context, bucketID, messageID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/status/archived.json", bucketID, messageID)
	_, err := s.client.Put(ctx, path, nil)
	return err
}

// Unarchive restores an archived message to active status.
// bucketID is the project ID, messageID is the message ID.
func (s *MessagesService) Unarchive(ctx context.Context, bucketID, messageID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/status/active.json", bucketID, messageID)
	_, err := s.client.Put(ctx, path, nil)
	return err
}
