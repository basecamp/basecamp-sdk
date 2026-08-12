package eventfeed

import (
	"context"
	"fmt"
	"iter"
	"math/rand/v2"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// Connector-owned constants (SPEC.md §23 "Constants" / Appendix A's
// EVENT_FEED_* rows). Exported where they back a configurable option;
// unexported where §23 pins them.
const (
	// DefaultConfirmationDeadline is the default confirmation deadline
	// (EVENT_FEED_CONFIRMATION_DEADLINE, 10s; configurable via
	// WithConfirmationDeadline).
	DefaultConfirmationDeadline = 10 * time.Second
	// DefaultLiveBufferCapacity is the default live-buffer capacity
	// (EVENT_FEED_LIVE_BUFFER_CAPACITY, 10,000 events; configurable via
	// WithLiveBufferCapacity; deliberately decoupled from the dedupe
	// capacity — only event-bearing frames are buffered).
	DefaultLiveBufferCapacity = 10_000

	// handshakeDeadline (EVENT_FEED_HANDSHAKE_DEADLINE, 10s) spans
	// dial-to-welcome: it is armed on entry to Connecting, before dial, so a
	// stalled TCP connect or HTTP upgrade cannot hang the connector.
	handshakeDeadline = 10 * time.Second
	// defaultStaleAfter (EVENT_FEED_STALE_AFTER, 7500ms) is the SDK-pinned
	// staleness detection policy: two missed 3-second server heartbeats plus
	// 25% grace. Not a public option; the conformance driver overrides it
	// in-package.
	defaultStaleAfter = 7500 * time.Millisecond
	// authFailureThreshold (EVENT_FEED_AUTH_FAILURE_THRESHOLD, 3) is the
	// shared connection-level authorization counter's terminal threshold.
	authFailureThreshold = 3
	// maxFrameBytes (EVENT_FEED_MAX_FRAME_BYTES, 1 MiB) bounds every inbound
	// frame; the transport enforces it while reading.
	maxFrameBytes = 1 << 20
	// closeCodeNormal is the WebSocket normal-closure status the connector
	// sends on every client-initiated close.
	closeCodeNormal = 1000
)

// Start selects the feed's entry mode (SPEC.md §23 "Options and Per-Language
// Naming"). The zero Start is StartResume.
type Start struct {
	kind     startKind
	eventID  int64
	position string
}

type startKind int

const (
	startResume startKind = iota
	startPresent
	startBeginning
	startAfter
	startAtPosition
)

// StartResume is the default entry mode: the stored position if any, else
// the present. It is the ONLY mode a configured checkpoint store positions —
// the three explicit modes below enter where they say they do, however full
// the store is, which is what makes a checkpointed feed replayable and
// resettable. A store still loads under every mode, and the run's first
// accepted page saves under the same key, so an explicit mode repoints the
// lineage rather than forking one.
func StartResume() Start { return Start{kind: startResume} }

// StartPresent enters at the present (since=now), whatever the store holds.
func StartPresent() Start { return Start{kind: startPresent} }

// StartBeginning enters at the beginning of served history (since=0),
// whatever the store holds.
func StartBeginning() Start { return Start{kind: startBeginning} }

// StartAfter enters just after the given event id (since=<id>), whatever the
// store holds.
func StartAfter(eventID int64) Start { return Start{kind: startAfter, eventID: eventID} }

// StartAtPosition enters at an explicit position token. Mutually exclusive
// with a checkpoint store: an explicit position and a stored lineage would
// contradict each other at entry.
func StartAtPosition(position string) Start {
	return Start{kind: startAtPosition, position: position}
}

// config is the connector's validated construction-time configuration.
type config struct {
	// origin is the canonicalized API base origin (CanonicalOrigin). One
	// input, two consumers: it is CheckpointKey.Origin, and it is the
	// same-origin reference every continuation and resume URL is validated
	// against before an authenticated follow.
	origin               string
	accountID            string
	minter               TicketMinter
	polls                PollSource
	filters              Filters
	start                Start
	transport            CableTransport
	clock                Clock
	store                CheckpointStore
	consumerNamespace    string
	confirmationDeadline time.Duration
	repairInterval       time.Duration
	dedupeCapacity       int
	liveBufferCapacity   int
	handler              SignalHandler
	observer             Observer

	// staleAfter is the staleness window — SPEC-pinned at 7500ms, so not a
	// public option; the tier-2 conformance driver overrides it in-package
	// (its scenarios' stalenessMs config).
	staleAfter time.Duration
	// rand is the uniform [0, 1) source behind the jitter draws — white-box
	// overridable in tests, no public option.
	rand func() float64
}

// Option configures a Connector, functional-options house style (SPEC.md §23
// "Options and Per-Language Naming" is the cross-language mapping table).
type Option func(*config)

// WithFilters narrows the feed to the given filters. Positions are
// filter-bound: changing filters starts a new checkpoint lineage. The three
// slices are snapshotted into connector-owned storage, so a caller may reuse
// or mutate its own arrays afterwards without reaching the subscription, the
// polls, or the checkpoint key (Filters.clone).
func WithFilters(f Filters) Option { return func(c *config) { c.filters = f.clone() } }

// WithStart selects the entry mode (default StartResume).
func WithStart(s Start) Option { return func(c *config) { c.start = s } }

// WithTransport replaces the default WebSocket transport — product seam, the
// documented extension point for custom WebSocket stacks.
func WithTransport(t CableTransport) Option { return func(c *config) { c.transport = t } }

// WithClock replaces the system clock — product seam, the extension point
// for embedded runtimes and deterministic tests.
func WithClock(cl Clock) Option { return func(c *config) { c.clock = cl } }

// WithCheckpointStore configures durable position persistence (default:
// none). A configured store requires WithConsumerNamespace.
func WithCheckpointStore(s CheckpointStore) Option { return func(c *config) { c.store = s } }

// WithConsumerNamespace names this consumer's checkpoint lineage — required
// whenever a store is configured (two independent consumers in one account
// must not share a lineage).
func WithConsumerNamespace(ns string) Option { return func(c *config) { c.consumerNamespace = ns } }

// WithConfirmationDeadline overrides the confirmation deadline (default 10s).
func WithConfirmationDeadline(d time.Duration) Option {
	return func(c *config) { c.confirmationDeadline = d }
}

// WithRepairInterval overrides the repair-poll cadence (default 60s; ±20%
// jitter is applied per cycle either way).
func WithRepairInterval(d time.Duration) Option { return func(c *config) { c.repairInterval = d } }

// WithDedupeCapacity overrides the delivered-id LRU capacity (default
// 10,000; must be positive — there is no dedupe-disabled mode).
func WithDedupeCapacity(n int) Option { return func(c *config) { c.dedupeCapacity = n } }

// WithLiveBufferCapacity overrides the live-buffer capacity (default 10,000
// events; must be positive).
func WithLiveBufferCapacity(n int) Option { return func(c *config) { c.liveBufferCapacity = n } }

// WithSignalHandler registers the semantic-signal handler. With none
// registered, every semantic signal is terminal (SPEC.md §23 "Semantic
// Signals").
func WithSignalHandler(h SignalHandler) Option { return func(c *config) { c.handler = h } }

// WithObserver registers lifecycle observability callbacks.
func WithObserver(o Observer) Option { return func(c *config) { c.observer = o } }

// Connector is the SPEC.md §23 Event Feed connector. Construct with New,
// consume with Events (single-shot), stop with Close.
type Connector struct {
	cfg       config
	consumed  atomic.Bool
	closeOnce sync.Once
	closed    chan struct{}
	hooks     testHooks
}

// testHooks are unexported in-package observation points for the tier-2/3
// harnesses (rendezvous without wall-clock sleeps). Production code never
// sets them.
type testHooks struct {
	stateChanged   func(connState)
	frameHandled   func(frameKind)
	catchUpEntered func(catchUpHandoff)
	// bufferOccupancy fires after every change to the state-machine-owned
	// live buffer's occupancy (events admitted minus events dropped) — the
	// tier-2 `expectBuffered` rendezvous.
	bufferOccupancy func(int)
	// signalRaised fires when a semantic signal is raised, before its
	// disposition is taken. It is the tier-2 `expectSignal` rendezvous, and it
	// is deliberately independent of handler registration: the default-terminal
	// fixtures assert the signal's exact payload with no handler installed,
	// which Observer.BufferOverflow (a count only) cannot carry.
	signalRaised func(Signal)
	// pumpBlocked fires when a frame pump hand-off finds the queue full — the
	// rendezvous for the staleness suspension rule's precondition.
	pumpBlocked func()
	// pumpReleased fires once a blocked hand-off completes AND its release
	// has re-armed the staleness window unsuspended. It is the rendezvous for
	// "the suspension has lifted": the state machine dequeuing the frame is
	// observable well before the pump's release runs, and only the release
	// settles which window a subsequent expiry is evaluated against.
	pumpReleased func()
	// pumpHandedOff fires once an item reaches the hand-off queue, reporting
	// whether it is the pump's terminating error.
	pumpHandedOff func(isErr bool)
	// frameDeferred fires when the in-flight-poll servicing takes one receive
	// out of band, reporting whether the deferral is an overflowing
	// admission. It is the rendezvous for "the seam call is now the only
	// thing this walk is waiting on", which is what makes the two deferral
	// classes' opposite ordering against the next Save assertable without
	// racing the seam call's own completion.
	frameDeferred func(overflow bool)
}

// New validates configuration and returns a Connector. New does no I/O —
// first I/O happens on the first Events iteration — and every validation
// failure is a usage-coded *TerminalError construction error with zero wire
// attempts (SPEC.md §23 "Consumer Surface").
//
// origin is the API base origin this feed is bound to (e.g.
// "https://3.basecampapi.com"), required and canonicalized with
// CanonicalOrigin. §23 gives it two consumers that must agree: it is
// CheckpointKey.Origin — the SDK supports configurable base URLs, so a
// position's lineage is origin-scoped — and it is the same-origin reference
// for the §8 validation every `next` continuation and 410 resume URL passes
// before an authenticated follow. It is a constructor input rather than an
// option precisely because both of those are unconditional; the Layer-1
// adapter that builds the minter and poll source over the generated
// operations knows it already.
func New(origin, accountID string, minter TicketMinter, polls PollSource, opts ...Option) (*Connector, error) {
	cfg := config{
		origin:               origin,
		accountID:            accountID,
		minter:               minter,
		polls:                polls,
		confirmationDeadline: DefaultConfirmationDeadline,
		repairInterval:       DefaultRepairInterval,
		dedupeCapacity:       DefaultDedupeCapacity,
		liveBufferCapacity:   DefaultLiveBufferCapacity,
		staleAfter:           defaultStaleAfter,
		rand:                 rand.Float64,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.transport == nil {
		cfg.transport = &WebSocketTransport{}
	}
	if cfg.clock == nil {
		cfg.clock = SystemClock()
	}
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}
	return &Connector{cfg: cfg, closed: make(chan struct{})}, nil
}

// validateConfig applies §23's construction-time validation, fail-closed. It
// also canonicalizes the base origin in place: the canonical form is what
// both the checkpoint key and continuation validation compare against.
func validateConfig(cfg *config) error {
	if cfg.origin == "" {
		return usageError("an API base origin is required")
	}
	canonical, err := CanonicalOrigin(cfg.origin)
	if err != nil {
		return usageError(err.Error())
	}
	if err := checkOriginScheme(canonical); err != nil {
		return err
	}
	if err := checkIdentityText("the API base origin", canonical); err != nil {
		return err
	}
	cfg.origin = canonical
	if cfg.accountID == "" {
		return usageError("accountID must be non-empty")
	}
	if err := checkIdentityText("accountID", cfg.accountID); err != nil {
		return err
	}
	if err := checkIdentityText("the consumer namespace", cfg.consumerNamespace); err != nil {
		return err
	}
	if cfg.minter == nil {
		return usageError("a TicketMinter is required")
	}
	if cfg.polls == nil {
		return usageError("a PollSource is required")
	}
	if err := cfg.filters.Validate(); err != nil {
		return err
	}
	if cfg.dedupeCapacity <= 0 {
		return usageError(fmt.Sprintf("dedupe capacity must be positive, got %d", cfg.dedupeCapacity))
	}
	if cfg.liveBufferCapacity <= 0 {
		return usageError(fmt.Sprintf("live buffer capacity must be positive, got %d", cfg.liveBufferCapacity))
	}
	if cfg.confirmationDeadline <= 0 {
		return usageError("confirmation deadline must be positive")
	}
	if cfg.repairInterval <= 0 {
		return usageError("repair interval must be positive")
	}
	switch cfg.start.kind {
	case startResume, startPresent, startBeginning:
		// No mode-specific validation.
	case startAfter:
		if cfg.start.eventID <= 0 {
			return usageError(fmt.Sprintf("StartAfter requires a positive event id, got %d", cfg.start.eventID))
		}
	case startAtPosition:
		if cfg.start.position == "" {
			return usageError("StartAtPosition requires a non-empty position")
		}
		if cfg.store != nil {
			return usageError("an explicit start position and a checkpoint store are mutually exclusive")
		}
	}
	if cfg.store != nil && cfg.consumerNamespace == "" {
		return usageError("a checkpoint store requires a consumer namespace")
	}
	return nil
}

// checkOriginScheme applies §23's Security Invariants to the configured base
// origin: https:// everywhere, http:// only for the §9 localhost/loopback
// carve-out — the same line checkCableURL draws for the cable URL (wss://
// with the same carve-out, via the same isLoopbackHost test) and the main Go
// client draws for its base URL. The origin is not merely where requests go:
// it is the trust anchor §8's same-origin algorithm validates every `next`
// continuation and 410 resume URL against, so a cleartext origin would make a
// cleartext continuation same-origin-valid and hand it to the authenticated
// poll seam. Its input is the canonical form, already lowercased with the
// default port stripped.
func checkOriginScheme(canonical string) error {
	u, err := url.Parse(canonical)
	if err != nil {
		return usageError(fmt.Sprintf("unparseable origin %q", canonical))
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopbackHost(u.Hostname()) {
			return nil
		}
		return usageError("the API base origin must use https (http is allowed only for localhost), got " + canonical)
	default:
		// The scheme is structural, never secret — safe to name.
		return usageError("the API base origin scheme " + `"` + u.Scheme + `"` + " is not http(s)")
	}
}

// Events returns the feed as a serial, deduplicated event sequence. It is
// single-shot: a second consumption yields exactly one usage-terminal error
// element. A terminal condition yields exactly one final (Event{}, error)
// element; cancellation, Close, and a consumer break end iteration with no
// error element — a clean stop, the feed is resumable by design.
func (c *Connector) Events(ctx context.Context) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		if !c.consumed.CompareAndSwap(false, true) {
			yield(Event{}, &TerminalError{
				Reason: ReasonUsage,
				Msg:    "Events is single-shot: this connector was already consumed",
			})
			return
		}
		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		select {
		case <-c.closed:
			// Closed before the first iteration: end cleanly with zero wire
			// attempts, deterministically.
			cancel()
		default:
		}
		go func() {
			select {
			case <-c.closed:
				cancel()
			case <-runCtx.Done():
			}
		}()
		newLoop(runCtx, &c.cfg, c.hooks).run(yield)
	}
}

// Close stops the feed: it abandons, never drains (undelivered buffered
// events are re-served from the last usable checkpoint on the next run).
// Idempotent and safe from any goroutine; the iterator observes it at the
// next loop turn and ends with no error element.
func (c *Connector) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return nil
}
