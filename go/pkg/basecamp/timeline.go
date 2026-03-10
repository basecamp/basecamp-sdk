package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// DefaultTimelineLimit is the default number of timeline events to return when no limit is specified.
const DefaultTimelineLimit = 100

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

// TimelineListOptions specifies options for listing timeline events.
type TimelineListOptions struct {
	// Limit is the maximum number of events to return.
	// If 0, uses DefaultTimelineLimit (100). Use -1 for unlimited.
	Limit int

	// Page, if positive, disables auto-pagination and returns only the first page.
	// NOTE: The page number itself is not yet honored due to OpenAPI client
	// limitations. Use 0 to paginate through all results up to Limit.
	Page int
}

// TimelineListResult contains the results from listing timeline events.
type TimelineListResult struct {
	// Events is the list of timeline events returned.
	Events []TimelineEvent
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// PersonProgressResult contains a person's activity timeline with pagination metadata.
type PersonProgressResult struct {
	Person *Person
	Events []TimelineEvent
	Meta   ListMeta
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
func (s *TimelineService) Progress(ctx context.Context, opts *TimelineListOptions) (result *TimelineListResult, err error) {
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

	totalCount := parseTotalCount(resp.HTTPResponse)

	var events []TimelineEvent
	if resp.JSON200 != nil {
		for _, ge := range *resp.JSON200 {
			events = append(events, timelineEventFromGenerated(ge))
		}
	}

	if opts != nil && opts.Page > 0 {
		return &TimelineListResult{Events: events, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	limit := DefaultTimelineLimit
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	if limit > 0 && len(events) >= limit {
		return &TimelineListResult{Events: events[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(events), limit)}}, nil
	}

	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(events), limit)
	if err != nil {
		return nil, err
	}

	for _, raw := range rawMore {
		var ge generated.TimelineEvent
		if err := json.Unmarshal(raw, &ge); err != nil {
			return nil, fmt.Errorf("failed to parse timeline event: %w", err)
		}
		events = append(events, timelineEventFromGenerated(ge))
	}

	return &TimelineListResult{Events: events, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// ProjectTimeline returns the activity timeline for a specific project.
func (s *TimelineService) ProjectTimeline(ctx context.Context, projectID int64, opts *TimelineListOptions) (result *TimelineListResult, err error) {
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

	totalCount := parseTotalCount(resp.HTTPResponse)

	var events []TimelineEvent
	if resp.JSON200 != nil {
		for _, ge := range *resp.JSON200 {
			events = append(events, timelineEventFromGenerated(ge))
		}
	}

	if opts != nil && opts.Page > 0 {
		return &TimelineListResult{Events: events, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	limit := DefaultTimelineLimit
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	if limit > 0 && len(events) >= limit {
		return &TimelineListResult{Events: events[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(events), limit)}}, nil
	}

	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(events), limit)
	if err != nil {
		return nil, err
	}

	for _, raw := range rawMore {
		var ge generated.TimelineEvent
		if err := json.Unmarshal(raw, &ge); err != nil {
			return nil, fmt.Errorf("failed to parse timeline event: %w", err)
		}
		events = append(events, timelineEventFromGenerated(ge))
	}

	return &TimelineListResult{Events: events, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// PersonProgress returns the activity timeline for a specific person.
func (s *TimelineService) PersonProgress(ctx context.Context, personID int64, opts *TimelineListOptions) (result *PersonProgressResult, err error) {
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

	totalCount := parseTotalCount(resp.HTTPResponse)

	result = &PersonProgressResult{}

	if resp.JSON200.Person.Id != 0 || resp.JSON200.Person.Name != "" {
		result.Person = &Person{
			ID:           resp.JSON200.Person.Id,
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

	if opts != nil && opts.Page > 0 {
		result.Meta = ListMeta{TotalCount: totalCount}
		return result, nil
	}

	limit := DefaultTimelineLimit
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	if limit > 0 && len(result.Events) >= limit {
		result.Events = result.Events[:limit]
		result.Meta = ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(resp.JSON200.Events), limit)}
		return result, nil
	}

	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(result.Events), limit)
	if err != nil {
		return nil, err
	}

	for _, raw := range rawMore {
		var ge generated.TimelineEvent
		if err := json.Unmarshal(raw, &ge); err != nil {
			return nil, fmt.Errorf("failed to parse timeline event: %w", err)
		}
		result.Events = append(result.Events, timelineEventFromGenerated(ge))
	}

	result.Meta = ListMeta{TotalCount: totalCount, Truncated: truncated}
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

	if ge.Creator.Id != 0 || ge.Creator.Name != "" {
		e.Creator = &Person{
			ID:           ge.Creator.Id,
			Name:         ge.Creator.Name,
			EmailAddress: ge.Creator.EmailAddress,
			AvatarURL:    ge.Creator.AvatarUrl,
			Admin:        ge.Creator.Admin,
			Owner:        ge.Creator.Owner,
		}
	}

	if ge.Bucket.Id != 0 || ge.Bucket.Name != "" {
		e.Bucket = &Bucket{
			ID:   ge.Bucket.Id,
			Name: ge.Bucket.Name,
			Type: ge.Bucket.Type,
		}
	}

	return e
}
