package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Schedule represents a Basecamp schedule (calendar) within a project.
type Schedule struct {
	ID                    int64     `json:"id"`
	Status                string    `json:"status"`
	VisibleToClients      bool      `json:"visible_to_clients"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
	Title                 string    `json:"title"`
	InheritsStatus        bool      `json:"inherits_status"`
	Type                  string    `json:"type"`
	URL                   string    `json:"url"`
	AppURL                string    `json:"app_url"`
	BookmarkURL           string    `json:"bookmark_url"`
	Position              int       `json:"position"`
	Bucket                *Bucket   `json:"bucket,omitempty"`
	Creator               *Person   `json:"creator,omitempty"`
	IncludeDueAssignments bool      `json:"include_due_assignments"`
	EntriesCount          int       `json:"entries_count"`
	EntriesURL            string    `json:"entries_url"`
}

// ScheduleEntry represents an event on a Basecamp schedule.
type ScheduleEntry struct {
	ID               int64      `json:"id"`
	Status           string     `json:"status"`
	VisibleToClients bool       `json:"visible_to_clients"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Title            string     `json:"title"`
	Summary          string     `json:"summary"`
	InheritsStatus   bool       `json:"inherits_status"`
	Type             string     `json:"type"`
	URL              string     `json:"url"`
	AppURL           string     `json:"app_url"`
	BookmarkURL      string     `json:"bookmark_url"`
	SubscriptionURL  string     `json:"subscription_url"`
	CommentsURL      string     `json:"comments_url"`
	CommentsCount    int        `json:"comments_count"`
	StartsAt         time.Time  `json:"starts_at"`
	EndsAt           time.Time  `json:"ends_at"`
	AllDay           bool       `json:"all_day"`
	Description      string     `json:"description"`
	Parent           *Parent    `json:"parent,omitempty"`
	Bucket           *Bucket    `json:"bucket,omitempty"`
	Creator          *Person    `json:"creator,omitempty"`
	Participants     []Person   `json:"participants,omitempty"`
}

// CreateScheduleEntryRequest specifies the parameters for creating a schedule entry.
type CreateScheduleEntryRequest struct {
	// Summary is the event title (required).
	Summary string `json:"summary"`
	// StartsAt is the event start time (required, ISO 8601 format).
	StartsAt string `json:"starts_at"`
	// EndsAt is the event end time (required, ISO 8601 format).
	EndsAt string `json:"ends_at"`
	// Description is the event details in HTML (optional).
	Description string `json:"description,omitempty"`
	// ParticipantIDs is a list of people IDs to assign (optional).
	ParticipantIDs []int64 `json:"participant_ids,omitempty"`
	// AllDay indicates if this is an all-day event (optional).
	AllDay bool `json:"all_day,omitempty"`
	// Notify triggers participant notifications when true (optional).
	Notify bool `json:"notify,omitempty"`
}

// UpdateScheduleEntryRequest specifies the parameters for updating a schedule entry.
type UpdateScheduleEntryRequest struct {
	// Summary is the event title (optional).
	Summary string `json:"summary,omitempty"`
	// StartsAt is the event start time (optional, ISO 8601 format).
	StartsAt string `json:"starts_at,omitempty"`
	// EndsAt is the event end time (optional, ISO 8601 format).
	EndsAt string `json:"ends_at,omitempty"`
	// Description is the event details in HTML (optional).
	Description string `json:"description,omitempty"`
	// ParticipantIDs is a list of people IDs to assign (optional).
	ParticipantIDs []int64 `json:"participant_ids,omitempty"`
	// AllDay indicates if this is an all-day event (optional).
	AllDay bool `json:"all_day,omitempty"`
	// Notify triggers participant notifications when true (optional).
	Notify bool `json:"notify,omitempty"`
}

// UpdateScheduleSettingsRequest specifies the parameters for updating schedule settings.
type UpdateScheduleSettingsRequest struct {
	// IncludeDueAssignments controls whether to-do due dates appear on the schedule.
	IncludeDueAssignments bool `json:"include_due_assignments"`
}

// SchedulesService handles schedule operations.
type SchedulesService struct {
	client *Client
}

// NewSchedulesService creates a new SchedulesService.
func NewSchedulesService(client *Client) *SchedulesService {
	return &SchedulesService{client: client}
}

// Get returns a schedule by ID.
// bucketID is the project ID, scheduleID is the schedule ID.
func (s *SchedulesService) Get(ctx context.Context, bucketID, scheduleID int64) (*Schedule, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/schedules/%d.json", bucketID, scheduleID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var schedule Schedule
	if err := resp.UnmarshalData(&schedule); err != nil {
		return nil, fmt.Errorf("failed to parse schedule: %w", err)
	}

	return &schedule, nil
}

// ListEntries returns all entries on a schedule.
// bucketID is the project ID, scheduleID is the schedule ID.
func (s *SchedulesService) ListEntries(ctx context.Context, bucketID, scheduleID int64) ([]ScheduleEntry, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/schedules/%d/entries.json", bucketID, scheduleID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	entries := make([]ScheduleEntry, 0, len(results))
	for _, raw := range results {
		var e ScheduleEntry
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("failed to parse schedule entry: %w", err)
		}
		entries = append(entries, e)
	}

	return entries, nil
}

// GetEntry returns a schedule entry by ID.
// bucketID is the project ID, entryID is the schedule entry ID.
func (s *SchedulesService) GetEntry(ctx context.Context, bucketID, entryID int64) (*ScheduleEntry, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/schedule_entries/%d.json", bucketID, entryID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var entry ScheduleEntry
	if err := resp.UnmarshalData(&entry); err != nil {
		return nil, fmt.Errorf("failed to parse schedule entry: %w", err)
	}

	return &entry, nil
}

// CreateEntry creates a new entry on a schedule.
// bucketID is the project ID, scheduleID is the schedule ID.
// Returns the created schedule entry.
func (s *SchedulesService) CreateEntry(ctx context.Context, bucketID, scheduleID int64, req *CreateScheduleEntryRequest) (*ScheduleEntry, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Summary == "" {
		return nil, ErrUsage("schedule entry summary is required")
	}
	if req.StartsAt == "" {
		return nil, ErrUsage("schedule entry starts_at is required")
	}
	if req.EndsAt == "" {
		return nil, ErrUsage("schedule entry ends_at is required")
	}

	path := fmt.Sprintf("/buckets/%d/schedules/%d/entries.json", bucketID, scheduleID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var entry ScheduleEntry
	if err := resp.UnmarshalData(&entry); err != nil {
		return nil, fmt.Errorf("failed to parse schedule entry: %w", err)
	}

	return &entry, nil
}

// UpdateEntry updates an existing schedule entry.
// bucketID is the project ID, entryID is the schedule entry ID.
// Returns the updated schedule entry.
func (s *SchedulesService) UpdateEntry(ctx context.Context, bucketID, entryID int64, req *UpdateScheduleEntryRequest) (*ScheduleEntry, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil {
		return nil, ErrUsage("update request is required")
	}

	path := fmt.Sprintf("/buckets/%d/schedule_entries/%d.json", bucketID, entryID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var entry ScheduleEntry
	if err := resp.UnmarshalData(&entry); err != nil {
		return nil, fmt.Errorf("failed to parse schedule entry: %w", err)
	}

	return &entry, nil
}

// GetEntryOccurrence returns a specific occurrence of a recurring schedule entry.
// bucketID is the project ID, entryID is the schedule entry ID, date is the occurrence date (YYYY-MM-DD format).
func (s *SchedulesService) GetEntryOccurrence(ctx context.Context, bucketID, entryID int64, date string) (*ScheduleEntry, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if date == "" {
		return nil, ErrUsage("occurrence date is required")
	}

	path := fmt.Sprintf("/buckets/%d/schedule_entries/%d/occurrences/%s.json", bucketID, entryID, date)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var entry ScheduleEntry
	if err := resp.UnmarshalData(&entry); err != nil {
		return nil, fmt.Errorf("failed to parse schedule entry occurrence: %w", err)
	}

	return &entry, nil
}

// UpdateSettings updates the settings for a schedule.
// bucketID is the project ID, scheduleID is the schedule ID.
// Returns the updated schedule.
func (s *SchedulesService) UpdateSettings(ctx context.Context, bucketID, scheduleID int64, req *UpdateScheduleSettingsRequest) (*Schedule, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil {
		return nil, ErrUsage("update settings request is required")
	}

	path := fmt.Sprintf("/buckets/%d/schedules/%d.json", bucketID, scheduleID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var schedule Schedule
	if err := resp.UnmarshalData(&schedule); err != nil {
		return nil, fmt.Errorf("failed to parse schedule: %w", err)
	}

	return &schedule, nil
}

// TrashEntry moves a schedule entry to the trash.
// bucketID is the project ID, entryID is the schedule entry ID.
// Trashed entries can be recovered from the trash.
func (s *SchedulesService) TrashEntry(ctx context.Context, bucketID, entryID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/status/trashed.json", bucketID, entryID)
	_, err := s.client.Put(ctx, path, nil)
	return err
}

// DeleteEntry permanently removes a schedule entry.
// bucketID is the project ID, entryID is the schedule entry ID.
// Note: This permanently deletes the entry. Use TrashEntry for recoverable deletion.
func (s *SchedulesService) DeleteEntry(ctx context.Context, bucketID, entryID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/schedule_entries/%d.json", bucketID, entryID)
	_, err := s.client.Delete(ctx, path)
	return err
}
