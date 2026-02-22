package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// ScheduleEntryListOptions specifies options for listing schedule entries.
type ScheduleEntryListOptions struct {
	// Limit is the maximum number of entries to return.
	// If 0 (default), returns all entries. Use a positive value to cap results.
	Limit int

	// Page, if non-zero, disables pagination and returns only the first page.
	// NOTE: The page number itself is not yet honored due to OpenAPI client
	// limitations. Use 0 to paginate through all results up to Limit.
	Page int

	// Status filters entries by status: "active", "archived", or "trashed".
	// If empty, returns active entries (API default).
	Status string
}

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
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	Summary          string    `json:"summary"`
	InheritsStatus   bool      `json:"inherits_status"`
	Type             string    `json:"type"`
	URL              string    `json:"url"`
	AppURL           string    `json:"app_url"`
	BookmarkURL      string    `json:"bookmark_url"`
	SubscriptionURL  string    `json:"subscription_url"`
	CommentsURL      string    `json:"comments_url"`
	CommentsCount    int       `json:"comments_count"`
	StartsAt         time.Time `json:"starts_at"`
	EndsAt           time.Time `json:"ends_at"`
	AllDay           bool      `json:"all_day"`
	Description      string    `json:"description"`
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
	Participants     []Person  `json:"participants,omitempty"`
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

// ScheduleEntryListResult contains the results from listing schedule entries.
type ScheduleEntryListResult struct {
	// Entries is the list of schedule entries returned.
	Entries []ScheduleEntry
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// SchedulesService handles schedule operations.
type SchedulesService struct {
	client *AccountClient
}

// NewSchedulesService creates a new SchedulesService.
func NewSchedulesService(client *AccountClient) *SchedulesService {
	return &SchedulesService{client: client}
}

// Get returns a schedule by ID.
func (s *SchedulesService) Get(ctx context.Context, scheduleID int64) (result *Schedule, err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "Get",
		ResourceType: "schedule", IsMutation: false,
		ResourceID: scheduleID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetScheduleWithResponse(ctx, s.client.accountID, scheduleID)
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

	schedule := scheduleFromGenerated(*resp.JSON200)
	return &schedule, nil
}

// ListEntries returns all entries on a schedule.
//
// By default, returns all entries (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of entries to return (0 = all)
//   - Page: if non-zero, disables pagination and returns first page only
//
// The returned ScheduleEntryListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *SchedulesService) ListEntries(ctx context.Context, scheduleID int64, opts *ScheduleEntryListOptions) (result *ScheduleEntryListResult, err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "ListEntries",
		ResourceType: "schedule_entry", IsMutation: false,
		ResourceID: scheduleID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Build params for generated client
	var params *generated.ListScheduleEntriesParams
	if opts != nil && opts.Status != "" {
		params = &generated.ListScheduleEntriesParams{
			Status: opts.Status,
		}
	}

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListScheduleEntriesWithResponse(ctx, s.client.accountID, scheduleID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var entries []ScheduleEntry
	if resp.JSON200 != nil {
		for _, ge := range *resp.JSON200 {
			entries = append(entries, scheduleEntryFromGenerated(ge))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &ScheduleEntryListResult{Entries: entries, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = all (default for entries), >0 = specific limit
	limit := 0 // default to all for entries
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(entries) >= limit {
		return &ScheduleEntryListResult{Entries: entries[:limit], Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, err := s.client.parent.FollowPagination(ctx, resp.HTTPResponse, len(entries), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var ge generated.ScheduleEntry
		if err := json.Unmarshal(raw, &ge); err != nil {
			return nil, fmt.Errorf("failed to parse schedule entry: %w", err)
		}
		entries = append(entries, scheduleEntryFromGenerated(ge))
	}

	return &ScheduleEntryListResult{Entries: entries, Meta: ListMeta{TotalCount: totalCount}}, nil
}

// GetEntry returns a schedule entry by ID.
func (s *SchedulesService) GetEntry(ctx context.Context, entryID int64) (result *ScheduleEntry, err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "GetEntry",
		ResourceType: "schedule_entry", IsMutation: false,
		ResourceID: entryID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetScheduleEntryWithResponse(ctx, s.client.accountID, entryID)
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

	entry := scheduleEntryFromGenerated(*resp.JSON200)
	return &entry, nil
}

// CreateEntry creates a new entry on a schedule.
// Returns the created schedule entry.
func (s *SchedulesService) CreateEntry(ctx context.Context, scheduleID int64, req *CreateScheduleEntryRequest) (result *ScheduleEntry, err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "CreateEntry",
		ResourceType: "schedule_entry", IsMutation: true,
		ResourceID: scheduleID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.Summary == "" {
		err = ErrUsage("schedule entry summary is required")
		return nil, err
	}
	if req.StartsAt == "" {
		err = ErrUsage("schedule entry starts_at is required")
		return nil, err
	}
	if req.EndsAt == "" {
		err = ErrUsage("schedule entry ends_at is required")
		return nil, err
	}

	startsAt, parseErr := time.Parse(time.RFC3339, req.StartsAt)
	if parseErr != nil {
		err = ErrUsage("schedule entry starts_at must be in RFC3339 format (e.g., 2024-01-15T09:00:00Z)")
		return nil, err
	}
	endsAt, parseErr := time.Parse(time.RFC3339, req.EndsAt)
	if parseErr != nil {
		err = ErrUsage("schedule entry ends_at must be in RFC3339 format (e.g., 2024-01-15T17:00:00Z)")
		return nil, err
	}

	body := generated.CreateScheduleEntryJSONRequestBody{
		Summary:        req.Summary,
		StartsAt:       startsAt,
		EndsAt:         endsAt,
		Description:    req.Description,
		ParticipantIds: req.ParticipantIDs,
		AllDay:         &req.AllDay,
		Notify:         &req.Notify,
	}

	resp, err := s.client.parent.gen.CreateScheduleEntryWithResponse(ctx, s.client.accountID, scheduleID, body)
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

	entry := scheduleEntryFromGenerated(*resp.JSON201)
	return &entry, nil
}

// UpdateEntry updates an existing schedule entry.
// Returns the updated schedule entry.
func (s *SchedulesService) UpdateEntry(ctx context.Context, entryID int64, req *UpdateScheduleEntryRequest) (result *ScheduleEntry, err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "UpdateEntry",
		ResourceType: "schedule_entry", IsMutation: true,
		ResourceID: entryID,
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

	body := generated.UpdateScheduleEntryJSONRequestBody{
		Description:    req.Description,
		ParticipantIds: req.ParticipantIDs,
		AllDay:         &req.AllDay,
		Notify:         &req.Notify,
		Summary:        req.Summary,
	}
	if req.StartsAt != "" {
		startsAt, parseErr := time.Parse(time.RFC3339, req.StartsAt)
		if parseErr != nil {
			err = ErrUsage("schedule entry starts_at must be in RFC3339 format (e.g., 2024-01-15T09:00:00Z)")
			return nil, err
		}
		body.StartsAt = startsAt
	}
	if req.EndsAt != "" {
		endsAt, parseErr := time.Parse(time.RFC3339, req.EndsAt)
		if parseErr != nil {
			err = ErrUsage("schedule entry ends_at must be in RFC3339 format (e.g., 2024-01-15T17:00:00Z)")
			return nil, err
		}
		body.EndsAt = endsAt
	}

	resp, err := s.client.parent.gen.UpdateScheduleEntryWithResponse(ctx, s.client.accountID, entryID, body)
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

	entry := scheduleEntryFromGenerated(*resp.JSON200)
	return &entry, nil
}

// GetEntryOccurrence returns a specific occurrence of a recurring schedule entry.
func (s *SchedulesService) GetEntryOccurrence(ctx context.Context, entryID int64, date string) (result *ScheduleEntry, err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "GetEntryOccurrence",
		ResourceType: "schedule_entry", IsMutation: false,
		ResourceID: entryID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if date == "" {
		err = ErrUsage("occurrence date is required")
		return nil, err
	}

	resp, err := s.client.parent.gen.GetScheduleEntryOccurrenceWithResponse(ctx, s.client.accountID, entryID, date)
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

	entry := scheduleEntryFromGenerated(*resp.JSON200)
	return &entry, nil
}

// UpdateSettings updates the settings for a schedule.
// Returns the updated schedule.
func (s *SchedulesService) UpdateSettings(ctx context.Context, scheduleID int64, req *UpdateScheduleSettingsRequest) (result *Schedule, err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "UpdateSettings",
		ResourceType: "schedule", IsMutation: true,
		ResourceID: scheduleID,
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
		err = ErrUsage("update settings request is required")
		return nil, err
	}

	body := generated.UpdateScheduleSettingsJSONRequestBody{
		IncludeDueAssignments: req.IncludeDueAssignments,
	}

	resp, err := s.client.parent.gen.UpdateScheduleSettingsWithResponse(ctx, s.client.accountID, scheduleID, body)
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

	schedule := scheduleFromGenerated(*resp.JSON200)
	return &schedule, nil
}

// TrashEntry moves a schedule entry to the trash.
// Trashed entries can be recovered from the trash.
func (s *SchedulesService) TrashEntry(ctx context.Context, entryID int64) (err error) {
	op := OperationInfo{
		Service: "Schedules", Operation: "TrashEntry",
		ResourceType: "schedule_entry", IsMutation: true,
		ResourceID: entryID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.TrashRecordingWithResponse(ctx, s.client.accountID, entryID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Note: Permanent deletion of schedule entries is not supported by the Basecamp API.
// Use TrashEntry() to move entries to trash (recoverable via the web UI).

// scheduleFromGenerated converts a generated Schedule to our clean type.
func scheduleFromGenerated(gs generated.Schedule) Schedule {
	s := Schedule{
		Status:                gs.Status,
		VisibleToClients:      gs.VisibleToClients,
		CreatedAt:             gs.CreatedAt,
		UpdatedAt:             gs.UpdatedAt,
		Title:                 gs.Title,
		InheritsStatus:        gs.InheritsStatus,
		Type:                  gs.Type,
		URL:                   gs.Url,
		AppURL:                gs.AppUrl,
		BookmarkURL:           gs.BookmarkUrl,
		Position:              int(gs.Position),
		IncludeDueAssignments: gs.IncludeDueAssignments,
		EntriesCount:          int(gs.EntriesCount),
		EntriesURL:            gs.EntriesUrl,
	}

	if derefInt64(gs.Id) != 0 {
		s.ID = derefInt64(gs.Id)
	}

	if derefInt64(gs.Bucket.Id) != 0 || gs.Bucket.Name != "" {
		s.Bucket = &Bucket{
			ID:   derefInt64(gs.Bucket.Id),
			Name: gs.Bucket.Name,
			Type: gs.Bucket.Type,
		}
	}

	if derefInt64(gs.Creator.Id) != 0 || gs.Creator.Name != "" {
		s.Creator = &Person{
			ID:           derefInt64(gs.Creator.Id),
			Name:         gs.Creator.Name,
			EmailAddress: gs.Creator.EmailAddress,
			AvatarURL:    gs.Creator.AvatarUrl,
			Admin:        gs.Creator.Admin,
			Owner:        gs.Creator.Owner,
		}
	}

	return s
}

// scheduleEntryFromGenerated converts a generated ScheduleEntry to our clean type.
func scheduleEntryFromGenerated(ge generated.ScheduleEntry) ScheduleEntry {
	e := ScheduleEntry{
		Status:           ge.Status,
		VisibleToClients: ge.VisibleToClients,
		CreatedAt:        ge.CreatedAt,
		UpdatedAt:        ge.UpdatedAt,
		Title:            ge.Title,
		Summary:          ge.Summary,
		InheritsStatus:   ge.InheritsStatus,
		Type:             ge.Type,
		URL:              ge.Url,
		AppURL:           ge.AppUrl,
		BookmarkURL:      ge.BookmarkUrl,
		SubscriptionURL:  ge.SubscriptionUrl,
		CommentsURL:      ge.CommentsUrl,
		CommentsCount:    int(ge.CommentsCount),
		StartsAt:         ge.StartsAt,
		EndsAt:           ge.EndsAt,
		AllDay:           ge.AllDay,
		Description:      ge.Description,
	}

	if derefInt64(ge.Id) != 0 {
		e.ID = derefInt64(ge.Id)
	}

	if derefInt64(ge.Parent.Id) != 0 || ge.Parent.Title != "" {
		e.Parent = &Parent{
			ID:     derefInt64(ge.Parent.Id),
			Title:  ge.Parent.Title,
			Type:   ge.Parent.Type,
			URL:    ge.Parent.Url,
			AppURL: ge.Parent.AppUrl,
		}
	}

	if derefInt64(ge.Bucket.Id) != 0 || ge.Bucket.Name != "" {
		e.Bucket = &Bucket{
			ID:   derefInt64(ge.Bucket.Id),
			Name: ge.Bucket.Name,
			Type: ge.Bucket.Type,
		}
	}

	if derefInt64(ge.Creator.Id) != 0 || ge.Creator.Name != "" {
		e.Creator = &Person{
			ID:           derefInt64(ge.Creator.Id),
			Name:         ge.Creator.Name,
			EmailAddress: ge.Creator.EmailAddress,
			AvatarURL:    ge.Creator.AvatarUrl,
			Admin:        ge.Creator.Admin,
			Owner:        ge.Creator.Owner,
		}
	}

	// Convert participants
	if len(ge.Participants) > 0 {
		e.Participants = make([]Person, 0, len(ge.Participants))
		for _, gp := range ge.Participants {
			p := Person{
				Name:         gp.Name,
				EmailAddress: gp.EmailAddress,
				AvatarURL:    gp.AvatarUrl,
				Admin:        gp.Admin,
				Owner:        gp.Owner,
			}
			if derefInt64(gp.Id) != 0 {
				p.ID = derefInt64(gp.Id)
			}
			e.Participants = append(e.Participants, p)
		}
	}

	return e
}
