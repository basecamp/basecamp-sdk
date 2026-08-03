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
	ID                 int64  `json:"id"`
	Type               string `json:"type,omitempty"`
	Title              string `json:"title,omitempty"`
	Section            string `json:"section,omitempty"`
	ContentExcerpt     string `json:"content_excerpt,omitempty"`
	BucketName         string `json:"bucket_name,omitempty"`
	ReadableIdentifier string `json:"readable_identifier,omitempty"`
	ReadableSGID       string `json:"readable_sgid,omitempty"`
	Subscribed         bool   `json:"subscribed,omitempty"`
	Named              bool   `json:"named,omitempty"`
	UnreadCount        int32  `json:"unread_count,omitempty"`
	ImageURL           string `json:"image_url,omitempty"`
	AppURL             string `json:"app_url,omitempty"`
	BookmarkURL        string `json:"bookmark_url,omitempty"`
	MemoryURL          string `json:"memory_url,omitempty"`
	// BubbleUpURL is the BC5-added URL for the Bubble Up record covering this
	// notification. Eligibility-gated — only present on items the current user
	// can bubble up.
	BubbleUpURL string `json:"bubble_up_url,omitempty"`
	// BubbleUpAt is the BC5-added scheduled resurfacing time when this item is
	// queued as a scheduled Bubble Up. A pointer so an absent scheduled time
	// omits cleanly on the wire instead of marshaling as the zero time
	// (0001-01-01T00:00:00Z); mirrors the *time.Time convention used for
	// Card.CompletedAt / Todo.CompletedAt.
	BubbleUpAt             *time.Time              `json:"bubble_up_at,omitempty"`
	UnreadURL              string                  `json:"unread_url,omitempty"`
	SubscriptionURL        string                  `json:"subscription_url,omitempty"`
	Creator                *Person                 `json:"creator,omitempty"`
	Participants           []Person                `json:"participants,omitempty"`
	PreviewableAttachments []PreviewableAttachment `json:"previewable_attachments,omitempty"`
	CreatedAt              time.Time               `json:"created_at"`
	UpdatedAt              time.Time               `json:"updated_at"`
	// ReadAt and UnreadAt are the read/unread transition times. Both are
	// optional and mutually exclusive in practice: an unread notification has
	// neither or only UnreadAt, a read one carries ReadAt. Pointers so "never
	// read" stays distinguishable from a real timestamp — a value-typed
	// time.Time would report 0001-01-01T00:00:00Z, and `,omitempty` cannot
	// suppress it because encoding/json never treats a struct as empty.
	ReadAt   *time.Time `json:"read_at,omitempty"`
	UnreadAt *time.Time `json:"unread_at,omitempty"`
}

// PreviewableAttachment represents a preview-renderable attachment surfaced on
// a Notification (e.g. images in a ping).
type PreviewableAttachment struct {
	ID          *int64 `json:"id,omitempty"`
	AppURL      string `json:"app_url,omitempty"`
	URL         string `json:"url,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Filename    string `json:"filename,omitempty"`
	Filesize    int64  `json:"filesize,omitempty"`
	Width       int32  `json:"width,omitempty"`
	Height      int32  `json:"height,omitempty"`
}

// NotificationsResult contains the notifications grouped by status.
type NotificationsResult struct {
	Unreads  []Notification `json:"unreads,omitempty"`
	Reads    []Notification `json:"reads,omitempty"`
	Memories []Notification `json:"memories,omitempty"`
	// BubbleUps is the BC5-added list of Bubble Up notifications. BC3 also
	// populates Memories as an alias for BC4-API consumer compat; new
	// integrations should prefer BubbleUps.
	BubbleUps []Notification `json:"bubble_ups,omitempty"`
	// ScheduledBubbleUps is the BC5-added list of scheduled Bubble Up
	// notifications (resurface time in the future). Omitted from the response
	// entirely when the request sets limit_bubble_ups=true.
	ScheduledBubbleUps []Notification `json:"scheduled_bubble_ups,omitempty"`
	// BubbleUpsCount is the total number of current bubble-ups, independent of
	// the limit_bubble_ups cap on the BubbleUps array.
	BubbleUpsCount int32 `json:"bubble_ups_count"`
	// ScheduledBubbleUpsCount is the total number of scheduled bubble-ups,
	// present even when limit_bubble_ups omits the ScheduledBubbleUps array.
	ScheduledBubbleUpsCount int32 `json:"scheduled_bubble_ups_count"`
}

// BubbleUpsResult contains a page-followed list of bubble-up notifications
// with pagination metadata.
type BubbleUpsResult struct {
	// BubbleUps is the combined list of current bubble-ups (first, ordered by
	// most recently bubbled up) followed by scheduled bubble-ups (ordered by
	// scheduled time).
	BubbleUps []Notification
	// Meta contains pagination metadata (total count, truncation).
	Meta ListMeta
}

// MyNotificationsService handles notification operations for the current user.
type MyNotificationsService struct {
	client *AccountClient
}

// NewMyNotificationsService creates a new MyNotificationsService.
func NewMyNotificationsService(client *AccountClient) *MyNotificationsService {
	return &MyNotificationsService{client: client}
}

// MyNotificationsGetOption customizes a MyNotifications Get request.
type MyNotificationsGetOption func(*generated.GetMyNotificationsParams)

// WithLimitBubbleUps caps the bubble_ups array at 2 current bubble-ups and
// omits the scheduled_bubble_ups array from the response (the counts are still
// returned). Use the BubbleUps method to page through all bubble-ups.
func WithLimitBubbleUps() MyNotificationsGetOption {
	return func(p *generated.GetMyNotificationsParams) { p.LimitBubbleUps = ptr(true) }
}

// Get returns notifications for the current user.
// page is optional; pass 0 to use the default (page 1).
//
// This preserves the original two-argument signature; adding a variadic
// parameter here would change the method's type and break method values and
// interface satisfaction for existing callers. Use GetWithOptions to tune the
// request with functional options.
func (s *MyNotificationsService) Get(ctx context.Context, page int32) (result *NotificationsResult, err error) {
	return s.get(ctx, page)
}

// GetWithOptions returns notifications for the current user, tuned by optional
// functional options (e.g. WithLimitBubbleUps). page is optional; pass 0 to use
// the default (page 1).
func (s *MyNotificationsService) GetWithOptions(ctx context.Context, page int32, opts ...MyNotificationsGetOption) (result *NotificationsResult, err error) {
	return s.get(ctx, page, opts...)
}

// get is the shared implementation behind Get and GetWithOptions; both report
// the same "Get" operation identity to hooks.
func (s *MyNotificationsService) get(ctx context.Context, page int32, opts ...MyNotificationsGetOption) (result *NotificationsResult, err error) {
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

	params := &generated.GetMyNotificationsParams{}
	if page > 0 {
		params.Page = &page
	}
	for _, opt := range opts {
		opt(params)
	}

	resp, err := s.client.parent.gen.GetMyNotificationsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	normalized, normalizeErr := normalizeEmbeddedPeopleJSON(resp.Body)
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

// BubbleUps returns the current user's current and scheduled bubble-ups.
// Current bubble-ups come first (ordered by most recently bubbled up), then
// scheduled bubble-ups (ordered by scheduled time). The list is paginated at
// 50 per page; by default this follows the Link header across all pages.
// Pass a positive page to disable auto-pagination and return only that page.
func (s *MyNotificationsService) BubbleUps(ctx context.Context, page int32) (result *BubbleUpsResult, err error) {
	op := OperationInfo{
		Service: "MyNotifications", Operation: "BubbleUps",
		ResourceType: "bubble_up", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.GetBubbleUpsParams
	if page > 0 {
		params = &generated.GetBubbleUpsParams{Page: &page}
	}

	resp, err := s.client.parent.gen.GetBubbleUpsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	items, err := decodeBubbleUpPage(resp.Body)
	if err != nil {
		return nil, err
	}

	totalCount := parseTotalCount(resp.HTTPResponse)

	// A positive page disables auto-pagination (single page only). The caller
	// still needs to know the returned page is a partial view, so report
	// Truncated when the response carries a rel="next" Link (more pages exist)
	// even though we deliberately do not follow it here.
	if page > 0 {
		truncated := parseNextLink(resp.HTTPResponse.Header.Get("Link")) != ""
		return &BubbleUpsResult{BubbleUps: items, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
	}

	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(items), 0)
	if err != nil {
		return nil, err
	}
	for _, raw := range rawMore {
		n, decErr := decodeNotificationItem(raw)
		if decErr != nil {
			return nil, decErr
		}
		items = append(items, n)
	}

	return &BubbleUpsResult{BubbleUps: items, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// decodeBubbleUpPage decodes a bubble-ups page (a bare JSON array of
// notifications) into clean Notification values, applying the embedded-people
// id normalization the Notification shape relies on.
func decodeBubbleUpPage(body []byte) ([]Notification, error) {
	normalized, normErr := normalizeEmbeddedPeopleJSON(body)
	if normErr != nil {
		normalized = body // fallback to raw
	}
	var items []Notification
	if err := json.Unmarshal(normalized, &items); err != nil {
		return nil, fmt.Errorf("failed to parse bubble-ups: %w", err)
	}
	return items, nil
}

// decodeNotificationItem decodes a single notification item (a raw JSON object
// from a followed pagination page) into a clean Notification, applying the same
// embedded-people id normalization as the first page.
func decodeNotificationItem(raw json.RawMessage) (Notification, error) {
	normalized, normErr := normalizeEmbeddedPeopleJSON(raw)
	if normErr != nil {
		normalized = raw // fallback to raw
	}
	var n Notification
	if err := json.Unmarshal(normalized, &n); err != nil {
		return Notification{}, fmt.Errorf("failed to parse bubble-up notification: %w", err)
	}
	return n, nil
}
