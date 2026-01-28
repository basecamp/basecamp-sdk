package basecamp

import (
	"context"
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
}

// SetClientVisibilityRequest specifies the parameters for setting client visibility.
type SetClientVisibilityRequest struct {
	VisibleToClients bool `json:"visible_to_clients"`
}

// RecordingsService handles recording operations.
// Recordings are the base type for most content in Basecamp.
type RecordingsService struct {
	client *Client
}

// NewRecordingsService creates a new RecordingsService.
func NewRecordingsService(client *Client) *RecordingsService {
	return &RecordingsService{client: client}
}

// List returns all recordings of a given type across projects.
// recordingType is required and specifies what type of recordings to list.
// Use the RecordingType constants (e.g., RecordingTypeTodo, RecordingTypeMessage).
func (s *RecordingsService) List(ctx context.Context, recordingType RecordingType, opts *RecordingsListOptions) (result []Recording, err error) {
	op := OperationInfo{
		Service: "Recordings", Operation: "List",
		ResourceType: "recording", IsMutation: false,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if recordingType == "" {
		err = ErrUsage("recording type is required")
		return nil, err
	}

	typeStr := string(recordingType)
	params := &generated.ListRecordingsParams{
		Type: typeStr,
	}

	if opts != nil {
		if len(opts.Bucket) > 0 {
			// Convert []int64 to comma-separated string
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

	resp, err := s.client.gen.ListRecordingsWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	recordings := make([]Recording, 0, len(*resp.JSON200))
	for _, gr := range *resp.JSON200 {
		recordings = append(recordings, recordingFromGenerated(gr))
	}

	return recordings, nil
}

// Get returns a recording by ID.
// bucketID is the project ID, recordingID is the recording ID.
func (s *RecordingsService) Get(ctx context.Context, bucketID, recordingID int64) (result *Recording, err error) {
	op := OperationInfo{
		Service: "Recordings", Operation: "Get",
		ResourceType: "recording", IsMutation: false,
		BucketID: bucketID, ResourceID: recordingID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetRecordingWithResponse(ctx, bucketID, recordingID)
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

// Trash moves a recording to the trash.
// bucketID is the project ID, recordingID is the recording ID.
// Trashed recordings can be recovered from the trash.
func (s *RecordingsService) Trash(ctx context.Context, bucketID, recordingID int64) (err error) {
	op := OperationInfo{
		Service: "Recordings", Operation: "Trash",
		ResourceType: "recording", IsMutation: true,
		BucketID: bucketID, ResourceID: recordingID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.TrashRecordingWithResponse(ctx, bucketID, recordingID)
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
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.ArchiveRecordingWithResponse(ctx, bucketID, recordingID)
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
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.UnarchiveRecordingWithResponse(ctx, bucketID, recordingID)
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
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	body := generated.SetClientVisibilityJSONRequestBody{
		VisibleToClients: visible,
	}

	resp, err := s.client.gen.SetClientVisibilityWithResponse(ctx, bucketID, recordingID, body)
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
