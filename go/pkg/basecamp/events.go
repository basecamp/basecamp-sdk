package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// DefaultEventLimit is the default number of events to return when no limit is specified.
const DefaultEventLimit = 100

// EventListOptions specifies options for listing events.
type EventListOptions struct {
	// Limit is the maximum number of events to return.
	// If 0, uses DefaultEventLimit (100). Use -1 for unlimited.
	Limit int

	// Page, if non-zero, disables pagination and returns only the first page.
	// NOTE: The page number itself is not yet honored due to OpenAPI client
	// limitations. Use 0 to paginate through all results up to Limit.
	Page int
}

// EventListResult contains the results from listing events.
type EventListResult struct {
	// Events is the list of events returned.
	Events []Event
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// Event represents a recording change event in Basecamp.
// An event is created any time a recording changes.
type Event struct {
	ID          int64         `json:"id"`
	RecordingID int64         `json:"recording_id"`
	Action      string        `json:"action"`
	Details     *EventDetails `json:"details,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	Creator     *Person       `json:"creator,omitempty"`
}

// EventDetails contains action-specific information for an event.
type EventDetails struct {
	// AddedPersonIDs is populated for assignment_changed actions.
	AddedPersonIDs []int64 `json:"added_person_ids,omitempty"`
	// RemovedPersonIDs is populated for assignment_changed actions.
	RemovedPersonIDs []int64 `json:"removed_person_ids,omitempty"`
	// NotifiedRecipientIDs is populated for completion events.
	NotifiedRecipientIDs []int64 `json:"notified_recipient_ids,omitempty"`
}

// EventsService handles event operations.
type EventsService struct {
	client *AccountClient
}

// NewEventsService creates a new EventsService.
func NewEventsService(client *AccountClient) *EventsService {
	return &EventsService{client: client}
}

// List returns all events for a recording.
//
// By default, returns up to 100 events. Use Limit: -1 for unlimited.
//
// Pagination options:
//   - Limit: maximum number of events to return (0 = 100, -1 = unlimited)
//   - Page: if non-zero, disables pagination and returns first page only
//
// The returned EventListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *EventsService) List(ctx context.Context, recordingID int64, opts *EventListOptions) (result *EventListResult, err error) {
	op := OperationInfo{
		Service: "Events", Operation: "List",
		ResourceType: "event", IsMutation: false,
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

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListEventsWithResponse(ctx, s.client.accountID, recordingID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var events []Event
	if resp.JSON200 != nil {
		for _, ge := range *resp.JSON200 {
			events = append(events, eventFromGenerated(ge))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &EventListResult{Events: events, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = default (100), -1 = unlimited, >0 = specific limit
	limit := DefaultEventLimit
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0 // unlimited
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	// Check if we already have enough items
	if limit > 0 && len(events) >= limit {
		return &EventListResult{Events: events[:limit], Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, err := s.client.parent.FollowPagination(ctx, resp.HTTPResponse, len(events), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var ge generated.Event
		if err := json.Unmarshal(raw, &ge); err != nil {
			return nil, fmt.Errorf("failed to parse event: %w", err)
		}
		events = append(events, eventFromGenerated(ge))
	}

	return &EventListResult{Events: events, Meta: ListMeta{TotalCount: totalCount}}, nil
}

// eventFromGenerated converts a generated Event to our clean type.
func eventFromGenerated(ge generated.Event) Event {
	e := Event{
		RecordingID: derefInt64(ge.RecordingId),
		Action:      ge.Action,
		CreatedAt:   ge.CreatedAt,
	}

	if derefInt64(ge.Id) != 0 {
		e.ID = derefInt64(ge.Id)
	}

	// Convert details
	if ge.Details.AddedPersonIds != nil || ge.Details.RemovedPersonIds != nil || ge.Details.NotifiedRecipientIds != nil {
		e.Details = &EventDetails{
			AddedPersonIDs:       ge.Details.AddedPersonIds,
			RemovedPersonIDs:     ge.Details.RemovedPersonIds,
			NotifiedRecipientIDs: ge.Details.NotifiedRecipientIds,
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

	return e
}
