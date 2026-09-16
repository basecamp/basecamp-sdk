package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// FeedEvent is one row of the account event feed: a thin pointer, not resource
// state. Refetch the referenced recording through the canonical resource APIs
// before acting on it. The same shape rides the inbox (InboxItem.Event).
type FeedEvent struct {
	// ID is the feed-global event id; the poll lane serves ids in strict
	// ascending order.
	ID int64 `json:"id"`
	// Kind is the event kind (e.g. "message_created").
	Kind string `json:"kind"`
	// Action is the action that produced the event (e.g. "created").
	Action string `json:"action"`
	// EventType is the cataloged event type (e.g. "message.created").
	EventType string `json:"event_type"`
	// BucketID is the bucket (project or circle) the recording lives in.
	BucketID int64 `json:"bucket_id"`
	// CreatorID is the person the action is attributed to.
	CreatorID int64 `json:"creator_id"`
	// PerformedByID is the agent that carried out a delegated action; nil when
	// the action was performed directly (the wire carries null). The effective
	// performer — what the performers/exclude_performers filters match — is
	// this when present, else CreatorID.
	PerformedByID *int64 `json:"performed_by_id"`
	// RecordingID is the recording the event references.
	RecordingID int64 `json:"recording_id"`
	// CreatedAt is the event's creation time.
	CreatedAt time.Time `json:"created_at"`
	// Details is the type-specific detail object, carried verbatim as the raw
	// JSON bytes the server sent — present only for the types that publish
	// one (boost.created: boost_id, boosted_event_id, boosted_event_type, the
	// latter two null for a boost on the recording itself; card.moved:
	// column_id, previous_column_id), nil for every other type. Raw so the
	// poll lane and the connector's push lane deliver byte-identical detail
	// objects: explicit nulls and members of newly cataloged types survive.
	// Decode with json.Unmarshal into a shape of your choosing.
	Details json.RawMessage `json:"details,omitempty"`
}

// EventFeedPage is one poll-lane page: the body envelope is the contract. The
// Link and X-Feed-Position response headers merely echo Next and Position.
type EventFeedPage struct {
	// Events are the page's rows, oldest first, in strict event-id order. A
	// page may be empty while the walk crosses history the filters exclude.
	Events []FeedEvent `json:"events"`
	// Position is the durable position after this page. Persist it only after
	// the page's events have been processed.
	Position string `json:"position"`
	// Next is the absolute continuation URL, present only while the current
	// walk has more to serve; empty means the walk reached its frozen head —
	// poll again later from Position. Follow it with PollEventsOptionsFromURL.
	Next string `json:"next,omitempty"`
}

// InboxItem is one addressed delivery. AddressingID is the item's own
// identity and the dedupe key: one event can address the same principal for
// several reasons, and each reason is its own item.
type InboxItem struct {
	AddressingID int64 `json:"addressing_id"`
	// Reason is why the principal was addressed: mentioned, assigned,
	// subscribed, watched, pinged, or boosted.
	Reason      string    `json:"reason"`
	AddressedAt time.Time `json:"addressed_at"`
	// Event is the addressing event, in the feed's shape.
	Event FeedEvent `json:"event"`
}

// InboxPage is one inbox page: items, the durable position, and the
// continuation URL while the walk has more to serve.
type InboxPage struct {
	Items    []InboxItem `json:"items"`
	Position string      `json:"position"`
	// Next is the absolute continuation URL, present only while the walk has
	// more to serve. Follow it with PollInboxOptionsFromURL.
	Next string `json:"next,omitempty"`
}

// StreamTicket is a minted live-stream credential. Ticket is an opaque
// replayable bearer for its window and URL embeds it, so neither is ever
// logged; connect to URL verbatim — the SDK never assembles cable topology.
type StreamTicket struct {
	Ticket string `json:"ticket"`
	// ExpiresIn is the ticket lifetime in seconds (about 120). Server-owned:
	// mint fresh per connection rather than scheduling on it.
	ExpiresIn int    `json:"expires_in"`
	URL       string `json:"url"`
}

// SinceNow enters a feed or inbox at the present, skipping history.
const SinceNow = "now"

// SinceEpoch replays all served history: back to the feed's epoch on the
// feed, the earliest retained item on the inbox.
const SinceEpoch = "0"

// PollEventsOptions selects the entry point and filters for PollEvents. Set
// exactly one of Since and Position; leave both empty to enter at the present.
// Filters are sent comma-joined; changing them invalidates a held Position
// (the server answers 409).
type PollEventsOptions struct {
	// Since is SinceNow, SinceEpoch, or a decimal event id to start after.
	Since string
	// Position is a resume token a previous page issued. Opaque.
	Position string
	// Types filters by cataloged event type (e.g. "message.created").
	Types []string
	// Buckets filters by bucket (project) id, at most 100.
	Buckets []int64
	// Creators filters by creator person id, at most 100.
	Creators []int64
	// Performers filters by effective performer: decimal ids or the literal
	// "self" (the request's own effective actor), at most 100.
	Performers []string
	// ExcludePerformers excludes effective performers, as Performers.
	// ExcludePerformers: []string{"self"} is the loop guard for an agent that
	// acts on what it hears.
	ExcludePerformers []string
	// ActorTypes filters by actor kind: "agent", "person", or both.
	ActorTypes []string
}

// PollInboxOptions selects the entry point and filters for PollInbox. Set
// exactly one of Since and Position; leave both empty to enter at the present.
type PollInboxOptions struct {
	// Since is SinceNow, SinceEpoch, or a decimal item id to start after.
	Since string
	// Position is a resume token a previous inbox page issued. Opaque.
	Position string
	// Reasons filters by addressing reason: mentioned, assigned, subscribed,
	// watched, pinged, boosted.
	Reasons []string
	// Types narrows by cataloged event type.
	Types []string
	// Buckets narrows by bucket id, at most 100.
	Buckets []int64
}

// FeedFilterMismatchError is the feed's 409: the held position was minted for
// a different filter set than the request presented. Re-enter with Since to
// acknowledge the filter change. Both digests are the bare srv2 filter digest
// BC3 publishes.
type FeedFilterMismatchError struct {
	// Err is the canonical SDK error (code api_error, HTTPStatus 409).
	Err *Error
	// PositionDigest is the digest of the filter set the position was minted for.
	PositionDigest string
	// FiltersDigest is the digest of the filter set this request presented.
	FiltersDigest string
}

// Error returns the server's message.
func (e *FeedFilterMismatchError) Error() string { return e.Err.Error() }

// Unwrap exposes the canonical error for errors.Is and errors.As.
func (e *FeedFilterMismatchError) Unwrap() error { return e.Err }

// FeedRequestError is the poll lanes' 400. Two cases share the status — a
// malformed position (re-enter with Since) and a malformed filter (fix the
// filters; a position reset will not help) — and Reason tells them apart:
// FeedReasonInvalidPosition or FeedReasonInvalidFilter. Reason is empty when
// the server predates bc3 #13362; treat that 400 as undifferentiated and
// surface it rather than guess between recovering and stopping.
type FeedRequestError struct {
	// Err is the canonical SDK error (code validation, HTTPStatus 400).
	Err *Error
	// Reason is "invalid_position", "invalid_filter", or "" when the server
	// did not say.
	Reason string
}

// Feed 400 reasons, as bc3 spells them.
const (
	FeedReasonInvalidPosition = "invalid_position"
	FeedReasonInvalidFilter   = "invalid_filter"
)

// Error returns the server's message.
func (e *FeedRequestError) Error() string { return e.Err.Error() }

// Unwrap exposes the canonical error for errors.Is and errors.As.
func (e *FeedRequestError) Unwrap() error { return e.Err }

// FeedPositionGoneError is PollEvents' 410: the held position predates the
// feed's epoch, an operational fence that can be raised. Resume is an absolute
// URL that re-enters the feed at the epoch (since=<EpochAfterID>) with the
// request's canonical filters preserved — validate it (same origin as the API
// base, no scheme downgrade) before following it. PollInbox never returns
// this type; its 410 is *InboxPositionGoneError, whose recovery differs.
type FeedPositionGoneError struct {
	// Err is the canonical SDK error (code api_error, HTTPStatus 410).
	Err *Error
	// EpochAfterID is the feed's epoch: the event id after which history is
	// servable.
	EpochAfterID int64
	// Resume is the absolute re-entry URL for the feed, at the epoch.
	Resume string
}

// Error returns the server's message.
func (e *FeedPositionGoneError) Error() string { return e.Err.Error() }

// Unwrap exposes the canonical error for errors.Is and errors.As.
func (e *FeedPositionGoneError) Unwrap() error { return e.Err }

// InboxPositionGoneError is PollInbox's 410: the held position fell behind
// the inbox's 30-day retention window. There is no epoch; Resume is an
// absolute URL that re-enters the inbox at since=0, the earliest retained
// item. A distinct type from *FeedPositionGoneError on purpose — the two
// recoveries are not interchangeable, so one errors.As arm cannot silently
// handle the wrong lane. Validate Resume before following it, as for the feed.
type InboxPositionGoneError struct {
	// Err is the canonical SDK error (code api_error, HTTPStatus 410).
	Err *Error
	// Resume is the absolute re-entry URL for the inbox, at since=0.
	Resume string
}

// Error returns the server's message.
func (e *InboxPositionGoneError) Error() string { return e.Err.Error() }

// Unwrap exposes the canonical error for errors.Is and errors.As.
func (e *InboxPositionGoneError) Unwrap() error { return e.Err }

// EventFeedService is the wire layer beneath the SPEC §23 event feed
// connector: the account event feed's catch-up poll lane, the agent inbox,
// and stream-ticket minting for the live lane. Pagination here is the body
// envelope — Position and Next — never the Link-header page walk, so nothing
// in this service follows pages automatically: one call is one page.
type EventFeedService struct {
	client *AccountClient
}

// NewEventFeedService creates a new EventFeedService.
func NewEventFeedService(client *AccountClient) *EventFeedService {
	return &EventFeedService{client: client}
}

// PollEvents fetches one page of the account event feed. A nil opts enters at
// the present (since=now). Returns *FeedRequestError on 400 (its Reason, when
// the server sent one, says whether the position or a filter was malformed),
// *FeedFilterMismatchError on 409, and *FeedPositionGoneError on 410, each
// wrapping the canonical *Error.
func (s *EventFeedService) PollEvents(ctx context.Context, opts *PollEventsOptions) (result *EventFeedPage, err error) {
	op := OperationInfo{
		Service: "EventFeed", Operation: "PollEvents",
		ResourceType: "feed_event", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.PollEventsParams
	if opts != nil {
		params = &generated.PollEventsParams{
			Since:             optionalString(opts.Since),
			Position:          optionalString(opts.Position),
			Types:             joinStrings(opts.Types),
			Buckets:           joinInt64s(opts.Buckets),
			Creators:          joinInt64s(opts.Creators),
			Performers:        joinStrings(opts.Performers),
			ExcludePerformers: joinStrings(opts.ExcludePerformers),
			ActorTypes:        joinStrings(opts.ActorTypes),
		}
	}
	resp, err := s.client.parent.gen.PollEventsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkFeedResponse(resp.HTTPResponse, resp.Body, feedLane); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}
	page := eventFeedPageFromGenerated(*resp.JSON200)
	return &page, nil
}

// PollInbox fetches one page of the authenticated agent's inbox. A nil opts
// enters at the present. People receive a bodyless 403 (the inbox is
// agents-only for now). Error mapping is as PollEvents for 400 and 409; the
// 410 is *InboxPositionGoneError, whose Resume re-enters at since=0 — never
// *FeedPositionGoneError.
func (s *EventFeedService) PollInbox(ctx context.Context, opts *PollInboxOptions) (result *InboxPage, err error) {
	op := OperationInfo{
		Service: "EventFeed", Operation: "PollInbox",
		ResourceType: "inbox_item", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.PollInboxParams
	if opts != nil {
		params = &generated.PollInboxParams{
			Since:    optionalString(opts.Since),
			Position: optionalString(opts.Position),
			Reasons:  joinStrings(opts.Reasons),
			Types:    joinStrings(opts.Types),
			Buckets:  joinInt64s(opts.Buckets),
		}
	}
	resp, err := s.client.parent.gen.PollInboxWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkFeedResponse(resp.HTTPResponse, resp.Body, inboxLane); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}
	page := inboxPageFromGenerated(*resp.JSON200)
	return &page, nil
}

// CreateStreamTicket mints a short-lived stream ticket and the exact
// WebSocket URL to connect to. Mint a fresh ticket per connection attempt;
// the mint is stateless, so it is safe to retry.
func (s *EventFeedService) CreateStreamTicket(ctx context.Context) (result *StreamTicket, err error) {
	op := OperationInfo{
		Service: "EventFeed", Operation: "CreateStreamTicket",
		ResourceType: "stream_ticket", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.CreateStreamTicketWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}
	ticket := streamTicketFromGenerated(*resp.JSON200)
	return &ticket, nil
}

// PollEventsOptionsFromURL turns a feed continuation URL — an EventFeedPage's
// Next, or a FeedPositionGoneError's Resume — into the options that re-issue
// PollEvents with the same query, so a walk can be followed through the
// generated operation rather than by fetching the URL raw. Only the query is
// read; the caller validates the URL's origin first (SPEC §23 "Continuation
// and Resume URL Validation"). Filters arrive comma-joined, as the server
// writes them; a Since and a Position may both be absent (a bare present entry).
func PollEventsOptionsFromURL(rawURL string) (*PollEventsOptions, error) {
	q, err := continuationQuery(rawURL)
	if err != nil {
		return nil, err
	}
	buckets, err := splitInt64s(q.Get("buckets"), "buckets")
	if err != nil {
		return nil, err
	}
	creators, err := splitInt64s(q.Get("creators"), "creators")
	if err != nil {
		return nil, err
	}
	return &PollEventsOptions{
		Since:             q.Get("since"),
		Position:          q.Get("position"),
		Types:             splitStrings(q.Get("types")),
		Buckets:           buckets,
		Creators:          creators,
		Performers:        splitStrings(q.Get("performers")),
		ExcludePerformers: splitStrings(q.Get("exclude_performers")),
		ActorTypes:        splitStrings(q.Get("actor_types")),
	}, nil
}

// PollInboxOptionsFromURL is PollEventsOptionsFromURL for the inbox lane: it
// reads an InboxPage's Next or a FeedPositionGoneError's Resume.
func PollInboxOptionsFromURL(rawURL string) (*PollInboxOptions, error) {
	q, err := continuationQuery(rawURL)
	if err != nil {
		return nil, err
	}
	buckets, err := splitInt64s(q.Get("buckets"), "buckets")
	if err != nil {
		return nil, err
	}
	return &PollInboxOptions{
		Since:    q.Get("since"),
		Position: q.Get("position"),
		Reasons:  splitStrings(q.Get("reasons")),
		Types:    splitStrings(q.Get("types")),
		Buckets:  buckets,
	}, nil
}

func continuationQuery(rawURL string) (url.Values, error) {
	if strings.TrimSpace(rawURL) == "" {
		return nil, ErrUsage("continuation URL is empty")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, ErrUsage(fmt.Sprintf("continuation URL is not a URL: %v", err))
	}
	if !u.IsAbs() {
		return nil, ErrUsage("continuation URL must be absolute")
	}
	// url.Values from URL.Query would drop a malformed pair silently, and a
	// dropped position turns a resume into a bare present entry that skips
	// events; ParseQuery reports the malformation instead.
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return nil, ErrUsage(fmt.Sprintf("continuation URL query is malformed: %v", err))
	}
	return q, nil
}

type feedLaneKind int

const (
	feedLane feedLaneKind = iota
	inboxLane
)

// checkFeedResponse is checkResponse plus the poll lanes' typed bodies: the
// 400's reason, the 409's digests, and the lane's own 410 — the feed's with
// its epoch, the inbox's with its since=0 resume — each wrapping the
// canonical error.
func checkFeedResponse(resp *http.Response, body []byte, lane feedLaneKind) error {
	err := checkResponse(resp, body)
	if err == nil || resp == nil {
		return err
	}
	base, ok := err.(*Error)
	if !ok {
		return err
	}
	switch resp.StatusCode {
	case http.StatusBadRequest:
		var request generated.FeedRequestErrorResponseContent
		if json.Unmarshal(body, &request) == nil && request.Error != "" {
			return &FeedRequestError{Err: base, Reason: deref(request.Reason)}
		}
	case http.StatusConflict:
		var mismatch generated.FeedFilterMismatchErrorResponseContent
		if json.Unmarshal(body, &mismatch) == nil && (mismatch.PositionDigest != "" || mismatch.FiltersDigest != "") {
			return &FeedFilterMismatchError{Err: base, PositionDigest: mismatch.PositionDigest, FiltersDigest: mismatch.FiltersDigest}
		}
	case http.StatusGone:
		if lane == inboxLane {
			var gone generated.InboxPositionGoneErrorResponseContent
			if json.Unmarshal(body, &gone) == nil && gone.Resume != "" {
				return &InboxPositionGoneError{Err: base, Resume: gone.Resume}
			}
			return err
		}
		// Decoded with a pointer rather than the generated int64 so a 410 that
		// omits (or nulls) the required epoch is not typed with a fabricated 0
		// boundary; it stays the canonical error.
		var gone struct {
			EpochAfterID *int64 `json:"epoch_after_id"`
			Resume       string `json:"resume"`
		}
		if json.Unmarshal(body, &gone) == nil && gone.Resume != "" && gone.EpochAfterID != nil {
			return &FeedPositionGoneError{Err: base, EpochAfterID: *gone.EpochAfterID, Resume: gone.Resume}
		}
	}
	return err
}

func eventFeedPageFromGenerated(gp generated.PollEventsResponseContent) EventFeedPage {
	page := EventFeedPage{
		Position: gp.Position,
		Next:     deref(gp.Next),
	}
	if gp.Events != nil {
		page.Events = make([]FeedEvent, 0, len(gp.Events))
		for _, ge := range gp.Events {
			page.Events = append(page.Events, feedEventFromGenerated(ge))
		}
	}
	return page
}

func inboxPageFromGenerated(gp generated.PollInboxResponseContent) InboxPage {
	page := InboxPage{
		Position: gp.Position,
		Next:     deref(gp.Next),
	}
	if gp.Items != nil {
		page.Items = make([]InboxItem, 0, len(gp.Items))
		for _, gi := range gp.Items {
			page.Items = append(page.Items, inboxItemFromGenerated(gi))
		}
	}
	return page
}

func inboxItemFromGenerated(gi generated.InboxItem) InboxItem {
	return InboxItem{
		AddressingID: gi.AddressingId,
		Reason:       gi.Reason,
		AddressedAt:  gi.AddressedAt,
		Event:        feedEventFromGenerated(gi.Event),
	}
}

func feedEventFromGenerated(ge generated.FeedEvent) FeedEvent {
	e := FeedEvent{
		ID:            ge.Id,
		Kind:          ge.Kind,
		Action:        ge.Action,
		EventType:     ge.EventType,
		BucketID:      ge.BucketId,
		CreatorID:     ge.CreatorId,
		PerformedByID: ge.PerformedById,
		RecordingID:   ge.RecordingId,
		CreatedAt:     ge.CreatedAt,
	}
	if ge.Details != nil {
		e.Details = append(json.RawMessage(nil), *ge.Details...)
	}
	return e
}

func streamTicketFromGenerated(gt generated.CreateStreamTicketResponseContent) StreamTicket {
	return StreamTicket{
		Ticket:    gt.Ticket,
		ExpiresIn: int(gt.ExpiresIn),
		URL:       gt.Url,
	}
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func joinStrings(values []string) *string {
	if len(values) == 0 {
		return nil
	}
	return ptr(strings.Join(values, ","))
}

func joinInt64s(values []int64) *string {
	if len(values) == 0 {
		return nil
	}
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = strconv.FormatInt(v, 10)
	}
	return ptr(strings.Join(parts, ","))
}

func splitStrings(joined string) []string {
	if joined == "" {
		return nil
	}
	return strings.Split(joined, ",")
}

func splitInt64s(joined, name string) ([]int64, error) {
	if joined == "" {
		return nil, nil
	}
	parts := strings.Split(joined, ",")
	values := make([]int64, 0, len(parts))
	for _, part := range parts {
		v, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, ErrUsage(fmt.Sprintf("continuation URL %s filter %q is not an integer id", name, part))
		}
		values = append(values, v)
	}
	return values, nil
}
