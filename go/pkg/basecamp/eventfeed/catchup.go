package eventfeed

import (
	"errors"
	"fmt"
	"time"
)

// The catch-up walk, the entry boundary, the drain, and streaming steady
// state (SPEC.md §23 transitions 16 and 20–26, plus the out-of-inventory
// invalid_continuation / poll_failed edges). A confirmed subscription enters
// here at the catchUpHandoff boundary and stays until the cycle ends; the
// whole walk runs on the consumer's goroutine, so a page's deliveries and its
// save are strictly ordered by construction rather than by a protocol.
//
// The dispositions that hand the walk a NEW cursor — a 410's gated resume, a
// 400-position's and a 409's reset cursor (transitions 17/18/19) — and the
// repair poll a fired `repair-poll` timer drives (transition 24) live in
// recovery.go; both re-enter this file's walk.

// Timer kinds this slice arms (SPEC.md §23 "Clock, Timers, and Virtual
// Time"; the other four arm in the lifecycle slice).
const (
	timerRepairPoll = "repair-poll"
	timerPollRetry  = "poll-retry"
)

// runCatchUp is the post-confirmation continuation: the catch-up walk, the
// drain, and then streaming. It owns the attempt from the hand-off onward —
// every exit path disposes it.
func (l *loop) runCatchUp(h catchUpHandoff) cycleOutcome {
	l.setState(stateCatchingUp)
	if l.hooks.catchUpEntered != nil {
		l.hooks.catchUpEntered(h)
	}
	if l.cfg.observer.CatchUpStarted != nil {
		l.cfg.observer.CatchUpStarted(h.entry)
	}
	// The consecutive-poll-failure index is per walk: a socket teardown ends
	// the previous walk, and from there the reconnect cycle's own counter
	// governs the wait.
	l.pollFailures = 0

	if out, done := l.walkThenDrain(h.at, h.entry, h.presentClass); done {
		return out
	}
	return l.stream(h.at)
}

// walkThenDrain runs one walk to its frozen head, drains the live buffer, and
// saves a held present-class entry position: transitions 16/22/23, shared by
// the post-confirmation catch-up and the repair poll (transition 24), which
// differ only in how they enter and what they return to.
func (l *loop) walkThenDrain(at *attempt, cursor Cursor, presentClass bool) (cycleOutcome, bool) {
	out, held, done := l.walk(at, cursor, presentClass)
	if done {
		return out, true
	}

	// Transition 22 → Draining: the walk reached its frozen head. Draining is
	// a bounded in-memory replay — no polls, no wire waits, no failure edge of
	// its own.
	l.setState(stateDraining)
	if out, done := l.drain(at); done {
		return out, true
	}
	// Transition 23's present-class amendment: the held entry position saves
	// only now — after every retained pre-cut event has been accepted. A
	// non-empty held position is exactly the walk's report that its FINAL
	// entry was present-class, which a mid-walk re-entry can change. The
	// invariant's other conjunct (every pre-cut loss condition explicitly
	// accepted) is structural: an overflow with no handler, or one whose
	// handler returned Terminate, took the Terminal(buffer_overflow) edge at
	// drop time and never reached this line.
	if held != "" {
		l.acceptPosition(held)
		l.saveCheckpoint(held)
	}
	if l.cfg.observer.CaughtUp != nil {
		l.cfg.observer.CaughtUp()
	}
	// The walk's last dispatch point, and by construction a SOCKET outcome
	// only: the overflow class is dispatched at the page that deferred it,
	// and the drain's scan already took the protocol-fatal carve-out. What is
	// left defers here, after the drain and the held save have completed —
	// the same "finish what the page started, then observe the socket"
	// ordering the page boundary applies, and §23's one deliberate
	// deferred-consumption case.
	return l.dispatchDeferred(at)
}

// walk is the catch-up page loop (transition 16): validate any continuation
// URL, poll, deliver the page, then take the page's position — saved per page
// on a position-resume entry, HELD on a present-class one — and follow `next`
// until the walk reaches its frozen head (an absent `next`).
//
// It returns the held present-class position (empty for position-resume
// entries, and empty once a re-entry has superseded a held one), and done=true
// with the cycle's outcome when the walk ended the cycle instead of reaching
// the head.
//
// Two details of the present-class case, both consequences of the invariant
// rather than extra policy: the ownership cut is taken once, right after the
// ENTRY page is accepted, and every later page's position is held too — so a
// walk torn down mid-flight has moved nothing durable and simply re-enters at
// the present, which is exactly what a held position means.
func (l *loop) walk(at *attempt, cursor Cursor, presentClass bool) (out cycleOutcome, held string, done bool) {
	cutTaken := false
	for {
		if l.runCtx.Err() != nil {
			l.disposeAttempt(at, nil)
			return cycleOutcome{kind: outcomeClosed}, "", true
		}
		// A frame the previous page's in-flight servicing deferred is
		// dispatched here, before anything else this iteration does: the page
		// that was in flight has been fully accepted, delivered and saved, so
		// this IS transition 21's page boundary.
		if out, done := l.dispatchDeferred(at); done {
			return out, "", true
		}
		// Continuation validation precedes the seam call, always: the poll
		// carries the caller's bearer, so a cross-origin or downgraded URL
		// must never be requested at all.
		if cursor.PageURL != "" {
			if terr := l.checkContinuation(cursor.PageURL); terr != nil {
				l.disposeAttempt(at, nil)
				return cycleOutcome{kind: outcomeTerminal, term: terr}, "", true
			}
		}
		page, superseded, err := l.pollPage(at, cursor)
		if l.runCtx.Err() != nil {
			l.disposeAttempt(at, nil)
			return cycleOutcome{kind: outcomeClosed}, "", true
		}
		if superseded {
			// The socket's own outcome, deferred while the seam call was in
			// flight, outlived the staleness window with the call still
			// outstanding. It IS the disposition — transition 21 — and its
			// teardown is what cancels the abandoned call.
			if out, done := l.dispatchDeferred(at); done {
				return out, "", true
			}
			// Structurally unreachable: only a socket outcome supersedes a
			// call, and every one of them ends the cycle. Fail closed onto
			// transition 21 rather than walk on over a spoken-for socket.
			l.disposeAttempt(at, nil)
			l.observeDisconnected("", errStaleConnection)
			return cycleOutcome{kind: outcomeFailed}, "", true
		}
		if err != nil {
			step, out, done := l.recoverPoll(at, cursor, err)
			if done {
				return out, "", true
			}
			cursor = step.cursor
			if step.reentry {
				// A re-entry IS an entry (transitions 17/18/19): the previous
				// entry's held position is dropped — nothing durable moved —
				// its class no longer governs, and a present-class re-entry
				// takes its own ownership cut on the new entry page.
				presentClass, held, cutTaken = step.presentClass, "", false
			}
			continue
		}
		// A successful page resets both consecutive-failure counters: the
		// poll-retry index, and the shared connection-level authorization
		// counter — the ONLY thing that resets the latter (a confirmation
		// proves the ticket, not the bearer).
		l.pollFailures = 0
		l.authFailures = 0
		// The page's rows advance the poll-lane reset cursor — served, not
		// delivered: dedupe suppression and a consumer break are both
		// irrelevant to what the poll lane has served.
		for _, ev := range page.Events {
			if ev.ID > l.lastPollServedID {
				l.lastPollServedID = ev.ID
			}
		}

		for _, ev := range page.Events {
			if !l.deliver(ev) {
				l.disposeAttempt(at, nil)
				return cycleOutcome{kind: outcomeClosed}, "", true
			}
		}
		if l.cfg.observer.PageDelivered != nil {
			l.cfg.observer.PageDelivered(len(page.Events), page.Position)
		}
		// The overflow the in-flight servicing deferred is dispatched HERE,
		// ahead of anything durable this page moves.
		if out, done := l.dispatchDeferredOverflow(at); done {
			return out, "", true
		}

		if presentClass {
			// Entry Boundary: the position is held, never saved mid-walk —
			// the entry cursor sits at the server's present head, so an
			// admitted straggler behind it is not poll-repairable and must be
			// delivered before anything durable moves.
			held = page.Position
			if !cutTaken {
				if out, done := l.ownershipCut(at); done {
					return out, "", true
				}
				cutTaken = true
			}
		} else {
			l.acceptPosition(page.Position)
			l.saveCheckpoint(page.Position)
		}

		if page.Next == "" {
			return cycleOutcome{}, held, false
		}
		// Transition 21 from inside the walk: the page boundary is where a
		// socket that died — or went stale — during the previous seam call is
		// observed. Following `next` on a dead socket would walk the whole
		// frozen head before noticing, delaying the reconnect cycle by the
		// length of the walk.
		if out, done := l.socketCheck(at); done {
			return out, "", true
		}
		cursor = Cursor{PageURL: page.Next}
	}
}

// deferredFrame is one pump receive taken out of band while a poll seam call
// was in flight and left for the walk's ordinary dispatch point. closed
// records the pump's channel closing, which carries no item.
//
// overflow separates the two deferral CLASSES, which have opposite ordering
// obligations against the next Save: an admission that would drop is a
// pre-cut loss condition, so §23's conjunctive save-ordering invariant puts
// its disposition BEFORE the page's position moves; a socket outcome is
// transition 21, which the page boundary deliberately observes AFTER the
// in-flight page has been accepted, delivered and saved.
type deferredFrame struct {
	item     pumpItem
	closed   bool
	overflow bool
}

// pollResult is one poll seam call's outcome, carried back from the call's
// own goroutine.
type pollResult struct {
	page PollPage
	err  error
}

// pollPage issues one poll seam call and keeps ADMITTING live events while it
// is in flight. The call runs on its own goroutine — it carries no state, so
// the state machine remains the only mutator — because an event arriving
// while the entry poll is outstanding is exactly the in-flight-at-entry
// straggler the live buffer exists to carry (SPEC.md §23 "Entry Boundary":
// "observed" is admission into the state-machine-owned buffer at or before
// the cut), and a state machine blocked inside the call cannot admit it.
//
// ONLY admissions are serviced here. The first receive that is not one — a
// socket failure, a disconnect frame, an invalid frame, or an admission that
// would overflow the buffer (whose drop-time signal dispatch belongs to
// admitLive) — is DEFERRED, and the call is still awaited to completion. That
// is what keeps transition 21's page boundary the place a dying socket is
// observed: the in-flight page is accepted, delivered and saved before the
// walk stops, and frame order is preserved because the servicing stops at the
// deferred receive.
//
// It reports superseded=true when it gave up on the call: a deferred SOCKET
// outcome is still awaited, but only for as long as the staleness window,
// because a PollSource that returns only on context cancellation would
// otherwise hold the consumer's goroutine forever behind a socket that has
// already spoken (and the staleness expiry that would tear that socket down
// is unobservable while the call is outstanding). The caller dispatches the
// deferred outcome, whose teardown cancels the abandoned call.
func (l *loop) pollPage(at *attempt, cursor Cursor) (page PollPage, superseded bool, err error) {
	done := make(chan pollResult, 1)
	go func() {
		// Cloned on the way out for the same reason WithFilters clones on the
		// way in: the seam is host code, and handing it the connector's own
		// backing arrays would let an adapter that sorts or dedupes in place
		// repoint the subscription's lineage from under a live feed.
		p, perr := l.cfg.polls.Poll(at.ctx, cursor, l.cfg.filters.clone())
		done <- pollResult{page: p, err: perr}
	}()
	for {
		select {
		case r := <-done:
			return r.page, false, r.err
		case item, ok := <-at.lc.frames:
			if !ok {
				l.deferred = &deferredFrame{closed: true}
			} else if handled, over := l.admitDuringPoll(item); handled {
				continue
			} else if over {
				// The buffer, not the socket: the call is unaffected and is
				// awaited to completion, and the drop's disposition runs
				// before this page's position moves anything durable.
				l.deferred = &deferredFrame{item: item, overflow: true}
				if l.hooks.frameDeferred != nil {
					l.hooks.frameDeferred(true)
				}
				r := <-done
				return r.page, false, r.err
			} else {
				l.deferred = &deferredFrame{item: item}
			}
			if l.hooks.frameDeferred != nil {
				l.hooks.frameDeferred(false)
			}
			return l.awaitSupersededPoll(at, done)
		}
	}
}

// awaitSupersededPoll waits out a seam call whose socket has already spoken.
// The in-flight page is still accepted, delivered and saved when the call
// completes — that ordering is what makes transition 21's page boundary the
// place a dying socket is observed — but the wait is BOUNDED by the same
// staleness window every other socket-open wait is: on the lapse the call is
// abandoned to the deferred outcome's teardown.
func (l *loop) awaitSupersededPoll(at *attempt, done <-chan pollResult) (page PollPage, superseded bool, err error) {
	for {
		staleTimer, staleGen := at.lc.stale.current()
		select {
		case r := <-done:
			return r.page, false, r.err
		case <-l.runCtx.Done():
			return PollPage{}, true, nil
		case <-at.lc.stale.rearmed():
		case <-staleTimer.C():
			if _, ok := at.lc.stale.evaluate(staleGen); ok {
				return PollPage{}, true, nil
			}
		}
	}
}

// admitDuringPoll handles one pump item received while a poll seam call is in
// flight, reporting whether it was fully handled and, when it was not,
// whether the reason is an admission that would OVERFLOW the live buffer. A
// correlated event frame is admitted to the live buffer; a ping, welcome,
// unknown type, or a frame carrying a foreign identifier is liveness only and
// skipped, exactly as the ordinary dispatch would (the pump already reset
// staleness). Everything else is left for the caller to defer.
func (l *loop) admitDuringPoll(item pumpItem) (handled bool, overflow bool) {
	if item.err != nil {
		return false, false
	}
	f, err := parseFrame(item.data)
	if err != nil {
		return false, false
	}
	switch f.kind {
	case frameMessage:
		if f.identifier != l.identifier {
			break
		}
		ev, derr := decodeMessageEvent(f.message)
		if derr != nil {
			return false, false
		}
		if l.buffer.full() {
			// An admission that drops is a drop-time signal dispatch with a
			// disposition that can end the cycle: it belongs to admitLive, at
			// the ordinary dispatch point.
			return false, true
		}
		l.buffer.add(ev)
	case frameDisconnect:
		return false, false
	case frameWelcome, framePing, frameConfirm, frameReject, frameUnknown:
		// Liveness only in a post-confirmation state.
	}
	if l.hooks.frameHandled != nil {
		l.hooks.frameHandled(f.kind)
	}
	return true, false
}

// dispatchDeferred dispatches the frame the in-flight-poll servicing deferred,
// if any, through the ordinary post-confirmation dispatch.
//
// Its two call sites are the walk's dispatch points — the page boundary and
// the walk's end — and deliberately not the `poll-retry` wait, which holds a
// timer this dispatch would not stop on teardown. Nothing is lost by the
// omission: a deferred receive is either the pump's terminating error, which
// nothing can follow, or a disconnect frame, the server's last word on the
// socket; both are dispatched at the next page boundary either way.
func (l *loop) dispatchDeferred(at *attempt) (cycleOutcome, bool) {
	d := l.deferred
	if d == nil {
		return cycleOutcome{}, false
	}
	l.deferred = nil
	if d.closed {
		return l.pumpExited(at, nil), true
	}
	return l.handleLiveFrame(at, nil, d.item, false)
}

// dispatchDeferredOverflow dispatches the deferral that is an admission the
// live buffer cannot take, and ONLY that one. It runs at every page, before
// the page's position is saved or held, because §23's conjunctive
// save-ordering invariant requires every pre-cut loss condition to have been
// explicitly accepted before the next Save: a Terminate disposition — or no
// handler — has to land in Terminal(`buffer_overflow`) with the durable
// checkpoint still where it was, or a restart skips the dropped event.
//
// Timing is what makes the dispatch point load-bearing rather than cosmetic:
// the buffer is still full here, so the admission drops exactly what it would
// have dropped in flight. Deferred to the walk's ordinary dispatch point, it
// would land after the drain had emptied the buffer and the signal would
// never fire at all.
//
// Socket outcomes keep their existing deferral to the page boundary below:
// the in-flight page is still accepted, delivered and saved before the walk
// observes a socket that died under it.
func (l *loop) dispatchDeferredOverflow(at *attempt) (cycleOutcome, bool) {
	if l.deferred == nil || !l.deferred.overflow {
		return cycleOutcome{}, false
	}
	return l.dispatchDeferred(at)
}

// ownershipCut performs SPEC.md §23's bounded admission pass: after the
// entry-poll response is accepted, receive from the frame pump's queue
// WITHOUT blocking until the queue is momentarily empty or the pass has
// dequeued liveBufferCapacity frames of any kind, whichever comes first. The
// cut is the completion of that pass, and the snapshot is what the
// state-machine-owned buffer holds at it.
//
// The bound counts dequeued frames, not admitted events: pings and control
// frames dequeue without admitting, so an event-counting bound would spin
// forever under heartbeat replenishment. The pass is deliberately not a
// drain-until-empty barrier — that races a concurrent sender and, under
// sustained arrival, never completes, so the entry position would never save.
func (l *loop) ownershipCut(at *attempt) (cycleOutcome, bool) {
	return l.admissionPass(at)
}

// socketCheck consumes what the socket has to say without waiting on it: a
// staleness firing observed here is evaluated under the same rule as in any
// select (a firing whose window overlapped a blocked hand-off is disregarded
// and re-armed), and the bounded admission pass then dispatches whatever the
// pump queued — a socket failure, a disconnect frame, or live events for the
// buffer.
func (l *loop) socketCheck(at *attempt) (cycleOutcome, bool) {
	staleTimer, staleGen := at.lc.stale.current()
	select {
	case <-staleTimer.C():
		if age, ok := at.lc.stale.evaluate(staleGen); ok {
			l.disposeAttempt(at, nil)
			if l.cfg.observer.StaleConnection != nil {
				l.cfg.observer.StaleConnection(age)
			}
			l.observeDisconnected("", errStaleConnection)
			return cycleOutcome{kind: outcomeFailed}, true
		}
	default:
	}
	return l.admissionPass(at)
}

// admissionPass is the bounded non-blocking pass over the frame pump's queue
// the ownership cut is defined as: receive without blocking until the queue is
// momentarily empty or liveBufferCapacity frames of ANY kind have been
// dequeued. Live events are admitted to the buffer, and every other frame
// takes its ordinary dispatch — so a socket failure or a disconnect frame the
// pump already queued ends the cycle here.
func (l *loop) admissionPass(at *attempt) (cycleOutcome, bool) {
	for dequeued := 0; dequeued < l.cfg.liveBufferCapacity; dequeued++ {
		select {
		case item, ok := <-at.lc.frames:
			if !ok {
				return l.pumpExited(at, nil), true
			}
			if out, done := l.handleLiveFrame(at, nil, item, false); done {
				return out, true
			}
		default:
			return cycleOutcome{}, false
		}
	}
	return cycleOutcome{}, false
}

// drain replays the live buffer through the dedupe LRU (transition 23). It is
// a bounded in-memory completion, not socket delivery: it dequeues no frames
// and takes no wire waits. Everything admitted up to this point is replayed —
// the pre-cut snapshot the save-ordering invariant is stated over, plus any
// straggler admitted after it, which is a superset and never a shortfall.
func (l *loop) drain(at *attempt) (cycleOutcome, bool) {
	// The scan runs under ONE budget for the whole drain — liveBufferCapacity
	// dequeues, the ownership cut's bound, for the ownership cut's reason: a
	// drain that re-scanned without limit would never complete under
	// sustained arrival, and the held position would never save.
	budget := l.cfg.liveBufferCapacity
	for {
		batch := l.buffer.take()
		if out, done := l.drainScan(at, &budget); done {
			return out, true
		}
		if len(batch) == 0 {
			return cycleOutcome{}, false
		}
		for _, ev := range batch {
			if l.runCtx.Err() != nil {
				l.disposeAttempt(at, nil)
				return cycleOutcome{kind: outcomeClosed}, true
			}
			if !l.deliver(ev) {
				l.disposeAttempt(at, nil)
				return cycleOutcome{kind: outcomeClosed}, true
			}
		}
	}
}

// drainScan is Draining's protocol-fatal carve-out, and the ONLY reason the
// drain looks at the frame queue at all. SPEC.md §23: "a raw
// `invalid_event_stream_command` observed during Draining is
// Terminal(`protocol_fatal`) immediately — the drain is not completed, the
// held entry position is NOT saved, and no `caught_up` is announced; only
// recoverable failures defer." Without the scan a fatal frame — one the walk
// left deferred, or one the pump has already queued — is seen only in
// Streaming, after the held save and the caught_up announcement have both
// happened.
//
// Everything else keeps Draining's deferred consumption, which is §23's one
// deliberate deferred-consumption case: a recoverable socket failure, a
// non-fatal disconnect, an invalid frame or a closed pump is parked in the
// same single deferral slot the in-flight-poll servicing uses and dispatched
// at the Streaming boundary, once the drain has completed. Correlated events
// are admitted, so the next take replays them in arrival order, and an
// admission that would drop takes its drop-time dispatch here — before the
// held save, like every other pre-cut loss condition.
func (l *loop) drainScan(at *attempt, budget *int) (cycleOutcome, bool) {
	if d := l.deferred; d != nil {
		// A fatal frame already in the slot is dispatched now rather than
		// after the save: the carve-out is about what governs, not about
		// which queue the frame is sitting in. Everything the pump has
		// queued arrived behind this one, so the scan stops here either way.
		if !d.closed {
			if f, ok := protocolFatalFrame(d.item); ok {
				l.deferred = nil
				return l.terminateProtocolFatal(at, f), true
			}
		}
		return cycleOutcome{}, false
	}
	for *budget > 0 {
		select {
		case item, ok := <-at.lc.frames:
			if !ok {
				l.deferred = &deferredFrame{closed: true}
				return cycleOutcome{}, false
			}
			*budget--
			handled, overflow := l.admitDuringPoll(item)
			switch {
			case handled:
			case overflow:
				if out, done := l.handleLiveFrame(at, nil, item, false); done {
					return out, true
				}
			default:
				if f, ok := protocolFatalFrame(item); ok {
					return l.terminateProtocolFatal(at, f), true
				}
				l.deferred = &deferredFrame{item: item}
				return cycleOutcome{}, false
			}
		default:
			return cycleOutcome{}, false
		}
	}
	return cycleOutcome{}, false
}

// terminateProtocolFatal takes the raw disconnect's state-generic terminal
// through the ordinary dispatch, so the frame is counted as handled and the
// disconnect observation carries the server's reason verbatim.
func (l *loop) terminateProtocolFatal(at *attempt, f frame) cycleOutcome {
	if l.hooks.frameHandled != nil {
		l.hooks.frameHandled(f.kind)
	}
	return l.dispatchDisconnect(at, nil, f)
}

// protocolFatalFrame reports whether one pump item is a raw
// `invalid_event_stream_command` disconnect — the one frame Draining is not
// allowed to defer.
func protocolFatalFrame(item pumpItem) (frame, bool) {
	if item.err != nil {
		return frame{}, false
	}
	f, err := parseFrame(item.data)
	if err != nil || f.kind != frameDisconnect || f.reason != disconnectReasonProtocolFatal {
		return frame{}, false
	}
	return f, true
}

// stream is the steady state (transitions 24–26): live events are delivered
// as they arrive, deduped, and never save — live ids do not advance the
// durable position. The jittered repair-poll timer is armed BEFORE the state
// is announced, so Streaming's exact timer set is {staleness, repair-poll}
// from the first instant an observer can see the state.
func (l *loop) stream(at *attempt) cycleOutcome {
	repair := l.cfg.clock.NewTimer(repairJitter(l.cfg.repairInterval, l.cfg.rand), timerRepairPoll)
	l.setState(stateStreaming)
	for {
		staleTimer, staleGen := at.lc.stale.current()
		select {
		case <-l.runCtx.Done():
			l.disposeAttempt(at, repair)
			return cycleOutcome{kind: outcomeClosed}
		case <-repair.C():
			// Transition 24 → CatchingUp: one repair walk from the connector's
			// current position, returning here through Draining. The next
			// cadence is armed before Streaming is re-announced, for the same
			// reason it is on first entry.
			out, done := l.repairWalk(at)
			if done {
				return out
			}
			repair = l.cfg.clock.NewTimer(repairJitter(l.cfg.repairInterval, l.cfg.rand), timerRepairPoll)
			l.setState(stateStreaming)
		case <-at.lc.stale.rearmed():
			continue
		case <-staleTimer.C():
			age, ok := at.lc.stale.evaluate(staleGen)
			if !ok {
				// Superseded by a frame the pump received first, or suspended
				// by a blocked hand-off and re-armed.
				continue
			}
			// Transition 25's staleness trigger.
			l.disposeAttempt(at, repair)
			if l.cfg.observer.StaleConnection != nil {
				l.cfg.observer.StaleConnection(age)
			}
			l.observeDisconnected("", errStaleConnection)
			return cycleOutcome{kind: outcomeFailed}
		case item, ok := <-at.lc.frames:
			if !ok {
				return l.pumpExited(at, repair)
			}
			if out, done := l.handleLiveFrame(at, repair, item, true); done {
				return out
			}
		}
	}
}

// deliver yields one event to the consumer unless the delivered-id LRU has
// already served it. Dedupe is symmetric across lanes — poll page, drain, and
// streaming all check before delivering and record the delivered id — so
// poll-vs-push duplication is suppressed by id regardless of which lane got
// there first. It reports whether the consumer is still consuming: a false
// yield (a `break`) ends iteration with no error element, so nothing is ever
// yielded after it.
func (l *loop) deliver(ev Event) bool {
	if l.stopped {
		return false
	}
	if l.dedupe.Seen(ev.ID) {
		return true
	}
	if !l.yield(ev, nil) {
		l.stopped = true
		return false
	}
	return true
}

// handleLiveFrame dispatches one pump item in a post-confirmation state.
// deliverLive selects the state's live-event disposition: Streaming delivers
// immediately; CatchingUp and the entry window admit to the live buffer
// instead (per-state invariants: catch-up delivers poll pages only). pending
// is the state's own timer, stopped with the attempt on teardown.
func (l *loop) handleLiveFrame(at *attempt, pending Timer, item pumpItem, deliverLive bool) (cycleOutcome, bool) {
	if item.err != nil {
		// Socket failure: peer close, read error, or the transport's
		// frame-size rejection (transitions 21/25).
		l.disposeAttempt(at, pending)
		l.observeDisconnected("", item.err)
		return cycleOutcome{kind: outcomeFailed}, true
	}
	f, err := parseFrame(item.data)
	if err != nil {
		// Invalid-frame class, parse shape: a socket-failure dispatch, never
		// terminal, never a silent skip.
		l.disposeAttempt(at, pending)
		l.observeDisconnected("", err)
		return cycleOutcome{kind: outcomeFailed}, true
	}
	if l.hooks.frameHandled != nil {
		l.hooks.frameHandled(f.kind)
	}
	switch f.kind {
	case frameDisconnect:
		// Reason-string dispatch applied state-generically: protocol-fatal is
		// terminal from every socket-open state; everything else is a socket
		// drop (transition 25).
		return l.dispatchDisconnect(at, pending, f), true
	case frameMessage:
		if f.identifier != l.identifier {
			return cycleOutcome{}, false
		}
		ev, derr := decodeMessageEvent(f.message)
		if derr != nil {
			// Invalid-frame class, decode shape.
			l.disposeAttempt(at, pending)
			l.observeDisconnected("", derr)
			return cycleOutcome{kind: outcomeFailed}, true
		}
		if !deliverLive {
			return l.admitLive(at, pending, ev)
		}
		if !l.deliver(ev) {
			l.disposeAttempt(at, pending)
			return cycleOutcome{kind: outcomeClosed}, true
		}
		return cycleOutcome{}, false
	default:
		// welcome, ping, unknown types, and a post-confirmation confirm or
		// reject: liveness only — the pump already reset staleness.
		return cycleOutcome{}, false
	}
}

// pumpExited handles the pump's channel closing without a terminating error —
// only possible under a cancellation race; treated as a socket failure.
func (l *loop) pumpExited(at *attempt, pending Timer) cycleOutcome {
	l.disposeAttempt(at, pending)
	l.observeDisconnected("", errors.New("event feed frame pump exited"))
	return cycleOutcome{kind: outcomeFailed}
}

// walkStep is the walk's next move after a failed poll: re-poll at cursor,
// with reentry marking transitions 17/18/19's NEW entry — the entry boundary
// restarts, under presentClass. A transient's re-poll of the same cursor is
// not a re-entry: nothing about the entry changed.
type walkStep struct {
	cursor       Cursor
	presentClass bool
	reentry      bool
}

// recoverPoll disposes a failed poll seam call onto its SPEC.md §23 outcome.
// It returns the walk's next step when the walk continues, or done=true with
// the outcome that ended the cycle. Seam errors are post-retry outcomes:
// the generated operation already spent its own §7 budget inside the call, so
// nothing here is a second per-request retry layer.
func (l *loop) recoverPoll(at *attempt, cursor Cursor, err error) (walkStep, cycleOutcome, bool) {
	var pe *PollError
	if !errors.As(err, &pe) {
		// The seam contract maps every generated outcome onto exactly one
		// kind; an unclassified failure is surfaced rather than retried, as
		// on the mint lane.
		l.disposeAttempt(at, nil)
		return walkStep{}, cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
			Reason: ReasonPollFailed, Msg: "unclassified poll failure", Err: err,
		}}, true
	}
	switch pe.Kind {
	case PollTransient, PollThrottled:
		// The self-loop inside CatchingUp: a wait, not a state change. The
		// same cursor is re-polled.
		l.pollFailures++
		out, done := l.waitPollRetry(at, pollRetryDelay(pe.RetryAfter, l.pollFailures, l.cfg.rand))
		return walkStep{cursor: cursor}, out, done
	case PollUnauthorized:
		// Recovery rides the reconnect cycle — the fresh mint/token pass —
		// incrementing the shared connection-level counter.
		l.authFailures++
		l.disposeAttempt(at, nil)
		l.observeDisconnected("", err)
		if l.authFailures >= authFailureThreshold {
			return walkStep{}, cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
				Reason: ReasonAuthorizationFailed,
				Msg:    fmt.Sprintf("%d consecutive connection-level authorization failures", l.authFailures),
				Err:    err,
			}}, true
		}
		return walkStep{}, cycleOutcome{kind: outcomeFailed, retryAfter: pe.RetryAfter}, true
	case PollFilterInvalid:
		// Transition 20: a configuration error a position reset won't help;
		// the server's message naming the offending list is preserved.
		l.disposeAttempt(at, nil)
		return walkStep{}, cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
			Reason: ReasonFilterInvalid, Msg: pe.Msg, Err: err,
		}}, true
	case PollRedirectRefused:
		// A 3xx whose Location failed the seam's per-hop validation: the
		// continuation edge, NEVER poll_failed. The Location is already
		// redacted to its origin by the adapter.
		l.disposeAttempt(at, nil)
		return walkStep{}, cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
			Reason: ReasonInvalidContinuation,
			Msg:    "the poll refused a redirect to " + pe.LocationOrigin,
			Err:    err,
		}}, true
	case PollUnrecoverable:
		l.disposeAttempt(at, nil)
		return walkStep{}, cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
			Reason: ReasonPollFailed, Msg: pe.Msg, Err: err,
		}}, true
	case PollGone, PollPositionInvalid, PollFilterChanged:
		// The re-entry matrix, whose disposition is a NEW cursor (recovery.go).
		return l.reenterWalk(at, pe)
	default:
		// An unknown kind is an adapter bug — the seam contract maps every
		// generated outcome onto exactly one kind. It is surfaced rather than
		// guessed into a re-entry, as on the mint lane: re-entering on an
		// unclassified failure would move the cursor off a condition nothing
		// here understands.
		l.disposeAttempt(at, nil)
		return walkStep{}, cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
			Reason: ReasonPollFailed,
			Msg:    fmt.Sprintf("unknown poll error kind %d", int(pe.Kind)),
			Err:    err,
		}}, true
	}
}

// waitPollRetry waits out the `poll-retry` timer while keeping the socket
// live: inbound frames are still consumed (admitted, not delivered — the walk
// has not reached its head), staleness still tears a dead socket down, and
// close still ends the cycle promptly.
func (l *loop) waitPollRetry(at *attempt, d time.Duration) (cycleOutcome, bool) {
	t := l.cfg.clock.NewTimer(d, timerPollRetry)
	for {
		staleTimer, staleGen := at.lc.stale.current()
		select {
		case <-l.runCtx.Done():
			l.disposeAttempt(at, t)
			return cycleOutcome{kind: outcomeClosed}, true
		case <-t.C():
			return cycleOutcome{}, false
		case <-at.lc.stale.rearmed():
			continue
		case <-staleTimer.C():
			age, ok := at.lc.stale.evaluate(staleGen)
			if !ok {
				continue
			}
			// Transition 21's staleness trigger.
			l.disposeAttempt(at, t)
			if l.cfg.observer.StaleConnection != nil {
				l.cfg.observer.StaleConnection(age)
			}
			l.observeDisconnected("", errStaleConnection)
			return cycleOutcome{kind: outcomeFailed}, true
		case item, ok := <-at.lc.frames:
			if !ok {
				return l.pumpExited(at, t), true
			}
			if out, done := l.handleLiveFrame(at, t, item, false); done {
				return out, true
			}
		}
	}
}
