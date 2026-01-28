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
	client *AccountClient
}

// NewMessageTypesService creates a new MessageTypesService.
func NewMessageTypesService(client *AccountClient) *MessageTypesService {
	return &MessageTypesService{client: client}
}

// List returns all message types in a project.
// bucketID is the project ID.
func (s *MessageTypesService) List(ctx context.Context, bucketID int64) (result []MessageType, err error) {
	op := OperationInfo{
		Service: "MessageTypes", Operation: "List",
		ResourceType: "message_type", IsMutation: false,
		BucketID: bucketID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListMessageTypesWithResponse(ctx, s.client.accountID, bucketID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	types := make([]MessageType, 0, len(*resp.JSON200))
	for _, gt := range *resp.JSON200 {
		types = append(types, messageTypeFromGenerated(gt))
	}
	return types, nil
}

// Get returns a message type by ID.
// bucketID is the project ID, typeID is the message type ID.
func (s *MessageTypesService) Get(ctx context.Context, bucketID, typeID int64) (result *MessageType, err error) {
	op := OperationInfo{
		Service: "MessageTypes", Operation: "Get",
		ResourceType: "message_type", IsMutation: false,
		BucketID: bucketID, ResourceID: typeID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetMessageTypeWithResponse(ctx, s.client.accountID, bucketID, typeID)
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

	msgType := messageTypeFromGenerated(resp.JSON200.MessageType)
	return &msgType, nil
}

// Create creates a new message type in a project.
// bucketID is the project ID.
// Returns the created message type.
func (s *MessageTypesService) Create(ctx context.Context, bucketID int64, req *CreateMessageTypeRequest) (result *MessageType, err error) {
	op := OperationInfo{
		Service: "MessageTypes", Operation: "Create",
		ResourceType: "message_type", IsMutation: true,
		BucketID: bucketID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.Name == "" {
		err = ErrUsage("message type name is required")
		return nil, err
	}
	if req.Icon == "" {
		err = ErrUsage("message type icon is required")
		return nil, err
	}

	body := generated.CreateMessageTypeJSONRequestBody{
		Name: req.Name,
		Icon: req.Icon,
	}

	resp, err := s.client.parent.gen.CreateMessageTypeWithResponse(ctx, s.client.accountID, bucketID, body)
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

	msgType := messageTypeFromGenerated(resp.JSON200.MessageType)
	return &msgType, nil
}

// Update updates an existing message type.
// bucketID is the project ID, typeID is the message type ID.
// Returns the updated message type.
func (s *MessageTypesService) Update(ctx context.Context, bucketID, typeID int64, req *UpdateMessageTypeRequest) (result *MessageType, err error) {
	op := OperationInfo{
		Service: "MessageTypes", Operation: "Update",
		ResourceType: "message_type", IsMutation: true,
		BucketID: bucketID, ResourceID: typeID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil {
		err = ErrUsage("update request is required")
		return nil, err
	}

	body := generated.UpdateMessageTypeJSONRequestBody{
		Name: req.Name,
		Icon: req.Icon,
	}

	resp, err := s.client.parent.gen.UpdateMessageTypeWithResponse(ctx, s.client.accountID, bucketID, typeID, body)
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

	msgType := messageTypeFromGenerated(resp.JSON200.MessageType)
	return &msgType, nil
}

// Delete deletes a message type from a project.
// bucketID is the project ID, typeID is the message type ID.
func (s *MessageTypesService) Delete(ctx context.Context, bucketID, typeID int64) (err error) {
	op := OperationInfo{
		Service: "MessageTypes", Operation: "Delete",
		ResourceType: "message_type", IsMutation: true,
		BucketID: bucketID, ResourceID: typeID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DeleteMessageTypeWithResponse(ctx, s.client.accountID, bucketID, typeID)
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
