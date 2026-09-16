package eventfeed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
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
	for _, r := range accountID {
		if r < '0' || r > '9' {
			// ForAccount panics on anything but digits; a configuration
			// value reaches this constructor as an error, never a panic.
			return nil, usageError("accountID must be numeric")
		}
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

// refusedHop is the per-call record the redirect policy writes when it will
// not take a hop: the refused Location reduced to its origin — data for the
// seam's redirect_refused kind, never rendered (§23: a hostile redirect can
// reflect the bearer into a host label). It travels on the call's context so
// the policy, which sees only the redirected request, can hand it back to the
// seam call that owns it.
type refusedHop struct {
	mu      sync.Mutex
	refused bool
	origin  string
}

type refusedHopKey struct{}

func (h *refusedHop) record(origin string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.refused, h.origin = true, origin
}

func (h *refusedHop) get() (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.origin, h.refused
}

// withRefusedHop attaches a fresh record to ctx for one seam call.
func withRefusedHop(ctx context.Context) (context.Context, *refusedHop) {
	hop := &refusedHop{}
	return context.WithValue(ctx, refusedHopKey{}, hop), hop
}

// feedRedirectPolicy is the basecamp.Client redirect policy the feed installs:
// every hop's resolved Location is validated against the API origin under §8's
// same-origin algorithm plus downgrade rejection, before any request reaches
// it. A same-origin hop is followed (the host's own origin, so the bearer may
// travel). Anything else is refused by returning http.ErrUseLastResponse —
// the client then hands the 3xx back as an ordinary response, which the
// generated call classifies by status — rather than an error, because
// net/http wraps a policy error in a url.Error that renders the refused
// Location whole, and that rendering would reach the operation hooks before
// any seam could redact it. The refused origin is recorded on the call's
// context for the seam to read.
func feedRedirectPolicy(origin string) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		refuse := len(via) >= 10
		if !refuse {
			refuse = checkContinuation(origin, req.URL.String()) != nil
		}
		if !refuse {
			return nil
		}
		if len(via) > 0 {
			if hop, ok := via[0].Context().Value(refusedHopKey{}).(*refusedHop); ok {
				hop.record(locationOrigin(req))
			}
		}
		return http.ErrUseLastResponse
	}
}

// errHopRefused is the fixed cause a refused hop carries: never the
// Location, never the status text.
var errHopRefused = errors.New("eventfeed: refused a redirect off the API origin")

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
	ctx, hop := withRefusedHop(ctx)
	ticket, err := m.svc.CreateStreamTicket(ctx)
	if err != nil {
		return StreamTicket{}, mapMintError(err, hop)
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
func mapMintError(err error, hop *refusedHop) error {
	if isCancellation(err) {
		return err
	}
	var apiErr *basecamp.Error
	if errors.As(err, &apiErr) && isRedirectStatus(apiErr.HTTPStatus) {
		// A mint that redirects is out of contract, and a fresh mint would
		// redirect the same way: unrecoverable, with a fixed cause — the
		// refused hop never reaches a rendering.
		_, _ = hop.get()
		return &MintError{Kind: MintUnrecoverable, Err: errHopRefused}
	}
	if !errors.As(err, &apiErr) {
		if isTransportFailure(err) {
			// DNS, TLS, a dropped connection: transient, it rides the
			// reconnect cycle.
			return &MintError{Kind: MintTransient, Err: err}
		}
		// Anything else the generated call produced without a status — a
		// 200 whose body did not decode, an empty response — is a
		// deterministic outcome: a fresh mint answers the same way, so it
		// is the malformed success §23 names.
		return &MintError{Kind: MintUnrecoverable, Err: err}
	}
	switch {
	case apiErr.HTTPStatus == http.StatusUnauthorized || apiErr.HTTPStatus == http.StatusForbidden:
		return &MintError{Kind: MintUnauthorized, Err: err}
	case apiErr.Retryable && apiErr.RetryAfter > 0:
		// A retryable outcome exhausted inside the seam whose last response
		// carried a parsed Retry-After, whatever its status (§6). Gated on
		// the outcome being retryable at all: a header on a 404 names no
		// wait worth taking.
		return &MintError{Kind: MintThrottled, RetryAfter: time.Duration(apiErr.RetryAfter) * time.Second, Err: err}
	case apiErr.Retryable || apiErr.Code == basecamp.CodeNetwork:
		return &MintError{Kind: MintTransient, Err: err}
	default:
		return &MintError{Kind: MintUnrecoverable, Err: err}
	}
}

// isTransportFailure reports an error that says nothing about the response
// — the HTTP stack failed on the way to or from the server (a url.Error or a
// net.Error), or the client's own resilience gate refused to send at all
// (the circuit breaker, bulkhead or rate limiter, which recover on their
// own clocks) — as opposed to one the generated call produced from a
// response it received.
func isTransportFailure(err error) bool {
	if errors.Is(err, basecamp.ErrCircuitOpen) || errors.Is(err, basecamp.ErrBulkheadFull) || errors.Is(err, basecamp.ErrRateLimited) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// checkContinuationQuery refuses a continuation or resume URL whose query does
// not parse whole. url.URL.Query silently drops a malformed pair, and if that
// pair is `position` the re-issued operation becomes a bare present entry —
// a walk that commits a new head while skipping the history it was following.
// The URL is server-supplied text, so the error names nothing of it.
func checkContinuationQuery(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("eventfeed: the continuation URL does not parse")
	}
	if _, err := url.ParseQuery(u.RawQuery); err != nil {
		return errors.New("eventfeed: the continuation URL's query does not parse whole")
	}
	return nil
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
		if err := checkContinuationQuery(cursor.PageURL); err != nil {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: err}
		}
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
	ctx, hop := withRefusedHop(ctx)
	page, err := p.svc.PollEvents(ctx, opts)
	if err != nil {
		return PollPage{}, mapPollError(err, hop)
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
		if err := checkContinuationQuery(cursor.PageURL); err != nil {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: err}
		}
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
	ctx, hop := withRefusedHop(ctx)
	page, err := p.svc.PollInbox(ctx, opts)
	if err != nil {
		return PollPage{}, mapPollError(err, hop)
	}
	if page == nil {
		return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: errors.New("the poll returned no page")}
	}
	events := make([]Event, 0, len(page.Items))
	for _, item := range page.Items {
		// The envelope's own required members, before the event's: a zero
		// addressing id would become the lane's dedupe and reset key.
		if item.AddressingID < 1 || item.Reason == "" || item.AddressedAt.IsZero() {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: fmt.Errorf("eventfeed: inbox item %d is missing a required member", item.AddressingID)}
		}
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
func mapPollError(err error, hop *refusedHop) error {
	if isCancellation(err) {
		return err
	}
	var apiErr *basecamp.Error
	if errors.As(err, &apiErr) && isRedirectStatus(apiErr.HTTPStatus) {
		// A 3xx the policy refused — or one the client never asked the
		// policy about: a bare 3xx with no Location, which is equally not a
		// page. The origin the policy recorded rides as data on
		// LocationOrigin (§9's fixed token where it recorded none), and the
		// cause is fixed text: nothing of the response reaches a rendering.
		origin := "unparsable"
		if recorded, ok := hop.get(); ok && recorded != "" {
			origin = recorded
		}
		return &PollError{Kind: PollRedirectRefused, LocationOrigin: origin, Err: errHopRefused}
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
	if !errors.As(err, &apiErr) {
		if isTransportFailure(err) {
			return &PollError{Kind: PollTransient, Err: err}
		}
		// A 200 that did not decode, or an empty response: the unexpected
		// shape §23 maps to poll_failed, since re-polling draws it again.
		return &PollError{Kind: PollUnrecoverable, Err: err}
	}
	switch {
	case apiErr.HTTPStatus == http.StatusBadRequest:
		if len(apiErr.Message) >= len(positionRejectedMessage) && apiErr.Message[:len(positionRejectedMessage)] == positionRejectedMessage {
			return &PollError{Kind: PollPositionInvalid, Msg: apiErr.Message, Err: err}
		}
		return &PollError{Kind: PollFilterInvalid, Msg: apiErr.Message, Err: err}
	case apiErr.HTTPStatus == http.StatusUnauthorized || apiErr.HTTPStatus == http.StatusForbidden:
		return &PollError{Kind: PollUnauthorized, Err: err}
	case apiErr.Retryable && apiErr.RetryAfter > 0:
		return &PollError{Kind: PollThrottled, RetryAfter: time.Duration(apiErr.RetryAfter) * time.Second, Err: err}
	case apiErr.Retryable || apiErr.Code == basecamp.CodeNetwork:
		return &PollError{Kind: PollTransient, Err: err}
	default:
		return &PollError{Kind: PollUnrecoverable, Err: err}
	}
}

// isRedirectStatus reports a 3xx the generated call surfaced as its own
// status: the redirect policy declined to follow it, or it carried no
// Location to follow.
func isRedirectStatus(status int) bool {
	return status >= 300 && status <= 399
}

// isCancellation reports a context-driven end to the call, which the seam
// returns as-is: the connector cancelled it and reads the context, not a kind.
func isCancellation(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// eventFromFeed maps the wrapper's FeedEvent onto the connector's Event. The
// details object is re-serialized from the wrapper's typed projection
// (FeedEventDetails carries the members the SDK models; the generated layer
// keeps it typed so every SDK decodes it, and drops members it does not
// know), so on the poll lane Event.Details is that projection — the push
// lane, which decodes the frame itself, keeps the server's bytes whole.
func eventFromFeed(fe basecamp.FeedEvent) (Event, error) {
	// The generated model is value-typed, so a row missing a required member
	// arrives as a zero value rather than a decode error; the push decoder
	// refuses the same shapes, and a zero id would reach the dedupe ledger
	// under a key nothing real can share.
	if fe.ID < 1 || fe.BucketID < 1 || fe.CreatorID < 1 || fe.RecordingID < 1 ||
		(fe.PerformedByID != nil && *fe.PerformedByID < 1) ||
		fe.Kind == "" || fe.EventType == "" || fe.Action == "" || fe.CreatedAt.IsZero() {
		return Event{}, fmt.Errorf("eventfeed: poll row %d is missing a required member", fe.ID)
	}
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
