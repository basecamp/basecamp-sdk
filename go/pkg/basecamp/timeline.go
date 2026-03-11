package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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
	// If 0, uses DefaultTimelineLimit (100). Any negative value means unlimited.
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
//
// Pagination options:
//   - Limit: maximum number of events to return (0 = DefaultTimelineLimit, negative = unlimited)
//   - Page: if non-zero, disables pagination and returns first page only
//
// The returned TimelineListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
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

	// Call generated client for first page (spec-conformant - no manual path construction)
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
//
// Pagination options:
//   - Limit: maximum number of events to return (0 = DefaultTimelineLimit, negative = unlimited)
//   - Page: if non-zero, disables pagination and returns first page only
//
// The returned TimelineListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
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

	// Call generated client for first page (spec-conformant - no manual path construction)
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
//
// Each page of this endpoint returns a wrapped response with {person, events}.
// Pagination is handled with a custom loop since followPagination expects bare
// arrays.
//
// Pagination options:
//   - Limit: maximum number of events to return (0 = DefaultTimelineLimit, negative = unlimited)
//   - Page: if non-zero, disables pagination and returns first page only
//
// The returned PersonProgressResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
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
		return nil, fmt.Errorf("unexpected empty response")
	}

	totalCount := parseTotalCount(resp.HTTPResponse)

	// Extract person from first page
	var person *Person
	if resp.JSON200.Person.Id != 0 || resp.JSON200.Person.Name != "" {
		person = &Person{
			ID:           resp.JSON200.Person.Id,
			Name:         resp.JSON200.Person.Name,
			EmailAddress: resp.JSON200.Person.EmailAddress,
			AvatarURL:    resp.JSON200.Person.AvatarUrl,
			Admin:        resp.JSON200.Person.Admin,
			Owner:        resp.JSON200.Person.Owner,
		}
	}

	// Parse events from first page
	var events []TimelineEvent
	for _, ge := range resp.JSON200.Events {
		events = append(events, timelineEventFromGenerated(ge))
	}

	if opts != nil && opts.Page > 0 {
		return &PersonProgressResult{Person: person, Events: events, Meta: ListMeta{TotalCount: totalCount}}, nil
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
		return &PersonProgressResult{
			Person: person,
			Events: events[:limit],
			Meta:   ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(events), limit)},
		}, nil
	}

	// Custom wrapped pagination for PersonProgress.
	// Each page returns {person, events} — we can't use followPagination which
	// expects bare arrays. Instead, follow the same approach: parse Link headers,
	// validate same-origin, fetch with doRequestURL.
	truncated := false
	if resp.HTTPResponse.Request != nil && resp.HTTPResponse.Request.URL != nil {
		nextLink := parseNextLink(resp.HTTPResponse.Header.Get("Link"))
		baseURL := resp.HTTPResponse.Request.URL.String()
		currentPageURL := baseURL

		for page := 2; nextLink != "" && page <= s.client.parent.httpOpts.MaxPages; page++ {
			// Resolve and validate URL
			nextURL := resolveURL(currentPageURL, nextLink)

			parsedURL, parseErr := url.Parse(nextURL)
			if parseErr != nil || !parsedURL.IsAbs() {
				return nil, fmt.Errorf("failed to resolve Link header URL %q against %q", nextLink, currentPageURL)
			}

			if !isSameOrigin(baseURL, nextURL) {
				return nil, fmt.Errorf("pagination Link header points to different origin: %s", nextURL)
			}

			// Check limit before fetching
			if limit > 0 && len(events) >= limit {
				truncated = true
				break
			}

			pageResp, fetchErr := s.client.parent.doRequestURL(ctx, "GET", nextURL, nil)
			if fetchErr != nil {
				return nil, fetchErr
			}

			// Parse wrapped response — we only need events
			var wrapper struct {
				Events []json.RawMessage `json:"events"`
			}
			if unmarshalErr := json.Unmarshal(pageResp.Data, &wrapper); unmarshalErr != nil {
				return nil, fmt.Errorf("failed to parse person progress page %d: %w", page, unmarshalErr)
			}

			for _, raw := range wrapper.Events {
				var ge generated.TimelineEvent
				if unmarshalErr := json.Unmarshal(raw, &ge); unmarshalErr != nil {
					return nil, fmt.Errorf("failed to parse timeline event: %w", unmarshalErr)
				}
				events = append(events, timelineEventFromGenerated(ge))
			}

			// Check limit after adding items from this page
			if limit > 0 && len(events) >= limit {
				excess := len(events) - limit
				if excess > 0 {
					events = events[:limit]
				}
				// Truncated if we dropped items OR more pages exist
				nextLink = parseNextLink(pageResp.Headers.Get("Link"))
				if excess > 0 || nextLink != "" {
					truncated = true
				}
				break
			}

			nextLink = parseNextLink(pageResp.Headers.Get("Link"))
			currentPageURL = nextURL
		}

		// If we exited the loop because of MaxPages (page > MaxPages with nextLink still set)
		if nextLink != "" && !truncated {
			truncated = true
			s.client.parent.logger.Warn("pagination capped", "maxPages", s.client.parent.httpOpts.MaxPages)
		}
	}

	return &PersonProgressResult{Person: person, Events: events, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
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
