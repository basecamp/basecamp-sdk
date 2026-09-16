package eventfeed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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
// host's, for one reason: redirects. A followed `next` or 410 `resume` URL
// carries the caller's bearer, and the client's default policy follows a
// cross-origin hop with the Authorization header stripped — which still
// egresses to the foreign origin. §23 "Continuation and Resume URL
// Validation" requires zero foreign egress, so the feed's client composes a
// guard over the host's transport (basecamp.WithTransportWrapper) that
// answers every 3xx of a seam call at the wire: the Location is reduced to
// its origin for the seam and stripped — with the body — before net/http,
// the operation hooks or any log sees it, and the 3xx reaches the generated
// call as a status it classifies. No hop is ever followed, same-origin
// included: the API never redirects a feed call, and a continuation is
// followed by re-issuing the operation, not by a hop. The guard acts only on
// the seams' own calls — it recognizes them by their routes beneath the
// configured base path — so every other request through the same client
// keeps the client's own redirect handling: a host can use it for its own
// refetches (Live.Client), downloads included, whose first hop legitimately
// 302s to a signed URL. The host keeps every other option — its hooks, logger,
// transport, auth strategy — by passing them through.

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
// guard installed last, so it wins over any transport wrapper in clientOpts
// (the host's transport itself, from WithTransport, is what the guard
// composes over). The client's base URL is the connector's origin: the checkpoint key's and the
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
	// The base URL is host configuration, and it reaches the request hooks
	// and logs whole through every request's URL, so a value carrying
	// userinfo is refused here — with a fixed message, since echoing it would
	// be the leak — and the origin is canonicalized from what remains.
	if u, err := url.Parse(cfg.BaseURL); err != nil || u.User != nil || !utf8.ValidString(cfg.BaseURL) {
		// Invalid UTF-8 is refused raw, as New refuses it: canonicalization
		// would rewrite every invalid byte to U+FFFD and let two different
		// broken origins collapse into one checkpoint lineage.
		return nil, usageError("the base URL must be valid UTF-8, parse, and carry no userinfo")
	}
	origin, err := CanonicalOrigin(cfg.BaseURL)
	if err != nil {
		return nil, usageError("the base URL does not name an origin")
	}
	if err := checkOriginScheme(origin); err != nil {
		return nil, err
	}
	opts := make([]basecamp.ClientOption, 0, len(clientOpts)+1)
	opts = append(opts, clientOpts...)
	// The guard is anchored at the base URL's path, so that path must be
	// the one the generated client resolves routes beneath: a dot segment
	// or a doubled slash would resolve to a different prefix than the one
	// recorded here, and the guard would miss the seams' own calls.
	basePath := "/"
	if u, _ := url.Parse(cfg.BaseURL); u != nil {
		if u.RawQuery != "" || u.Fragment != "" || u.ForceQuery || u.RawFragment != "" {
			return nil, usageError("the base URL must carry no query or fragment")
		}
		// The rules below read the DECODED path, and the client sends the
		// escaped one: `/api%2f` decodes to `/api/`, passes both of them,
		// and goes on the wire as a request whose path is `/api%2f/...` —
		// where the guard, anchored at `/api/`, finds no account segment
		// and lets the 3xx through to be followed, off the API origin,
		// with the request's query. RawPath is non-empty exactly when the
		// escaped path is not the canonical encoding of the decoded one,
		// so refusing it here closes every escaped spelling rather than
		// the one that was found.
		if u.RawPath != "" {
			return nil, usageError("the base URL path must be canonical: no percent-encoded segments")
		}
		if strings.Contains(u.Path, "//") {
			return nil, usageError("the base URL path must be canonical: no dot segments, no doubled slashes")
		}
		if strings.Trim(u.Path, "/") != "" {
			if clean := path.Clean(u.Path); clean != strings.TrimSuffix(u.Path, "/") {
				return nil, usageError("the base URL path must be canonical: no dot segments, no doubled slashes")
			}
			basePath = "/" + strings.Trim(u.Path, "/") + "/"
		}
	}
	opts = append(opts, basecamp.WithTransportWrapper(redirectGuardWrapper{basePath: basePath}))
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
// (InboxLane). Each call is a fresh source with its own walk state — one
// per connector, so a walk's order is held across its pages and two
// connectors over one binding never read each other's.
func (l *Live) Polls() PollSource { return &livePolls{svc: l.svc, lane: l.lane} }

// Connect builds the Connector over these seams: New with this binding's
// origin, account and lane, plus opts. The lane is the binding's — the seams
// are bound to one lane's operation — so a WithLane in opts naming another
// lane is a usage error, never a silent override in either direction.
func (l *Live) Connect(opts ...Option) (*Connector, error) {
	all := make([]Option, 0, len(opts)+1)
	all = append(all, WithLane(l.lane))
	all = append(all, opts...)
	c, err := New(l.origin, l.accountID, l.Minter(), l.Polls(), all...)
	if err != nil {
		return nil, err
	}
	if c.cfg.lane != l.lane {
		return nil, usageError(fmt.Sprintf("the lane is bound by NewLive (%s); build a second Live for the %s lane", l.lane, c.cfg.lane))
	}
	return c, nil
}

// refusedHop is the per-call record the redirect guard writes when it
// answers a 3xx: the refused Location reduced to its origin — data for the
// seam's redirect_refused kind, never rendered (§23: a hostile redirect can
// reflect the bearer into a host label). It travels on the call's context,
// the only per-call channel between a seam call and the transport under it,
// so the guard — which sees only the wire exchange — can hand the origin
// back to the call that owns it. The refusal itself does not depend on the
// record: the guard answers a 3xx off the request's route, so a record that
// does not survive costs the origin, never the egress guarantee. A host hook
// that returns a context unrelated to the one it was handed is what loses it
// — the same channel carries the client's own per-request state (the retry
// attempt, the download projection marker) and the caller's cancellation —
// and the seam then reports `unrecorded` rather than claiming the server
// sent a Location it could not parse.
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

// redirectGuardWrapper installs redirectGuard over the client's transport,
// anchored at the configured base URL's path ("/" when it has none).
type redirectGuardWrapper struct{ basePath string }

func (w redirectGuardWrapper) WrapTransport(inner http.RoundTripper) http.RoundTripper {
	return &redirectGuard{inner: inner, basePath: w.basePath}
}

// redirectGuard is the RoundTripper the feed composes over the host's
// transport: every 3xx answered to a seam call is answered here, at the
// wire, before net/http's redirect loop can parse a Location, follow it, or
// render it into a url.Error that the operation hooks would receive whole.
// The Location is reduced to its origin on the call's refusedHop record —
// the one component the seam contract lets a redirect_refused error carry —
// then the header and the body are dropped, and the 3xx goes on as a bare
// status the generated call classifies. One mechanism covers every shape of
// the class: a foreign Location, a same-origin one, a downgraded one, a
// Location net/http could not parse, and a 3xx with none. A seam call is
// recognized by its request path — the three feed operations' own routes,
// exactly, beneath the configured base path — rather than by the per-call
// record on its context, which a host hook that returns a fresh context
// would drop; the record, when it survives, carries the origin back to the
// seam, and the strip does not depend on it. Every other request passes
// through untouched: the client's other operations — a download's
// dispatching 302 above all — keep their own redirect handling.
//
// The guard also marks a seam call's response body, so that a read that
// fails after the headers arrived — a reset stream, a connection cut
// mid-body — reaches the seam as a bodyReadError the classifier can tell
// from a body that arrived whole and did not decode.
type redirectGuard struct {
	inner    http.RoundTripper
	basePath string
}

func (g *redirectGuard) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := g.inner.RoundTrip(req)
	if err != nil || resp == nil || !isFeedOperationPath(g.basePath, req.URL.Path) {
		return resp, err
	}
	if !isRedirectStatus(resp.StatusCode) {
		if resp.Body != nil && resp.Body != http.NoBody {
			resp.Body = &markedBody{ReadCloser: resp.Body}
		}
		return resp, nil
	}
	if hop, ok := req.Context().Value(refusedHopKey{}).(*refusedHop); ok {
		hop.record(locationOrigin(req.URL, resp.Header.Get("Location")))
	}
	resp.Header.Del("Location")
	resp.Header.Del("Content-Location")
	if resp.Body != nil {
		_ = resp.Body.Close()
	}
	resp.Body = http.NoBody
	resp.ContentLength = 0
	resp.Header.Del("Content-Length")
	return resp, nil
}

// isFeedOperationPath reports a request path that is exactly one of the
// three feed operations' routes — {account}/events.json,
// {account}/inbox.json, {account}/events/stream_ticket.json — directly
// beneath the configured base path (basePath, "/"-terminated): the routes
// the seams issue, whether from a fresh cursor or a re-issued continuation.
// Anchoring at the base path and at the account segment keeps a recording's
// audit trail (/{account}/recordings/{id}/events.json) and every other
// route a host issues through the same client outside the guard.
func isFeedOperationPath(basePath, path string) bool {
	rest, ok := strings.CutPrefix(path, basePath)
	if !ok {
		return false
	}
	account, route, ok := strings.Cut(rest, "/")
	if !ok || account == "" || !isDigits(account) {
		return false
	}
	return route == "events.json" || route == "inbox.json" || route == "events/stream_ticket.json"
}

// markedBody wraps a seam call's response body so a read failure after the
// headers — as opposed to io.EOF, the body's own end — surfaces as a
// bodyReadError.
type markedBody struct{ io.ReadCloser }

func (b *markedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		err = &bodyReadError{err: err}
	}
	return n, err
}

// bodyReadError is a response body that could not be read whole: the
// connection ended, the stream was reset — a transport failure by
// construction, whatever type the HTTP stack chose for it.
type bodyReadError struct{ err error }

func (e *bodyReadError) Error() string {
	return "eventfeed: reading the response body: " + e.err.Error()
}
func (e *bodyReadError) Unwrap() error { return e.err }

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// errHopRefused is the fixed cause a refused hop carries: never the
// Location, never the status text.
var errHopRefused = errors.New("eventfeed: refused a redirect off the API origin")

// The two fixed tokens a redirect_refused carries in place of an origin.
// originUnparsable is §9's: the Location was absent, or yielded no complete
// origin. originUnrecorded is the other thing that can be true — the guard
// refused a hop, and the seam cannot say where to: the call's record travels
// on its context, and a host hook that returns a context unrelated to the
// one it was handed drops it. They are distinct because they say different
// things: the first is a statement about the server's header, the second
// about this connector's own bookkeeping, and an operator reading the first
// for the second would go looking for a malformed Location that never was.
const (
	originUnparsable = "unparsable"
	originUnrecorded = "unrecorded"
)

// locationOrigin reduces a refused hop's Location — resolved against the
// request's URL first, as a relative or scheme-relative reference would be
// — to its origin, the one component the seam contract lets a
// redirect_refused error carry; or the fixed token `unparsable` when the
// header is absent or yields no complete origin (§9).
func locationOrigin(base *url.URL, location string) string {
	if location == "" || base == nil {
		return originUnparsable
	}
	ref, err := url.Parse(location)
	if err != nil {
		return originUnparsable
	}
	origin, err := CanonicalOrigin(base.ResolveReference(ref).String())
	if err != nil {
		return originUnparsable
	}
	return origin
}

// continuationIsResume reports a followed URL that re-enters at a since —
// a 410's resume, which starts a new walk — as opposed to a position
// continuation of the walk in progress.
func continuationIsResume(pageURL string) bool {
	u, err := url.Parse(pageURL)
	if err != nil {
		return false
	}
	return u.Query().Get("since") != ""
}

// liveMinter is the TicketMinter over CreateStreamTicket.
type liveMinter struct {
	svc *basecamp.EventFeedService
}

func (m *liveMinter) MintStreamTicket(ctx context.Context) (StreamTicket, error) {
	ctx, hop := withRefusedHop(ctx)
	ticket, err := m.svc.CreateStreamTicket(ctx)
	if err != nil {
		return StreamTicket{}, mapMintError(ctx, err, hop)
	}
	if ticket == nil || ticket.Ticket == "" || ticket.URL == "" || ticket.ExpiresIn <= 0 {
		// A malformed success: the mint answered 200 without the credential,
		// the URL the connector must dial, or a positive lifetime. Nothing
		// to dial and nothing a retry changes.
		return StreamTicket{}, &MintError{Kind: MintUnrecoverable, Err: errors.New("the mint returned no ticket, no url, or no lifetime")}
	}
	return StreamTicket{Ticket: ticket.Ticket, ExpiresIn: ticket.ExpiresIn, URL: ticket.URL}, nil
}

// mapMintError maps a CreateStreamTicket outcome onto exactly one MintErrorKind.
func mapMintError(ctx context.Context, err error, hop *refusedHop) error {
	if isCancellation(ctx, err) {
		return err
	}
	var apiErr *basecamp.Error
	if errors.As(err, &apiErr) && isRedirectStatus(apiErr.HTTPStatus) {
		// A mint that redirects is out of contract, and a fresh mint would
		// redirect the same way: unrecoverable, with a fixed cause — the
		// guard already reduced the hop to an origin nothing here renders.
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
// net.Error), the connection ended mid-body before the parser could read a
// whole response (io.ErrUnexpectedEOF, or a bare io.EOF from a body read),
// or the client's own resilience gate refused to send at all (the circuit
// breaker, bulkhead or rate limiter, which recover on their own clocks) — as
// opposed to one the generated call produced from a response it received
// whole, such as a body that did not decode.
func isTransportFailure(err error) bool {
	if errors.Is(err, basecamp.ErrCircuitOpen) || errors.Is(err, basecamp.ErrBulkheadFull) || errors.Is(err, basecamp.ErrRateLimited) {
		return true
	}
	var bodyErr *bodyReadError
	if errors.As(err, &bodyErr) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
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
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return errors.New("eventfeed: the continuation URL's query does not parse whole")
	}
	for _, v := range values {
		if len(v) > 1 {
			// The wrapper reads each key once (the first value); the API
			// would read the last. A repeated key has no single meaning
			// the seam can re-issue faithfully.
			return errors.New("eventfeed: the continuation URL repeats a query key")
		}
	}
	return nil
}

// livePolls is the PollSource over PollEvents or PollInbox. It remembers the
// last key of the page it served so a continuation's first row can be held
// to the walk's strict order across pages; a fresh cursor starts a new walk.
type livePolls struct {
	svc  *basecamp.EventFeedService
	lane Lane

	mu      sync.Mutex
	lastKey int64
}

func (p *livePolls) Poll(ctx context.Context, cursor Cursor, filters Filters) (PollPage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cursor.PageURL == "" || continuationIsResume(cursor.PageURL) {
		// A fresh cursor, or a 410's resume (a since re-entry), starts a
		// new walk; only a position continuation carries the order over.
		p.lastKey = 0
	}
	var page PollPage
	var err error
	if p.lane == InboxLane {
		page, err = p.pollInbox(ctx, cursor, filters)
	} else {
		page, err = p.pollEvents(ctx, cursor, filters)
	}
	if err != nil {
		return PollPage{}, err
	}
	if len(page.Events) > 0 {
		if page.Events[0].Key() <= p.lastKey {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: fmt.Errorf("eventfeed: the continuation's first row %d does not follow the previous page", page.Events[0].Key())}
		}
		p.lastKey = page.Events[len(page.Events)-1].Key()
	}
	return page, nil
}

func (p *livePolls) pollEvents(ctx context.Context, cursor Cursor, filters Filters) (PollPage, error) {
	opts := &basecamp.PollEventsOptions{
		Since:             cursor.Since,
		Position:          cursor.Position,
		Types:             filters.Types,
		Buckets:           filters.Buckets,
		Creators:          filters.Creators,
		Performers:        formatIDs(filters.Performers),
		ExcludePerformers: formatIDs(filters.ExcludePerformers),
		ActorTypes:        filters.ActorTypes,
	}
	if cursor.PageURL != "" {
		// A validated continuation or resume URL: the same operation,
		// re-issued at the URL's cursor under the lane's OWN canonical
		// filters. The URL's filter dimensions are not trusted — they must
		// equal the lane's, or the URL is refused — and only its cursor is
		// read; the connector validated the origin before this call.
		if err := checkContinuationQuery(cursor.PageURL); err != nil {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: err}
		}
		parsed, err := basecamp.PollEventsOptionsFromURL(cursor.PageURL)
		if err != nil {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: errContinuationUnparsable}
		}
		if err := checkContinuationCursor(parsed.Position, parsed.Since); err != nil {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: err}
		}
		if !sameStrings(parsed.Types, opts.Types) || !sameInt64s(parsed.Buckets, opts.Buckets) || !sameInt64s(parsed.Creators, opts.Creators) ||
			!sameStrings(parsed.Performers, opts.Performers) || !sameStrings(parsed.ExcludePerformers, opts.ExcludePerformers) || !sameStrings(parsed.ActorTypes, opts.ActorTypes) {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: errContinuationFilters}
		}
		opts.Since, opts.Position = parsed.Since, parsed.Position
	}
	ctx, hop := withRefusedHop(ctx)
	page, err := p.svc.PollEvents(ctx, opts)
	if err != nil {
		return PollPage{}, mapPollError(ctx, err, hop, p.lane)
	}
	if page == nil {
		return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: errors.New("the poll returned no page")}
	}
	if err := checkNextCursor(page.Next); err != nil {
		return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: err}
	}
	if page.Events == nil {
		// The envelope's events member is required; a page without one is
		// not an empty page but a malformed one, and saving its position
		// would skip whatever it should have carried.
		return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: errors.New("eventfeed: the page carries no events member")}
	}
	events := make([]Event, 0, len(page.Events))
	for _, fe := range page.Events {
		ev, err := eventFromFeed(fe)
		if err != nil {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: err}
		}
		if err := checkPageOrder(events, ev); err != nil {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: err}
		}
		events = append(events, ev)
	}
	return PollPage{Events: events, Position: page.Position, Next: page.Next}, nil
}

func (p *livePolls) pollInbox(ctx context.Context, cursor Cursor, filters Filters) (PollPage, error) {
	opts := &basecamp.PollInboxOptions{
		Since:    cursor.Since,
		Position: cursor.Position,
		Reasons:  filters.Reasons,
		Types:    filters.Types,
		Buckets:  filters.Buckets,
	}
	if cursor.PageURL != "" {
		if err := checkContinuationQuery(cursor.PageURL); err != nil {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: err}
		}
		parsed, err := basecamp.PollInboxOptionsFromURL(cursor.PageURL)
		if err != nil {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: errContinuationUnparsable}
		}
		if err := checkContinuationCursor(parsed.Position, parsed.Since); err != nil {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: err}
		}
		if !sameStrings(parsed.Reasons, opts.Reasons) || !sameStrings(parsed.Types, opts.Types) || !sameInt64s(parsed.Buckets, opts.Buckets) {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: errContinuationFilters}
		}
		opts.Since, opts.Position = parsed.Since, parsed.Position
	}
	ctx, hop := withRefusedHop(ctx)
	page, err := p.svc.PollInbox(ctx, opts)
	if err != nil {
		return PollPage{}, mapPollError(ctx, err, hop, p.lane)
	}
	if page == nil {
		return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: errors.New("the poll returned no page")}
	}
	if err := checkNextCursor(page.Next); err != nil {
		return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: err}
	}
	if page.Items == nil {
		return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: errors.New("eventfeed: the page carries no items member")}
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
		if err := checkPageOrder(events, ev); err != nil {
			return PollPage{}, &PollError{Kind: PollUnrecoverable, Err: err}
		}
		events = append(events, ev)
	}
	return PollPage{Events: events, Position: page.Position, Next: page.Next}, nil
}

// checkPageOrder holds a page to the contract's strict order of the lane's
// identity (PollPage.Events): a row whose key does not exceed the previous
// row's is a malformed page — reordered or duplicated logical deliveries
// the connector must not commit a position over.
func checkPageOrder(sofar []Event, next Event) error {
	if n := len(sofar); n > 0 && next.Key() <= sofar[n-1].Key() {
		return fmt.Errorf("eventfeed: poll page rows are not in strict key order at %d", next.Key())
	}
	return nil
}

// checkResumeCursor holds a 410's resume URL to the fence its body declares:
// the feed's re-enters at since=<epoch_after_id>, the inbox's at since=0, and
// neither carries a position. A resume that re-enters elsewhere — since=now,
// a different id — would have an accepting handler skip retained history.
// The URL is server-supplied text, so the error names nothing of it; its
// origin is the connector's to validate before the URL is followed.
func checkResumeCursor(resume, since string) error {
	u, err := url.Parse(resume)
	if err != nil {
		return errors.New("eventfeed: the 410's resume URL does not parse")
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return errors.New("eventfeed: the 410's resume URL's query does not parse whole")
	}
	if len(values["since"]) != 1 || values["since"][0] != since || len(values["position"]) != 0 {
		return errors.New("eventfeed: the 410's resume URL does not re-enter at the fence the body declares")
	}
	return nil
}

// positionRejectedMessage is the leading text of bc3's 400 for a malformed
// (or foreign-account) position — the one 400 that is recoverable by a
// since= re-entry. Every other 400 names an offending filter and is the
// configuration error a position reset cannot help.
const positionRejectedMessage = "Unrecognized position"

// mapPollError maps a PollEvents/PollInbox outcome onto exactly one
// PollErrorKind (SPEC.md §23 "Seam Contracts"). The lane decides what a 410
// must carry: the feed's names its epoch, the inbox's carries none.
func mapPollError(ctx context.Context, err error, hop *refusedHop, lane Lane) error {
	if isCancellation(ctx, err) {
		return err
	}
	var apiErr *basecamp.Error
	if errors.As(err, &apiErr) && isRedirectStatus(apiErr.HTTPStatus) {
		// A 3xx the guard answered: the origin it recorded rides as data on
		// LocationOrigin, and the cause is fixed text — nothing of the
		// response reaches a rendering. A refusal whose record did not reach
		// the seam reports `unrecorded`, never `unparsable`: the guard
		// refuses the hop off the request's route, so zero egress holds
		// either way, but the two tokens answer different questions and only
		// one of them is about the server's header.
		origin := originUnrecorded
		if recorded, ok := hop.get(); ok && recorded != "" {
			origin = recorded
		}
		return &PollError{Kind: PollRedirectRefused, LocationOrigin: origin, Err: errHopRefused}
	}
	var mismatch *basecamp.FeedFilterMismatchError
	if errors.As(err, &mismatch) {
		if !isServerDigest(mismatch.PositionDigest) || !isServerDigest(mismatch.FiltersDigest) {
			// The conflict verdict is the status; the digests are what the
			// connector reports and compares. A 409 whose digests are
			// missing or not bare 16-hex is not the documented conflict but
			// a malformed response. (Holding the shape at the wrapper's
			// decode, for every consumer, is tracked in #915.)
			return &PollError{Kind: PollUnrecoverable, Err: errors.New("eventfeed: the 409's digests are missing or malformed")}
		}
		return &PollError{Kind: PollFilterChanged, PositionDigest: mismatch.PositionDigest, FiltersDigest: mismatch.FiltersDigest, Err: err}
	}
	// The two lanes' 410s are two generated types on purpose (the feed's
	// names its epoch and re-enters there; the inbox's has none and re-enters
	// at since=0), so each maps in its own arm, on its own lane only, and
	// its resume URL must re-enter at the fence the body declares.
	var feedGone *basecamp.FeedPositionGoneError
	if errors.As(err, &feedGone) {
		if lane != AccountLane {
			return &PollError{Kind: PollUnrecoverable, Err: errors.New("eventfeed: the inbox answered with the feed's 410 shape")}
		}
		if err := checkResumeCursor(feedGone.Resume, strconv.FormatInt(feedGone.EpochAfterID, 10)); err != nil {
			return &PollError{Kind: PollUnrecoverable, Err: err}
		}
		return &PollError{Kind: PollGone, EpochAfterID: feedGone.EpochAfterID, ResumeURL: feedGone.Resume, Err: err}
	}
	var inboxGone *basecamp.InboxPositionGoneError
	if errors.As(err, &inboxGone) {
		if lane != InboxLane {
			return &PollError{Kind: PollUnrecoverable, Err: errors.New("eventfeed: the feed answered with the inbox's 410 shape")}
		}
		if err := checkResumeCursor(inboxGone.Resume, "0"); err != nil {
			return &PollError{Kind: PollUnrecoverable, Err: err}
		}
		return &PollError{Kind: PollGone, ResumeURL: inboxGone.Resume, Err: err}
	}
	var request *basecamp.FeedRequestError
	if errors.As(err, &request) {
		// The 400's reason keys the recover-versus-stop split. A server
		// that sent none (it predates bc3 #13362) leaves the message as its
		// only signal, and that is the fallback below — never a guess
		// between the two on any other basis.
		switch request.Reason {
		case basecamp.FeedReasonInvalidPosition:
			return &PollError{Kind: PollPositionInvalid, Msg: request.Err.Message, Err: err}
		case basecamp.FeedReasonInvalidFilter:
			return &PollError{Kind: PollFilterInvalid, Msg: request.Err.Message, Err: err}
		case "":
			// Falls through to the message fallback below.
		default:
			// A reason the contract does not name: the 400 is surfaced
			// as undifferentiated, never guessed from its message.
			return &PollError{Kind: PollUnrecoverable, Err: errors.New("eventfeed: the 400 carries a reason the contract does not name")}
		}
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

// isRedirectStatus reports a 3xx: at the guard, a response to answer; at
// the seam, the status the generated call surfaced once the guard had.
func isRedirectStatus(status int) bool {
	return status >= 300 && status <= 399
}

// isCancellation reports an end to the call driven by the CONNECTOR's own
// context, which the seam returns as-is: the connector cancelled it and reads
// the context, not a kind. Judged on the context's state, not on the error
// alone — the HTTP client's own timeout also surfaces as a wrapped
// DeadlineExceeded, and that one is a transport failure to classify.
func isCancellation(ctx context.Context, err error) bool {
	return ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded))
}

// errContinuationUnparsable is the fixed cause for a continuation whose query
// the wrapper could not turn into options: the wrapper's own error names the
// offending value, which is server-chosen text a rendering must not carry.
var errContinuationUnparsable = errors.New("eventfeed: the continuation URL carries a filter value that does not parse")

// checkNextCursor holds a page's `next` to a walk continuation's shape: it
// carries `position` and never `since`. A `next` that would re-enter at a
// since — a present entry, which commits the head and skips the rest of the
// walk — makes the page malformed, refused the way a positionless page is:
// no re-entry, no save. The URL is server text; the error names none of it.
func checkNextCursor(next string) error {
	if next == "" {
		return nil
	}
	u, err := url.Parse(next)
	if err != nil {
		return errors.New("eventfeed: the page's next URL does not parse")
	}
	values, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return errors.New("eventfeed: the page's next URL's query does not parse whole")
	}
	if len(values["position"]) != 1 || values["position"][0] == "" || len(values["since"]) != 0 {
		return errors.New("eventfeed: the page's next URL must continue at a position, never a since")
	}
	return nil
}

// errContinuationFilters is the fixed cause for a followed URL whose filter
// dimensions differ from the lane's own: the URL's filters are never
// trusted, and a continuation that would change the lineage is refused.
var errContinuationFilters = errors.New("eventfeed: the continuation URL's filters differ from the lane's")

// sameStrings and sameInt64s compare two filter dimensions as sets — order
// and repetition aside, as the srv2 digest reads them.
func sameStrings(a, b []string) bool { return sameSet(a, b) }

func sameInt64s(a, b []int64) bool { return sameSet(a, b) }

func sameSet[T comparable](a, b []T) bool {
	as, bs := map[T]struct{}{}, map[T]struct{}{}
	for _, v := range a {
		as[v] = struct{}{}
	}
	for _, v := range b {
		bs[v] = struct{}{}
	}
	if len(as) != len(bs) {
		return false
	}
	for v := range as {
		if _, ok := bs[v]; !ok {
			return false
		}
	}
	return true
}

// isServerDigest reports a bare srv2 digest: exactly 16 lowercase hex.
func isServerDigest(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, r := range s {
		digit, hex := r >= '0' && r <= '9', r >= 'a' && r <= 'f'
		if !digit && !hex {
			return false
		}
	}
	return true
}

// checkContinuationCursor requires a followed URL to carry exactly one cursor
// — a position or a since — so a server URL that omits it, or spells it under
// a key the parser does not know, is never re-issued as a bare present entry
// that commits the head and skips the rest of the walk.
func checkContinuationCursor(position, since string) error {
	if (position == "") == (since == "") {
		return errors.New("eventfeed: the continuation URL must carry exactly one of position and since")
	}
	return nil
}

// eventFromFeed maps the wrapper's FeedEvent onto the connector's Event. The
// details object passes through as the bytes the server sent, under the push
// decoder's rule — an object is kept whole, null is absent, anything else is
// refused — so the two lanes deliver byte-identical detail objects: explicit
// nulls and the members of newly cataloged types survive on both.
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
	if trimmed := bytes.TrimSpace(fe.Details); len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) {
		// The push decoder's rule for the same bytes: an object, valid
		// UTF-8, no lone surrogate escape — so both lanes deliver the same
		// document or refuse the same one.
		if !isJSONObject(trimmed) || !utf8.Valid(trimmed) || hasLoneSurrogateEscape(trimmed) {
			return Event{}, fmt.Errorf("eventfeed: poll row %d carries details that are not a well-formed object", fe.ID)
		}
		ev.Details = json.RawMessage(slices.Clone(trimmed))
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
