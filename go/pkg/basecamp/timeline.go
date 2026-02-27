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
	client *AccountClient
}

// NewTimelineService creates a new TimelineService.
func NewTimelineService(client *AccountClient) *TimelineService {
	return &TimelineService{client: client}
}

// Progress returns the account-wide activity feed.
// This shows recent activity across all projects.
func (s *TimelineService) Progress(ctx context.Context) (result []TimelineEvent, err error) {
	op := OperationInfo{
		Service: "Timeline", Operation: "Progress",
		ResourceType: "timeline_event", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetProgressReportWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	events := make([]TimelineEvent, 0, len(*resp.JSON200))
	for _, ge := range *resp.JSON200 {
		events = append(events, timelineEventFromGenerated(ge))
	}

	return events, nil
}

// ProjectTimeline returns the activity timeline for a specific project.
func (s *TimelineService) ProjectTimeline(ctx context.Context, projectID int64) (result []TimelineEvent, err error) {
	op := OperationInfo{
		Service: "Timeline", Operation: "ProjectTimeline",
		ResourceType: "timeline_event", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetProjectTimelineWithResponse(ctx, s.client.accountID, projectID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	events := make([]TimelineEvent, 0, len(*resp.JSON200))
	for _, ge := range *resp.JSON200 {
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
func (s *TimelineService) PersonProgress(ctx context.Context, personID int64) (result *PersonProgressResponse, err error) {
	op := OperationInfo{
		Service: "Timeline", Operation: "PersonProgress",
		ResourceType: "timeline_event", IsMutation: false,
		ResourceID: personID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetPersonProgressWithResponse(ctx, s.client.accountID, personID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	result = &PersonProgressResponse{}

	if derefInt64(resp.JSON200.Person.Id) != 0 || resp.JSON200.Person.Name != "" {
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

	if derefInt64(ge.Bucket.Id) != 0 || ge.Bucket.Name != "" {
		e.Bucket = &Bucket{
			ID:   derefInt64(ge.Bucket.Id),
			Name: ge.Bucket.Name,
			Type: ge.Bucket.Type,
		}
	}

	return e
}
