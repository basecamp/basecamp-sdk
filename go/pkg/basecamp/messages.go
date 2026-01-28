package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
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
	client *AccountClient
}

// NewMessagesService creates a new MessagesService.
func NewMessagesService(client *AccountClient) *MessagesService {
	return &MessagesService{client: client}
}

// List returns all messages on a message board.
// bucketID is the project ID, boardID is the message board ID.
func (s *MessagesService) List(ctx context.Context, bucketID, boardID int64) (result []Message, err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "List",
		ResourceType: "message", IsMutation: false,
		BucketID: bucketID, ResourceID: boardID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.ListMessagesWithResponse(ctx, s.client.accountID, bucketID, boardID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	messages := make([]Message, 0, len(*resp.JSON200))
	for _, gm := range *resp.JSON200 {
		messages = append(messages, messageFromGenerated(gm))
	}
	return messages, nil
}

// Get returns a message by ID.
// bucketID is the project ID, messageID is the message ID.
func (s *MessagesService) Get(ctx context.Context, bucketID, messageID int64) (result *Message, err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Get",
		ResourceType: "message", IsMutation: false,
		BucketID: bucketID, ResourceID: messageID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.GetMessageWithResponse(ctx, s.client.accountID, bucketID, messageID)
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

	message := messageFromGenerated(resp.JSON200.Message)
	return &message, nil
}

// Create creates a new message on a message board.
// bucketID is the project ID, boardID is the message board ID.
// Returns the created message.
func (s *MessagesService) Create(ctx context.Context, bucketID, boardID int64, req *CreateMessageRequest) (result *Message, err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Create",
		ResourceType: "message", IsMutation: true,
		BucketID: bucketID, ResourceID: boardID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.Subject == "" {
		err = ErrUsage("message subject is required")
		return nil, err
	}

	body := generated.CreateMessageJSONRequestBody{
		Subject: req.Subject,
		Content: req.Content,
		Status:  req.Status,
	}
	if req.CategoryID != 0 {
		body.CategoryId = &req.CategoryID
	}

	resp, err := s.client.gen.CreateMessageWithResponse(ctx, s.client.accountID, bucketID, boardID, body)
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

	message := messageFromGenerated(resp.JSON200.Message)
	return &message, nil
}

// Update updates an existing message.
// bucketID is the project ID, messageID is the message ID.
// Returns the updated message.
func (s *MessagesService) Update(ctx context.Context, bucketID, messageID int64, req *UpdateMessageRequest) (result *Message, err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Update",
		ResourceType: "message", IsMutation: true,
		BucketID: bucketID, ResourceID: messageID,
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

	body := generated.UpdateMessageJSONRequestBody{
		Subject: req.Subject,
		Content: req.Content,
		Status:  req.Status,
	}
	if req.CategoryID != 0 {
		body.CategoryId = &req.CategoryID
	}

	resp, err := s.client.gen.UpdateMessageWithResponse(ctx, s.client.accountID, bucketID, messageID, body)
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

	message := messageFromGenerated(resp.JSON200.Message)
	return &message, nil
}

// Pin pins a message to the top of the message board.
// bucketID is the project ID, messageID is the message ID.
func (s *MessagesService) Pin(ctx context.Context, bucketID, messageID int64) (err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Pin",
		ResourceType: "message", IsMutation: true,
		BucketID: bucketID, ResourceID: messageID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.PinMessageWithResponse(ctx, s.client.accountID, bucketID, messageID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Unpin unpins a message from the top of the message board.
// bucketID is the project ID, messageID is the message ID.
func (s *MessagesService) Unpin(ctx context.Context, bucketID, messageID int64) (err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Unpin",
		ResourceType: "message", IsMutation: true,
		BucketID: bucketID, ResourceID: messageID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.UnpinMessageWithResponse(ctx, s.client.accountID, bucketID, messageID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Trash moves a message to the trash.
// bucketID is the project ID, messageID is the message ID.
// Trashed messages can be recovered from the trash.
func (s *MessagesService) Trash(ctx context.Context, bucketID, messageID int64) (err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Trash",
		ResourceType: "message", IsMutation: true,
		BucketID: bucketID, ResourceID: messageID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.TrashRecordingWithResponse(ctx, s.client.accountID, bucketID, messageID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Archive moves a message to the archive.
// bucketID is the project ID, messageID is the message ID.
// Archived messages can be unarchived.
func (s *MessagesService) Archive(ctx context.Context, bucketID, messageID int64) (err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Archive",
		ResourceType: "message", IsMutation: true,
		BucketID: bucketID, ResourceID: messageID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.ArchiveRecordingWithResponse(ctx, s.client.accountID, bucketID, messageID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Unarchive restores an archived message to active status.
// bucketID is the project ID, messageID is the message ID.
func (s *MessagesService) Unarchive(ctx context.Context, bucketID, messageID int64) (err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Unarchive",
		ResourceType: "message", IsMutation: true,
		BucketID: bucketID, ResourceID: messageID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.UnarchiveRecordingWithResponse(ctx, s.client.accountID, bucketID, messageID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// messageFromGenerated converts a generated Message to our clean Message type.
func messageFromGenerated(gm generated.Message) Message {
	m := Message{
		Status:    gm.Status,
		Subject:   gm.Subject,
		Content:   gm.Content,
		Type:      gm.Type,
		URL:       gm.Url,
		AppURL:    gm.AppUrl,
		CreatedAt: gm.CreatedAt,
		UpdatedAt: gm.UpdatedAt,
	}

	if gm.Id != nil {
		m.ID = *gm.Id
	}

	// Convert nested types
	if gm.Parent.Id != nil || gm.Parent.Title != "" {
		m.Parent = &Parent{
			ID:     derefInt64(gm.Parent.Id),
			Title:  gm.Parent.Title,
			Type:   gm.Parent.Type,
			URL:    gm.Parent.Url,
			AppURL: gm.Parent.AppUrl,
		}
	}

	if gm.Bucket.Id != nil || gm.Bucket.Name != "" {
		m.Bucket = &Bucket{
			ID:   derefInt64(gm.Bucket.Id),
			Name: gm.Bucket.Name,
			Type: gm.Bucket.Type,
		}
	}

	if gm.Creator.Id != nil || gm.Creator.Name != "" {
		m.Creator = &Person{
			ID:           derefInt64(gm.Creator.Id),
			Name:         gm.Creator.Name,
			EmailAddress: gm.Creator.EmailAddress,
			AvatarURL:    gm.Creator.AvatarUrl,
			Admin:        gm.Creator.Admin,
			Owner:        gm.Creator.Owner,
		}
	}

	if gm.Category.Id != nil || gm.Category.Name != "" {
		m.Category = &MessageType{
			ID:        derefInt64(gm.Category.Id),
			Name:      gm.Category.Name,
			Icon:      gm.Category.Icon,
			CreatedAt: gm.Category.CreatedAt,
			UpdatedAt: gm.Category.UpdatedAt,
		}
	}

	return m
}
