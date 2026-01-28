package basecamp

import (
	"context"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// TimelineEvent represents an activity event in the timeline.
type TimelineEvent struct {
	ID                int64     `json:"id"`
	CreatedAt         time.Time `json:"created_at"`
	Kind              string    `json:"kind"`
	ParentRecordingID int64     `json:"parent_recording_id"`
	URL               string    `json:"url"`
	AppURL            string    `json:"app_url"`
	Creator           *Person   `json:"creator,omitempty"`
	Action            string    `json:"action"`
	Target            string    `json:"target"`
	Title             string    `json:"title"`
	SummaryExcerpt    string    `json:"summary_excerpt"`
	Bucket            *Bucket   `json:"bucket,omitempty"`
}

// TimelineService handles timeline and progress operations.
type TimelineService struct {
	client *Client
}

// NewTimelineService creates a new TimelineService.
func NewTimelineService(client *Client) *TimelineService {
	return &TimelineService{client: client}
}

// Progress returns the account-wide activity feed.
// This shows recent activity across all projects.
func (s *TimelineService) Progress(ctx context.Context) ([]TimelineEvent, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetProgressReportWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	events := make([]TimelineEvent, 0, len(resp.JSON200.Events))
	for _, ge := range resp.JSON200.Events {
		events = append(events, timelineEventFromGenerated(ge))
	}

	return events, nil
}

// ProjectTimeline returns the activity timeline for a specific project.
func (s *TimelineService) ProjectTimeline(ctx context.Context, projectID int64) ([]TimelineEvent, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetProjectTimelineWithResponse(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	events := make([]TimelineEvent, 0, len(resp.JSON200.Events))
	for _, ge := range resp.JSON200.Events {
		events = append(events, timelineEventFromGenerated(ge))
	}

	return events, nil
}

// PersonProgressResponse contains a person's activity timeline.
type PersonProgressResponse struct {
	Person *Person         `json:"person"`
	Events []TimelineEvent `json:"events"`
}

// PersonProgress returns the activity timeline for a specific person.
func (s *TimelineService) PersonProgress(ctx context.Context, personID int64) (*PersonProgressResponse, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetPersonProgressWithResponse(ctx, personID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	result := &PersonProgressResponse{}

	if resp.JSON200.Person.Id != nil || resp.JSON200.Person.Name != "" {
		result.Person = &Person{
			ID:           derefInt64(resp.JSON200.Person.Id),
			Name:         resp.JSON200.Person.Name,
			EmailAddress: resp.JSON200.Person.EmailAddress,
			AvatarURL:    resp.JSON200.Person.AvatarUrl,
			Admin:        resp.JSON200.Person.Admin,
			Owner:        resp.JSON200.Person.Owner,
		}
	}

	result.Events = make([]TimelineEvent, 0, len(resp.JSON200.Events))
	for _, ge := range resp.JSON200.Events {
		result.Events = append(result.Events, timelineEventFromGenerated(ge))
	}

	return result, nil
}

// timelineEventFromGenerated converts a generated TimelineEvent to our clean type.
func timelineEventFromGenerated(ge generated.TimelineEvent) TimelineEvent {
	e := TimelineEvent{
		Kind:           ge.Kind,
		URL:            ge.Url,
		AppURL:         ge.AppUrl,
		Action:         ge.Action,
		Target:         ge.Target,
		Title:          ge.Title,
		SummaryExcerpt: ge.SummaryExcerpt,
	}

	if ge.Id != nil {
		e.ID = *ge.Id
	}
	if ge.ParentRecordingId != nil {
		e.ParentRecordingID = *ge.ParentRecordingId
	}

	e.CreatedAt = ge.CreatedAt

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

	if ge.Bucket.Id != nil || ge.Bucket.Name != "" {
		e.Bucket = &Bucket{
			ID:   derefInt64(ge.Bucket.Id),
			Name: ge.Bucket.Name,
			Type: ge.Bucket.Type,
		}
	}

	return e
}
