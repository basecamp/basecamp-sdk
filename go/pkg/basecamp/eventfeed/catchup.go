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
	// handler returned Terminate, took the Terminal(buffer_overflow) edge in
	// the goroutine that observed the drop and never reached this line.
	if held != "" {
		l.acceptPosition(held)
		l.saveCheckpoint(held)
	}
	if l.cfg.observer.CaughtUp != nil {
		l.cfg.observer.CaughtUp()
	}
	// The walk's last dispatch point, and by construction a SOCKET outcome
	// only: nothing else is ever deferred — an overflowing admission is
	// dispatched where it is observed, and the drain's scan already took the
	// protocol-fatal carve-out. What is left defers here, after the drain and
	// the held save have completed — the same "finish what the page started,
	// then observe the socket" ordering the page boundary applies, and §23's
	// one deliberate deferred-consumption case.
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
		p := l.pollPage(at, cursor)
		if p.ended {
			// Servicing the socket during the call ended the cycle: an
			// overflowing admission whose disposition was not Accept, dispatched
			// where it was observed rather than parked for a cut this call may
			// never reach. The attempt is already disposed.
			return p.out, "", true
		}
		if l.runCtx.Err() != nil {
			l.disposeAttempt(at, nil)
			return cycleOutcome{kind: outcomeClosed}, "", true
		}
		if p.superseded {
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
		if p.err != nil {
			// A socket outcome deferred while THIS call was in flight is
			// dispatched before the poll error is classified. Transition 21's
			// finish-the-page ordering is about a page that SUCCEEDED: it
			// exists so an accepted page's deliveries and save are not
			// stranded by the socket's death. A failed poll has no such page,
			// and recoverPoll's terminal and re-entry branches dispose the
			// attempt — which discards the deferral — so leaving it parked
			// loses the server's own verdict. Concretely: an
			// `invalid_event_stream_command` observed during CatchingUp,
			// reported instead as authorization_failed or poll_failed, or
			// retried indefinitely behind poll-retry waits when the error is
			// transient.
			if out, done := l.dispatchDeferred(at); done {
				return out, "", true
			}
			step, out, done := l.recoverPoll(at, cursor, p.err)
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
		page := p.page
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
// Every deferral is a SOCKET outcome (or the pump's exit), which is transition
// 21: the page boundary deliberately observes it AFTER the in-flight page has
// been accepted, delivered and saved. Nothing else defers — in particular an
// admission that drops is a pre-cut loss condition whose disposition runs at
// drop time, in the goroutine that received the frame.
type deferredFrame struct {
	item   pumpItem
	closed bool
}

// pollResult is one poll seam call's outcome, carried back from the call's
// own goroutine.
type pollResult struct {
	page PollPage
	err  error
}

// pollAttempt is one poll-page attempt as the walk sees it: the page the seam
// call served, or the reason the walk has no page to work with.
type pollAttempt struct {
	// page and err are the seam call's own outcome, when it completed.
	page PollPage
	err  error
	// ended reports that servicing the socket while the call was in flight
	// ENDED THE CYCLE — an admission that dropped and whose disposition was
	// Terminate, or which had no handler at all. out carries the outcome, the
	// attempt is already disposed, and page/err are meaningless.
	ended bool
	out   cycleOutcome
	// superseded reports the call was abandoned: a deferred socket outcome
	// outlived the staleness window with the call still outstanding, or the
	// consumer went away. The caller dispatches the deferral, whose teardown
	// cancels the abandoned call.
	superseded bool
}

// pollPage issues one poll seam call and keeps ADMITTING live events while it
// is in flight. The call runs on its own goroutine — it carries no state, so
// the state machine remains the only mutator — because an event arriving
// while the entry poll is outstanding is exactly the in-flight-at-entry
// straggler the live buffer exists to carry (SPEC.md §23 "Entry Boundary":
// "observed" is admission into the state-machine-owned buffer at or before
// the cut), and a state machine blocked inside the call cannot admit it.
//
// ONLY admissions are serviced here, and an admission that DROPS is serviced
// too: the drop's BufferOverflow signal is dispatched at drop time, on this
// goroutine — the consumer's — because that is §23's first consumer-context
// opportunity after the condition arises. "Before the next save" is the rule's
// outer bound, not an appointment to keep: an in-flight call can stall
// indefinitely or fail outright, so a signal parked until it returns is a
// signal that may never be dispatched at all. An Accept keeps awaiting the
// call; anything else ends the cycle right here (ended=true).
//
// The first receive that is not an admission — a socket failure, a disconnect
// frame, an invalid frame — is DEFERRED, and the call is still awaited to
// completion. That is what keeps transition 21's page boundary the place a
// dying socket is observed: the in-flight page is accepted, delivered and saved
// before the walk stops, and frame order is preserved because the servicing
// stops at the deferred receive.
//
// It reports superseded=true when it gave up on the call: a deferred SOCKET
// outcome is still awaited, but only for as long as the staleness window,
// because a PollSource that returns only on context cancellation would
// otherwise hold the consumer's goroutine forever behind a socket that has
// already spoken (and the staleness expiry that would tear that socket down
// is unobservable while the call is outstanding). The caller dispatches the
// deferred outcome, whose teardown cancels the abandoned call.
func (l *loop) pollPage(at *attempt, cursor Cursor) pollAttempt {
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
			return pollAttempt{page: r.page, err: r.err}
		case item, ok := <-at.lc.frames:
			if !ok {
				l.deferred = &deferredFrame{closed: true}
			} else {
				handled, out, ended := l.admitDuringPoll(at, item)
				if ended {
					// The drop's own disposition. Disposing the attempt is what
					// returns the abandoned call, so nothing waits on it.
					return pollAttempt{ended: true, out: out}
				}
				if handled {
					continue
				}
				l.deferred = &deferredFrame{item: item}
			}
			// The superseded-poll bound starts HERE, at the deferral, and is
			// read before the hook fires so nothing observing the deferral
			// can race the deadline into existence behind it.
			deadline := l.cfg.clock.Now().Add(l.cfg.staleAfter)
			if l.hooks.frameDeferred != nil {
				l.hooks.frameDeferred()
			}
			return l.awaitSupersededPoll(at, done, deadline)
		}
	}
}

// awaitSupersededPoll waits out a seam call whose socket has already spoken.
// The in-flight page is still accepted, delivered and saved when the call
// completes — that ordering is what makes transition 21's page boundary the
// place a dying socket is observed — but the wait is BOUNDED: on the lapse the
// call is abandoned to the deferred outcome's teardown.
//
// The bound is `deadline`: a FIXED instant, read from the injected clock at
// the deferral and moved by nothing afterwards. It is deliberately NOT the staleness
// verdict this wait used to borrow, and the difference is the whole point.
// Every OTHER socket-open wait in the connector keeps DRAINING the hand-off
// queue while it waits, which is what makes §23's suspension rule sound
// there: a full queue really does prove the peer is outrunning a connector
// that is still consuming. This wait is the one that deliberately stops
// consuming — a frame is already parked in the deferral slot, and a second
// receive would have nowhere to go. So the queue it stopped draining fills,
// the pump blocks in handOff, and the suspension gets granted against a
// premise that is false by construction: every firing is disregarded and
// re-armed, forever, while a compliant PollSource that returns only on
// cancellation holds the consumer's goroutine. A peer that keeps sending is
// then enough to keep an already-dead socket's verdict from ever landing.
//
// The staleness firing and the re-arm wake are demoted to WAKE-UPS; the
// deadline alone decides. evaluate is still called on a firing, and still
// re-arms a suspended window — that re-arm is what guarantees the next wake
// when the pump is blocked and no frame can deliver one — so the lapse is
// observed at the first wake at or after the deadline.
//
// A dedicated timer would express this more directly, and is deliberately not
// used: SPEC.md §23 pins exactly six timer kinds AND every state's exact
// timer set, both asserted by the cross-SDK fixtures, so a seventh kind is a
// spec change across six SDKs rather than a fix to the Go reference.
func (l *loop) awaitSupersededPoll(at *attempt, done <-chan pollResult, deadline time.Time) pollAttempt {
	for {
		staleTimer, staleGen := at.lc.stale.current()
		select {
		case r := <-done:
			return pollAttempt{page: r.page, err: r.err}
		case <-l.runCtx.Done():
			return pollAttempt{superseded: true}
		case <-at.lc.stale.rearmed():
		case <-staleTimer.C():
			if _, ok := at.lc.stale.evaluate(staleGen); ok {
				return pollAttempt{superseded: true}
			}
		}
		if !l.cfg.clock.Now().Before(deadline) {
			return pollAttempt{superseded: true}
		}
	}
}

// admitDuringPoll handles one pump item received out of band — while a poll
// seam call is in flight, or during the drain's scan — reporting whether it was
// fully handled and, when handling it ended the cycle, that outcome. A
// correlated event frame is admitted to the live buffer; a ping, welcome,
// unknown type, or a frame carrying a foreign identifier is liveness only and
// skipped, exactly as the ordinary dispatch would (the pump already reset
// staleness). Everything else is left for the caller.
//
// An admission that DROPS is handled here rather than handed back, because
// this goroutine is the consumer's and drop time is §23's first
// consumer-context opportunity: admitLive raises the signal and takes its
// disposition, which is Accept (handled, the walk continues) or the
// Terminal(buffer_overflow) that ends the cycle with nothing durable moved.
func (l *loop) admitDuringPoll(at *attempt, item pumpItem) (handled bool, out cycleOutcome, ended bool) {
	if item.err != nil {
		return false, cycleOutcome{}, false
	}
	f, err := parseFrame(item.data)
	if err != nil {
		return false, cycleOutcome{}, false
	}
	var ev Event
	admit := false
	switch f.kind {
	case frameMessage:
		if f.identifier != l.identifier {
			break
		}
		e, derr := decodeMessageEvent(f.message)
		if derr != nil {
			return false, cycleOutcome{}, false
		}
		ev, admit = e, true
	case frameDisconnect:
		return false, cycleOutcome{}, false
	case frameWelcome, framePing, frameConfirm, frameReject, frameUnknown:
		// Liveness only in a post-confirmation state.
	}
	if l.hooks.frameHandled != nil {
		l.hooks.frameHandled(f.kind)
	}
	if admit {
		if out, done := l.admitLive(at, nil, ev); done {
			return true, out, true
		}
	}
	return true, cycleOutcome{}, false
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
// and takes no wire waits. Everything the buffer STILL HOLDS is replayed —
// the pre-cut snapshot the save-ordering invariant is stated over, plus any
// straggler admitted after it, less anything an overflowing admission evicted
// along the way, which is the one subtraction the invariant already allows
// for and which the BufferOverflow signal reports before the held save.
func (l *loop) drain(at *attempt) (cycleOutcome, bool) {
	// The scan runs under ONE budget for the whole drain — pumpDepth+1
	// dequeues: the depth of the queue being scanned, plus the one frame the
	// pump may already have READ and be blocked handing off.
	//
	// It is deliberately NOT the ownership cut's liveBufferCapacity bound.
	// That bound answers a different question ("how large an admission pass
	// may the cut take before the entry position must be allowed to save"),
	// it is caller-configurable down to 1, and it measures the state
	// machine's buffer rather than the pump's queue. Spending it on ordinary
	// frames is exactly what let a fatal frame hide: one queued ping under
	// WithLiveBufferCapacity(1) consumed the whole scan, and the
	// invalid_event_stream_command behind it went unseen until Streaming —
	// after the held save and the caught_up announcement the carve-out exists
	// to prevent.
	//
	// pumpDepth+1 is the bound the carve-out's guarantee falls out of, and the
	// +1 is load-bearing rather than slack. The drain-start boundary the
	// guarantee is stated over is "every frame the pump had ALREADY READ",
	// not "every frame sitting in the queue", and those differ by exactly
	// one: the hand-off queue is FIFO and holds at most pumpDepth items,
	// while the pump — a single goroutine, so at most one hand-off in flight
	// — may be blocked inside handOff with one more. A budget of pumpDepth
	// spends itself on that frame's predecessors; the first dequeue releases
	// the blocked hand-off, the held frame enters at the TAIL, and if it is
	// the `invalid_event_stream_command` the carve-out exists for, the scan
	// stops one short of it. The drain then completes, the held entry
	// position saves and `caught_up` announces — precisely what the carve-out
	// forbids. pumpDepth+1 reaches it, and cannot grow further: one reader
	// goroutine can hold no more than one frame outside the queue.
	//
	// It still terminates — at most pumpDepth+1 dequeues for the whole drain,
	// after which the scan admits nothing, the buffer empties and the drain
	// ends — which is what an unbounded scan-until-empty could not promise
	// against a sustained sender, and the reason the held position would
	// otherwise never save.
	//
	// What the bound does NOT promise is a fatal frame that both ARRIVES
	// after the drain began and sits behind pumpDepth other frames that also
	// arrived after it. That one is consumed at the Streaming boundary under
	// §23's ordinary deferred-consumption rule: it was not observable when
	// the drain started, so no ordering between it and the drain's completion
	// was ever established.
	//
	// The replay dequeues ONE event at a time, and the scan runs before each
	// one. Both halves matter. Scanning first is what keeps the
	// protocol-fatal carve-out ahead of every delivery; dequeuing singly is
	// what keeps the drain inside the live buffer's capacity, which is a
	// bound on events held AT ONCE (SPEC.md §23 sizes the connector's whole
	// memory ceiling off it). Taking the buffer's whole contents into a batch
	// instead let the buffer read as empty while `capacity` events were still
	// pending in that batch, so the scan could admit another full capacity
	// without dropping anything — twice the configured retention and twice
	// the published ceiling, during exactly the slow drain the bound is for.
	//
	// Because nothing is held outside the buffer, an admission that overflows
	// evicts the oldest events still pending — including ones this drain has
	// not replayed yet. That is the same pre-cut loss condition the buffer
	// always had, taking its drop-time dispatch in drainScan, before the held
	// save: the conjunctive invariant asks that every such loss be explicitly
	// accepted, not that a drain be exempt from the capacity it was
	// configured with.
	//
	// Termination is unchanged, and now falls out of two independently
	// decreasing quantities: each turn either delivers one buffered event or
	// finds the buffer empty and ends, while the scan's budget bounds total
	// admissions. Total replay work stays the buffer's occupancy at entry
	// plus pumpDepth+1.
	budget := pumpDepth + 1
	for {
		if out, done := l.drainScan(at, &budget); done {
			return out, true
		}
		ev, ok := l.buffer.shift()
		if !ok {
			return cycleOutcome{}, false
		}
		if !l.deliver(ev) {
			l.disposeAttempt(at, nil)
			return cycleOutcome{kind: outcomeClosed}, true
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
//
// budget is the drain's shared dequeue allowance, sized at the scanned
// queue's own depth (see drain); an ordinary frame consuming it must never be
// able to end the scan short of a fatal frame the pump had already queued.
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
			handled, out, ended := l.admitDuringPoll(at, item)
			switch {
			case ended:
				return out, true
			case handled:
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
//
// Cancellation is checked HERE, per delivery, rather than only at the loop's
// dispatch points, because Close is callable from the consumer's own loop
// body and from any observer or handler callback — all of which run on this
// goroutine, between one delivery and the next. Close cancels synchronously
// (Connector.Close), so this check is what makes "no delivery begins after
// Close returns" true mid-page and mid-drain, not merely at the next page
// boundary. It reports the same false every caller already routes onto the
// Closed edge, and deliberately does NOT latch `stopped`: cancellation is not
// a consumer break.
func (l *loop) deliver(ev Event) bool {
	if l.stopped || l.runCtx.Err() != nil {
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
		// continuation edge, NEVER poll_failed. This edge exposes the rejected
		// Location redacted to its ORIGIN — a hostile continuation's path and
		// query are exactly what must not be echoed — so the seam error is not
		// retained as the cause: PollError.Error renders its underlying Err and
		// Unwrap hands it out, and the generated error's text routinely carries
		// the request URL in full. The classification survives on a sanitized
		// cause carrying the same redaction the terminal promises.
		l.disposeAttempt(at, nil)
		return walkStep{}, cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
			Reason: ReasonInvalidContinuation,
			Msg:    "the poll refused a redirect to " + pe.LocationOrigin,
			Err:    &PollError{Kind: PollRedirectRefused, LocationOrigin: pe.LocationOrigin},
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
