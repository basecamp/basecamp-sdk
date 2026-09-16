package eventfeed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// The Layer-1 adapters: the TicketMinter and PollSource seams bound to the
// generated CreateStreamTicket, PollEvents and PollInbox operations through
// the basecamp.EventFeedService wrapper (SPEC.md §23 "Seam Contracts"). One
// seam call is one fully-governed generated call — the operation keeps its
// SPEC §7 retry budget, backoff and Retry-After inside the seam, and each
// adapter maps every §6/§7 outcome onto exactly one seam error kind.
//
// The adapters build their own basecamp.Client rather than borrowing the
// host's, for one reason: the redirect policy. A followed `next` or 410
// `resume` URL carries the caller's bearer, and the client's default policy
// follows a cross-origin hop with the Authorization header stripped — which
// still egresses to the foreign origin. §23 "Continuation and Resume URL
// Validation" requires zero foreign egress, so the feed's client refuses a
// cross-origin or downgraded hop before any request is issued
// (basecamp.WithCheckRedirect), and the refusal reaches the connector as the
// poll seam's redirect_refused kind. The host keeps every other option — its
// hooks, logger, transport, auth strategy — by passing them through, and can
// use the same client for its own refetches (Live.Client): a policy that
// refuses cross-origin redirects is safe for every API call, since the API
// never legitimately redirects off its own origin.

// Live is the connector's wire binding: the seams for one account on one
// lane, over the generated operations.
type Live struct {
	client    *basecamp.Client
	svc       *basecamp.EventFeedService
	origin    string
	accountID string
	lane      Lane
}

// NewLive builds the seams for accountID on lane over a basecamp.Client
// constructed from cfg, tokens and clientOpts — with the feed's redirect
// policy installed last, so it wins over any policy in clientOpts. The
// client's base URL is the connector's origin: the checkpoint key's and the
// same-origin reference every continuation is validated against.
func NewLive(cfg *basecamp.Config, tokens basecamp.TokenProvider, accountID string, lane Lane, clientOpts ...basecamp.ClientOption) (*Live, error) {
	if cfg == nil {
		return nil, usageError("a basecamp.Config is required")
	}
	if accountID == "" {
		return nil, usageError("accountID must be non-empty")
	}
	if lane != AccountLane && lane != InboxLane {
		return nil, usageError(fmt.Sprintf("unknown lane %s", lane))
	}
	origin, err := CanonicalOrigin(cfg.BaseURL)
	if err != nil {
		return nil, usageError(err.Error())
	}
	if err := checkOriginScheme(origin); err != nil {
		return nil, err
	}
	opts := make([]basecamp.ClientOption, 0, len(clientOpts)+1)
	opts = append(opts, clientOpts...)
	opts = append(opts, basecamp.WithCheckRedirect(feedRedirectPolicy(origin)))
	client := basecamp.NewClient(cfg, tokens, opts...)
	return &Live{
		client:    client,
		svc:       client.ForAccount(accountID).EventFeed(),
		origin:    origin,
		accountID: accountID,
		lane:      lane,
	}, nil
}

// Client is the basecamp.Client the seams call through. A host that refetches
// the resources the feed points at can use it for those calls too.
func (l *Live) Client() *basecamp.Client { return l.client }

// Origin is the canonical API origin the client is bound to — pass it to New.
func (l *Live) Origin() string { return l.origin }

// Minter is the TicketMinter seam over CreateStreamTicket.
func (l *Live) Minter() TicketMinter { return &liveMinter{svc: l.svc} }

// Polls is the PollSource seam over PollEvents (AccountLane) or PollInbox
// (InboxLane).
func (l *Live) Polls() PollSource { return &livePolls{svc: l.svc, lane: l.lane} }

// Connect builds the Connector over these seams: New with this binding's
// origin, account and lane, plus opts.
func (l *Live) Connect(opts ...Option) (*Connector, error) {
	all := make([]Option, 0, len(opts)+1)
	all = append(all, opts...)
	all = append(all, WithLane(l.lane))
	return New(l.origin, l.accountID, l.Minter(), l.Polls(), all...)
}

// redirectRefusedError is what the feed's redirect policy returns to the HTTP
// client for a hop it will not take. It carries the refused Location reduced
// to its origin — data for the seam's redirect_refused kind, never rendered
// (§23: a hostile redirect can reflect the bearer into a host label).
type redirectRefusedError struct {
	locationOrigin string
}

func (e *redirectRefusedError) Error() string {
	return "eventfeed: refused a redirect off the API origin"
}

// feedRedirectPolicy is the basecamp.Client redirect policy the feed installs:
// every hop's resolved Location is validated against the API origin under §8's
// same-origin algorithm plus downgrade rejection, before any request reaches
// it. A same-origin hop is followed (the host's own origin, so the bearer may
// travel); anything else is refused with the Location's origin recorded.
func feedRedirectPolicy(origin string) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return &redirectRefusedError{locationOrigin: locationOrigin(req)}
		}
		if terr := checkContinuation(origin, req.URL.String()); terr != nil {
			return &redirectRefusedError{locationOrigin: locationOrigin(req)}
		}
		return nil
	}
}

// locationOrigin reduces a refused hop's target to its origin — the one
// component the seam contract lets a redirect_refused error carry — or the
// fixed token `unparsable` when the URL yields no complete origin (§9).
func locationOrigin(req *http.Request) string {
	origin, err := CanonicalOrigin(req.URL.String())
	if err != nil {
		return "unparsable"
	}
	return origin
}

// liveMinter is the TicketMinter over CreateStreamTicket.
type liveMinter struct {
	svc *basecamp.EventFeedService
}

func (m *liveMinter) MintStreamTicket(ctx context.Context) (StreamTicket, error) {
	ticket, err := m.svc.CreateStreamTicket(ctx)
	if err != nil {
		return StreamTicket{}, mapMintError(err)
	}
	if ticket == nil || ticket.Ticket == "" || ticket.URL == "" {
		// A malformed success: the mint answered 200 without the credential
		// or the URL the connector must dial. Nothing to dial and nothing a
		// retry changes.
		return StreamTicket{}, &MintError{Kind: MintUnrecoverable, Err: errors.New("the mint returned no ticket or no url")}
	}
	return StreamTicket{Ticket: ticket.Ticket, ExpiresIn: ticket.ExpiresIn, URL: ticket.URL}, nil
}

// mapMintError maps a CreateStreamTicket outcome onto exactly one MintErrorKind.
func mapMintError(err error) error {
	if isCancellation(err) {
		return err
	}
	var refused *redirectRefusedError
	if errors.As(err, &refused) {
		// A mint that redirects is out of contract, and a fresh mint would
		// redirect the same way: unrecoverable, carrying the typed error
		// alone (see mapPollError on why never the url.Error chain).
		return &MintError{Kind: MintUnrecoverable, Err: refused}
	}
	var apiErr *basecamp.Error
	if !errors.As(err, &apiErr) {
		// A transport-level failure the generated call could not classify
		// (DNS, TLS, a dropped connection): transient, it rides the
		// reconnect cycle.
		return &MintError{Kind: MintTransient, Err: err}
	}
	switch {
	case apiErr.HTTPStatus == http.StatusUnauthorized || apiErr.HTTPStatus == http.StatusForbidden:
		return &MintError{Kind: MintUnauthorized, Err: err}
	case apiErr.RetryAfter > 0:
		// A retryable outcome exhausted inside the seam whose last response
		// carried a parsed Retry-After, whatever its status (§6).
		return &MintError{Kind: MintThrottled, RetryAfter: time.Duration(apiErr.RetryAfter) * time.Second, Err: err}
	case apiErr.Retryable || apiErr.Code == basecamp.CodeNetwork:
		return &MintError{Kind: MintTransient, Err: err}
	default:
		return &MintError{Kind: MintUnrecoverable, Err: err}
	}
}

// livePolls is the PollSource over PollEvents or PollInbox.
type livePolls struct {
	svc  *basecamp.EventFeedService
	lane Lane
}

func (p *livePolls) Poll(ctx context.Context, cursor Cursor, filters Filters) (PollPage, error) {
	if p.lane == InboxLane {
		return p.pollInbox(ctx, cursor, filters)
	}
	return p.pollEvents(ctx, cursor, filters)
}

func (p *livePolls) pollEvents(ctx context.Context, cursor Cursor, filters Filters) (PollPage, error) {
	var opts *basecamp.PollEventsOptions
	if cursor.PageURL != "" {
		// A validated continuation or resume URL: the same operation,
		// re-issued with the query the server wrote into it. Only the query
		// is read; the connector validated the origin before this call.
		parsed, err := basecamp.PollEventsOptionsFromURL(cursor.PageURL)
		if err != nil {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: err}
		}
		opts = parsed
	} else {
		opts = &basecamp.PollEventsOptions{
			Since:             cursor.Since,
			Position:          cursor.Position,
			Types:             filters.Types,
			Buckets:           filters.Buckets,
			Creators:          filters.Creators,
			Performers:        formatIDs(filters.Performers),
			ExcludePerformers: formatIDs(filters.ExcludePerformers),
			ActorTypes:        filters.ActorTypes,
		}
	}
	page, err := p.svc.PollEvents(ctx, opts)
	if err != nil {
		return PollPage{}, mapPollError(err)
	}
	if page == nil {
		return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: errors.New("the poll returned no page")}
	}
	events := make([]Event, 0, len(page.Events))
	for _, fe := range page.Events {
		ev, err := eventFromFeed(fe)
		if err != nil {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: err}
		}
		events = append(events, ev)
	}
	return PollPage{Events: events, Position: page.Position, Next: page.Next}, nil
}

func (p *livePolls) pollInbox(ctx context.Context, cursor Cursor, filters Filters) (PollPage, error) {
	var opts *basecamp.PollInboxOptions
	if cursor.PageURL != "" {
		parsed, err := basecamp.PollInboxOptionsFromURL(cursor.PageURL)
		if err != nil {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: err}
		}
		opts = parsed
	} else {
		opts = &basecamp.PollInboxOptions{
			Since:    cursor.Since,
			Position: cursor.Position,
			Reasons:  filters.Reasons,
			Types:    filters.Types,
			Buckets:  filters.Buckets,
		}
	}
	page, err := p.svc.PollInbox(ctx, opts)
	if err != nil {
		return PollPage{}, mapPollError(err)
	}
	if page == nil {
		return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: errors.New("the poll returned no page")}
	}
	events := make([]Event, 0, len(page.Items))
	for _, item := range page.Items {
		ev, err := eventFromFeed(item.Event)
		if err != nil {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: err}
		}
		ev.Addressing = &Addressing{ID: item.AddressingID, Reason: item.Reason, AddressedAt: item.AddressedAt}
		events = append(events, ev)
	}
	return PollPage{Events: events, Position: page.Position, Next: page.Next}, nil
}

// positionRejectedMessage is the leading text of bc3's 400 for a malformed
// (or foreign-account) position — the one 400 that is recoverable by a
// since= re-entry. Every other 400 names an offending filter and is the
// configuration error a position reset cannot help.
const positionRejectedMessage = "Unrecognized position"

// mapPollError maps a PollEvents/PollInbox outcome onto exactly one
// PollErrorKind (SPEC.md §23 "Seam Contracts").
func mapPollError(err error) error {
	if isCancellation(err) {
		return err
	}
	var refused *redirectRefusedError
	if errors.As(err, &refused) {
		// The typed error alone, never the chain: net/http wraps a refused
		// hop in a url.Error whose rendering carries the refused Location
		// whole — exactly the value a hostile redirect can put the bearer
		// into, and exactly what no rendering may carry (§23). The origin
		// rides as data on LocationOrigin.
		return &PollError{Kind: PollRedirectRefused, LocationOrigin: refused.locationOrigin, Err: refused}
	}
	var mismatch *basecamp.FeedFilterMismatchError
	if errors.As(err, &mismatch) {
		return &PollError{Kind: PollFilterChanged, PositionDigest: mismatch.PositionDigest, FiltersDigest: mismatch.FiltersDigest, Err: err}
	}
	var gone *basecamp.FeedPositionGoneError
	if errors.As(err, &gone) {
		var epoch int64
		if gone.EpochAfterID != nil {
			epoch = *gone.EpochAfterID
		}
		return &PollError{Kind: PollGone, EpochAfterID: epoch, ResumeURL: gone.Resume, Err: err}
	}
	var apiErr *basecamp.Error
	if !errors.As(err, &apiErr) {
		return &PollError{Kind: PollTransient, Err: err}
	}
	switch {
	case apiErr.HTTPStatus == http.StatusBadRequest:
		if len(apiErr.Message) >= len(positionRejectedMessage) && apiErr.Message[:len(positionRejectedMessage)] == positionRejectedMessage {
			return &PollError{Kind: PollPositionInvalid, Msg: apiErr.Message, Err: err}
		}
		return &PollError{Kind: PollFilterInvalid, Msg: apiErr.Message, Err: err}
	case apiErr.HTTPStatus == http.StatusUnauthorized || apiErr.HTTPStatus == http.StatusForbidden:
		return &PollError{Kind: PollUnauthorized, Err: err}
	case apiErr.RetryAfter > 0:
		return &PollError{Kind: PollThrottled, RetryAfter: time.Duration(apiErr.RetryAfter) * time.Second, Err: err}
	case apiErr.Retryable || apiErr.Code == basecamp.CodeNetwork:
		return &PollError{Kind: PollTransient, Err: err}
	default:
		return &PollError{Kind: PollUnrecoverable, Err: err}
	}
}

// isCancellation reports a context-driven end to the call, which the seam
// returns as-is: the connector cancelled it and reads the context, not a kind.
func isCancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// eventFromFeed maps the wrapper's FeedEvent onto the connector's Event. The
// typed details are re-serialized verbatim: Event.Details is the raw object,
// so a consumer decodes it against the type's documented shape as on the push
// lane.
func eventFromFeed(fe basecamp.FeedEvent) (Event, error) {
	ev := Event{
		ID:            fe.ID,
		Kind:          fe.Kind,
		EventType:     fe.EventType,
		Action:        fe.Action,
		CreatedAt:     fe.CreatedAt,
		BucketID:      fe.BucketID,
		CreatorID:     fe.CreatorID,
		PerformedByID: fe.PerformedByID,
		RecordingID:   fe.RecordingID,
	}
	if fe.Details != nil {
		raw, err := json.Marshal(fe.Details)
		if err != nil {
			return Event{}, fmt.Errorf("eventfeed: encoding event %d details: %w", fe.ID, err)
		}
		ev.Details = raw
	}
	return ev, nil
}

// formatIDs renders ids for the wrapper's string-typed performer lists (which
// also admit the server's `self` literal; the connector never sends it).
func formatIDs(ids []int64) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = strconv.FormatInt(id, 10)
	}
	return out
}
