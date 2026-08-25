package eventfeed

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// The seam interfaces (SPEC.md §23 "Seam Contracts") isolate the connector
// from wire I/O, time, and persistence. Layer-1 adapters over the generated
// CreateStreamTicket/PollEvents operations plug in at TicketMinter and
// PollSource; one seam call is one fully-governed generated call — the
// generated operation keeps its full SPEC §7 contract (retry budget, backoff,
// Retry-After) inside the seam, and the connector never adds a second
// per-request retry layer. Clock is defined in clock.go; CheckpointStore in
// checkpoint.go.

// StreamTicket is a minted stream credential — the response of one generated
// CreateStreamTicket call.
type StreamTicket struct {
	// Ticket is the opaque bearer credential. Never logged.
	Ticket string
	// ExpiresIn is the ticket lifetime in seconds (~120). Server-owned and
	// never used for client-side scheduling: expiry is arbitrated by the
	// server, and the connector always mints fresh.
	ExpiresIn int
	// URL is the cable URL to connect to, verbatim — the connector never
	// assembles cable topology client-side. The ticket rides in its query
	// string, so the URL is never logged unredacted.
	URL string
}

// TicketMinter mints stream tickets. Each call is one fully-governed
// generated CreateStreamTicket call.
type TicketMinter interface {
	// MintStreamTicket mints a fresh ticket. ctx is the cancellation channel:
	// the connector cancels it on close, caller cancellation, and any
	// teardown of the attempt the call belongs to; a cancelled call must
	// return promptly.
	MintStreamTicket(ctx context.Context) (StreamTicket, error)
}

// MintErrorKind classifies a failed mint seam call. The adapter maps every
// SPEC §6/§7 outcome of the generated call onto exactly one kind.
type MintErrorKind int

const (
	// MintTransient — a retryable outcome exhausted inside the seam; rides
	// the reconnect cycle (Backoff).
	MintTransient MintErrorKind = iota + 1
	// MintThrottled — 429/503; RetryAfter honored as the floor of the next
	// reconnect delay.
	MintThrottled
	// MintUnauthorized — 401/403; increments the shared connection-level
	// authorization counter (terminal authorization_failed at threshold 3).
	MintUnauthorized
	// MintUnrecoverable — any non-retryable non-401/403 outcome (404, 422, a
	// malformed success) → Terminal(mint_failed), generated error attached.
	MintUnrecoverable
)

// String returns the kind's name.
func (k MintErrorKind) String() string {
	switch k {
	case MintTransient:
		return "transient"
	case MintThrottled:
		return "throttled"
	case MintUnauthorized:
		return "unauthorized"
	case MintUnrecoverable:
		return "unrecoverable"
	default:
		return fmt.Sprintf("MintErrorKind(%d)", int(k))
	}
}

// MintError is a failed mint seam call, classified.
type MintError struct {
	// Kind classifies the failure.
	Kind MintErrorKind
	// RetryAfter is the server-directed wait, when present (throttled).
	RetryAfter time.Duration
	// Err is the underlying (generated) error.
	Err error
}

// Error implements the error interface. The rendering never carries the
// ticket or a mint URL's query string, and is bounded by §9's
// MAX_ERROR_MESSAGE_LENGTH: Err is server-influenced text a TicketMinter
// composed.
func (e *MintError) Error() string {
	msg := "event feed mint failed (" + e.Kind.String() + ")"
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return truncateErrorText(msg)
}

// Unwrap exposes the underlying cause for errors.Is / errors.As traversal.
func (e *MintError) Unwrap() error {
	return e.Err
}

// Cursor addresses one poll page. Exactly one field is set; the zero Cursor
// is the bare present entry (the server treats it as since=now).
type Cursor struct {
	// Position is the durable resume/repair token (in-memory authoritative
	// within a run; durable via write-through when saves succeed).
	Position string
	// Since is "now", "0", or a decimal event id.
	Since string
	// PageURL is an absolute URL: a `next` continuation or a 410 resume URL.
	// Same-origin + no-downgrade validated before any poll call; never
	// persisted.
	PageURL string
}

// PollPage is one poll-lane page. The body envelope is the contract — never
// bind to response headers.
type PollPage struct {
	// Events are the page's rows, in strict event-id order.
	Events []Event
	// Position is the durable position after this page — the only thing that
	// ever advances the checkpoint.
	Position string
	// Next is the continuation URL; empty means the walk reached its frozen
	// head. Bound to that walk and never persisted.
	Next string
}

// PollSource serves poll pages. Each call is one fully-governed generated
// PollEvents call.
type PollSource interface {
	// Poll fetches one page at cursor under filters. ctx is the cancellation
	// channel: triggered on close, caller cancellation, and any teardown of
	// the attempt the call belongs to — a superseded poll must not stall
	// reconnection or return into a disposed attempt.
	Poll(ctx context.Context, cursor Cursor, filters Filters) (PollPage, error)
}

// PollErrorKind classifies a failed poll seam call. The adapter maps every
// SPEC §6/§7 outcome of the generated call onto exactly one kind.
type PollErrorKind int

const (
	// PollTransient — a retryable outcome exhausted inside the seam; retries
	// on the poll-retry timer, never terminal.
	PollTransient PollErrorKind = iota + 1
	// PollThrottled — 429/503; RetryAfter waited exactly, cap-exempt.
	PollThrottled
	// PollPositionInvalid — 400-position: re-enter since=<last poll-served
	// id> (or a present-class entry with none).
	PollPositionInvalid
	// PollFilterInvalid — 400-filter → Terminal(filter_invalid); Msg carries
	// the server's message naming the offending list, verbatim.
	PollFilterInvalid
	// PollFilterChanged — 409: discard the held position, re-enter since=.
	PollFilterChanged
	// PollGone — 410: EpochAfterID and ResumeURL set; dispatched as the
	// FeedGap semantic signal (a 410 never silently auto-continues).
	PollGone
	// PollUnauthorized — 401/403 after the seam's own refresh/retry budget;
	// rides the reconnect cycle, incrementing the shared authorization
	// counter.
	PollUnauthorized
	// PollRedirectRefused — a 3xx whose Location fails per-hop
	// same-origin/no-downgrade validation (auto-follow is disabled inside the
	// seam) → Terminal(invalid_continuation), never unrecoverable.
	PollRedirectRefused
	// PollUnrecoverable — any non-retryable outcome outside the feed's
	// 400/409/410 matrix (404, 405, an unexpected shape) →
	// Terminal(poll_failed), generated error attached.
	PollUnrecoverable
)

// String returns the kind's name.
func (k PollErrorKind) String() string {
	switch k {
	case PollTransient:
		return "transient"
	case PollThrottled:
		return "throttled"
	case PollPositionInvalid:
		return "position_invalid"
	case PollFilterInvalid:
		return "filter_invalid"
	case PollFilterChanged:
		return "filter_changed"
	case PollGone:
		return "gone"
	case PollUnauthorized:
		return "unauthorized"
	case PollRedirectRefused:
		return "redirect_refused"
	case PollUnrecoverable:
		return "unrecoverable"
	default:
		return fmt.Sprintf("PollErrorKind(%d)", int(k))
	}
}

// PollError is a failed poll seam call, classified.
type PollError struct {
	// Kind classifies the failure.
	Kind PollErrorKind
	// RetryAfter is the server-directed wait, when present (throttled).
	RetryAfter time.Duration
	// EpochAfterID is the 410 body's epoch_after_id (gone only).
	EpochAfterID int64
	// ResumeURL is the 410 body's resume URL (gone only).
	ResumeURL string
	// Msg carries the server's message for filter_invalid, naming the
	// offending list, verbatim.
	Msg string
	// LocationOrigin is the refused redirect Location, redacted to its
	// origin (redirect_refused only).
	LocationOrigin string
	// Err is the underlying (generated) error.
	Err error
}

// Error implements the error interface. The composed result is bounded by
// §9's MAX_ERROR_MESSAGE_LENGTH: Msg carries the server's message verbatim.
func (e *PollError) Error() string {
	msg := "event feed poll failed (" + e.Kind.String() + ")"
	if e.Msg != "" {
		msg += ": " + e.Msg
	}
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return truncateErrorText(msg)
}

// Unwrap exposes the underlying cause for errors.Is / errors.As traversal.
func (e *PollError) Unwrap() error {
	return e.Err
}

// CableTransport dials WebSocket connections — product seam, not a test hook:
// the documented extension point for custom WebSocket stacks.
type CableTransport interface {
	// Dial opens exactly one connection to wsURL, passed through untouched
	// (query string included), negotiating subprotocol "actioncable-v1-json"
	// and sending no Origin header. It must not auto-reconnect, must not
	// interpret, filter, or swallow application text frames, and refuses
	// redirects (§23 Security Invariants). ctx is the cancellation channel —
	// the connector cancels it on handshake-deadline expiry and on close,
	// and a cancelled dial must return promptly. maxFrameBytes must be
	// positive and enforced while reading (e.g. a read-limit on the socket),
	// rejecting an over-limit message without materializing it; a
	// non-positive value is refused as a usage error before any I/O — there
	// is no unlimited mode.
	Dial(ctx context.Context, wsURL string, maxFrameBytes int64) (CableConn, error)
}

// CableConn is one live cable connection.
type CableConn interface {
	// ReadFrame returns the next raw text frame VERBATIM — including
	// {"type":"disconnect",...} frames. That verbatim-ness is the whole point
	// of the seam: stock Action Cable discards the disconnect reason, and the
	// terminal/non-terminal distinction lives only in this raw frame. A peer
	// close surfaces as *CloseError; a message over the dial's maxFrameBytes
	// surfaces as an error matching ErrFrameOversize.
	ReadFrame(ctx context.Context) ([]byte, error)
	// WriteFrame sends one raw text frame. Close and ctx cancellation must
	// unblock an in-progress write; a write failure takes the current
	// state's socket-failure path.
	WriteFrame(ctx context.Context, data []byte) error
	// Close is idempotent, safe from any goroutine, and unblocks ReadFrame
	// and WriteFrame.
	Close(code int, reason string) error
}

// ErrFrameOversize reports an inbound message that exceeded the dial's
// maxFrameBytes. The default transport maps its WebSocket stack's read-limit
// rejection to this sentinel, and a custom CableTransport must surface its
// own the same way (matching via errors.Is): the size violation is one of
// §23's three invalid-frame shapes, and Observer.Disconnected must carry that
// invalid-frame indication — a classification a stack-specific untyped error
// cannot carry across the seam. The rendering is fixed prose: the over-limit
// frame is never materialized, so there is nothing to quote.
var ErrFrameOversize = errors.New("eventfeed: inbound cable frame exceeds the dial's maximum frame size")

// CloseError is returned by CableConn.ReadFrame when the peer closed the
// socket, carrying the WebSocket close status.
type CloseError struct {
	// Code is the WebSocket close code.
	Code int
	// Reason is the close reason, if any.
	Reason string
}

// Error implements the error interface. Reason is peer-supplied, so the
// rendering is bounded by §9's MAX_ERROR_MESSAGE_LENGTH like every other
// rendering of peer-derived text in this package, and the type is flat — it
// retains no cause a chain walk could recover the unbounded original from.
// RFC 6455 already caps a close reason at 123 bytes and the default transport
// enforces it, so the truncation binds only on a transport that does not.
func (e *CloseError) Error() string {
	if e.Reason != "" {
		return truncateErrorText(fmt.Sprintf("cable connection closed by peer: code %d: %s", e.Code, e.Reason))
	}
	return fmt.Sprintf("cable connection closed by peer: code %d", e.Code)
}

// DialErrorKind classifies a failed dial.
type DialErrorKind int

const (
	// DialTransient — an ordinary dial failure; rides the reconnect cycle
	// (Backoff).
	DialTransient DialErrorKind = iota + 1
	// DialPolicy — a permanent refusal the transport detected (a redirect
	// encountered, a scheme the invariants forbid, an unparseable URL) →
	// Terminal(invalid_cable_url), never Backoff: a fresh mint returns the
	// same unusable URL.
	DialPolicy
)

// String returns the kind's name.
func (k DialErrorKind) String() string {
	switch k {
	case DialTransient:
		return "transient"
	case DialPolicy:
		return "policy"
	default:
		return fmt.Sprintf("DialErrorKind(%d)", int(k))
	}
}

// DialError is a failed dial, classified. Its fields are exported because a
// custom CableTransport must be able to construct one, and that is exactly
// why the type itself cannot enforce ticket secrecy: the transport holds the
// full ticket-bearing URL, and Error renders whatever the constructor
// stored. The guarantee is therefore split. The AUTHOR owes the discipline
// below — this package's own compositions keep every Reason and cause on a
// closed vocabulary — and the connector does not depend on a custom
// transport honoring it: it treats every seam-returned DialError as
// untrusted, reading only its Kind and never copying its text or retaining
// its cause onto an observer or terminal surface.
type DialError struct {
	// Kind classifies the failure. It is the one field the connector reads
	// from a seam-returned DialError.
	Kind DialErrorKind
	// Reason names the policy violation (policy only). Never the URL's query
	// string.
	Reason string
	// Err is the underlying cause. Never the dialed URL or an error that
	// renders it — the ticket rides in its query string, and url.Error
	// renders the full URL. The built-in transport stores only causes
	// flattened to a closed vocabulary (dialFailure).
	Err error
}

// Error implements the error interface. The rendering never carries the
// dialed URL's query string when the DialError is this package's own — the
// built-in transport composes reasons and causes from closed vocabularies —
// and the composed result is bounded by §9's MAX_ERROR_MESSAGE_LENGTH like
// every other URL-derived rendering here (CloseError is the precedent):
// Reason can embed a mint-URL component — a scheme or an explicit port —
// whose length url.Parse does not bound.
func (e *DialError) Error() string {
	msg := "cable dial failed (" + e.Kind.String() + ")"
	if e.Reason != "" {
		msg += ": " + e.Reason
	}
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return truncateErrorText(msg)
}

// Unwrap exposes the underlying cause for errors.Is / errors.As traversal.
func (e *DialError) Unwrap() error {
	return e.Err
}

// Signal is a semantic signal — a condition that changes what the feed can
// promise (SPEC.md §23 "Semantic Signals"). A closed union of two distinct
// types: BufferOverflow and FeedGap.
type Signal interface{ isSignal() }

// BufferOverflow reports live-buffer overflow: the oldest buffered events
// were dropped. A dropped lower-id event behind the present position is not
// poll-repairable, so overflow during the entry window invalidates
// completeness — its disposition must be taken before any save.
type BufferOverflow struct {
	// DroppedIDs are the exact event ids dropped — "dropped" is unambiguous.
	DroppedIDs []int64
	// DroppedCount is the number of events dropped.
	DroppedCount int
}

func (BufferOverflow) isSignal() {}

// FeedGap reports a 410: the feed's history before EpochAfterID is gone. A
// 410 never silently auto-continues.
type FeedGap struct {
	// EpochAfterID is the 410 body's epoch_after_id.
	EpochAfterID int64
	// ResumeURL is the server-provided resume URL (it preserves the
	// canonical filter set).
	ResumeURL string
}

func (FeedGap) isSignal() {}

// Disposition is a signal handler's decision.
type Disposition int

const (
	// Accept continues the feed: a FeedGap resumes via the provided resume
	// URL; a BufferOverflow means the consumer owns the acknowledged
	// incompleteness (acceptance is not license to skip retained
	// deliveries).
	Accept Disposition = iota + 1
	// Terminate ends the feed with the signal's terminal reason
	// (buffer_overflow / feed_gap), with no save.
	Terminate
)

// SignalHandler decides a semantic signal's disposition. A registered handler
// is invoked exactly once per signal, synchronously, on the consumer's
// execution context, before its disposition takes effect. With no handler
// registered, every semantic signal is terminal.
type SignalHandler func(Signal) Disposition

// Observer is optional lifecycle observability in the httptrace.ClientTrace
// style: a struct of optional funcs, extensible without breaking
// implementers. All callbacks fire on the consumer's execution context, never
// concurrently with a delivery. All are observability-only — none carries a
// disposition (semantic dispositions live exclusively in the SignalHandler).
type Observer struct {
	// Connecting fires before each connect attempt.
	Connecting func(attempt int, delay time.Duration)
	// Connected fires when a dial succeeds.
	Connected func()
	// Confirmed fires on confirm_subscription.
	Confirmed func()
	// Disconnected fires when a socket is torn down.
	Disconnected func(reason string, err error)
	// CatchUpStarted fires when a poll walk begins.
	CatchUpStarted func(cursor Cursor)
	// PageDelivered fires after a poll page's events were delivered.
	PageDelivered func(events int, position string)
	// Checkpoint fires after a page's position was saved — after that page's
	// events were accepted.
	Checkpoint func(position string)
	// CheckpointSaveFailed fires when a save fails; save failures never kill
	// the feed.
	CheckpointSaveFailed func(err error)
	// CaughtUp fires when the walk is done and the buffer drained
	// (→ Streaming).
	CaughtUp func()
	// Gap fires on a 410 — observability only; the disposition lives in the
	// SignalHandler.
	Gap func(epochAfterID int64, resumeURL string)
	// PositionRejected fires on a 400-position or 409 re-entry.
	PositionRejected func(kind PollErrorKind)
	// StaleConnection fires when staleness tears a socket down.
	StaleConnection func(sinceLastFrame time.Duration)
	// BufferOverflow fires when the live buffer drops events — observability
	// only; the disposition lives in the SignalHandler.
	BufferOverflow func(droppedCount int)
}
