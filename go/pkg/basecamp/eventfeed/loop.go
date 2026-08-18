package eventfeed

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// The connector run loop (SPEC.md §23 "State Machine"). The whole protocol
// executes on the consumer's goroutine — ranging Events IS the state machine
// (design decision 3) — with one auxiliary reader pump per live socket, one
// transient goroutine per in-flight poll seam call (catchup.go's pollPage,
// which carries no state: the state machine stays the only mutator), and no
// mutex-guarded connector state. This file carries Idle through
// the confirm/reject/deadline paths (§23 transitions 1–15, the universal
// Closed edge, and the out-of-inventory usage / mint_failed /
// invalid_cable_url / buffer_overflow edges), the reconnect cycle every
// continuable failure returns to, and the frame pump with the staleness
// policy it feeds; a confirmed subscription hands off at the typed
// catchUpHandoff boundary below to the catch-up walk, the entry boundary, and
// streaming, which live in catchup.go.
//
// What a reconnect preserves and what it rebuilds is a §23 distinction, not
// an implementation detail. Preserved on the loop, across every attempt: the
// in-memory position (authoritative for resume within the run — the store is
// never re-read after its one load), the highest id the poll lane has served
// (the reset cursor a 400-position or 409 re-enters at), the delivered-id
// LRU, the live buffer of admitted-but-undelivered events, and the shared
// authorization counter, which only a successful poll page resets. Rebuilt
// per attempt: a freshly minted ticket (the connector never stores a mint URL
// across attempts), the socket, the pump, the staleness holder, every timer,
// and the consecutive-poll-failure index. The failed-cycle count grows per
// failed cycle and resets on confirmation.

// connState enumerates SPEC.md §23's 11 states. String renders the tier-2
// fixture spelling (snake_case).
type connState int

const (
	stateIdle connState = iota
	stateBackoff
	stateMinting
	stateConnecting
	stateAwaitingWelcome
	stateAwaitingConfirmation
	stateCatchingUp
	stateDraining
	stateStreaming
	stateTerminal
	stateClosed
)

// String renders the fixture spelling of the state.
func (s connState) String() string {
	switch s {
	case stateIdle:
		return "idle"
	case stateBackoff:
		return "backoff"
	case stateMinting:
		return "minting"
	case stateConnecting:
		return "connecting"
	case stateAwaitingWelcome:
		return "awaiting_welcome"
	case stateAwaitingConfirmation:
		return "awaiting_confirmation"
	case stateCatchingUp:
		return "catching_up"
	case stateDraining:
		return "draining"
	case stateStreaming:
		return "streaming"
	case stateTerminal:
		return "terminal"
	case stateClosed:
		return "closed"
	default:
		return fmt.Sprintf("connState(%d)", int(s))
	}
}

// Timer kinds (SPEC.md §23 "Clock, Timers, and Virtual Time": six kinds,
// kebab-case; the four armed by the lifecycle are here, and `repair-poll`
// and `poll-retry` are armed by the walk, in catchup.go).
const (
	timerHandshakeDeadline    = "handshake-deadline"
	timerConfirmationDeadline = "confirmation-deadline"
	timerBackoff              = "backoff"
	timerStaleness            = "staleness"
)

// Disconnect reason literals (class-1 wire literals, SPEC.md §23 "Disconnect
// Dispatch"). Dispatch is on the reason string, never the reconnect flag
// alone; `remote` and unrecognized reasons need no literal here — they take
// the socket-drop default.
const (
	disconnectReasonUnauthorized  = "unauthorized"
	disconnectReasonProtocolFatal = "invalid_event_stream_command"
)

// sincePresent is the `since` value that enters at the server's present head
// (the bare entry is equivalent). Every cursor spelled with it is
// present-class (SPEC.md §23 "Entry Boundary").
const sincePresent = "now"

// errStaleConnection reports a staleness teardown.
var errStaleConnection = errors.New("event feed connection stale: no inbound frames within the staleness window")

// attempt is one connect cycle's mutable ownership: its cancellation scope
// (cancelled on teardown, so an in-flight seam call or dial belonging to the
// attempt returns promptly) and, once dialed, the live connection.
type attempt struct {
	ctx    context.Context
	cancel context.CancelFunc
	lc     *liveConn
}

// pumpDepth is the frame pump's bounded hand-off queue depth (SPEC.md §23
// "Cable Protocol Details": bounded, blocking, never dropping — the
// state-machine-owned live buffer is the only place a frame can ever be
// dropped).
const pumpDepth = 256

// pumpItem is one hand-off from the reader pump: a raw frame, or the read
// error that ended the pump.
type pumpItem struct {
	data []byte
	err  error
}

// liveConn is one live socket with its reader pump and staleness holder.
type liveConn struct {
	conn   CableConn
	frames chan pumpItem
	stale  *staleHolder
	hooks  testHooks
}

// newLiveConn arms staleness (socket open) and then starts the pump, in that
// order, so the timer exists before the first frame can reset it.
func newLiveConn(ctx context.Context, conn CableConn, clock Clock, staleAfter time.Duration, hooks testHooks) *liveConn {
	lc := &liveConn{
		conn:   conn,
		frames: make(chan pumpItem, pumpDepth),
		stale:  newStaleHolder(clock, staleAfter),
		hooks:  hooks,
	}
	go lc.pump(ctx)
	return lc
}

// pump reads frames and hands them to the state machine over the bounded
// queue. The staleness reset happens HERE, at frame receipt — pump-side, so
// frame-vs-deadline ordering is defined at the transport boundary regardless
// of queue depth or consumer latency (SPEC.md §23 "Cable Protocol Details").
// The pump exits by sending the terminating read error (unless the attempt
// was already cancelled) and closing the channel; the closed channel is the
// join signal disposal drains to.
func (lc *liveConn) pump(ctx context.Context) {
	defer close(lc.frames)
	for {
		data, err := lc.conn.ReadFrame(ctx)
		if err != nil {
			lc.handOff(ctx, pumpItem{err: err})
			return
		}
		lc.stale.reset()
		if !lc.handOff(ctx, pumpItem{data: data}) {
			return
		}
	}
}

// handOff passes one item to the state machine over the bounded queue. The
// queue never drops: at capacity the pump BLOCKS, propagating back-pressure
// through the socket to TCP (SPEC.md §23 "Cable Protocol Details" — the
// state-machine-owned live buffer is the only place a frame can be dropped).
// A blocked hand-off suspends the staleness EVALUATION for as long as it
// lasts: a full queue proves the peer is sending faster than the connector
// consumes — the opposite of a dead peer — and a pump that is not reading
// cannot observe the resets that would prove liveness, so the absence of one
// is not evidence. It reports whether the item was handed off.
func (lc *liveConn) handOff(ctx context.Context, item pumpItem) bool {
	select {
	case lc.frames <- item:
		lc.handedOff(item)
		return true
	default:
	}
	lc.stale.suspend()
	defer func() {
		lc.stale.resume()
		if lc.hooks.pumpReleased != nil {
			lc.hooks.pumpReleased()
		}
	}()
	if lc.hooks.pumpBlocked != nil {
		lc.hooks.pumpBlocked()
	}
	select {
	case lc.frames <- item:
		lc.handedOff(item)
		return true
	case <-ctx.Done():
		return false
	}
}

func (lc *liveConn) handedOff(item pumpItem) {
	if lc.hooks.pumpHandedOff != nil {
		lc.hooks.pumpHandedOff(item.err != nil)
	}
}

// dispose tears the live connection down: closes the socket, cancels the
// attempt (its in-flight seam calls and the pump — same context), joins the
// pump, and stops the staleness timer.
//
// The CLOSE COMES FIRST, and the order is load-bearing rather than
// stylistic. §23 requires the connector to close the still-open socket
// explicitly — the rejected subscription Action Cable leaves open, and every
// terminal — and a close is only observable to the peer as a close frame.
// Cancellation is allowed to kill the connection outright (the seam contract
// says a cancelled read returns promptly, and the default transport's library
// aborts the socket to do it), so cancelling first races the close handshake
// and the peer sees an abrupt teardown instead. Close is documented to unblock
// ReadFrame, so the pump still exits; the cancel that follows is what returns
// any in-flight seam call promptly.
func (lc *liveConn) dispose(cancel context.CancelFunc) {
	_ = lc.conn.Close(closeCodeNormal, "")
	cancel()
	for range lc.frames { //nolint:revive // draining to the pump's close is the join
	}
	lc.stale.stop()
}

// staleHolder owns the per-socket staleness timer: armed at socket open,
// re-armed by the pump on every inbound frame of any kind. Because the pump
// swaps timers concurrently with the state machine's select, firings carry a
// generation: a firing whose generation is no longer current was superseded
// by a frame the pump received first, and is disregarded.
//
// Suspension while the pump is blocked on a full hand-off queue is realized
// AT EVALUATION, not at arming (SPEC.md §23 "Cable Protocol Details"): the
// timer stays armed throughout — `staleness` remains in every socket-open
// state's exact timer set — and a firing whose window overlapped a
// pump-blocked interval is disregarded and re-armed rather than dispatched. A
// firing whose window the pump spent reading is authoritative.
type staleHolder struct {
	mu      sync.Mutex
	clock   Clock
	d       time.Duration
	timer   Timer
	gen     int
	last    time.Time // last frame receipt (or arm); Now readings, deltas only
	stopped bool
	// blocked counts hand-offs currently blocked on the full queue, and
	// suspended latches whether the ARMED window has overlapped one: a firing
	// is evidence of a dead peer only if the pump spent the whole window able
	// to observe frames.
	blocked   int
	suspended bool
	// expired latches an AUTHORITATIVE firing that the state machine has not
	// observed yet — a window that closed while the consumer was inside a
	// delivery or a callback, with the pump reading throughout. Once latched
	// nothing re-arms over it: the fired timer stays put so the next select
	// the state machine runs still picks the firing up, and evaluate reports
	// it whatever else has happened since.
	expired bool
	// rearm wakes the state machine when the window is swapped from the pump
	// goroutine: a select pass holds ONE timer, so a swap is invisible to a
	// parked select until something else wakes it.
	rearm chan struct{}
}

func newStaleHolder(clock Clock, d time.Duration) *staleHolder {
	// The window's origin is read BEFORE the timer is armed, here and in arm.
	// Arming publishes the timer — a virtual clock's advance, or a real
	// clock's scheduler, can run between the two — and a `last` taken after
	// that lands INSIDE the window, understating the silence the expiry
	// reports (0s for a full expiry, once the advance is the one that fires
	// it). Taken before, it can only sit at or fractionally ahead of the
	// window's start, so the reported age is never short.
	last := clock.Now()
	return &staleHolder{
		clock: clock,
		d:     d,
		timer: clock.NewTimer(d, timerStaleness),
		last:  last,
		rearm: make(chan struct{}, 1),
	}
}

// rearmed is the wake channel for window swaps.
func (h *staleHolder) rearmed() <-chan struct{} { return h.rearm }

// current returns the armed timer and its generation for one select pass.
func (h *staleHolder) current() (Timer, int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.timer, h.gen
}

// reset re-arms the timer — pump-side, per inbound frame. The frame's receipt
// instant becomes the new window's origin only if the window was actually
// re-armed; a frame that arrived after the deadline had already fired does
// not move it (see arm).
func (h *staleHolder) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopped {
		return
	}
	// Read before arming, for newStaleHolder's reason.
	now := h.clock.Now()
	if h.arm() {
		h.last = now
	}
}

// arm replaces the armed timer with a fresh window, reporting whether it did.
// Callers hold h.mu.
//
// Stop's result is the ORDERING DISCRIMINATOR, not a formality: it reports
// whether the timer was still pending, so a false return means the deadline
// had already fired before this call. The generation guard's own premise is
// that a superseding frame is one "the pump received first" — but a bare
// gen++ supersedes unconditionally, including for a frame that arrived AFTER
// the window closed. SPEC.md §23 pins that ordering the other way: the reset
// happens pump-side at frame receipt precisely "so frame-vs-deadline ordering
// is well-defined at the transport boundary regardless of queue depth or
// consumer latency: a fired staleness deadline observed on return from a slow
// delivery is authoritative". Re-arming over such a firing erases it — and
// the state machine, which may be inside a slow delivery or an Observer
// callback and holding no timer at all, would never see the expired window,
// nor the timer that carried it once it was swapped out.
//
// So an already-fired window is LATCHED rather than superseded, and nothing
// re-arms afterwards. The one exemption is a window that overlapped a blocked
// hand-off: a full queue is a fast peer, not a dead one, so that firing is
// not evidence and re-arms as before.
func (h *staleHolder) arm() bool {
	if h.expired {
		return false
	}
	if !h.timer.Stop() && !h.suspended {
		h.expired = true
		return false
	}
	h.timer = h.clock.NewTimer(h.d, timerStaleness)
	h.gen++
	select {
	case h.rearm <- struct{}{}:
	default:
	}
	// The new window starts suspended only if the pump is blocked right now:
	// it is already unable to observe a reset.
	h.suspended = h.blocked > 0
	return true
}

// suspend marks the armed window as overlapping a blocked hand-off. A full
// queue means the peer is sending faster than the connector consumes — the
// opposite of a dead peer — and a pump that is not reading cannot observe the
// resets that would prove liveness.
func (h *staleHolder) suspend() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.blocked++
	h.suspended = true
}

// resume releases one blocked hand-off. When the last one clears, the window
// is re-armed fresh: the hand-off completing is itself proof the peer was
// sending, and the only window that can testify to a dead peer is one the pump
// spent reading from end to end.
func (h *staleHolder) resume() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.blocked--
	if h.blocked == 0 && !h.stopped {
		// suspended is true here by construction (suspend set it, and only arm
		// clears it — which cannot have run with blocked > 0), so this never
		// latches: a firing over a blocked window is not evidence. It is still
		// a no-op if an expiry was latched BEFORE the block, which is right —
		// that window closed while the pump was reading.
		h.arm()
	}
}

// evaluate decides one observed firing, returning the age of the silence when
// the firing is authoritative. It is disregarded when a frame the pump
// received first already superseded it (a stale generation), and when its
// window overlapped a pump-blocked interval — in which case the window is
// re-armed here, so the state's exact timer set is unchanged.
//
// A LATCHED expiry outranks both tests: it was already decided authoritative
// at the moment the deadline beat the frame that followed it (arm), and by
// then the window is over — a hand-off that blocks afterwards says nothing
// about it, and no re-arm can have moved the generation, since the latch
// stops them.
func (h *staleHolder) evaluate(gen int) (time.Duration, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopped {
		return 0, false
	}
	if h.expired {
		return h.clock.Now().Sub(h.last), true
	}
	if gen != h.gen {
		return 0, false
	}
	if h.suspended {
		h.arm()
		return 0, false
	}
	return h.clock.Now().Sub(h.last), true
}

// stop cancels the timer permanently.
func (h *staleHolder) stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stopped = true
	h.timer.Stop()
}

// liveBuffer is the state-machine-owned live-event buffer (SPEC.md §23
// "Semantic Signals"): only event-bearing frames are admitted, so it is
// denominated in events and every dropped entry has an id.
type liveBuffer struct {
	capacity int
	events   []Event
	// onChange reports the buffer's occupancy after every change. Test-only
	// (testHooks.bufferOccupancy); nil in production.
	onChange func(int)
}

func newLiveBuffer(capacity int, onChange func(int)) *liveBuffer {
	return &liveBuffer{capacity: capacity, onChange: onChange}
}

// add admits ev, returning the ids of any events dropped to make room —
// oldest first.
func (b *liveBuffer) add(ev Event) []int64 {
	var dropped []int64
	for len(b.events) >= b.capacity {
		dropped = append(dropped, b.events[0].ID)
		// Zeroed before the reslice, for shift's reason: the reslice removes
		// the evicted event logically, but the slice that results still
		// points into the same backing array, whose prefix keeps that
		// event's strings reachable until a later reallocation. Under
		// sustained overflow that is a second buffer's worth of payload held
		// beyond the ceiling §23 publishes — retained by events that no
		// longer count toward occupancy.
		b.events[0] = Event{}
		b.events = b.events[1:]
	}
	b.events = append(b.events, ev)
	b.changed()
	return dropped
}

// shift removes and returns the oldest buffered event, reporting false when
// the buffer is empty. It is the drain's ONLY read, one event at a time: an
// event leaves the buffer exactly when it is about to be delivered, so
// occupancy always accounts for everything still pending and the capacity
// stays a bound on events held at once.
//
// Taking the whole buffer in one batch instead — the shape this replaced —
// let a drain hold `capacity` events outside the buffer while the buffer
// refilled to `capacity` behind them, so a slow drain retained twice the
// configured number of events and twice SPEC.md §23's published memory
// ceiling. Counting that batch against admission instead of removing it would
// need the batch to be evictable too (an overflow drops the OLDEST, which
// during a drain are precisely the taken ones), which is this method with a
// second container in front of it.
func (b *liveBuffer) shift() (Event, bool) {
	if len(b.events) == 0 {
		return Event{}, false
	}
	ev := b.events[0]
	// Re-slicing alone would pin the whole backing array through the
	// drain; zeroing the vacated slot lets it go.
	b.events[0] = Event{}
	b.events = b.events[1:]
	b.changed()
	return ev, true
}

func (b *liveBuffer) changed() {
	if b.onChange != nil {
		b.onChange(len(b.events))
	}
}

// catchUpHandoff is the typed boundary between the subscription lifecycle
// (this file) and the catch-up walk (SPEC.md §23 transitions 16–26): it
// carries everything a freshly confirmed subscription owns. The walk consumes
// it in catchup.go through loop.catchUp; the lifecycle side does not change
// shape for what the walk does with it.
type catchUpHandoff struct {
	// at is the live attempt: its cancellation scope, the open socket, the
	// running frame pump, and the armed staleness holder.
	at *attempt
	// entry is the selected entry cursor for the catch-up walk.
	entry Cursor
	// presentClass reports whether entry resolves at the server's present
	// head — the Entry Boundary's hold-then-save discipline applies — rather
	// than in served history (per-page saves).
	presentClass bool
	// buffer holds live events admitted before confirmation; its pre-cut
	// contents become the entry snapshot.
	buffer *liveBuffer
}

// cycleOutcome is what one connect cycle (or the catch-up continuation)
// reports back to the outer run loop.
type cycleOutcome struct {
	kind outcomeKind
	// retryAfter floors the next Backoff delay (server-directed; cap-exempt).
	retryAfter time.Duration
	// term is the terminal error element (outcomeTerminal only).
	term *TerminalError
}

type outcomeKind int

const (
	// outcomeFailed — a continuable failure: the attempt is already disposed
	// and the loop enters Backoff (transitions 4/7/9/14/15).
	outcomeFailed outcomeKind = iota + 1
	// outcomeTerminal — the feed ends with exactly one typed error element.
	outcomeTerminal
	// outcomeClosed — close()/cancellation: the universal edge to Closed, no
	// error element.
	outcomeClosed
)

// loop is one Events consumption's state machine. It runs entirely on the
// consumer's goroutine.
type loop struct {
	cfg    *config
	hooks  testHooks
	runCtx context.Context
	yield  func(Event, error) bool

	state connState
	// failedCycles is the reconnect-cycle failure count n — incremented per
	// failed cycle, reset on confirmation (transition 11 resets only this).
	failedCycles int
	// authFailures is the shared connection-level authorization counter:
	// unauthorized mints, `unauthorized` disconnects, and unauthorized polls
	// increment it; it resets ONLY on a successful poll page (catchup.go) —
	// never on confirmation.
	authFailures int

	// pollFailures is the consecutive-poll-failure index k behind the
	// `poll-retry` timer — separate from failedCycles, reset by any
	// successful poll page and by socket teardown.
	pollFailures int
	// position is the connector's in-memory position: authoritative for
	// resume and repair within the run, seeded by the checkpoint load under
	// the resume entry mode and advanced only by accepted poll pages. The
	// store is write-through durability; this field is never re-read from it
	// mid-run.
	position string
	// reentry latches the cursor a re-entry selected (transitions 17/18/19)
	// as RECONNECT state, held until a page replaces it. A re-entry's cursor
	// is the connector's whole answer to a position the server refused, went
	// gone on, or whose lineage a filter change ended — and it lives only in
	// the walk that chose it. Without the latch, a socket torn down before
	// the re-entry's first page lands drops back to a position the server
	// already rejected, or to the configured start mode, which for a
	// poll-served reset cursor means silently jumping to the present.
	reentry *reentryCursor
	// lastPollServedID is the highest event id the POLL lane has served on
	// this run — the reset cursor transitions 18/19 re-enter at, tracked
	// independently of delivery, dedupe, and the live lane (a live-delivered
	// id is never a reset cursor). Empty pages advance nothing; with no
	// poll-served id the re-entry is present-class.
	lastPollServedID int64
	// stopped latches a consumer break (a false yield): nothing is ever
	// yielded after it.
	stopped bool

	identifier     string
	subscribeFrame []byte
	buffer         *liveBuffer
	dedupe         *dedupe

	// deferred is the one pump receive the in-flight-poll servicing took out
	// of band and left for the walk's ordinary dispatch point (catchup.go's
	// pollPage). It belongs to the attempt that produced it: disposal clears
	// it, so a dead attempt's last frame can never leak into the next one.
	deferred *deferredFrame

	// catchUp is the post-confirmation continuation: the catch-up walk, the
	// entry boundary, the drain, and streaming (catchup.go).
	catchUp func(catchUpHandoff) cycleOutcome
}

func newLoop(runCtx context.Context, cfg *config, hooks testHooks) *loop {
	l := &loop{
		cfg:    cfg,
		hooks:  hooks,
		runCtx: runCtx,
		state:  stateIdle,
		buffer: newLiveBuffer(cfg.liveBufferCapacity, hooks.bufferOccupancy),
		dedupe: newDedupe(cfg.dedupeCapacity),
	}
	// Built once; identical bytes on every (re)connection and retransmit.
	l.identifier = subscribeIdentifier(cfg.filters)
	l.subscribeFrame = subscribeCommand(l.identifier)
	l.catchUp = l.runCatchUp
	return l
}

func (l *loop) setState(s connState) {
	l.state = s
	if l.hooks.stateChanged != nil {
		l.hooks.stateChanged(s)
	}
}

// run executes the state machine. The checkpoint load comes first — exactly
// once, before the first mint, so its failure is terminal with zero wire
// attempts. Transition 1: the first cycle is then immediate — no backoff.
// Terminal outcomes yield exactly one error element; the universal Closed
// edge yields none.
func (l *loop) run(yield func(Event, error) bool) {
	l.yield = yield
	if l.runCtx.Err() != nil {
		l.setState(stateClosed)
		return
	}
	if terr := l.loadCheckpoint(); terr != nil {
		l.emitTerminal(terr)
		return
	}
	var delay time.Duration
	for {
		out := l.runCycle(delay)
		switch out.kind {
		case outcomeClosed:
			l.setState(stateClosed)
			return
		case outcomeTerminal:
			l.emitTerminal(out.term)
			return
		case outcomeFailed:
			// The failed attempt is fully disposed, so the exact
			// outstanding-timer set on entry to Backoff is {backoff}.
			l.failedCycles++
			l.setState(stateBackoff)
			d := reconnectDelay(out.retryAfter, l.failedCycles, l.cfg.rand)
			t := l.cfg.clock.NewTimer(d, timerBackoff)
			select {
			case <-t.C():
				// Transition 2: Backoff → Minting; a fresh ticket is ALWAYS
				// minted next.
				delay = d
			case <-l.runCtx.Done():
				t.Stop()
				l.setState(stateClosed)
				return
			}
		}
	}
}

// runCycle runs one pass Minting → Connecting → AwaitingWelcome →
// AwaitingConfirmation, continuing into the catch-up handoff on
// confirmation. Every exit path has disposed the attempt (socket closed,
// pump joined, timers stopped) before returning — except the confirmed
// handoff, which transfers ownership to the catch-up continuation.
// emitTerminal is the ONE place a terminal element reaches the consumer, and
// it defers to Close.
//
// §23 makes close() a universal edge from every non-absorbing state and ends
// the iterator with NO error element, so a terminal must never outrank a Close
// the consumer has already taken. The race is real and is not hypothetical
// scheduling paranoia: the frame pump runs from socket open, so by the time a
// consumer callback calls Close there can already be a fatal frame queued, and
// the state machine's next select has two ready cases with no ordering between
// them. Go may take the frame.
//
// Checking here rather than at each select is deliberate. There are many
// selects and one exit, so the invariant belongs at the exit — a per-select
// check is a rule every future select has to remember, which is the shape that
// produced this defect in the first place.
func (l *loop) emitTerminal(term *TerminalError) {
	if l.runCtx.Err() != nil {
		l.setState(stateClosed)
		return
	}
	l.setState(stateTerminal)
	l.yield(Event{}, term)
}

func (l *loop) runCycle(delay time.Duration) cycleOutcome {
	at := &attempt{}
	at.ctx, at.cancel = context.WithCancel(l.runCtx)
	defer at.cancel()

	// Transitions 1/2 → Minting. No timers are armed here (per-state
	// invariant: Minting's set is {}); mint cancellation rides the attempt
	// context.
	l.setState(stateMinting)
	if l.cfg.observer.Connecting != nil {
		l.cfg.observer.Connecting(l.failedCycles+1, delay)
	}
	// The observer runs on this goroutine and may Close from inside it, which
	// cancels synchronously (Connector.Close). Re-checking here is what turns
	// that cancellation into the universal Closed edge BEFORE the run's first
	// wire act, rather than one mint after it.
	if l.runCtx.Err() != nil {
		return cycleOutcome{kind: outcomeClosed}
	}
	ticket, err := l.cfg.minter.MintStreamTicket(at.ctx)
	if l.runCtx.Err() != nil {
		return cycleOutcome{kind: outcomeClosed}
	}
	if err != nil {
		return l.classifyMintFailure(err)
	}

	// Transition 3 → Connecting: dial the mint's url verbatim. The cable-URL
	// policy pre-check runs before anything is armed — a policy violation
	// recurs on every re-mint, so it is Terminal(invalid_cable_url), never
	// Backoff. The error never carries the URL: the ticket rides in its
	// query string.
	l.setState(stateConnecting)
	if derr := checkCableURL(ticket.URL); derr != nil {
		return cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
			Reason: ReasonInvalidCableURL, Msg: derr.Reason, Err: derr,
		}}
	}
	// The handshake deadline arms on entry to Connecting, BEFORE dial — it
	// spans dial-to-welcome, so a stalled dial expires it (transition 7).
	hs := l.cfg.clock.NewTimer(handshakeDeadline, timerHandshakeDeadline)
	type dialResult struct {
		conn CableConn
		err  error
	}
	dialCh := make(chan dialResult, 1)
	go func() {
		conn, dialErr := l.cfg.transport.Dial(at.ctx, ticket.URL, maxFrameBytes)
		dialCh <- dialResult{conn: conn, err: dialErr}
	}()
	var conn CableConn
	select {
	case r := <-dialCh:
		if l.runCtx.Err() != nil {
			hs.Stop()
			if r.conn != nil {
				_ = r.conn.Close(closeCodeNormal, "")
			}
			return cycleOutcome{kind: outcomeClosed}
		}
		if r.err != nil {
			hs.Stop()
			var derr *DialError
			if errors.As(r.err, &derr) && derr.Kind == DialPolicy {
				return cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
					Reason: ReasonInvalidCableURL, Msg: derr.Reason, Err: derr,
				}}
			}
			return cycleOutcome{kind: outcomeFailed} // transition 7
		}
		conn = r.conn
	case <-hs.C():
		// Transition 7: the deadline expired mid-dial — cancel the pending
		// dial (the seam contract requires a prompt return) and dispose any
		// connection it raced to open.
		at.cancel()
		if r := <-dialCh; r.conn != nil {
			_ = r.conn.Close(closeCodeNormal, "")
		}
		return cycleOutcome{kind: outcomeFailed}
	case <-l.runCtx.Done():
		at.cancel()
		if r := <-dialCh; r.conn != nil {
			_ = r.conn.Close(closeCodeNormal, "")
		}
		hs.Stop()
		return cycleOutcome{kind: outcomeClosed}
	}

	// Transition 6 → AwaitingWelcome: staleness arms at socket open, the
	// pump starts, and the handshake deadline keeps running to `welcome`.
	//
	// The arming comes FIRST, before Observer.Connected, and the order is
	// load-bearing rather than stylistic (#763). Observer callbacks are host
	// code running on the consumer's goroutine — a log write, a metrics
	// emission, an error-tracker breadcrumb — and this one sat between the
	// socket opening and the window that measures silence on it. Whatever it
	// spent became time the window did not count: newStaleHolder reads its
	// origin at construction, so the window began when the callback returned,
	// not when the socket opened, and a peer that went silent immediately was
	// given that much extra grace before staleness could fire. §23 says the
	// window arms at socket open, and this line is what that means.
	//
	// Starting the pump ahead of the callback is the same trade and harmless:
	// the pump only fills a bounded channel that this goroutine drains later,
	// so nothing is observed out of order — Connected is observability, not a
	// gate.
	at.lc = newLiveConn(at.ctx, conn, l.cfg.clock, l.cfg.staleAfter, l.hooks)
	if l.cfg.observer.Connected != nil {
		l.cfg.observer.Connected()
	}
	return l.awaitConfirmation(at, hs)
}

// classifyMintFailure maps a failed mint seam call onto transitions 4/5 or
// the out-of-inventory mint_failed edge. An error the adapter did not
// classify (not a *MintError) is treated as unrecoverable: the seam contract
// maps every generated outcome onto exactly one kind, and retrying an
// unclassifiable failure risks a tight loop on a permanent condition.
func (l *loop) classifyMintFailure(err error) cycleOutcome {
	var me *MintError
	if !errors.As(err, &me) {
		return cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
			Reason: ReasonMintFailed, Msg: "unclassified mint failure", Err: err,
		}}
	}
	switch me.Kind {
	case MintTransient:
		return cycleOutcome{kind: outcomeFailed} // transition 4
	case MintThrottled:
		// Transition 4; Retry-After floors the next Backoff delay.
		return cycleOutcome{kind: outcomeFailed, retryAfter: me.RetryAfter}
	case MintUnauthorized:
		// Transition 4 below the threshold; transition 5 — the shared
		// connection-level counter's terminal — at the 3rd consecutive.
		l.authFailures++
		if l.authFailures >= authFailureThreshold {
			return cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
				Reason: ReasonAuthorizationFailed,
				Msg:    fmt.Sprintf("%d consecutive connection-level authorization failures", l.authFailures),
				Err:    err,
			}}
		}
		return cycleOutcome{kind: outcomeFailed, retryAfter: me.RetryAfter}
	case MintUnrecoverable:
		return cycleOutcome{kind: outcomeTerminal, term: &TerminalError{Reason: ReasonMintFailed, Err: err}}
	default:
		return cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
			Reason: ReasonMintFailed, Msg: fmt.Sprintf("unknown mint error kind %d", int(me.Kind)), Err: err,
		}}
	}
}

// awaitConfirmation drives AwaitingWelcome and AwaitingConfirmation
// (transitions 8–15 plus the state-generic protocol-fatal and invalid-frame
// dispatch). deadline is the running handshake-deadline on entry; `welcome`
// re-arms it as the confirmation-deadline (transition 8).
func (l *loop) awaitConfirmation(at *attempt, deadline Timer) cycleOutcome {
	l.setState(stateAwaitingWelcome)
	for {
		staleTimer, staleGen := at.lc.stale.current()
		select {
		case <-l.runCtx.Done():
			l.disposeAttempt(at, deadline)
			return cycleOutcome{kind: outcomeClosed}
		case <-deadline.C():
			// Transition 9 (handshake lapse) or 14 (confirmation lapse):
			// full teardown — conn, pump, and ALL the attempt's timers —
			// then a jittered fresh-ticket retry.
			lapsed := errDeadlineLapsed(l.state)
			l.disposeAttempt(at, nil)
			l.observeDisconnected("", lapsed)
			return cycleOutcome{kind: outcomeFailed}
		case <-at.lc.stale.rearmed():
			continue
		case <-staleTimer.C():
			age, ok := at.lc.stale.evaluate(staleGen)
			if !ok {
				// Superseded by a frame the pump received first, or suspended
				// by a blocked hand-off and re-armed.
				continue
			}
			// Staleness expiry — rows 9/15's staleness trigger.
			l.disposeAttempt(at, deadline)
			if l.cfg.observer.StaleConnection != nil {
				l.cfg.observer.StaleConnection(age)
			}
			l.observeDisconnected("", errStaleConnection)
			return cycleOutcome{kind: outcomeFailed}
		case item, ok := <-at.lc.frames:
			if !ok {
				// The pump exited without a terminating error — only
				// possible under a cancellation race; treat as a socket
				// failure.
				l.disposeAttempt(at, deadline)
				l.observeDisconnected("", errors.New("event feed frame pump exited"))
				return cycleOutcome{kind: outcomeFailed}
			}
			if out, done := l.handleFrame(at, &deadline, item); done {
				return out
			}
		}
	}
}

// writeSubscribe sends the subscribe command, BOUNDED by the phase deadline
// that is running when it is sent.
//
// This is the one place the state machine hands control to the transport
// synchronously while a deadline governs the phase, and left synchronous it
// defeats that deadline outright. `CableConn.WriteFrame` may block — a peer
// whose receive window has closed, or any seam implementation that returns
// only when its context is cancelled — and the context in question is
// `at.ctx`, which nothing cancels but a teardown this very write is blocking.
// The state machine cannot return to its select, so neither the handshake nor
// the confirmation deadline can ever be observed, and the feed hangs after
// `welcome` until the consumer closes the connector.
//
// Running the write on its own goroutine and selecting the deadline against
// it restores the bound: a lapse takes transition 9/14 exactly as it would
// have had the write never blocked, and disposal's cancel is what returns the
// abandoned write (the seam contract requires a cancelled write to return
// promptly, and Close is documented to unblock it too). Staleness is
// deliberately not in this select — the phase deadline is the tighter bound
// on a handshake that is going nowhere, and adding a second timer here would
// duplicate the generation dance for no additional guarantee.
func (l *loop) writeSubscribe(at *attempt, deadline *Timer) (cycleOutcome, bool) {
	written := make(chan error, 1)
	go func() { written <- at.lc.conn.WriteFrame(at.ctx, l.subscribeFrame) }()
	select {
	case werr := <-written:
		if werr != nil {
			l.disposeAttempt(at, *deadline)
			l.observeDisconnected("", werr)
			return cycleOutcome{kind: outcomeFailed}, true
		}
		if l.hooks.subscribeWritten != nil {
			l.hooks.subscribeWritten()
		}
		return cycleOutcome{}, false
	case <-l.runCtx.Done():
		l.disposeAttempt(at, *deadline)
		return cycleOutcome{kind: outcomeClosed}, true
	case <-(*deadline).C():
		// The deadline is spent, so disposal is handed nil: the attempt's
		// timer set is empty from here, as on every other lapse.
		lapsed := errDeadlineLapsed(l.state)
		l.disposeAttempt(at, nil)
		l.observeDisconnected("", lapsed)
		return cycleOutcome{kind: outcomeFailed}, true
	}
}

// errDeadlineLapsed names the deadline that lapsed in the given state.
func errDeadlineLapsed(s connState) error {
	if s == stateAwaitingWelcome {
		return errors.New("event feed handshake deadline lapsed before welcome")
	}
	return errors.New("event feed confirmation deadline lapsed before confirm_subscription")
}

// handleFrame dispatches one pump item in AwaitingWelcome /
// AwaitingConfirmation. It returns done=true when the cycle is over —
// including the confirmed handoff's own outcome flowing back through it.
func (l *loop) handleFrame(at *attempt, deadline *Timer, item pumpItem) (cycleOutcome, bool) {
	if item.err != nil {
		// Socket failure: peer close, read error, or the transport's
		// frame-size rejection (rows 9/15).
		l.disposeAttempt(at, *deadline)
		l.observeDisconnected("", item.err)
		return cycleOutcome{kind: outcomeFailed}, true
	}
	f, err := parseFrame(item.data)
	if err != nil {
		// Invalid-frame class, parse shape: a peer protocol violation
		// dispatched as a socket failure — never terminal, never a silent
		// skip (SPEC.md §23 "Cable Protocol Details").
		l.disposeAttempt(at, *deadline)
		l.observeDisconnected("", err)
		return cycleOutcome{kind: outcomeFailed}, true
	}
	if l.hooks.frameHandled != nil {
		l.hooks.frameHandled(f.kind)
	}
	switch f.kind {
	case frameWelcome:
		// Transition 8 on the first welcome; a duplicate welcome resends
		// the identical subscribe bytes (stock behavior — the server
		// absorbs identical retransmits) without touching the deadline.
		if out, done := l.writeSubscribe(at, deadline); done {
			return out, true
		}
		if l.state == stateAwaitingWelcome {
			if !(*deadline).Stop() {
				// The handshake deadline had ALREADY FIRED. Stop's result is
				// the ordering discriminator here for the same reason it is
				// in staleHolder.arm: the frame and the expiry were both
				// ready and the select picked one at random. Re-arming over
				// the firing would swap the expired timer out for a fresh
				// confirmation window and accept a welcome that arrived past
				// the deadline — transition 9's lapse, silently converted
				// into a live handshake half the time it happens.
				lapsed := errDeadlineLapsed(l.state)
				l.disposeAttempt(at, nil)
				l.observeDisconnected("", lapsed)
				return cycleOutcome{kind: outcomeFailed}, true
			}
			*deadline = l.cfg.clock.NewTimer(l.cfg.confirmationDeadline, timerConfirmationDeadline)
			l.setState(stateAwaitingConfirmation)
		}
		return cycleOutcome{}, false
	case framePing, frameUnknown:
		// Liveness only — the pump already reset staleness.
		return cycleOutcome{}, false
	case frameConfirm:
		if l.state != stateAwaitingConfirmation || f.identifier != l.identifier {
			// Pre-subscribe, or a foreign identifier: ignored.
			return cycleOutcome{}, false
		}
		// Transition 11: cancel the deadline, reset the attempt counter
		// (the authorization counter resets only on a successful poll
		// page), select the entry cursor, hand off to catch-up.
		if !(*deadline).Stop() {
			// The confirmation deadline had already fired — the same select
			// race the welcome branch above reads Stop for, and the same
			// answer. Proceeding here is worse than proceeding there: the
			// connector would announce Confirmed and enter catch-up on a
			// subscription whose confirmation arrived past transition 14's
			// deadline, so the lapse is not merely swallowed but overwritten
			// by a successful handoff.
			lapsed := errDeadlineLapsed(l.state)
			l.disposeAttempt(at, nil)
			l.observeDisconnected("", lapsed)
			return cycleOutcome{kind: outcomeFailed}, true
		}
		l.failedCycles = 0
		if l.cfg.observer.Confirmed != nil {
			l.cfg.observer.Confirmed()
		}
		entry, present := l.entryCursor()
		return l.catchUp(catchUpHandoff{at: at, entry: entry, presentClass: present, buffer: l.buffer}), true
	case frameReject:
		if l.state != stateAwaitingConfirmation || f.identifier != l.identifier {
			// A foreign identifier, or a rejection that arrived before the
			// connector had sent any subscribe command at all. §23 draws
			// transition 12 only from AwaitingConfirmation, and the confirm
			// branch above already gates on exactly that. Ungated, this is
			// the connector's single most severe verdict — Terminal with
			// ZERO reconnects — produced against zero subscription attempts
			// by one unsolicited frame from a peer that has only completed
			// the WebSocket handshake.
			return cycleOutcome{}, false
		}
		// Transition 12: always terminal — cancel the deadline, explicitly
		// close the still-open socket (Action Cable leaves a rejected
		// socket open), ZERO reconnects.
		l.disposeAttempt(at, *deadline)
		return cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
			Reason: ReasonSubscriptionRejected,
			Msg:    "the server rejected the EventsChannel subscription",
		}}, true
	case frameDisconnect:
		return l.dispatchDisconnect(at, *deadline, f), true
	case frameMessage:
		if f.identifier != l.identifier {
			return cycleOutcome{}, false
		}
		ev, derr := decodeMessageEvent(f.message)
		if derr != nil {
			// Invalid-frame class, decode shape: same socket-failure
			// disposition as the parse shape.
			l.disposeAttempt(at, *deadline)
			l.observeDisconnected("", derr)
			return cycleOutcome{kind: outcomeFailed}, true
		}
		return l.admitLive(at, *deadline, ev)
	}
	return cycleOutcome{}, false
}

// dispatchDisconnect applies §23's reason-string dispatch — never the
// reconnect flag alone — to a raw disconnect text frame read pre-confirm.
// The attempt is disposed first either way: a disconnect frame is the
// server's last word on this socket.
func (l *loop) dispatchDisconnect(at *attempt, deadline Timer, f frame) cycleOutcome {
	preWelcome := l.state == stateAwaitingWelcome
	l.disposeAttempt(at, deadline)
	l.observeDisconnected(observableDisconnectReason(f.reason), nil)
	switch f.reason {
	case disconnectReasonProtocolFatal:
		// Transition 13 — and the state-generic rule for AwaitingWelcome:
		// the server's own protocol verdict is terminal from every
		// socket-open state, never retried into.
		return cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
			Reason: ReasonProtocolFatal,
			Msg:    "server disconnected: " + f.reason,
		}}
	case disconnectReasonUnauthorized:
		if preWelcome {
			// Rows 9/10: `unauthorized` arrives only pre-welcome at the
			// verified head — a fresh-ticket retry below the shared
			// counter's threshold, terminal at the 3rd consecutive.
			l.authFailures++
			if l.authFailures >= authFailureThreshold {
				return cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
					Reason: ReasonAuthorizationFailed,
					Msg:    fmt.Sprintf("%d consecutive connection-level authorization failures", l.authFailures),
				}}
			}
			return cycleOutcome{kind: outcomeFailed}
		}
		// Wire-impossible post-welcome: a socket drop (row 15) with NO
		// counter increment — the reconnect cycle re-mints, and a genuinely
		// revoked user's mint then fails and increments the counter.
		return cycleOutcome{kind: outcomeFailed}
	default:
		// `remote`, unrecognized reasons, and any reconnect flag: a socket
		// drop (rows 9/15). Unknown reasons are never guessed into a
		// terminal class and never increment the authorization counter.
		return cycleOutcome{kind: outcomeFailed}
	}
}

// admitLive admits a live event to the buffer — the live buffer is the only
// carrier of an in-flight-at-entry straggler — and dispatches the
// BufferOverflow semantic signal at drop time, the first consumer-context
// opportunity (SPEC.md §23 "Semantic Signals").
//
// It is the single place a drop is dispatched from, on whichever path observed
// it: the pre-confirmation and Streaming dispatches, the bounded admission
// passes, the drain's scan, and the servicing of frames arriving while a poll
// seam call is in flight. Every one of them runs on the consumer's goroutine,
// which is what makes drop time an opportunity to dispatch on rather than
// something to park until a later cut — a cut a stalled or failing call may
// never reach.
func (l *loop) admitLive(at *attempt, deadline Timer, ev Event) (cycleOutcome, bool) {
	dropped := l.buffer.add(ev)
	if len(dropped) == 0 {
		return cycleOutcome{}, false
	}
	signal := BufferOverflow{DroppedIDs: dropped, DroppedCount: len(dropped)}
	if l.hooks.signalRaised != nil {
		l.hooks.signalRaised(signal)
	}
	if l.cfg.observer.BufferOverflow != nil {
		l.cfg.observer.BufferOverflow(len(dropped))
	}
	if l.cfg.handler != nil && l.cfg.handler(signal) == Accept {
		// Accept: the consumer owns the acknowledged incompleteness; the
		// feed continues (acceptance is not license to skip retained
		// deliveries — the catch-up slice's save-ordering invariant).
		return cycleOutcome{}, false
	}
	// No handler — or Terminate: Terminal(buffer_overflow), no save.
	l.disposeAttempt(at, deadline)
	return cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
		Reason: ReasonBufferOverflow,
		Msg:    fmt.Sprintf("live buffer overflow: %d event(s) dropped", len(dropped)),
	}}, true
}

// reentryCursor is a latched re-entry: the cursor transitions 17/18/19
// selected and the entry class it carries.
type reentryCursor struct {
	cursor       Cursor
	presentClass bool
}

// acceptPosition records a position an accepted poll page moved the connector
// to. It is also what releases a latched re-entry — "until a page replaces
// it" is exactly this assignment, so a present-class re-entry stays latched
// through its whole walk and drain (nothing durable has moved until the held
// save) while a position-resume one is released by its first saved page.
//
// position is non-empty at both call sites, and must stay that way: the walk
// refuses a page that carries no position outright, and the held save is
// guarded on `held != ""`. An empty one assigned here does not preserve the
// old cursor — entryCursor selects on `l.position != ""`, so it falls through
// to a bare present entry and silently skips history. (`l.position = ""` IS
// legitimate, but only from reenterAtResetCursor, which means it deliberately:
// the server refused the cursor and there is nothing to resume from.)
func (l *loop) acceptPosition(position string) {
	l.position = position
	l.reentry = nil
}

// entryCursor selects the catch-up entry cursor and whether it is
// present-class (SPEC.md §23 "Entry Boundary"). Three sources, in priority
// order, and the distinction between them is the point:
//
//   - A LATCHED re-entry wins outright. It is the connector's live answer to
//     a 410/400-position/409, still unreplaced by a page, and every other
//     source is a cursor the server has already refused or superseded.
//   - The IN-MEMORY position — accepted from a page earlier in this run, or
//     seeded by the checkpoint load under the resume mode — comes next: it is
//     authoritative within a run, so a reconnect resumes where the feed
//     actually got to rather than re-entering at the mode's original cursor.
//   - Otherwise the configured Start mode, which is the ONLY source for the
//     explicit modes on a fresh run. `StartPresent`, `StartBeginning` and
//     `StartAfter` promise `since=now`, `since=0` and `since=<id>`; only
//     `StartResume` is defined as "the stored position if any", which is why
//     the load seeds the in-memory position for that mode alone (see
//     loadCheckpoint) rather than here.
func (l *loop) entryCursor() (Cursor, bool) {
	if l.reentry != nil {
		return l.reentry.cursor, l.reentry.presentClass
	}
	if l.position != "" {
		return Cursor{Position: l.position}, false
	}
	switch l.cfg.start.kind {
	case startPresent:
		return Cursor{Since: sincePresent}, true
	case startBeginning:
		return Cursor{Since: "0"}, false
	case startAfter:
		return Cursor{Since: strconv.FormatInt(l.cfg.start.eventID, 10)}, false
	case startAtPosition:
		return Cursor{Position: l.cfg.start.position}, false
	default: // startResume
		return Cursor{}, true
	}
}

// disposeAttempt tears one attempt down: stops the phase deadline (nil when
// it already fired), cancels the attempt context — which also cancels any
// in-flight seam call or write belonging to it — closes the connection,
// joins the pump, and stops the staleness timer. The next state is entered
// with the attempt's timer set empty.
func (l *loop) disposeAttempt(at *attempt, deadline Timer) {
	// The deferred frame belongs to the attempt that produced it: a disposed
	// attempt's last out-of-band receive is gone with its socket.
	l.deferred = nil
	if deadline != nil {
		deadline.Stop()
	}
	if at.lc != nil {
		at.lc.dispose(at.cancel)
	} else {
		at.cancel()
	}
}

// observeDisconnected reports one socket teardown. Both arguments are reduced
// to closed vocabularies first: reason by observableDisconnectReason, err by
// observableSocketError.
func (l *loop) observeDisconnected(reason string, err error) {
	if l.cfg.observer.Disconnected != nil {
		l.cfg.observer.Disconnected(truncateErrorText(reason), observableSocketError(err))
	}
}

// errSocketFailed is the generic cause an unrecognized socket failure is
// reported as.
var errSocketFailed = errors.New("event feed socket failed")

// observableSocketError reduces a teardown cause to what may be logged.
//
// The connector's OWN errors pass through: they are sentinels and typed values
// it constructs, so their text is its own. Anything else came out of a seam —
// CableConn.ReadFrame, WriteFrame, or a CableTransport's Dial — and a seam is
// host code wrapping a library. CableTransport is a documented extension point,
// and the seam contract requires those errors not to render the cable URL; this
// is what makes the connector not DEPEND on that requirement being met, because
// Observer.Disconnected is a logging surface and the ticket rides in the URL
// the peer was dialed with.
//
// Recognition is by TYPE and sentinel identity, never by matching text, so no
// arm can be widened by something a peer wrote. An unrecognized cause degrades
// to errSocketFailed: the direction of failure is "less diagnostic", never
// "leaks". The typed values the connector needs to ACT on are matched before
// this is ever called — dispatch reads them directly — so nothing here changes
// a verdict.
func observableSocketError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errStaleConnection),
		errors.Is(err, errCableConnClosed),
		errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err
	}
	var ce *CloseError
	if errors.As(err, &ce) {
		// Flat by construction and renders only its code — an integer cannot
		// carry a credential. See CloseError.Error.
		return ce
	}
	var ife *invalidFrameError
	if errors.As(err, &ife) {
		return ife
	}
	var de *DialError
	if errors.As(err, &de) {
		return de
	}
	var terr *TerminalError
	if errors.As(err, &terr) {
		return terr
	}
	return errSocketFailed
}

// disconnectReasonOther is what an unrecognized disconnect reason is reported
// as. Deliberately not the peer's text, and deliberately not a truncation of
// it.
const disconnectReasonOther = "other"

// observableDisconnectReason maps a raw disconnect reason onto the closed set
// the observer may see.
//
// The raw string is PEER-SUPPLIED and Observer.Disconnected is a logging
// surface, so forwarding it — even bounded — hands the peer a channel into the
// host's logs. §23 declares the ticket an "opaque bearer credential; never
// logged", and a peer that echoes the ticket-bearing URL it was dialed with
// puts it there. Truncation does not help: it bounds the length of a leak, not
// whether there is one. This is the identical trap dialFailure documents three
// review rounds of, and the same answer — a closed vocabulary — applies, for
// the same reason: to strip a credential out of arbitrary text you must MODEL
// it, and "opaque" is precisely the assumption that forbids modelling it.
//
// Nothing is lost that the observer could act on. Dispatch reads the raw reason
// itself, so the two reasons that CHANGE behavior are named exactly; everything
// else is, by construction, a reason this connector does not act on. An
// operator needing the verbatim string reads it from the server's own logs,
// where it did not have to cross a trust boundary to arrive.
func observableDisconnectReason(raw string) string {
	switch raw {
	case disconnectReasonUnauthorized, disconnectReasonProtocolFatal:
		return raw
	case "":
		return ""
	default:
		return disconnectReasonOther
	}
}

// checkpointKey is this run's durable identity — all four parts, always. The
// identity type and the store seam it names are in checkpoint.go; only the
// run-scoped derivation and the two calls through the seam are here.
func (l *loop) checkpointKey() CheckpointKey {
	return CheckpointKey{
		Origin:            l.cfg.origin,
		AccountID:         l.cfg.accountID,
		ConsumerNamespace: l.cfg.consumerNamespace,
		FilterKey:         l.cfg.filters.FilterKey(),
	}
}

// loadCheckpoint runs the store's load exactly once, on the first iteration
// and BEFORE the first mint. Loaded seeds the in-memory position UNDER THE
// RESUME MODE (which is authoritative from then on); Missing proceeds to a
// present-class entry — no stored cursor is not an error; Failed is
// Terminal(checkpoint_load) with zero wire attempts, because collapsing it to
// Missing would silently start at the present and skip history. It runs
// whenever a store is configured, including under an explicit entry mode: the
// lineage's identity is settled before anything durable can move.
//
// Only `StartResume` is defined as "the stored position if any" (SPEC.md §23
// "Consumer Ergonomics"). `StartPresent`, `StartBeginning` and `StartAfter`
// promise `since=now`, `since=0` and `since=<id>`, and a caller pairing one
// with a store means it: seeding the position from the load would make every
// explicit mode behave as resume the moment the store had anything in it, so
// a checkpointed feed could never be deliberately replayed or reset. The load
// still HAPPENS under those modes — its failure edge and its lineage identity
// are not mode-dependent — the value is simply not what the entry is taken
// from. The store is written under the same key either way, so the run's
// first accepted page repoints the lineage.
func (l *loop) loadCheckpoint() *TerminalError {
	if l.cfg.store == nil {
		return nil
	}
	position, ok, err := l.cfg.store.Load(l.runCtx, l.checkpointKey())
	// One cancellation check, covering EVERY load result rather than only the
	// failing one. The load runs on the first iteration and before the first
	// mint, which is exactly the window a prompt Close lands in, and each
	// result misbehaves differently if it is not checked: a failure becomes
	// Terminal(checkpoint_load) — diagnosing the consumer's store for the
	// consumer's own shutdown — a found-but-empty result becomes the same
	// terminal, and a successful one lets the run walk on and fire
	// Observer.Connecting after Close returned. §23 ends a closed iterator with
	// no error element; the run's next check takes the Closed edge.
	//
	// Checked on the CONTEXT, not on the error's shape: a store is under no
	// obligation to wrap ctx.Err(), and one returning its own error type would
	// be misclassified.
	if l.runCtx.Err() != nil {
		//nolint:nilerr // Deliberate: any error here is real, but it is not a
		// STORE failure — it is this run being closed underneath a load that
		// was already in flight.
		return nil
	}
	if err != nil {
		return &TerminalError{
			Reason: ReasonCheckpointLoad,
			Msg:    "the checkpoint store failed to load the stored position",
			Err:    err,
		}
	}
	if ok && position == "" {
		// A store reporting FOUND with an empty position is a store failure,
		// not a missing checkpoint, and the seam is where that has to be
		// enforced: the built-in FileStore already classifies it this way
		// (filestore.go, "an empty position cannot be told apart from having
		// none"), but a custom store is under no obligation to, and nothing
		// downstream can tell the two apart afterwards.
		//
		// Left unenforced it is a SILENT history skip rather than a loud
		// failure. entryCursor selects on `l.position != ""`, so an empty
		// position falls through to the StartResume default — a bare present
		// entry, which is present-class — and the feed resumes at the server's
		// head having skipped everything between the stored position and now,
		// with no signal of any kind. Terminal(checkpoint_load) before any
		// wire attempt is the same edge a load error takes.
		//
		// Checked regardless of start mode, for this function's stated reason:
		// the load happens under every mode, and its failure edge is not
		// mode-dependent — only which value the entry is taken from is.
		return &TerminalError{
			Reason: ReasonCheckpointLoad,
			Msg:    "the checkpoint store reported a stored position that is empty; an empty position cannot be told apart from having none",
		}
	}
	if ok && l.cfg.start.kind == startResume {
		l.position = position
	}
	return nil
}

// saveCheckpoint write-throughs one accepted position. The connector's own
// position tracking is in-memory and authoritative for resume and repair
// within the run; the store is durability only, so a failed save is reported
// through the observer and the feed continues — subsequent saves are still
// attempted, and the live cursor is neither regressed nor blanked.
func (l *loop) saveCheckpoint(position string) {
	if l.cfg.store == nil {
		return
	}
	// The store call runs under the durable gate, so a save either commenced
	// before Close or does not commence at all (durableGate). A refused save is
	// silent by design: Close abandons, and reporting a save that was declined
	// BECAUSE the consumer closed would be reporting the consumer's own
	// decision back to it as a store failure.
	if !l.cfg.durable.begin() {
		return
	}
	err := l.cfg.store.Save(l.runCtx, l.checkpointKey(), position)
	if err != nil {
		if l.cfg.observer.CheckpointSaveFailed != nil {
			l.cfg.observer.CheckpointSaveFailed(err)
		}
		return
	}
	if l.cfg.observer.Checkpoint != nil {
		l.cfg.observer.Checkpoint(position)
	}
}
