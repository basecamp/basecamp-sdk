package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// RecordingType represents a type of recording in Basecamp.
type RecordingType string

// Recording types supported by the Basecamp API.
const (
	RecordingTypeComment        RecordingType = "Comment"
	RecordingTypeDocument       RecordingType = "Document"
	RecordingTypeDoor           RecordingType = "Door"
	RecordingTypeKanbanCard     RecordingType = "Kanban::Card"
	RecordingTypeKanbanStep     RecordingType = "Kanban::Step"
	RecordingTypeMessage        RecordingType = "Message"
	RecordingTypeQuestionAnswer RecordingType = "Question::Answer"
	RecordingTypeScheduleEntry  RecordingType = "Schedule::Entry"
	RecordingTypeTodo           RecordingType = "Todo"
	RecordingTypeTodolist       RecordingType = "Todolist"
	RecordingTypeUpload         RecordingType = "Upload"
	RecordingTypeVault          RecordingType = "Vault"
)

// Recording represents a generic Basecamp recording.
// Recordings are the base type for most content in Basecamp including
// messages, todos, comments, documents, and more.
type Recording struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status"`
	Type             string    `json:"type"`
	URL              string    `json:"url"`
	AppURL           string    `json:"app_url"`
	BookmarkURL      string    `json:"bookmark_url"`
	// BubbleUpURL is the URL of the Bubble Up record for this recording. Optional
	// on this polymorphic projection: recordings/_recording.json.jbuilder emits
	// the key only when the caller passes bubbleupable, and todolists/_todolist
	// is the only partial that does — so a Todolist-shaped instance carries it
	// and the other recording types do not.
	BubbleUpURL string `json:"bubble_up_url,omitempty"`
	Content     string `json:"content,omitempty"`
	// ContentAttachments and DescriptionAttachments are the rich text companion
	// arrays carried through the generic recording projection. A given recording
	// is one type, so it carries only the array matching its rich text attribute
	// (ContentAttachments for a Comment/Message, DescriptionAttachments for a
	// Todo/Card); a webhook-sourced recording carries neither. These are
	// optional and non-nullable, and their absent-vs-present-empty distinction
	// is meaningful (a matching type with no inline files serves an empty [],
	// while a non-matching/webhook recording omits the key). Modeled as a
	// pointer to a slice with omitempty so all three wire states round-trip
	// faithfully: nil pointer (absent) is omitted, a non-nil pointer to an
	// empty slice re-encodes as [], and a populated one as the list — matching
	// the nullable/optional modeling in every other SDK. See RichTextAttachment.
	ContentAttachments     *[]RichTextAttachment `json:"content_attachments,omitempty"`
	DescriptionAttachments *[]RichTextAttachment `json:"description_attachments,omitempty"`
	CommentsCount          int                   `json:"comments_count,omitempty"`
	CommentsURL            string                `json:"comments_url,omitempty"`
	SubscriptionURL        string                `json:"subscription_url,omitempty"`
	// BoostsCount/BoostsURL, Subject, GroupOn, From, and RepliesCount/RepliesURL
	// are type-specific fields carried by the account-wide aggregate feeds
	// (/messages.json, /comments.json, /checkins.json, /forwards.json), whose
	// type-specific partials render them on top of the base recording projection.
	// They are populated only on the matching recording type (boosts_* on
	// boostable recordings, subject on Message, group_on on Question::Answer,
	// from/replies_* on Inbox::Forward), so they are pointer-backed: nil (absent)
	// omits, and an explicit value — including an empty string or a zero count —
	// round-trips, per SPEC.md §10.
	BoostsCount  *int               `json:"boosts_count,omitempty"`
	BoostsURL    *string            `json:"boosts_url,omitempty"`
	Subject      *string            `json:"subject,omitempty"`
	Category     *RecordingCategory `json:"category,omitempty"`
	GroupOn      *string            `json:"group_on,omitempty"`
	From         *string            `json:"from,omitempty"`
	RepliesCount *int               `json:"replies_count,omitempty"`
	RepliesURL   *string            `json:"replies_url,omitempty"`
	// Position, Description, and Service are door-specific (external-link)
	// fields, populated only on Door recordings returned by the type=Door
	// recordings query (the only endpoint that returns the full door shape).
	// See spec/api-gaps/external-links-doors.md.
	Position    int32        `json:"position,omitempty"`
	Description string       `json:"description,omitempty"`
	Service     *DoorService `json:"service,omitempty"`
	Parent      *Parent      `json:"parent,omitempty"`
	Bucket      *Bucket      `json:"bucket,omitempty"`
	Creator     *Person      `json:"creator,omitempty"`
}

// RecordingCategory is a message category (type): id, display name, and icon.
// Present on categorized Message recordings (e.g. the /messages.json feed).
type RecordingCategory struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Icon is optional (a category may omit it); pointer-backed so an absent icon
	// (nil) is distinct from an explicit empty string, per SPEC.md §10.
	Icon *string `json:"icon,omitempty"`
}

// DoorService describes the recognized external service backing an external
// link (Door recording): its display name, a canonical example URL, a short
// code (or "other" for a generic link), the URL patterns Basecamp recognizes,
// and human supporting text.
type DoorService struct {
	Name           string   `json:"name,omitempty"`
	ExampleURL     string   `json:"example_url,omitempty"`
	Code           string   `json:"code,omitempty"`
	ValidPatterns  []string `json:"valid_patterns,omitempty"`
	SupportingText string   `json:"supporting_text,omitempty"`
}

// DefaultRecordingLimit is the default number of recordings to return when no limit is specified.
const DefaultRecordingLimit = 100

// RecordingsListOptions specifies options for listing recordings.
type RecordingsListOptions struct {
	// Bucket filters by project IDs (comma-separated or slice).
	// Defaults to all active projects visible to the user.
	Bucket []int64

	// Status filters by recording status: "active", "archived", or "trashed".
	// Defaults to "active".
	Status string

	// Sort specifies the sort field: "created_at" or "updated_at".
	// Defaults to "created_at".
	Sort string

	// Direction specifies the sort direction: "desc" or "asc".
	// Defaults to "desc".
	Direction string

	// Limit is the maximum number of recordings to return.
	// If 0, uses DefaultRecordingLimit (100). Use -1 for unlimited.
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination:
	// exactly one request, no Link rel="next" follow (SPEC §8). A positive
	// Limit still trims that page; the per-operation default limit does not
	// apply to it. Use 0 to paginate through all results up to Limit.
	Page int
}

// RecordingListResult contains the results from listing recordings.
type RecordingListResult struct {
	// Recordings is the list of recordings returned.
	Recordings []Recording
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// SetClientVisibilityRequest specifies the parameters for setting client visibility.
type SetClientVisibilityRequest struct {
	VisibleToClients bool `json:"visible_to_clients"`
}

// RecordingsService handles recording operations.
// Recordings are the base type for most content in Basecamp.
type RecordingsService struct {
	client *AccountClient
}

// NewRecordingsService creates a new RecordingsService.
func NewRecordingsService(client *AccountClient) *RecordingsService {
	return &RecordingsService{client: client}
}

// List returns all recordings of a given type across projects.
// recordingType is required and specifies what type of recordings to list.
// Use the RecordingType constants (e.g., RecordingTypeTodo, RecordingTypeMessage).
//
// By default, returns up to 100 recordings. Use Limit: -1 for unlimited.
//
// Pagination options:
//   - Limit: maximum number of recordings to return (0 = 100, -1 = unlimited)
//   - Page: if positive, fetches only that page and disables auto-pagination
//
// The returned RecordingListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *RecordingsService) List(ctx context.Context, recordingType RecordingType, opts *RecordingsListOptions) (result *RecordingListResult, err error) {
	op := OperationInfo{
		Service: "Recordings", Operation: "List",
		ResourceType: "recording", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if recordingType == "" {
		err = ErrUsage("recording type is required")
		return nil, err
	}

	// Build params for generated client
	typeStr := string(recordingType)
	params := &generated.ListRecordingsParams{
		Type: typeStr,
	}
	if opts != nil {
		if opts.Page > 0 {
			var page *int32
			if page, err = pageParam(opts.Page); err != nil {
				return nil, err
			}
			params.Page = page
		}
		if len(opts.Bucket) > 0 {
			bucketStrs := make([]string, len(opts.Bucket))
			for i, b := range opts.Bucket {
				bucketStrs[i] = fmt.Sprintf("%d", b)
			}
			params.Bucket = ptr(strings.Join(bucketStrs, ","))
		}
		if opts.Status != "" {
			params.Status = &opts.Status
		}
		if opts.Sort != "" {
			params.Sort = &opts.Sort
		}
		if opts.Direction != "" {
			params.Direction = &opts.Direction
		}
	}

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListRecordingsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var recordings []Recording
	if resp.JSON200 != nil {
		for _, gr := range *resp.JSON200 {
			recordings = append(recordings, recordingFromGenerated(gr))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		keep, truncated := pageCap(len(recordings), opts.Limit, resp.HTTPResponse)
		return &RecordingListResult{Recordings: recordings[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
	}

	// Determine limit: 0 = default (100), -1 = unlimited, >0 = specific limit
	limit := DefaultRecordingLimit
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0 // unlimited
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	// Check if we already have enough items
	if limit > 0 && len(recordings) >= limit {
		return &RecordingListResult{Recordings: recordings[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(recordings), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(recordings), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gr generated.Recording
		if err := json.Unmarshal(raw, &gr); err != nil {
			return nil, fmt.Errorf("failed to parse recording: %w", err)
		}
		recordings = append(recordings, recordingFromGenerated(gr))
	}

	return &RecordingListResult{Recordings: recordings, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Trash moves a recording to the trash.
// Trashed recordings can be recovered from the trash.
func (s *RecordingsService) Trash(ctx context.Context, recordingID int64) (err error) {
	op := OperationInfo{
		Service: "Recordings", Operation: "Trash",
		ResourceType: "recording", IsMutation: true,
		ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.TrashRecordingWithResponse(ctx, s.client.accountID, recordingID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Archive archives a recording.
// Archived recordings are hidden but not deleted.
func (s *RecordingsService) Archive(ctx context.Context, recordingID int64) (err error) {
	op := OperationInfo{
		Service: "Recordings", Operation: "Archive",
		ResourceType: "recording", IsMutation: true,
		ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ArchiveRecordingWithResponse(ctx, s.client.accountID, recordingID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Unarchive restores an archived recording to active status.
func (s *RecordingsService) Unarchive(ctx context.Context, recordingID int64) (err error) {
	op := OperationInfo{
		Service: "Recordings", Operation: "Unarchive",
		ResourceType: "recording", IsMutation: true,
		ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.UnarchiveRecordingWithResponse(ctx, s.client.accountID, recordingID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// SetClientVisibility sets whether a recording is visible to clients.
// visible specifies whether the recording should be visible to clients.
// Returns the updated recording.
// Note: Not all recordings support client visibility. Some inherit visibility from their parent.
func (s *RecordingsService) SetClientVisibility(ctx context.Context, recordingID int64, visible bool) (result *Recording, err error) {
	op := OperationInfo{
		Service: "Recordings", Operation: "SetClientVisibility",
		ResourceType: "recording", IsMutation: true,
		ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	body := generated.SetClientVisibilityJSONRequestBody{
		VisibleToClients: visible,
	}

	resp, err := s.client.parent.gen.SetClientVisibilityWithResponse(ctx, s.client.accountID, recordingID, body)
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

	recording := recordingFromGenerated(*resp.JSON200)
	return &recording, nil
}

// recordingFromGenerated converts a generated Recording to our clean type.
func recordingFromGenerated(gr generated.Recording) Recording {
	r := Recording{
		Status:           gr.Status,
		VisibleToClients: gr.VisibleToClients,
		CreatedAt:        gr.CreatedAt,
		UpdatedAt:        gr.UpdatedAt,
		Title:            gr.Title,
		InheritsStatus:   gr.InheritsStatus,
		Type:             gr.Type,
		URL:              gr.Url,
		AppURL:           gr.AppUrl,
		BookmarkURL:      deref(gr.BookmarkUrl),
		BubbleUpURL:      deref(gr.BubbleUpUrl),
		Content:          deref(gr.Content),
		CommentsCount:    int(deref(gr.CommentsCount)),
		CommentsURL:      deref(gr.CommentsUrl),
		SubscriptionURL:  deref(gr.SubscriptionUrl),
		BoostsURL:        gr.BoostsUrl,
		Subject:          gr.Subject,
		From:             gr.From,
		RepliesURL:       gr.RepliesUrl,
	}
	if gr.BoostsCount != nil {
		v := int(*gr.BoostsCount)
		r.BoostsCount = &v
	}
	if gr.RepliesCount != nil {
		v := int(*gr.RepliesCount)
		r.RepliesCount = &v
	}
	if gr.GroupOn != nil {
		s := gr.GroupOn.String()
		r.GroupOn = &s
	}

	if gr.Id != 0 {
		r.ID = gr.Id
	}

	if gr.Parent != nil {
		r.Parent = &Parent{
			ID:     gr.Parent.Id,
			Title:  gr.Parent.Title,
			Type:   gr.Parent.Type,
			URL:    gr.Parent.Url,
			AppURL: gr.Parent.AppUrl,
		}
	}

	if gr.Bucket.Id != 0 || gr.Bucket.Name != "" {
		r.Bucket = &Bucket{
			ID:   gr.Bucket.Id,
			Name: gr.Bucket.Name,
			Type: gr.Bucket.Type,
		}
	}

	if gr.Creator.Id != 0 || gr.Creator.Name != "" {
		creator := personFromGenerated(gr.Creator)
		r.Creator = &creator
	}

	if gr.Category != nil {
		r.Category = &RecordingCategory{
			ID:   gr.Category.Id,
			Name: gr.Category.Name,
			Icon: gr.Category.Icon,
		}
	}

	r.ContentAttachments = richTextAttachmentsPtrFromGenerated(gr.ContentAttachments)
	r.DescriptionAttachments = richTextAttachmentsPtrFromGenerated(gr.DescriptionAttachments)

	// Door-specific fields (populated only for type=Door recordings).
	r.Position = deref(gr.Position)
	r.Description = deref(gr.Description)
	if gr.Service != nil {
		r.Service = &DoorService{
			Name:           deref(gr.Service.Name),
			ExampleURL:     deref(gr.Service.ExampleUrl),
			Code:           deref(gr.Service.Code),
			ValidPatterns:  append([]string(nil), gr.Service.ValidPatterns...),
			SupportingText: deref(gr.Service.SupportingText),
		}
	}

	return r
}
