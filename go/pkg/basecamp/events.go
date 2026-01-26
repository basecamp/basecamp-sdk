package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
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
	client *Client
}

// NewEventsService creates a new EventsService.
func NewEventsService(client *Client) *EventsService {
	return &EventsService{client: client}
}

// List returns all events for a recording.
// bucketID is the project ID, recordingID is the ID of the recording.
func (s *EventsService) List(ctx context.Context, bucketID, recordingID int64) ([]Event, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/events.json", bucketID, recordingID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	events := make([]Event, 0, len(results))
	for _, raw := range results {
		var e Event
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("failed to parse event: %w", err)
		}
		events = append(events, e)
	}

	return events, nil
}
