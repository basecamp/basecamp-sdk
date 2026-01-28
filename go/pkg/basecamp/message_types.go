package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
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

	resp, err := s.client.gen.ListMessageTypesWithResponse(ctx, bucketID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	types := make([]MessageType, 0, len(resp.JSON200.MessageTypes))
	for _, gt := range resp.JSON200.MessageTypes {
		types = append(types, messageTypeFromGenerated(gt))
	}
	return types, nil
}

// Get returns a message type by ID.
// bucketID is the project ID, typeID is the message type ID.
func (s *MessageTypesService) Get(ctx context.Context, bucketID, typeID int64) (*MessageType, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetMessageTypeWithResponse(ctx, bucketID, typeID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	msgType := messageTypeFromGenerated(resp.JSON200.MessageType)
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

	body := generated.CreateMessageTypeJSONRequestBody{
		Name: req.Name,
		Icon: req.Icon,
	}

	resp, err := s.client.gen.CreateMessageTypeWithResponse(ctx, bucketID, body)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	msgType := messageTypeFromGenerated(resp.JSON200.MessageType)
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

	body := generated.UpdateMessageTypeJSONRequestBody{
		Name: req.Name,
		Icon: req.Icon,
	}

	resp, err := s.client.gen.UpdateMessageTypeWithResponse(ctx, bucketID, typeID, body)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	msgType := messageTypeFromGenerated(resp.JSON200.MessageType)
	return &msgType, nil
}

// Delete deletes a message type from a project.
// bucketID is the project ID, typeID is the message type ID.
func (s *MessageTypesService) Delete(ctx context.Context, bucketID, typeID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.DeleteMessageTypeWithResponse(ctx, bucketID, typeID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// messageTypeFromGenerated converts a generated MessageType to our clean MessageType type.
func messageTypeFromGenerated(gt generated.MessageType) MessageType {
	mt := MessageType{
		Name:      gt.Name,
		Icon:      gt.Icon,
		CreatedAt: gt.CreatedAt,
		UpdatedAt: gt.UpdatedAt,
	}

	if gt.Id != nil {
		mt.ID = *gt.Id
	}

	return mt
}
