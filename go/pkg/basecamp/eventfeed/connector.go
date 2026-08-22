package eventfeed

import (
	"context"
	"fmt"
	"iter"
	"math/rand/v2"
	"net/url"
	"sync"
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
	// claimTerminal serializes the publication of a terminal element against
	// Close. Installed by Events, which is the only thing that builds a run.
	claimTerminal func() bool
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
	cfg   config
	hooks testHooks

	// mu guards the close latch and the active run's cancellation, which have
	// to move together: close() is a universal edge from every non-absorbing
	// state, so cancellation must be VISIBLE to the run before Close returns
	// (see Close). The lock is held only across those two field accesses —
	// never across a seam call, a yield, or an observer callback — so a Close
	// taken from inside any of them cannot contend with the run goroutine.
	mu sync.Mutex
	// consumed latches the single-shot claim (Events). Guarded by mu, in the
	// SAME critical section that publishes runDone: claimed-but-unpublished
	// was a window in which a concurrent Wait — synchronized with the claim
	// through the usage terminal a second consumption yields — read a nil
	// runDone and returned while the run it should await was starting.
	consumed bool
	// isClosed latches Close, so a run that starts afterwards cancels itself
	// immediately rather than making one wire attempt first.
	isClosed bool
	// cancelRun cancels the active run's context. Registered by Events for
	// exactly the span of one iteration, nil otherwise.
	cancelRun context.CancelFunc
	// terminalClaimed records that a terminal element won the race against
	// Close and will be published. It is written under this mutex so the two
	// decisions are ordered rather than merely likely to be.
	terminalClaimed bool
	// runDone is closed when the iteration function returns. Published by
	// Events at the start of a run and never cleared, so Wait either finds no
	// run at all or a channel that is closed exactly when the run has exited.
	runDone chan struct{}
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
	// frameDeferred fires when the in-flight-poll servicing parks one receive —
	// always a socket outcome — for the walk's next dispatch point. It is the
	// rendezvous for "the seam call is now the only thing this walk is waiting
	// on", which is what makes transition 21's deferral assertable without
	// racing the seam call's own completion.
	frameDeferred func()
	// supersededWake fires each time the grace phase that follows a deferral
	// takes a WAKE — a staleness firing or a window swap — and before the
	// deadline that wake is checked against. Wakes are allowed to be early, and
	// the deadline is what decides; the difference is invisible from outside,
	// which is why the phase's immunity to frame resets could not be asserted
	// without it. A frame arriving inside the phase re-arms staleness and
	// produces exactly one such wake, and a test must know that wake has been
	// consumed before it can assert the phase still ends on its own.
	supersededWake func()
	// subscribeWritten fires once the subscribe command has been written and
	// BEFORE the phase deadline is stopped. It exists for the one ordering
	// this package cannot otherwise assert: the deadline expiring in the
	// window between a successful write and the Stop that cancels it —
	// externally a coin flip between two ready select cases, and the reason
	// Stop's result must be read rather than discarded.
	subscribeWritten func()
	// runContext fires with the active run's context as soon as it exists.
	// Close's guarantee is a statement ABOUT that context, so a test needs the
	// context itself; every in-struct proxy for it (a cleared cancel func, the
	// close latch) is set by a broken Close too, and asserting on one would be
	// vacuous.
	runContext func(context.Context)
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
	return &Connector{cfg: cfg}, nil
}

// validateConfig applies §23's construction-time validation, fail-closed. It
// also canonicalizes the base origin in place: the canonical form is what
// both the checkpoint key and continuation validation compare against.
func validateConfig(cfg *config) error {
	if cfg.origin == "" {
		return usageError("an API base origin is required")
	}
	// Validated RAW, before canonicalization: CanonicalOrigin lowercases
	// through strings.ToLower, which rewrites every invalid byte to U+FFFD,
	// so a post-canonical check would pass two origins differing only in
	// invalid bytes AFTER they had collapsed to one canonical form — one
	// checkpoint lineage for two configured origins, the exact many-to-one
	// identity checkIdentityText exists to refuse. Canonicalization of valid
	// UTF-8 cannot produce invalid UTF-8, so the canonical form needs no
	// second check.
	if err := checkIdentityText("the API base origin", cfg.origin); err != nil {
		return err
	}
	canonical, err := CanonicalOrigin(cfg.origin)
	if err != nil {
		return usageError(err.Error())
	}
	if err := checkOriginScheme(canonical); err != nil {
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
		runCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		// The single-shot claim is taken under c.mu, in the SAME critical
		// section that publishes runDone — not before it. A claim taken first
		// (the old atomic CompareAndSwap) left a window between winning and
		// publishing in which the claim was already OBSERVABLE — a second
		// consumption yields the usage terminal — while Wait still read a nil
		// runDone and returned at once, reporting quiescence for a run that
		// had started and not exited. One section leaves Wait nothing to
		// linearize between: it runs wholly before the claim (no run to
		// await) or wholly after runDone exists.
		//
		// Register this run's cancellation under the same lock Close latches
		// under, so the two cannot straddle each other: a Close that already
		// latched cancels here, and one that latches later finds the func and
		// cancels it itself. Either way runCtx is done before Close returns —
		// there is no window in which the run proceeds past a returned Close.
		c.mu.Lock()
		if c.consumed {
			c.mu.Unlock()
			cancel()
			yield(Event{}, &TerminalError{
				Reason: ReasonUsage,
				Msg:    "Events is single-shot: this connector was already consumed",
			})
			return
		}
		c.consumed = true
		// Installed before the loop is built, and under the same mutex Close
		// takes, so the run cannot reach a terminal before the claim exists.
		c.cfg.claimTerminal = c.claimTerminal
		// runDone is CLOSED rather than cleared on exit: clearing it would
		// leave a window where Wait reads nil — and returns at once — while
		// this function has not finished unwinding, which is the one instant
		// Wait exists to cover.
		c.runDone = done
		if c.isClosed {
			// Closed before the first iteration: end cleanly with zero wire
			// attempts, deterministically.
			cancel()
		} else {
			c.cancelRun = cancel
		}
		c.mu.Unlock()
		defer cancel()
		defer close(done)
		// Fired only once the cancellation is REGISTERED: before that point
		// the run is protected by the isClosed latch checked just above, not
		// by the cancel func, so a context handed out earlier would invite an
		// assertion about a window the latch already covers.
		if c.hooks.runContext != nil {
			c.hooks.runContext(runCtx)
		}
		defer func() {
			c.mu.Lock()
			c.cancelRun = nil
			c.mu.Unlock()
		}()
		newLoop(runCtx, &c.cfg, c.hooks).run(yield)
	}
}

// Close stops the feed: it abandons, never drains (undelivered buffered
// events are re-served from the last usable checkpoint on the next run).
// Idempotent and safe from any goroutine, including the iteration goroutine
// itself. The iterator ends with no error element.
//
// Cancellation is SYNCHRONOUS with the return: an active run's context is
// cancelled here, on the caller's goroutine, so the cancellation is visible
// before Close returns, and any seam call (mint, dial, poll) or delivery that
// begins afterwards begins on an ALREADY-CANCELLED context. §23 makes close()
// a universal edge from every non-absorbing state, and delegating the cancel to
// an independently scheduled goroutine would leave the edge merely eventual — a
// Close taken inside Observer.Connecting would return, and the mint one
// statement later would still go out.
//
// What that does NOT promise, and used to: that no seam call can BEGIN after
// Close returns. Close is not the only goroutine involved. The run goroutine
// checks its context at each dispatch point and then acts, so a Close landing
// between one of those checks and the call it guards cannot stop the call from
// starting — only from starting on a live context. Closing that window would
// mean holding a lock across every seam call, i.e. across arbitrary host code,
// which trades a benign race for a deadlock reachable from any callback.
//
// A CHECKPOINT SAVE is the one effect where that matters, because it is the
// only one that outlives the process: a store may ignore the cancelled context,
// and the built-in one documents that it deliberately does. So a save decided
// just before Close can still be written just after. That is intended. The
// position was accepted and its events were delivered before Close was called,
// and the store is what tells the next run where those deliveries stopped —
// dropping the write would silently re-deliver them. Everything else that can
// start in the window is a read whose context is already dead: it returns
// promptly, its result is discarded, and the run takes the Closed edge.
//
// # Ordering a second connector over the same store is the CONSUMER's to do
//
// Close does not order it, and no gate inside Close can. Wait does: the run
// goroutine has exited, so no save can be in flight, by construction rather
// than by a window narrow enough to be unlikely. Await the iterator's
// termination — or Wait — before opening a second connector over the same
// checkpoint store.
//
// What Close deliberately does NOT do is wait for the run goroutine to exit.
// Close is callable from inside a consumer callback — an observer, a signal
// handler, the iteration's own loop body — every one of which runs ON that
// goroutine, so waiting for it would deadlock on the caller. Cancellation
// visible before the return is the guarantee; the run then unwinds through
// its own dispatch points, closing the socket and joining the pump.
// The cancellation fires while callers are still SERIALIZED, which is what
// makes the guarantee hold for every caller rather than only the first.
// Clearing the field and cancelling outside the lock leaves a window: the
// first Close can be descheduled after unlocking and before invoking cancel,
// and a concurrent second Close then observes nil, takes no action and
// returns — with the run context still live. Its caller has been told the feed
// is closed while a delivery or a seam call can still begin, which is the one
// thing Close promises does not happen. Holding the mutex across the cancel is
// safe: a CancelFunc runs no consumer code, and any context.AfterFunc it
// triggers runs on its own goroutine, so it cannot re-enter Close.
// claimTerminal decides, atomically against Close, whether a terminal element
// may still be published.
//
// emitTerminal used to read the run context and then yield, which is two acts
// with a gap: a Close landing in that gap cancels the run and RETURNS while the
// yield goes ahead, so a consumer that closed still receives an error element —
// the one thing §23 says close() does not produce. A context read cannot fix
// that, because the thing being raced is not the context's value but the order
// of two decisions.
//
// So the decision moves under the mutex Close already serializes on. Whichever
// call takes it first wins outright: a Close that arrives first makes this
// return false and the run takes the Closed edge, while a terminal claimed
// first is published even though Close cancels a moment later — which is
// correct, because the feed had already terminated when the consumer closed.
func (c *Connector) claimTerminal() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isClosed {
		return false
	}
	c.terminalClaimed = true
	return true
}

func (c *Connector) Close() error {
	c.mu.Lock()
	c.isClosed = true
	if c.cancelRun != nil {
		c.cancelRun()
		c.cancelRun = nil
	}
	c.mu.Unlock()
	return nil
}

// Wait blocks until the run goroutine has exited, and returns immediately when
// no run is active. It is the quiescence point Close deliberately is not.
//
// Use it before opening a second connector over the same checkpoint store from
// a goroutine that does not own the range loop. A consumer that DOES own the
// loop needs nothing extra: the iterator returning is the same guarantee — the
// run goroutine has exited by then, so no save can be in flight.
//
// It is not callable from a consumer callback. Every callback — an observer, a
// signal handler, the loop body — runs ON the run goroutine, so waiting for
// that goroutine from inside one waits for itself. Close is the call that is
// safe from anywhere; this is the one that is safe from anywhere ELSE.
func (c *Connector) Wait() {
	c.mu.Lock()
	done := c.runDone
	c.mu.Unlock()
	if done != nil {
		<-done
	}
}
