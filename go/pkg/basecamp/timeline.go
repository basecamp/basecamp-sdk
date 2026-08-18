package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
	"github.com/basecamp/basecamp-sdk/go/pkg/types"
)

// DefaultTimelineLimit is the default number of timeline events to return when no limit is specified.
const DefaultTimelineLimit = 100

// TimelineEvent represents an activity event in the timeline.
type TimelineEvent struct {
	ID int64 `json:"id"`
	// CreatedAt is optional — nil means the API did not send it. A value type
	// here would fabricate 0001-01-01T00:00:00Z for an absent timestamp, which
	// no consumer can tell from a real one.
	CreatedAt *time.Time `json:"created_at,omitempty"`
	// Kind is an open, non-exhaustive vocabulary (BC3 adds new kinds over time);
	// treat unrecognized values as valid. Common values include message_created,
	// todo_created, todo_completed, upload_created, schedule_entry_created,
	// chat_transcript_rollup, dock_created, and project_access_changed.
	Kind              string  `json:"kind"`
	ParentRecordingID int64   `json:"parent_recording_id"`
	URL               string  `json:"url"`
	AppURL            string  `json:"app_url"`
	Creator           *Person `json:"creator,omitempty"`
	Action            string  `json:"action"`
	Target            string  `json:"target"`
	Title             string  `json:"title"`
	SummaryExcerpt    string  `json:"summary_excerpt"`
	// AvatarsSample holds avatar URLs of participants, populated for
	// chat_transcript_rollup events and empty otherwise.
	AvatarsSample []string `json:"avatars_sample,omitempty"`
	Bucket        *Bucket  `json:"bucket,omitempty"`
	// Data carries schedule-entry timing, present only for schedule_entry_created
	// and schedule_entry_rescheduled events.
	Data *TimelineEventData `json:"data,omitempty"`
	// Attachments holds files attached to the event's recording. It is
	// heterogeneous: an upload-kind recording contributes its full Upload shape,
	// other recordings contribute rich-text attachment/blob partials. Each
	// element is an optional-field superset; only the fields of the variant it
	// represents are populated.
	Attachments []TimelineAttachment `json:"attachments,omitempty"`
}

// TimelineEventData carries schedule-entry timing for schedule_entry_* events.
// StartsAt and EndsAt are date-or-timestamp (*types.FlexibleTime): a full ISO
// 8601 timestamp for timed entries, or a bare date when AllDay is true. The
// bounds are required-and-nullable — always present, value may be null — so
// nil means the API sent null, and re-marshals as null rather than a
// fabricated instant. Nil-check before calling time methods on them: the
// promoted calls compile unchanged and panic at runtime on a null bound.
type TimelineEventData struct {
	AllDay   bool                `json:"all_day"`
	StartsAt *types.FlexibleTime `json:"starts_at"`
	EndsAt   *types.FlexibleTime `json:"ends_at"`
}

// TimelineAttachment is a single timeline-event attachment: an optional-field
// superset over two wire variants — a full Upload recording (upload-kind
// recordings) and a rich-text attachment/blob partial (all other recordings).
// Only the fields of the variant an instance represents are populated.
type TimelineAttachment struct {
	// Every field is optional and pointer-backed: the superset populates only the
	// fields of the variant an instance represents (Upload-recording vs rich-text
	// blob), so an absent field must stay nil and re-marshal as omitted rather
	// than a fabricated sentinel. Per SPEC.md §10 an empty string is NOT an
	// acceptable stand-in for absence, so the optional strings are *string too.
	ID *int64 `json:"id,omitempty"`

	// Shared by both variants.
	ContentType *string `json:"content_type,omitempty"`
	ByteSize    *int64  `json:"byte_size,omitempty"`
	Filename    *string `json:"filename,omitempty"`
	DownloadURL *string `json:"download_url,omitempty"`
	// Width and Height are null for non-image blobs and may be float-spelled
	// (1024.0) on the wire; narrowed to *int32 here (nil when absent/null).
	Width  *int32 `json:"width,omitempty"`
	Height *int32 `json:"height,omitempty"`

	// Upload-recording variant — the full uploads/_upload projection.
	Type            *string    `json:"type,omitempty"`
	Title           *string    `json:"title,omitempty"`
	Status          *string    `json:"status,omitempty"`
	InheritsStatus  *bool      `json:"inherits_status,omitempty"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
	RecordingURL    *string    `json:"url,omitempty"`
	AppURL          *string    `json:"app_url,omitempty"`
	BookmarkURL     *string    `json:"bookmark_url,omitempty"`
	SubscriptionURL *string    `json:"subscription_url,omitempty"`
	CommentsCount   *int32     `json:"comments_count,omitempty"`
	CommentsURL     *string    `json:"comments_url,omitempty"`
	BoostsCount     *int32     `json:"boosts_count,omitempty"`
	BoostsURL       *string    `json:"boosts_url,omitempty"`
	Position        *int32     `json:"position,omitempty"`
	Parent          *Parent    `json:"parent,omitempty"`
	Bucket          *Bucket    `json:"bucket,omitempty"`
	Creator         *Person    `json:"creator,omitempty"`
	Description     *string    `json:"description,omitempty"`
	// DescriptionAttachments is pointer-backed so all three wire states round-trip
	// faithfully: nil (key absent), a non-nil empty slice (present "[]"), and a
	// populated list. A plain slice with omitempty would drop a present empty
	// array on re-marshal, losing the documented present-vs-absent distinction.
	DescriptionAttachments *[]RichTextAttachment `json:"description_attachments,omitempty"`
	AppDownloadURL         *string               `json:"app_download_url,omitempty"`
	VisibleToClients       *bool                 `json:"visible_to_clients,omitempty"`

	// Rich-text attachment/blob variant.
	AttachableSGID *string `json:"attachable_sgid,omitempty"`
	SGID           *string `json:"sgid,omitempty"`
	StatusURL      *string `json:"status_url,omitempty"`
	Caption        *string `json:"caption,omitempty"`
	Key            *string `json:"key,omitempty"`
	Previewable    *bool   `json:"previewable,omitempty"`
	PreviewURL     *string `json:"preview_url,omitempty"`
	ThumbnailURL   *string `json:"thumbnail_url,omitempty"`
}

// TimelineListOptions specifies options for listing timeline events.
type TimelineListOptions struct {
	// Limit is the maximum number of events to return.
	// If 0, uses DefaultTimelineLimit (100). Any negative value means unlimited.
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination:
	// exactly one request, no Link rel="next" follow (SPEC §8). A positive
	// Limit still trims that page; the per-operation default limit does not
	// apply to it. Use 0 to paginate through all results up to Limit.
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
//   - Page: if positive, fetches only that page and disables auto-pagination
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
	var params *generated.GetProgressReportParams
	if opts != nil && opts.Page > 0 {
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params = &generated.GetProgressReportParams{Page: page}
	}

	resp, err := s.client.parent.gen.GetProgressReportWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
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
		keep, truncated := pageCap(len(events), opts.Limit, resp.HTTPResponse)
		return &TimelineListResult{Events: events[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
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
//   - Page: if positive, fetches only that page and disables auto-pagination
//
// The returned TimelineListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *TimelineService) ProjectTimeline(ctx context.Context, projectID int64, opts *TimelineListOptions) (result *TimelineListResult, err error) {
	op := OperationInfo{
		Service: "Timeline", Operation: "ProjectTimeline",
		ResourceType: "timeline_event", IsMutation: false,
		ProjectID: projectID,
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
	var params *generated.GetProjectTimelineParams
	if opts != nil && opts.Page > 0 {
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params = &generated.GetProjectTimelineParams{Page: page}
	}

	resp, err := s.client.parent.gen.GetProjectTimelineWithResponse(ctx, s.client.accountID, projectID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
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
		keep, truncated := pageCap(len(events), opts.Limit, resp.HTTPResponse)
		return &TimelineListResult{Events: events[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
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
//   - Page: if positive, fetches only that page and disables auto-pagination
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

	var params *generated.GetPersonProgressParams
	if opts != nil && opts.Page > 0 {
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params = &generated.GetPersonProgressParams{Page: page}
	}

	resp, err := s.client.parent.gen.GetPersonProgressWithResponse(ctx, s.client.accountID, personID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	totalCount := parseTotalCount(resp.HTTPResponse)

	// Extract person from first page. The generated member stopped being a
	// pointer when the wrapper's two members were modeled @required: BC3 writes
	// both unconditionally, so there is no absent case to represent. The public
	// result keeps its *Person to avoid a break for callers.
	p := personFromGenerated(resp.JSON200.Person)
	person := &p

	// Parse events from first page
	var events []TimelineEvent
	for _, ge := range resp.JSON200.Events {
		events = append(events, timelineEventFromGenerated(ge))
	}

	if opts != nil && opts.Page > 0 {
		keep, truncated := pageCap(len(events), opts.Limit, resp.HTTPResponse)
		return &PersonProgressResult{Person: person, Events: events[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
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
		Kind:           deref(ge.Kind),
		URL:            deref(ge.Url),
		AppURL:         deref(ge.AppUrl),
		Action:         deref(ge.Action),
		Target:         deref(ge.Target),
		Title:          deref(ge.Title),
		SummaryExcerpt: deref(ge.SummaryExcerpt),
	}

	if ge.Id != nil {
		e.ID = *ge.Id
	}
	if ge.ParentRecordingId != nil {
		e.ParentRecordingID = *ge.ParentRecordingId
	}

	e.CreatedAt = ge.CreatedAt

	if ge.Creator != nil {
		creator := personFromGenerated(*ge.Creator)
		e.Creator = &creator
	}

	if ge.Bucket != nil {
		e.Bucket = &Bucket{
			ID:   ge.Bucket.Id,
			Name: ge.Bucket.Name,
			Type: ge.Bucket.Type,
		}
	}

	if ge.AvatarsSample != nil {
		e.AvatarsSample = append([]string(nil), ge.AvatarsSample...)
	}

	// data is present only on schedule_entry_* events.
	if ge.Data != nil {
		e.Data = &TimelineEventData{
			AllDay:   ge.Data.AllDay,
			StartsAt: ge.Data.StartsAt,
			EndsAt:   ge.Data.EndsAt,
		}
	}

	if ge.Attachments != nil {
		e.Attachments = make([]TimelineAttachment, 0, len(ge.Attachments))
		for _, ga := range ge.Attachments {
			e.Attachments = append(e.Attachments, timelineAttachmentFromGenerated(ga))
		}
	}

	return e
}

// UnmarshalJSON routes decoding through the generated TimelineAttachment so the
// public struct handles the float-encoded integers (1024.0) and null dimensions
// the BC3 API emits for width/height. Mirrors RichTextAttachment.UnmarshalJSON.
// Because TimelineEvent has no custom UnmarshalJSON, its Attachments elements
// invoke this automatically on direct decode.
func (a *TimelineAttachment) UnmarshalJSON(data []byte) error {
	var ga generated.TimelineAttachment
	if err := json.Unmarshal(data, &ga); err != nil {
		return err
	}
	*a = timelineAttachmentFromGenerated(ga)
	return nil
}

// timelineAttachmentFromGenerated converts a generated TimelineAttachment (the
// optional-field superset) to the clean public type. Width and Height are
// optional/nullable *types.FlexInt in the generated type; a nil pointer leaves
// the public *int32 nil, and a present value is narrowed to int32.
func timelineAttachmentFromGenerated(ga generated.TimelineAttachment) TimelineAttachment {
	a := TimelineAttachment{
		ID:               ga.Id,
		ContentType:      ga.ContentType,
		ByteSize:         ga.ByteSize,
		Filename:         ga.Filename,
		DownloadURL:      ga.DownloadUrl,
		Type:             ga.Type,
		Title:            ga.Title,
		Status:           ga.Status,
		InheritsStatus:   ga.InheritsStatus,
		CreatedAt:        ga.CreatedAt,
		UpdatedAt:        ga.UpdatedAt,
		RecordingURL:     ga.Url,
		AppURL:           ga.AppUrl,
		BookmarkURL:      ga.BookmarkUrl,
		SubscriptionURL:  ga.SubscriptionUrl,
		CommentsCount:    ga.CommentsCount,
		CommentsURL:      ga.CommentsUrl,
		BoostsCount:      ga.BoostsCount,
		BoostsURL:        ga.BoostsUrl,
		Position:         ga.Position,
		Description:      ga.Description,
		AppDownloadURL:   ga.AppDownloadUrl,
		VisibleToClients: ga.VisibleToClients,
		AttachableSGID:   ga.AttachableSgid,
		SGID:             ga.Sgid,
		StatusURL:        ga.StatusUrl,
		Caption:          ga.Caption,
		Key:              ga.Key,
		Previewable:      ga.Previewable,
		PreviewURL:       ga.PreviewUrl,
		ThumbnailURL:     ga.ThumbnailUrl,
	}
	if ga.Width != nil {
		w := int32(*ga.Width)
		a.Width = &w
	}
	if ga.Height != nil {
		h := int32(*ga.Height)
		a.Height = &h
	}
	if ga.Parent != nil {
		a.Parent = &Parent{
			ID:     ga.Parent.Id,
			Title:  ga.Parent.Title,
			Type:   ga.Parent.Type,
			URL:    ga.Parent.Url,
			AppURL: ga.Parent.AppUrl,
		}
	}
	if ga.Bucket != nil {
		a.Bucket = &Bucket{
			ID:   ga.Bucket.Id,
			Name: ga.Bucket.Name,
			Type: ga.Bucket.Type,
		}
	}
	if ga.Creator != nil {
		creator := personFromGenerated(*ga.Creator)
		a.Creator = &creator
	}
	a.DescriptionAttachments = richTextAttachmentsPtrFromGenerated(ga.DescriptionAttachments)
	return a
}
