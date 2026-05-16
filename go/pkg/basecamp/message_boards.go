package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// MessageBoard represents a Basecamp message board in a project.
type MessageBoard struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients,omitempty"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Type             string    `json:"type"`
	URL              string    `json:"url"`
	AppURL           string    `json:"app_url"`
	BookmarkURL      string    `json:"bookmark_url,omitempty"`
	MessagesCount    int       `json:"messages_count"`
	MessagesURL      string    `json:"messages_url"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
}

// MessageBoardsService handles message board operations.
type MessageBoardsService struct {
	client *AccountClient
}

// NewMessageBoardsService creates a new MessageBoardsService.
func NewMessageBoardsService(client *AccountClient) *MessageBoardsService {
	return &MessageBoardsService{client: client}
}

// Get returns a message board by ID.
func (s *MessageBoardsService) Get(ctx context.Context, boardID int64) (result *MessageBoard, err error) {
	op := OperationInfo{
		Service: "MessageBoards", Operation: "Get",
		ResourceType: "message_board", IsMutation: false,
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

	resp, err := s.client.parent.gen.GetMessageBoardWithResponse(ctx, s.client.accountID, boardID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	board := messageBoardFromGenerated(*resp.JSON200)
	return &board, nil
}

// messageBoardFromGenerated converts a generated MessageBoard to our clean MessageBoard type.
func messageBoardFromGenerated(gb generated.MessageBoard) MessageBoard {
	mb := MessageBoard{
		Status:           gb.Status,
		VisibleToClients: gb.VisibleToClients,
		Title:            gb.Title,
		InheritsStatus:   gb.InheritsStatus,
		Type:             gb.Type,
		URL:              gb.Url,
		AppURL:           gb.AppUrl,
		BookmarkURL:      gb.BookmarkUrl,
		MessagesCount:    int(gb.MessagesCount),
		MessagesURL:      gb.MessagesUrl,
		CreatedAt:        gb.CreatedAt,
		UpdatedAt:        gb.UpdatedAt,
	}

	if gb.Id != 0 {
		mb.ID = gb.Id
	}

	if gb.Bucket.Id != 0 || gb.Bucket.Name != "" {
		mb.Bucket = &Bucket{
			ID:   gb.Bucket.Id,
			Name: gb.Bucket.Name,
			Type: gb.Bucket.Type,
		}
	}

	if gb.Creator.Id != 0 || gb.Creator.Name != "" {
		creator := personFromGenerated(gb.Creator)
		mb.Creator = &creator
	}

	return mb
}
