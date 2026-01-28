package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
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
func (s *MessageBoardsService) Get(ctx context.Context, bucketID, boardID int64) (result *MessageBoard, err error) {
	op := OperationInfo{
		Service: "MessageBoards", Operation: "Get",
		ResourceType: "message_board", IsMutation: false,
		BucketID: bucketID, ResourceID: boardID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetMessageBoardWithResponse(ctx, bucketID, boardID)
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

	board := messageBoardFromGenerated(resp.JSON200.MessageBoard)
	return &board, nil
}

// messageBoardFromGenerated converts a generated MessageBoard to our clean MessageBoard type.
func messageBoardFromGenerated(gb generated.MessageBoard) MessageBoard {
	mb := MessageBoard{
		Status:        gb.Status,
		Title:         gb.Title,
		Type:          gb.Type,
		URL:           gb.Url,
		AppURL:        gb.AppUrl,
		MessagesCount: int(gb.MessagesCount),
		MessagesURL:   gb.MessagesUrl,
		CreatedAt:     gb.CreatedAt,
		UpdatedAt:     gb.UpdatedAt,
	}

	if gb.Id != nil {
		mb.ID = *gb.Id
	}

	if gb.Bucket.Id != nil || gb.Bucket.Name != "" {
		mb.Bucket = &Bucket{
			ID:   derefInt64(gb.Bucket.Id),
			Name: gb.Bucket.Name,
			Type: gb.Bucket.Type,
		}
	}

	if gb.Creator.Id != nil || gb.Creator.Name != "" {
		mb.Creator = &Person{
			ID:           derefInt64(gb.Creator.Id),
			Name:         gb.Creator.Name,
			EmailAddress: gb.Creator.EmailAddress,
			AvatarURL:    gb.Creator.AvatarUrl,
			Admin:        gb.Creator.Admin,
			Owner:        gb.Creator.Owner,
		}
	}

	return mb
}
