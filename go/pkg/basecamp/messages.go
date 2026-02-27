package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// DefaultMessageLimit is the default number of messages to return when no limit is specified.
const DefaultMessageLimit = 100

// Message represents a Basecamp message on a message board.
type Message struct {
	ID          int64        `json:"id"`
	Status      string       `json:"status"`
	Subject     string       `json:"subject"`
	Content     string       `json:"content"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Type        string       `json:"type"`
	URL         string       `json:"url"`
	AppURL      string       `json:"app_url"`
	Parent      *Parent      `json:"parent,omitempty"`
	Bucket      *Bucket      `json:"bucket,omitempty"`
	Creator     *Person      `json:"creator,omitempty"`
	Category    *MessageType `json:"category,omitempty"`
	BoostsCount int          `json:"boosts_count,omitempty"`
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

// MessageListOptions specifies options for listing messages.
type MessageListOptions struct {
	// Limit is the maximum number of messages to return.
	// If 0, uses DefaultMessageLimit (100). Use -1 for unlimited.
	Limit int

	// Page, if non-zero, disables pagination and returns only the first page.
	// NOTE: The page number itself is not yet honored due to OpenAPI client
	// limitations. Use 0 to paginate through all results up to Limit.
	Page int
}

// MessageListResult contains the results from listing messages.
type MessageListResult struct {
	// Messages is the list of messages returned.
	Messages []Message
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// MessagesService handles message operations.
type MessagesService struct {
	client *AccountClient
}

// NewMessagesService creates a new MessagesService.
func NewMessagesService(client *AccountClient) *MessagesService {
	return &MessagesService{client: client}
}

// List returns messages on a message board.
//
// By default, returns up to 100 messages. Use Limit: -1 for unlimited.
//
// Pagination options:
//   - Limit: maximum number of messages to return (0 = 100, -1 = unlimited)
//   - Page: if non-zero, disables pagination and returns first page only
//
// The returned MessageListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *MessagesService) List(ctx context.Context, boardID int64, opts *MessageListOptions) (result *MessageListResult, err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "List",
		ResourceType: "message", IsMutation: false,
		ResourceID: boardID,
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
	resp, err := s.client.parent.gen.ListMessagesWithResponse(ctx, s.client.accountID, boardID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var messages []Message
	if resp.JSON200 != nil {
		for _, gm := range *resp.JSON200 {
			messages = append(messages, messageFromGenerated(gm))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &MessageListResult{Messages: messages, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = default (100), -1 = unlimited, >0 = specific limit
	limit := DefaultMessageLimit
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0 // unlimited
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	// Check if we already have enough items
	if limit > 0 && len(messages) >= limit {
		return &MessageListResult{Messages: messages[:limit], Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, err := s.client.parent.FollowPagination(ctx, resp.HTTPResponse, len(messages), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gm generated.Message
		if err := json.Unmarshal(raw, &gm); err != nil {
			return nil, fmt.Errorf("failed to parse message: %w", err)
		}
		messages = append(messages, messageFromGenerated(gm))
	}

	return &MessageListResult{Messages: messages, Meta: ListMeta{TotalCount: totalCount}}, nil
}

// Get returns a message by ID.
func (s *MessagesService) Get(ctx context.Context, messageID int64) (result *Message, err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Get",
		ResourceType: "message", IsMutation: false,
		ResourceID: messageID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetMessageWithResponse(ctx, s.client.accountID, messageID)
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

	message := messageFromGenerated(*resp.JSON200)
	return &message, nil
}

// Create creates a new message on a message board.
// Returns the created message.
func (s *MessagesService) Create(ctx context.Context, boardID int64, req *CreateMessageRequest) (result *Message, err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Create",
		ResourceType: "message", IsMutation: true,
		ResourceID: boardID,
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

	resp, err := s.client.parent.gen.CreateMessageWithResponse(ctx, s.client.accountID, boardID, body)
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

	message := messageFromGenerated(*resp.JSON201)
	return &message, nil
}

// Update updates an existing message.
// Returns the updated message.
func (s *MessagesService) Update(ctx context.Context, messageID int64, req *UpdateMessageRequest) (result *Message, err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Update",
		ResourceType: "message", IsMutation: true,
		ResourceID: messageID,
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

	resp, err := s.client.parent.gen.UpdateMessageWithResponse(ctx, s.client.accountID, messageID, body)
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

	message := messageFromGenerated(*resp.JSON200)
	return &message, nil
}

// Pin pins a message to the top of the message board.
func (s *MessagesService) Pin(ctx context.Context, messageID int64) (err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Pin",
		ResourceType: "message", IsMutation: true,
		ResourceID: messageID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.PinMessageWithResponse(ctx, s.client.accountID, messageID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Unpin unpins a message from the top of the message board.
func (s *MessagesService) Unpin(ctx context.Context, messageID int64) (err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Unpin",
		ResourceType: "message", IsMutation: true,
		ResourceID: messageID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.UnpinMessageWithResponse(ctx, s.client.accountID, messageID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Trash moves a message to the trash.
// Trashed messages can be recovered from the trash.
func (s *MessagesService) Trash(ctx context.Context, messageID int64) (err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Trash",
		ResourceType: "message", IsMutation: true,
		ResourceID: messageID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.TrashRecordingWithResponse(ctx, s.client.accountID, messageID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Archive moves a message to the archive.
// Archived messages can be unarchived.
func (s *MessagesService) Archive(ctx context.Context, messageID int64) (err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Archive",
		ResourceType: "message", IsMutation: true,
		ResourceID: messageID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ArchiveRecordingWithResponse(ctx, s.client.accountID, messageID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Unarchive restores an archived message to active status.
func (s *MessagesService) Unarchive(ctx context.Context, messageID int64) (err error) {
	op := OperationInfo{
		Service: "Messages", Operation: "Unarchive",
		ResourceType: "message", IsMutation: true,
		ResourceID: messageID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.UnarchiveRecordingWithResponse(ctx, s.client.accountID, messageID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// messageFromGenerated converts a generated Message to our clean Message type.
func messageFromGenerated(gm generated.Message) Message {
	m := Message{
		Status:      gm.Status,
		Subject:     gm.Subject,
		Content:     gm.Content,
		Type:        gm.Type,
		URL:         gm.Url,
		AppURL:      gm.AppUrl,
		CreatedAt:   gm.CreatedAt,
		UpdatedAt:   gm.UpdatedAt,
		BoostsCount: int(gm.BoostsCount),
	}

	if derefInt64(gm.Id) != 0 {
		m.ID = derefInt64(gm.Id)
	}

	// Convert nested types
	if derefInt64(gm.Parent.Id) != 0 || gm.Parent.Title != "" {
		m.Parent = &Parent{
			ID:     derefInt64(gm.Parent.Id),
			Title:  gm.Parent.Title,
			Type:   gm.Parent.Type,
			URL:    gm.Parent.Url,
			AppURL: gm.Parent.AppUrl,
		}
	}

	if derefInt64(gm.Bucket.Id) != 0 || gm.Bucket.Name != "" {
		m.Bucket = &Bucket{
			ID:   derefInt64(gm.Bucket.Id),
			Name: gm.Bucket.Name,
			Type: gm.Bucket.Type,
		}
	}

	if derefInt64(gm.Creator.Id) != 0 || gm.Creator.Name != "" {
		m.Creator = &Person{
			ID:           derefInt64(gm.Creator.Id),
			Name:         gm.Creator.Name,
			EmailAddress: gm.Creator.EmailAddress,
			AvatarURL:    gm.Creator.AvatarUrl,
			Admin:        gm.Creator.Admin,
			Owner:        gm.Creator.Owner,
		}
	}

	if derefInt64(gm.Category.Id) != 0 || gm.Category.Name != "" {
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
