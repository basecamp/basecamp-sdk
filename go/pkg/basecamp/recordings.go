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
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
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

	// Page, if non-zero, disables pagination and returns only the first page.
	// NOTE: The page number itself is not yet honored due to OpenAPI client
	// limitations. Use 0 to paginate through all results up to Limit.
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
//   - Page: if non-zero, disables pagination and returns first page only
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
		if len(opts.Bucket) > 0 {
			bucketStrs := make([]string, len(opts.Bucket))
			for i, b := range opts.Bucket {
				bucketStrs[i] = fmt.Sprintf("%d", b)
			}
			params.Bucket = strings.Join(bucketStrs, ",")
		}
		if opts.Status != "" {
			params.Status = opts.Status
		}
		if opts.Sort != "" {
			params.Sort = opts.Sort
		}
		if opts.Direction != "" {
			params.Direction = opts.Direction
		}
	}

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListRecordingsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
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
		return &RecordingListResult{Recordings: recordings, Meta: ListMeta{TotalCount: totalCount}}, nil
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
		return &RecordingListResult{Recordings: recordings[:limit], Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, err := s.client.parent.FollowPagination(ctx, resp.HTTPResponse, len(recordings), limit)
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

	return &RecordingListResult{Recordings: recordings, Meta: ListMeta{TotalCount: totalCount}}, nil
}

// Get returns a recording by ID.
// bucketID is the project ID, recordingID is the recording ID.
func (s *RecordingsService) Get(ctx context.Context, bucketID, recordingID int64) (result *Recording, err error) {
	op := OperationInfo{
		Service: "Recordings", Operation: "Get",
		ResourceType: "recording", IsMutation: false,
		BucketID: bucketID, ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetRecordingWithResponse(ctx, s.client.accountID, bucketID, recordingID)
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

	recording := recordingFromGenerated(*resp.JSON200)
	return &recording, nil
}

// Trash moves a recording to the trash.
// bucketID is the project ID, recordingID is the recording ID.
// Trashed recordings can be recovered from the trash.
func (s *RecordingsService) Trash(ctx context.Context, bucketID, recordingID int64) (err error) {
	op := OperationInfo{
		Service: "Recordings", Operation: "Trash",
		ResourceType: "recording", IsMutation: true,
		BucketID: bucketID, ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.TrashRecordingWithResponse(ctx, s.client.accountID, bucketID, recordingID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Archive archives a recording.
// bucketID is the project ID, recordingID is the recording ID.
// Archived recordings are hidden but not deleted.
func (s *RecordingsService) Archive(ctx context.Context, bucketID, recordingID int64) (err error) {
	op := OperationInfo{
		Service: "Recordings", Operation: "Archive",
		ResourceType: "recording", IsMutation: true,
		BucketID: bucketID, ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ArchiveRecordingWithResponse(ctx, s.client.accountID, bucketID, recordingID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Unarchive restores an archived recording to active status.
// bucketID is the project ID, recordingID is the recording ID.
func (s *RecordingsService) Unarchive(ctx context.Context, bucketID, recordingID int64) (err error) {
	op := OperationInfo{
		Service: "Recordings", Operation: "Unarchive",
		ResourceType: "recording", IsMutation: true,
		BucketID: bucketID, ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.UnarchiveRecordingWithResponse(ctx, s.client.accountID, bucketID, recordingID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// SetClientVisibility sets whether a recording is visible to clients.
// bucketID is the project ID, recordingID is the recording ID.
// visible specifies whether the recording should be visible to clients.
// Returns the updated recording.
// Note: Not all recordings support client visibility. Some inherit visibility from their parent.
func (s *RecordingsService) SetClientVisibility(ctx context.Context, bucketID, recordingID int64, visible bool) (result *Recording, err error) {
	op := OperationInfo{
		Service: "Recordings", Operation: "SetClientVisibility",
		ResourceType: "recording", IsMutation: true,
		BucketID: bucketID, ResourceID: recordingID,
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

	resp, err := s.client.parent.gen.SetClientVisibilityWithResponse(ctx, s.client.accountID, bucketID, recordingID, body)
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

	recording := recordingFromGenerated(resp.JSON200.Recording)
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
		BookmarkURL:      gr.BookmarkUrl,
	}

	if gr.Id != nil {
		r.ID = *gr.Id
	}

	if gr.Parent.Id != nil || gr.Parent.Title != "" {
		r.Parent = &Parent{
			ID:     derefInt64(gr.Parent.Id),
			Title:  gr.Parent.Title,
			Type:   gr.Parent.Type,
			URL:    gr.Parent.Url,
			AppURL: gr.Parent.AppUrl,
		}
	}

	if gr.Bucket.Id != nil || gr.Bucket.Name != "" {
		r.Bucket = &Bucket{
			ID:   derefInt64(gr.Bucket.Id),
			Name: gr.Bucket.Name,
			Type: gr.Bucket.Type,
		}
	}

	if gr.Creator.Id != nil || gr.Creator.Name != "" {
		r.Creator = &Person{
			ID:           derefInt64(gr.Creator.Id),
			Name:         gr.Creator.Name,
			EmailAddress: gr.Creator.EmailAddress,
			AvatarURL:    gr.Creator.AvatarUrl,
			Admin:        gr.Creator.Admin,
			Owner:        gr.Creator.Owner,
		}
	}

	return r
}
