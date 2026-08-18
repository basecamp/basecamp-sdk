// Slice-4c acceptance tests: the reconnect cycle now that catch-up, the
// entry boundary, and streaming exist — fresh-ticket reconnect (acceptance
// (a)), the full-jitter backoff envelope and its counter discipline, what a
// reconnect preserves versus rebuilds, and the staleness policy's evaluation
// rule (SPEC.md §23 transitions 2/4/7/9/14/15/21/25 and the addenda's
// pump-side reset, full-queue suspension, and Draining deferral). Structured
// to mirror tier-2 fixtures 05, 07, and 17; all time flows through the
// injected feedtest.Clock, and mint/connect/poll counts count seam calls.
package eventfeed_test

import (
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed/feedtest"
)

// staleAfter is SPEC §23's pinned staleness window (EVENT_FEED_STALE_AFTER).
const staleAfter = 7500 * time.Millisecond

// TestFreshTicketReconnectAfterTTL is acceptance (a) (fixture 05): a stream
// runs past the first ticket's server-owned expires_in — which must schedule
// nothing client-side — and is then severed. The reconnect pass mints a FRESH
// ticket and dials the SECOND mint's URL, and one catch-up poll from the
// in-memory position precedes any live delivery. Staleness and repair-poll
// are configured out of the window, as the fixture does, so the sever is the
// only thing that ends the socket.
func TestFreshTicketReconnectAfterTTL(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store, eventfeed.WithRepairInterval(999_999*time.Second))
	h.conn.SetStaleAfter(999_999 * time.Second)
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-2"})
	base := runtime.NumGoroutine()
	h.start()

	conn1 := h.driveToSubscribed()
	conn1.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()
	assertPositions(t, store.Saves(), "pos-1")

	// 121s of virtual silence: past the ~120s ticket TTL, which is
	// server-owned and never used for client-side scheduling — nothing is due.
	h.clock.Advance(121 * time.Second)
	if got := h.minter.Calls(); got != 1 {
		t.Fatalf("mint seam calls after the TTL elapsed = %d, want 1 (expires_in schedules nothing)", got)
	}

	conn1.FailReads(errors.New("connection reset by peer"))
	h.awaitTimer(timerBackoff)
	if !conn1.Closed() {
		t.Fatal("the sever must dispose the attempt's socket")
	}
	// The whole attempt is disposed before Backoff is entered: no repair-poll
	// or staleness timer survives it.
	assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
	if d := h.fireTimer(timerBackoff); d < 0 || d > time.Second {
		t.Fatalf("first backoff delay = %s, want within [0, 1s] (full jitter, n=1)", d)
	}

	conn2 := h.driveToSubscribed()
	conn2.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	urls := h.tr.DialedURLs()
	if len(urls) != 2 || urls[0] != ticket(1).URL || urls[1] != ticket(2).URL {
		t.Fatalf("dialed URLs = %v, want each mint's url verbatim, in order", urls)
	}
	if got := h.minter.Calls(); got != 2 {
		t.Fatalf("mint seam calls = %d, want 2 (a fresh ticket on EVERY pass)", got)
	}
	calls := h.polls.Calls()
	if len(calls) != 2 || calls[1].Cursor != (eventfeed.Cursor{Position: "pos-1"}) {
		t.Fatalf("poll calls = %+v, want the reconnect walking from the in-memory position", calls)
	}
	assertPositions(t, store.Saves(), "pos-1", "pos-2")
	if got := len(store.Loads()); got != 1 {
		t.Fatalf("store loads = %d, want 1 (the store is never re-read within a run)", got)
	}
	assertIDs(t, h.deliveredIDs())
	assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1})
	if conn2.Closed() {
		t.Fatal("the reconnected socket must be open")
	}

	h.conn.Close()
	h.join()
	assertGoroutinesSettle(t, base)
}

// TestReconnectResumesFromTheInMemoryPosition (fixture 17's catch-up half): a
// `remote` disconnect re-enters at the position the connector already holds —
// not the stored one, and not the configured Start mode — as a POSITION-RESUME
// entry, with the delivered-id LRU carried across the reconnect so a re-served
// page repeats nothing.
func TestReconnectResumesFromTheInMemoryPosition(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(101)}, Position: "pos-1"})
	// The re-entry page re-serves 101 (it sits at the safety horizon) plus 102.
	h.polls.ScriptPage(eventfeed.PollPage{
		Events:   []eventfeed.Event{pollEvent(101), pollEvent(102)},
		Position: "pos-2",
	})
	h.start()

	conn1 := h.driveToSubscribed()
	conn1.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()
	first := h.awaitBoundary()
	if first.Entry != (eventfeed.Cursor{Position: "pos-0"}) || first.PresentClass {
		t.Fatalf("first entry = {%+v present=%v}, want the loaded position, position-resume", first.Entry, first.PresentClass)
	}

	conn1.Serve(frameDisconnect("remote", true))
	h.awaitTimer(timerBackoff)
	h.fireTimer(timerBackoff)
	conn2 := h.driveToSubscribed()
	conn2.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	second := h.awaitBoundary()
	if second.Entry != (eventfeed.Cursor{Position: "pos-1"}) || second.PresentClass {
		t.Fatalf("re-entry = {%+v present=%v}, want the in-memory position, position-resume", second.Entry, second.PresentClass)
	}
	if got := len(store.Loads()); got != 1 {
		t.Fatalf("store loads = %d, want 1 (in-memory position is authoritative for resume within a run)", got)
	}
	assertIDs(t, h.deliveredIDs(), 101, 102)
	assertLedger(t, h.ledger(), []string{"event 101", "save pos-1", "event 102", "save pos-2"})
}

// TestReconnectCarriesTheAdmittedStragglerIntoTheNextEntry: the live buffer is
// state-machine-owned, not socket-owned. An event admitted before the first
// socket died is still the only carrier of that straggler — the poll lane
// never serves an id at or below a present-class entry's cursor — so it is
// delivered on the next attempt BEFORE that entry's held position saves. The
// conjunctive save-ordering invariant holds across the reconnect.
func TestReconnectCarriesTheAdmittedStragglerIntoTheNextEntry(t *testing.T) {
	store := feedtest.NewStore() // Missing → a present-class entry
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()

	conn1 := h.driveToSubscribed()
	h.serveSettled(conn1, frameMessage(noFilterIdentifier, 41))
	h.awaitFrameHandled("message")
	conn1.FailReads(errors.New("connection reset by peer"))
	h.awaitTimer(timerBackoff)
	if got := h.polls.CallCount(); got != 0 {
		t.Fatalf("poll seam calls = %d, want 0 (the first attempt never confirmed)", got)
	}
	h.fireTimer(timerBackoff)

	conn2 := h.driveToSubscribed()
	conn2.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	b := h.awaitBoundary()
	if len(b.Buffered) != 1 || b.Buffered[0].ID != 41 {
		t.Fatalf("buffered at the second hand-off = %v, want the straggler admitted before the first socket died", b.Buffered)
	}
	assertLedger(t, h.ledger(), []string{"event 41", "save pos-1"})
	assertIDs(t, h.deliveredIDs(), 41)
}

// TestBackoffEnvelopeGrowsAndResetsOnConfirmation: the reconnect-cycle delay
// is full-jitter over the failed-cycle count n — uniform(0, min(60s, 1s ×
// 2^(n−1))) — so with the draw pinned at its upper bound the envelope doubles
// per failed cycle; confirmation resets that counter (transition 11) and the
// next failure starts at n=1 again.
func TestBackoffEnvelopeGrowsAndResetsOnConfirmation(t *testing.T) {
	h := newHarness(t)
	// A degenerate always-1 draw makes the envelope itself assertable; the
	// formula's bounds are pinned in backoff_test.go.
	h.conn.SetRand(func() float64 { return 1 })
	for i := 1; i <= 5; i++ {
		h.minter.ScriptTicket(ticket(i))
	}
	for i := 0; i < 3; i++ {
		h.tr.FailNextDial(errors.New("dial tcp: connection refused"))
	}
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()

	for _, want := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second} {
		if d := h.fireTimer(timerBackoff); d != want {
			t.Fatalf("backoff delay = %s, want %s (full-jitter envelope, doubling per failed cycle)", d, want)
		}
	}

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()
	if got := len(h.tr.Dials()); got != 4 {
		t.Fatalf("connect seam calls = %d, want 4 (three refused dials, then the live one)", got)
	}

	conn.FailReads(errors.New("connection reset by peer"))
	if d := h.fireTimer(timerBackoff); d != time.Second {
		t.Fatalf("post-confirmation backoff delay = %s, want 1s — confirmation resets the failed-cycle count", d)
	}
}

// TestStalenessSuspendedWhileThePumpIsBlocked is the G-R16/17/18 evaluation
// rule: staleness is suspended while the pump is blocked on a full hand-off
// queue — a full queue is a fast peer, not a dead one, and a pump that is not
// reading cannot observe the resets that would prove it. Suspension is an
// EVALUATION rule, not a timer-set change: the timer stays armed, and a firing
// whose window overlapped a pump-blocked interval is disregarded and re-armed
// rather than dispatched. A firing over a window the pump spent reading is
// authoritative.
func TestStalenessSuspendedWhileThePumpIsBlocked(t *testing.T) {
	h := newHarness(t)
	blocked := make(chan struct{}, 1)
	// Blocks and releases are counted, not merely signalled: a release is what
	// re-arms the window unsuspended, and the second half of the test is only
	// meaningful once every block the pump took has been released.
	var mu sync.Mutex
	var blocks, releases int
	h.conn.OnPumpBlocked(func() {
		mu.Lock()
		blocks++
		mu.Unlock()
		select {
		case blocked <- struct{}{}:
		default:
		}
	})
	h.conn.OnPumpReleased(func() {
		mu.Lock()
		releases++
		mu.Unlock()
	})
	h.pauseAfter = 1
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	// The consumer parks inside the delivery of the first live event, so the
	// state machine stops dequeuing while the peer keeps sending.
	conn.Serve(frameMessage(noFilterIdentifier, 1))
	h.waitUntil("the consumer parked mid-delivery", func() bool { return len(h.deliveredIDs()) == 1 })
	for i := 0; i <= eventfeed.ExportPumpDepth; i++ {
		conn.Serve(framePing())
	}
	select {
	case <-blocked:
	case <-time.After(watchdog):
		t.Fatal("the pump never blocked on a full hand-off queue")
	}

	h.clock.Advance(staleAfter)
	h.resume()

	// The queue drains and the socket survives: the firing is disregarded, not
	// dispatched. A dispatched firing disposes the attempt, so the socket
	// closes and the queued frames are never handled — checked on every turn
	// so that failure reports itself rather than timing out.
	deadline := time.After(watchdog)
	for pings := 0; pings <= eventfeed.ExportPumpDepth; {
		if conn.Closed() {
			t.Fatal("a staleness firing whose window overlapped a pump block must be disregarded, not dispatched")
		}
		select {
		case kind := <-h.handled:
			if kind == "ping" {
				pings++
			}
		case <-time.After(time.Millisecond):
		case <-deadline:
			t.Fatal("the hand-off queue never drained")
		}
	}
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("a suspended staleness firing is never terminal; got %v", terminal)
	}
	// The last ping being HANDLED only proves the state machine dequeued it —
	// the dequeue is what unblocks the pump, so the pump's release still races
	// this point, and the release is what re-arms the window unsuspended. An
	// advance taken before it lands fires a window the release then supersedes
	// on generation grounds, leaving nothing to tear the socket down. All the
	// pings are handled here, so no further hand-off can block: once every
	// block has been released, the armed window is settled.
	h.waitUntil("every blocked hand-off was released", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return blocks > 0 && releases == blocks
	})
	// Disregarded AND re-armed — the exact per-state set is unchanged.
	assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1})

	// The pump is reading again: this window is authoritative.
	h.clock.Advance(staleAfter)
	h.awaitTimer(timerBackoff)
	if !conn.Closed() {
		t.Fatal("a firing over a reading window must tear the socket down")
	}
	assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
}

// TestStalenessDuringDrainDefersToTheStreamingBoundary: Draining is a bounded
// in-memory replay with no failure edge of its own, so a staleness expiry
// observed while it runs is consumed at the Streaming boundary — transition 23
// completes the drain (retained deliveries, then the held position's save and
// caught_up), and only then does transition 25 handle the failure.
func TestStalenessDuringDrainDefersToTheStreamingBoundary(t *testing.T) {
	var mu sync.Mutex
	var caughtUp int
	obs := eventfeed.Observer{CaughtUp: func() {
		mu.Lock()
		caughtUp++
		mu.Unlock()
	}}
	store := feedtest.NewStore()
	h := storedHarness(t, store, eventfeed.WithObserver(obs))
	h.pauseAfter = 1
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()

	conn := h.driveToSubscribed()
	h.serveSettled(conn, frameMessage(noFilterIdentifier, 41))
	h.awaitFrameHandled("message")
	conn.Serve(frameConfirm(noFilterIdentifier))

	// The consumer parks inside the drain's delivery of the retained event.
	h.waitUntil("the drain parked mid-delivery", func() bool { return len(h.deliveredIDs()) == 1 })
	h.clock.Advance(staleAfter)
	h.resume()

	h.awaitTimer(timerBackoff)
	assertLedger(t, h.ledger(), []string{"event 41", "save pos-1"})
	assertPositions(t, store.Saves(), "pos-1")
	mu.Lock()
	if caughtUp != 1 {
		mu.Unlock()
		t.Fatalf("caught_up announcements = %d, want 1 (the drain completes before the failure is handled)", caughtUp)
	}
	mu.Unlock()
	if !conn.Closed() {
		t.Fatal("the deferred staleness expiry must tear the socket down at the Streaming boundary")
	}
	assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("staleness is never terminal; got %v", terminal)
	}
}

// TestWalkFailureBetweenPages is transition 21 from inside the walk itself: a
// socket death and a staleness expiry that arise while a multi-page walk is in
// flight are both observed at the page boundary — the walk stops there rather
// than following `next` on a socket that is already gone — and the reconnect
// re-enters at the last accepted page's position with the saved checkpoints
// kept.
func TestWalkFailureBetweenPages(t *testing.T) {
	// setup drives a two-page walk whose FIRST poll seam call runs interrupt
	// while it is in flight, so the failure is queued for the state machine
	// before the first page is even accepted — the walk's own page boundary is
	// then the only place it can be observed.
	setup := func(t *testing.T, interrupt func(h *harness, conn *feedtest.Conn, errQueued <-chan struct{})) *harness {
		t.Helper()
		store := feedtest.NewStore()
		store.Stored("pos-0")
		h := storedHarness(t, store)
		h.minter.ScriptTicket(ticket(1))
		h.minter.ScriptTicket(ticket(2))
		next := testOrigin + "/999/events.json?after=101"
		h.polls.ScriptPage(eventfeed.PollPage{
			Events:   []eventfeed.Event{pollEvent(101)},
			Position: "pos-1",
			Next:     next,
		})
		h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(102)}, Position: "pos-2"})

		errQueued := make(chan struct{}, 1)
		h.conn.OnPumpHandedOff(func(isErr bool) {
			if !isErr {
				return
			}
			select {
			case errQueued <- struct{}{}:
			default:
			}
		})
		var conn *feedtest.Conn
		var once sync.Once
		h.polls.OnCall(func(feedtest.PollCall) {
			once.Do(func() { interrupt(h, conn, errQueued) })
		})
		h.start()
		conn = h.driveToSubscribed()
		conn.Serve(frameConfirm(noFilterIdentifier))
		h.awaitTimer(timerBackoff)

		if got := h.polls.CallCount(); got != 1 {
			t.Fatalf("poll seam calls = %d, want 1 — the walk must not follow `next` on a dead socket", got)
		}
		if !conn.Closed() {
			t.Fatal("the mid-walk failure must dispose the attempt")
		}
		assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
		if _, terminal, _ := h.snapshot(); terminal != nil {
			t.Fatalf("a mid-walk failure is never terminal; got %v", terminal)
		}
		assertPositions(t, store.Saves(), "pos-1")
		return h
	}

	t.Run("socket drop", func(t *testing.T) {
		setup(t, func(_ *harness, conn *feedtest.Conn, errQueued <-chan struct{}) {
			conn.FailReads(errors.New("connection reset by peer"))
			select {
			case <-errQueued:
			case <-time.After(watchdog):
				t.Error("the pump never handed the socket failure to the state machine")
			}
		})
	})

	t.Run("staleness expiry", func(t *testing.T) {
		h := setup(t, func(h *harness, _ *feedtest.Conn, _ <-chan struct{}) { h.clock.Advance(staleAfter) })

		// The reconnect re-enters at the accepted page's position, keeping the
		// checkpoint the walk already saved.
		h.fireTimer(timerBackoff)
		conn := h.driveToSubscribed()
		conn.Serve(frameConfirm(noFilterIdentifier))
		h.awaitStreaming()
		calls := h.polls.Calls()
		if len(calls) != 2 || calls[1].Cursor != (eventfeed.Cursor{Position: "pos-1"}) {
			t.Fatalf("poll calls = %+v, want the re-entry at the last accepted position", calls)
		}
		assertIDs(t, h.deliveredIDs(), 101, 102)
	})
}
