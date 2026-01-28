package basecamp

import (
	"context"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

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
// bucketID is the project ID, recordingID is the ID of the recording.
func (s *EventsService) List(ctx context.Context, bucketID, recordingID int64) (result []Event, err error) {
	op := OperationInfo{
		Service: "Events", Operation: "List",
		ResourceType: "event", IsMutation: false,
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

	resp, err := s.client.parent.gen.ListEventsWithResponse(ctx, s.client.accountID, bucketID, recordingID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	events := make([]Event, 0, len(*resp.JSON200))
	for _, ge := range *resp.JSON200 {
		events = append(events, eventFromGenerated(ge))
	}

	return events, nil
}

// eventFromGenerated converts a generated Event to our clean type.
func eventFromGenerated(ge generated.Event) Event {
	e := Event{
		RecordingID: derefInt64(ge.RecordingId),
		Action:      ge.Action,
		CreatedAt:   ge.CreatedAt,
	}

	if ge.Id != nil {
		e.ID = *ge.Id
	}

	// Convert details
	if ge.Details.AddedPersonIds != nil || ge.Details.RemovedPersonIds != nil || ge.Details.NotifiedRecipientIds != nil {
		e.Details = &EventDetails{
			AddedPersonIDs:       ge.Details.AddedPersonIds,
			RemovedPersonIDs:     ge.Details.RemovedPersonIds,
			NotifiedRecipientIDs: ge.Details.NotifiedRecipientIds,
		}
	}

	if ge.Creator.Id != nil || ge.Creator.Name != "" {
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
