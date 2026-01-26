package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
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
func (s *RecordingsService) List(ctx context.Context, recordingType RecordingType, opts *RecordingsListOptions) ([]Recording, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if recordingType == "" {
		return nil, ErrUsage("recording type is required")
	}

	// Build query parameters
	params := url.Values{}
	params.Set("type", string(recordingType))

	if opts != nil {
		if len(opts.Bucket) > 0 {
			bucketStrs := make([]string, len(opts.Bucket))
			for i, b := range opts.Bucket {
				bucketStrs[i] = fmt.Sprintf("%d", b)
			}
			params.Set("bucket", strings.Join(bucketStrs, ","))
		}
		if opts.Status != "" {
			params.Set("status", opts.Status)
		}
		if opts.Sort != "" {
			params.Set("sort", opts.Sort)
		}
		if opts.Direction != "" {
			params.Set("direction", opts.Direction)
		}
	}

	path := "/projects/recordings.json?" + params.Encode()
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	recordings := make([]Recording, 0, len(results))
	for _, raw := range results {
		var r Recording
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("failed to parse recording: %w", err)
		}
		recordings = append(recordings, r)
	}

	return recordings, nil
}

// Get returns a recording by ID.
// bucketID is the project ID, recordingID is the recording ID.
func (s *RecordingsService) Get(ctx context.Context, bucketID, recordingID int64) (*Recording, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d.json", bucketID, recordingID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var recording Recording
	if err := resp.UnmarshalData(&recording); err != nil {
		return nil, fmt.Errorf("failed to parse recording: %w", err)
	}

	return &recording, nil
}

// Trash moves a recording to the trash.
// bucketID is the project ID, recordingID is the recording ID.
// Trashed recordings can be recovered from the trash.
func (s *RecordingsService) Trash(ctx context.Context, bucketID, recordingID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/status/trashed.json", bucketID, recordingID)
	_, err := s.client.Put(ctx, path, nil)
	return err
}

// Archive archives a recording.
// bucketID is the project ID, recordingID is the recording ID.
// Archived recordings are hidden but not deleted.
func (s *RecordingsService) Archive(ctx context.Context, bucketID, recordingID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/status/archived.json", bucketID, recordingID)
	_, err := s.client.Put(ctx, path, nil)
	return err
}

// Unarchive restores an archived recording to active status.
// bucketID is the project ID, recordingID is the recording ID.
func (s *RecordingsService) Unarchive(ctx context.Context, bucketID, recordingID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/status/active.json", bucketID, recordingID)
	_, err := s.client.Put(ctx, path, nil)
	return err
}

// SetClientVisibility sets whether a recording is visible to clients.
// bucketID is the project ID, recordingID is the recording ID.
// visible specifies whether the recording should be visible to clients.
// Returns the updated recording.
// Note: Not all recordings support client visibility. Some inherit visibility from their parent.
func (s *RecordingsService) SetClientVisibility(ctx context.Context, bucketID, recordingID int64, visible bool) (*Recording, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	req := &SetClientVisibilityRequest{
		VisibleToClients: visible,
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/client_visibility.json", bucketID, recordingID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var recording Recording
	if err := resp.UnmarshalData(&recording); err != nil {
		return nil, fmt.Errorf("failed to parse recording: %w", err)
	}

	return &recording, nil
}
