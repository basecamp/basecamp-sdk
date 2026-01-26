package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// MessageType represents a Basecamp message type (category) in a project.
type MessageType struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateMessageTypeRequest specifies the parameters for creating a message type.
type CreateMessageTypeRequest struct {
	// Name is the message type name (required).
	Name string `json:"name"`
	// Icon is the message type icon (required).
	Icon string `json:"icon"`
}

// UpdateMessageTypeRequest specifies the parameters for updating a message type.
type UpdateMessageTypeRequest struct {
	// Name is the message type name (optional).
	Name string `json:"name,omitempty"`
	// Icon is the message type icon (optional).
	Icon string `json:"icon,omitempty"`
}

// MessageTypesService handles message type operations.
type MessageTypesService struct {
	client *Client
}

// NewMessageTypesService creates a new MessageTypesService.
func NewMessageTypesService(client *Client) *MessageTypesService {
	return &MessageTypesService{client: client}
}

// List returns all message types in a project.
// bucketID is the project ID.
func (s *MessageTypesService) List(ctx context.Context, bucketID int64) ([]MessageType, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/categories.json", bucketID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	types := make([]MessageType, 0, len(results))
	for _, raw := range results {
		var t MessageType
		if err := json.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("failed to parse message type: %w", err)
		}
		types = append(types, t)
	}

	return types, nil
}

// Get returns a message type by ID.
// bucketID is the project ID, typeID is the message type ID.
func (s *MessageTypesService) Get(ctx context.Context, bucketID, typeID int64) (*MessageType, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/categories/%d.json", bucketID, typeID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var msgType MessageType
	if err := resp.UnmarshalData(&msgType); err != nil {
		return nil, fmt.Errorf("failed to parse message type: %w", err)
	}

	return &msgType, nil
}

// Create creates a new message type in a project.
// bucketID is the project ID.
// Returns the created message type.
func (s *MessageTypesService) Create(ctx context.Context, bucketID int64, req *CreateMessageTypeRequest) (*MessageType, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Name == "" {
		return nil, ErrUsage("message type name is required")
	}
	if req.Icon == "" {
		return nil, ErrUsage("message type icon is required")
	}

	path := fmt.Sprintf("/buckets/%d/categories.json", bucketID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var msgType MessageType
	if err := resp.UnmarshalData(&msgType); err != nil {
		return nil, fmt.Errorf("failed to parse message type: %w", err)
	}

	return &msgType, nil
}

// Update updates an existing message type.
// bucketID is the project ID, typeID is the message type ID.
// Returns the updated message type.
func (s *MessageTypesService) Update(ctx context.Context, bucketID, typeID int64, req *UpdateMessageTypeRequest) (*MessageType, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil {
		return nil, ErrUsage("update request is required")
	}

	path := fmt.Sprintf("/buckets/%d/categories/%d.json", bucketID, typeID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var msgType MessageType
	if err := resp.UnmarshalData(&msgType); err != nil {
		return nil, fmt.Errorf("failed to parse message type: %w", err)
	}

	return &msgType, nil
}

// Delete deletes a message type from a project.
// bucketID is the project ID, typeID is the message type ID.
func (s *MessageTypesService) Delete(ctx context.Context, bucketID, typeID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/categories/%d.json", bucketID, typeID)
	_, err := s.client.Delete(ctx, path)
	return err
}
