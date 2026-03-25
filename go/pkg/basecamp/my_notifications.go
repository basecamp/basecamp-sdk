package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Notification represents a single notification item.
type Notification struct {
	ID                 int64     `json:"id"`
	Type               string    `json:"type,omitempty"`
	Title              string    `json:"title,omitempty"`
	Section            string    `json:"section,omitempty"`
	ContentExcerpt     string    `json:"content_excerpt,omitempty"`
	BucketName         string    `json:"bucket_name,omitempty"`
	ReadableIdentifier string    `json:"readable_identifier,omitempty"`
	ReadableSGID       string    `json:"readable_sgid,omitempty"`
	Subscribed         bool      `json:"subscribed,omitempty"`
	Named              bool      `json:"named,omitempty"`
	UnreadCount        int32     `json:"unread_count,omitempty"`
	ImageURL           string    `json:"image_url,omitempty"`
	AppURL             string    `json:"app_url,omitempty"`
	BookmarkURL        string    `json:"bookmark_url,omitempty"`
	MemoryURL          string    `json:"memory_url,omitempty"`
	UnreadURL          string    `json:"unread_url,omitempty"`
	SubscriptionURL    string    `json:"subscription_url,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	ReadAt             time.Time `json:"read_at,omitempty"`
	UnreadAt           time.Time `json:"unread_at,omitempty"`
}

// NotificationsResult contains the notifications grouped by status.
type NotificationsResult struct {
	Unreads  []Notification `json:"unreads,omitempty"`
	Reads    []Notification `json:"reads,omitempty"`
	Memories []Notification `json:"memories,omitempty"`
}

// MyNotificationsService handles notification operations for the current user.
type MyNotificationsService struct {
	client *AccountClient
}

// NewMyNotificationsService creates a new MyNotificationsService.
func NewMyNotificationsService(client *AccountClient) *MyNotificationsService {
	return &MyNotificationsService{client: client}
}

// Get returns notifications for the current user.
// page is optional; pass 0 to use the default (page 1).
func (s *MyNotificationsService) Get(ctx context.Context, page int32) (result *NotificationsResult, err error) {
	op := OperationInfo{
		Service: "MyNotifications", Operation: "Get",
		ResourceType: "notification", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.GetMyNotificationsParams
	if page > 0 {
		params = &generated.GetMyNotificationsParams{
			Page: page,
		}
	}

	resp, err := s.client.parent.gen.GetMyNotificationsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	normalized, normalizeErr := normalizeJSON(resp.Body)
	if normalizeErr != nil {
		normalized = resp.Body // fallback to raw
	}

	var notifications NotificationsResult
	if err = json.Unmarshal(normalized, &notifications); err != nil {
		return nil, fmt.Errorf("failed to parse notifications: %w", err)
	}

	return &notifications, nil
}

// MarkAsRead marks items as read by their readable SGIDs.
func (s *MyNotificationsService) MarkAsRead(ctx context.Context, readables []string) (err error) {
	op := OperationInfo{
		Service: "MyNotifications", Operation: "MarkAsRead",
		ResourceType: "notification", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if len(readables) == 0 {
		err = ErrUsage("at least one readable SGID is required")
		return err
	}

	body := generated.MarkAsReadJSONRequestBody{
		Readables: readables,
	}

	resp, err := s.client.parent.gen.MarkAsReadWithResponse(ctx, s.client.accountID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}
