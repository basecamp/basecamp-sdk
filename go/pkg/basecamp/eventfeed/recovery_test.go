// Slice-4d acceptance tests: the repair poll (SPEC.md §23 transition 24) and
// the re-entry matrix a poll's 410, 400-position, or 409 takes (transitions
// 17/18/19), plus the shared authorization counter's poll-lane share and the
// two-lane poll-retry timing. Structured to mirror tier-2 fixtures 16, 23,
// 25, 27, and 29's repair half — the 400/409 rows have no PR-2 fixture (they
// are pinned at PR 4), so their coverage here is written to the same shape.
package eventfeed_test

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed/feedtest"
)

// signalLedger is the tier-2 driver's handlerInvocations record expressed as a
// Go SignalHandler: it returns a scripted disposition and appends one
// {kind, disposition} entry per invocation, in the fixtures' own spellings.
// The record is the ONLY thing distinguishing handler-Terminate from
// no-handler-terminal (their visible outcomes are identical), so the
// assertions check the exact set, never mere presence — which is what makes
// them red against a bypassed handler.
type signalLedger struct {
	mu      sync.Mutex
	give    eventfeed.Disposition
	entries []string
	signals []eventfeed.Signal
}

// handler is the registered SignalHandler: invoked exactly once per signal,
// synchronously, on the consumer's goroutine, before its disposition takes
// effect.
func (l *signalLedger) handler(s eventfeed.Signal) eventfeed.Disposition {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.signals = append(l.signals, s)
	l.entries = append(l.entries, signalKind(s)+"/"+dispositionName(l.give))
	return l.give
}

func (l *signalLedger) invocations() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.entries...)
}

func (l *signalLedger) first() eventfeed.Signal {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.signals) == 0 {
		return nil
	}
	return l.signals[0]
}

// signalKind renders a signal's fixture spelling.
func signalKind(s eventfeed.Signal) string {
	switch s.(type) {
	case eventfeed.FeedGap:
		return "feedGap"
	case eventfeed.BufferOverflow:
		return "bufferOverflow"
	default:
		return fmt.Sprintf("%T", s)
	}
}

// dispositionName renders a disposition's fixture spelling.
func dispositionName(d eventfeed.Disposition) string {
	if d == eventfeed.Accept {
		return "accept"
	}
	return "terminate"
}

func assertInvocations(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("handler invocations = %v, want exactly %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("handler invocations = %v, want exactly %v", got, want)
		}
	}
}

// gonePoll builds the 410 the poll seam reports: the body's epoch_after_id and
// resume URL, classified.
func gonePoll(resume string) error {
	return &eventfeed.PollError{Kind: eventfeed.PollGone, EpochAfterID: 100, ResumeURL: resume}
}

// TestFeedGapAcceptedResumesViaProvidedURL is fixture 16: a 410 with an
// accepting handler fires Observer.Gap, invokes the handler exactly once, and
// re-enters via the URL the SERVER provided — verbatim, and as a present-class
// entry, so the resume page's position is held until the admitted straggler
// has been delivered.
func TestFeedGapAcceptedResumesViaProvidedURL(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	ledger := &signalLedger{give: eventfeed.Accept}
	var gaps []string
	obs := eventfeed.Observer{Gap: func(epochAfterID int64, resumeURL string) {
		gaps = append(gaps, fmt.Sprintf("gap %d %s", epochAfterID, resumeURL))
	}}
	h := storedHarness(t, store,
		eventfeed.WithSignalHandler(ledger.handler),
		eventfeed.WithObserver(obs))
	h.minter.ScriptTicket(ticket(1))
	resume := testOrigin + "/999/events.json?since=now"
	h.polls.ScriptError(gonePoll(resume))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()

	conn := h.driveToSubscribed()
	h.serveSettled(conn, frameMessage(noFilterIdentifier, 41))
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	if len(gaps) != 1 || gaps[0] != fmt.Sprintf("gap 100 %s", resume) {
		t.Fatalf("Observer.Gap = %v, want exactly one firing carrying the epoch and resume URL", gaps)
	}
	assertInvocations(t, ledger.invocations(), "feedGap/accept")
	calls := h.polls.Calls()
	if len(calls) != 2 {
		t.Fatalf("poll seam calls = %d, want 2", len(calls))
	}
	if calls[1].Cursor != (eventfeed.Cursor{PageURL: resume}) {
		t.Fatalf("re-entry cursor = %+v, want the provided resume URL verbatim", calls[1].Cursor)
	}
	// The resume is a present-class entry: the held position saves only after
	// the buffered straggler is delivered. A position-resume treatment would
	// save first and fail this ledger.
	assertLedger(t, h.ledger(), []string{"event 41", "save pos-1"})
	assertPositions(t, store.Saves(), "pos-1")
	if conn.Closed() {
		t.Fatal("an accepted gap continues the feed on the same socket")
	}
	assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1})
	if got := h.minter.Calls(); got != 1 {
		t.Fatalf("mint seam calls = %d, want 1", got)
	}
}

// TestFeedGapDefaultTerminal is fixture 23: with no handler registered a 410
// is the typed terminal — it never silently auto-continues. Observer.gap still
// fires (observability is not disposition), the resume URL is never requested,
// and nothing is saved.
func TestFeedGapDefaultTerminal(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	var gaps int
	h := storedHarness(t, store, eventfeed.WithObserver(eventfeed.Observer{
		Gap: func(int64, string) { gaps++ },
	}))
	h.minter.ScriptTicket(ticket(1))
	resume := testOrigin + "/999/events.json?since=now"
	h.polls.ScriptError(gonePoll(resume))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"}) // never reached
	base := runtime.NumGoroutine()
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.join()
	assertGoroutinesSettle(t, base)

	_, terminal, elements := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonFeedGap {
		t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonFeedGap)
	}
	if elements != 1 {
		t.Fatalf("iteration elements = %d, want exactly one terminal element", elements)
	}
	if gaps != 1 {
		t.Fatalf("Observer.Gap firings = %d, want 1 (observability is not disposition)", gaps)
	}
	if got := h.polls.CallCount(); got != 1 {
		t.Fatalf("poll seam calls = %d, want 1 (the resume URL is never followed)", got)
	}
	assertPositions(t, store.Saves())
	if got := h.minter.Calls(); got != 1 {
		t.Fatalf("mint seam calls = %d, want 1 (terminal, never retried into)", got)
	}
	if !conn.Closed() {
		t.Fatal("teardown must close the socket")
	}
	assertTimers(t, h.clock, map[string]int{})
}

// TestFeedGapHandlerTerminate is fixture 25: the Terminate branch reaches the
// same visible outcome as fixture 23 — Terminal(feed_gap), no save, the resume
// never followed — and is distinguished from it ONLY by the invocation record.
func TestFeedGapHandlerTerminate(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	ledger := &signalLedger{give: eventfeed.Terminate}
	h := storedHarness(t, store, eventfeed.WithSignalHandler(ledger.handler))
	h.minter.ScriptTicket(ticket(1))
	resume := testOrigin + "/999/events.json?since=now"
	h.polls.ScriptError(gonePoll(resume))
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.join()

	assertInvocations(t, ledger.invocations(), "feedGap/terminate")
	gap, ok := ledger.first().(eventfeed.FeedGap)
	if !ok || gap.EpochAfterID != 100 || gap.ResumeURL != resume {
		t.Fatalf("signal = %+v, want FeedGap{EpochAfterID:100, ResumeURL:%q}", ledger.first(), resume)
	}
	_, terminal, _ := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonFeedGap {
		t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonFeedGap)
	}
	if strings.Contains(terminal.Msg, resume) {
		t.Fatalf("terminal message %q must not carry the resume URL", terminal.Msg)
	}
	if got := h.polls.CallCount(); got != 1 {
		t.Fatalf("poll seam calls = %d, want 1 (Terminate never follows the resume)", got)
	}
	assertPositions(t, store.Saves())
	if !conn.Closed() {
		t.Fatal("teardown must close the socket")
	}
	assertTimers(t, h.clock, map[string]int{})
}

// TestFeedGapAcceptedHostileResumeIsInvalidContinuation is fixture 27: the
// handler accepts — which is what reaches the resume-follow path at all — and
// the cross-origin resume URL then fails §8 validation BEFORE any request.
// The terminal is invalid_continuation, not feed_gap: the handler accepted;
// the URL failed.
func TestFeedGapAcceptedHostileResumeIsInvalidContinuation(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	ledger := &signalLedger{give: eventfeed.Accept}
	h := storedHarness(t, store, eventfeed.WithSignalHandler(ledger.handler))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptError(gonePoll("https://attacker.example.com/999/events.json?since=now&token=secret"))
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.join()

	assertInvocations(t, ledger.invocations(), "feedGap/accept")
	_, terminal, _ := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonInvalidContinuation {
		t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonInvalidContinuation)
	}
	if !strings.Contains(terminal.Msg, "https://attacker.example.com") {
		t.Fatalf("terminal message %q should name the rejected origin", terminal.Msg)
	}
	if strings.Contains(terminal.Msg, "token=secret") || strings.Contains(terminal.Msg, "/999/") {
		t.Fatalf("terminal message %q must be redacted to the origin", terminal.Msg)
	}
	if got := h.polls.CallCount(); got != 1 {
		t.Fatalf("poll seam calls = %d, want 1 (zero requests to the foreign origin)", got)
	}
	assertPositions(t, store.Saves())
	if !conn.Closed() {
		t.Fatal("teardown must close the socket")
	}
	assertTimers(t, h.clock, map[string]int{})
}

// TestFeedGapAcceptedWithNoResumeURLIsInvalidContinuation: accepting a 410
// whose body carried no resume URL cannot be honored — the connector must not
// silently reposition at the present head — so it takes the same
// invalid_continuation edge an unfollowable URL does, before any request.
func TestFeedGapAcceptedWithNoResumeURLIsInvalidContinuation(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	ledger := &signalLedger{give: eventfeed.Accept}
	h := storedHarness(t, store, eventfeed.WithSignalHandler(ledger.handler))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollGone, EpochAfterID: 100})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.join()

	assertInvocations(t, ledger.invocations(), "feedGap/accept")
	_, terminal, _ := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonInvalidContinuation {
		t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonInvalidContinuation)
	}
	if got := h.polls.CallCount(); got != 1 {
		t.Fatalf("poll seam calls = %d, want 1 (no present-class fallback poll)", got)
	}
	assertPositions(t, store.Saves())
	if !conn.Closed() {
		t.Fatal("teardown must close the socket")
	}
	assertTimers(t, h.clock, map[string]int{})
}

// TestRepairPollWalksFromTheInMemoryPosition is fixture 29's repair half: the
// fired `repair-poll` timer drives one walk from the connector's in-memory
// position — authoritative for repair within the run even though its save
// FAILED — delivers and checkpoints the repaired page, returns to Streaming
// with the cadence re-armed, and the next cycle repairs from the position the
// last page moved it to.
func TestRepairPollWalksFromTheInMemoryPosition(t *testing.T) {
	store := feedtest.NewStore() // Missing → present-class entry
	store.FailNextSave(errors.New("disk full"))
	var saveFailures int
	h := storedHarness(t, store, eventfeed.WithObserver(eventfeed.Observer{
		CheckpointSaveFailed: func(error) { saveFailures++ },
	}))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(71)}, Position: "pos-2"})
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-3"})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	if d := h.fireTimer(timerRepairPoll); d < 48*time.Second || d > 72*time.Second {
		t.Fatalf("repair-poll delay = %s, want within 60s ± 20%%", d)
	}
	h.awaitStreaming()
	if d := h.fireTimer(timerRepairPoll); d < 48*time.Second || d > 72*time.Second {
		t.Fatalf("second repair-poll delay = %s, want within 60s ± 20%%", d)
	}
	h.awaitStreaming()

	calls := h.polls.Calls()
	if len(calls) != 3 {
		t.Fatalf("poll seam calls = %d, want 3 (entry + two repair walks)", len(calls))
	}
	if calls[1].Cursor != (eventfeed.Cursor{Position: "pos-1"}) {
		t.Fatalf("first repair cursor = %+v, want the in-memory position (the failed save never regresses it)", calls[1].Cursor)
	}
	if calls[2].Cursor != (eventfeed.Cursor{Position: "pos-2"}) {
		t.Fatalf("second repair cursor = %+v, want the position the repaired page moved it to", calls[2].Cursor)
	}
	if saveFailures != 1 {
		t.Fatalf("CheckpointSaveFailed firings = %d, want 1", saveFailures)
	}
	assertIDs(t, h.deliveredIDs(), 71)
	assertPositions(t, store.Saves(), "pos-1", "pos-2", "pos-3")
	if got := h.minter.Calls(); got != 1 {
		t.Fatalf("mint seam calls = %d, want 1 (a repair walk keeps the socket)", got)
	}
	assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1})
}

// TestRepairWalkDedupesAgainstStreamedDeliveries: a repair page re-serving an
// id streaming already delivered is suppressed by the delivered-id LRU — the
// rule is symmetric across lanes — while the page's own position still
// advances (a position is a poll cursor, not an event acknowledgment).
func TestRepairWalkDedupesAgainstStreamedDeliveries(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.polls.ScriptPage(eventfeed.PollPage{
		Events:   []eventfeed.Event{pollEvent(81), pollEvent(82)},
		Position: "pos-2",
	})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()
	conn.Serve(frameMessage(noFilterIdentifier, 81))
	h.waitUntil("live delivery", func() bool { return len(h.deliveredIDs()) == 1 })

	h.fireTimer(timerRepairPoll)
	h.awaitStreaming()

	assertIDs(t, h.deliveredIDs(), 81, 82)
	assertPositions(t, store.Saves(), "pos-1", "pos-2")
}

// TestRepairWalkBuffersLiveFramesAndDrainsThem: while a repair walk runs the
// state is CatchingUp, whose delivery invariant is poll pages only — a live
// frame arriving mid-walk is admitted to the buffer and replayed on the way
// back through Draining, after the walk's own pages.
func TestRepairWalkBuffersLiveFramesAndDrainsThem(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	next := testOrigin + "/999/events.json?after=91"
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.polls.ScriptPage(eventfeed.PollPage{
		Events:   []eventfeed.Event{pollEvent(91)},
		Position: "pos-2",
		Next:     next,
	})
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(92)}, Position: "pos-3"})

	var conn *feedtest.Conn
	h.polls.OnCall(func(call feedtest.PollCall) {
		if call.Cursor != (eventfeed.Cursor{Position: "pos-1"}) {
			return
		}
		// Inside the repair walk's first seam call: serve a live event and wait
		// for the pump to take it, so it is queued for the state machine before
		// the page is accepted.
		conn.Serve(frameMessage(noFilterIdentifier, 55))
		conn.Serve(framePing())
		deadline := time.Now().Add(watchdog)
		for conn.Pending() > 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
	})
	h.start()

	conn = h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()
	h.fireTimer(timerRepairPoll)
	h.awaitStreaming()

	assertLedger(t, h.ledger(), []string{
		"save pos-1",
		"event 91", "save pos-2",
		"event 92", "save pos-3",
		"event 55",
	})
}

// TestPositionInvalidReEntersAtLastPollServedID is transition 18: a
// 400-position re-enters at `since=<last poll-served id>` on the SAME socket,
// keeping the checkpoints the walk already saved.
func TestPositionInvalidReEntersAtLastPollServedID(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	var rejected []eventfeed.PollErrorKind
	h := storedHarness(t, store, eventfeed.WithObserver(eventfeed.Observer{
		PositionRejected: func(kind eventfeed.PollErrorKind) { rejected = append(rejected, kind) },
	}))
	h.minter.ScriptTicket(ticket(1))
	next := testOrigin + "/999/events.json?after=102"
	h.polls.ScriptPage(eventfeed.PollPage{
		Events:   []eventfeed.Event{pollEvent(101), pollEvent(102)},
		Position: "pos-1",
		Next:     next,
	})
	h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollPositionInvalid})
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(103)}, Position: "pos-2"})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	calls := h.polls.Calls()
	if len(calls) != 3 {
		t.Fatalf("poll seam calls = %d, want 3", len(calls))
	}
	if calls[2].Cursor != (eventfeed.Cursor{Since: "102"}) {
		t.Fatalf("re-entry cursor = %+v, want since=<last poll-served id>", calls[2].Cursor)
	}
	if len(rejected) != 1 || rejected[0] != eventfeed.PollPositionInvalid {
		t.Fatalf("Observer.PositionRejected = %v, want one position_invalid", rejected)
	}
	assertIDs(t, h.deliveredIDs(), 101, 102, 103)
	assertPositions(t, store.Saves(), "pos-1", "pos-2")
	if got := h.minter.Calls(); got != 1 {
		t.Fatalf("mint seam calls = %d, want 1 (a re-entry keeps the socket)", got)
	}
}

// TestPositionInvalidWithNoPollServedIDReEntersPresentClass: with no id ever
// served by the POLL lane, the re-entry falls back to `since=now` — a
// present-class entry, so its position is held until the buffered live event
// has been delivered. A live-delivered id is never a reset cursor.
func TestPositionInvalidWithNoPollServedIDReEntersPresentClass(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollPositionInvalid})
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()

	conn := h.driveToSubscribed()
	// Admitted live BEFORE the re-entry: its id must not position the re-entry.
	h.serveSettled(conn, frameMessage(noFilterIdentifier, 41))
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	calls := h.polls.Calls()
	if len(calls) != 2 {
		t.Fatalf("poll seam calls = %d, want 2", len(calls))
	}
	if calls[1].Cursor != (eventfeed.Cursor{Since: "now"}) {
		t.Fatalf("re-entry cursor = %+v, want the present-class since=now fallback", calls[1].Cursor)
	}
	assertLedger(t, h.ledger(), []string{"event 41", "save pos-1"})
}

// TestResetCursorIgnoresLiveDeliveredIDs: the reset cursor is POLL-LANE-ONLY.
// A repair walk whose 400-position lands after streaming has DELIVERED a live
// id still re-enters present-class, because the poll lane has served no id —
// re-entering at the live id would skip the un-polled gap behind it forever.
func TestResetCursorIgnoresLiveDeliveredIDs(t *testing.T) {
	h := newHarness(t)
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"}) // entry: serves no id
	h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollPositionInvalid})
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-2"})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()
	conn.Serve(frameMessage(noFilterIdentifier, 55))
	h.waitUntil("live delivery", func() bool { return len(h.deliveredIDs()) == 1 })

	h.fireTimer(timerRepairPoll)
	h.awaitStreaming()

	calls := h.polls.Calls()
	if len(calls) != 3 {
		t.Fatalf("poll seam calls = %d, want 3", len(calls))
	}
	if calls[1].Cursor != (eventfeed.Cursor{Position: "pos-1"}) {
		t.Fatalf("repair cursor = %+v, want the in-memory position", calls[1].Cursor)
	}
	if calls[2].Cursor != (eventfeed.Cursor{Since: "now"}) {
		t.Fatalf("re-entry cursor = %+v, want since=now — a live-delivered id is never a reset cursor", calls[2].Cursor)
	}
	assertIDs(t, h.deliveredIDs(), 55)
}

// TestFilterChangedDiscardsTheHeldPosition is transition 19: a 409 re-enters
// like a 400-position AND discards the held position — so an attempt torn down
// before the re-entry serves a page reconnects at the present rather than back
// at the position the server just refused. With no poll-served id the reset
// cursor IS the present, and the reconnect re-enters at the latched
// `since=now` spelling rather than the bare entry: §23 treats the two
// identically ("`since=now` — and the bare present entry, which the server
// treats identically"), and re-entering at the LATCHED cursor is what keeps
// the poll-served-id case (below) from decaying into a present-class jump.
func TestFilterChangedDiscardsTheHeldPosition(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	var rejected []eventfeed.PollErrorKind
	h := storedHarness(t, store, eventfeed.WithObserver(eventfeed.Observer{
		PositionRejected: func(kind eventfeed.PollErrorKind) { rejected = append(rejected, kind) },
	}))
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollFilterChanged})
	h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollTransient, Err: errors.New("502")})
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()

	conn1 := h.driveToSubscribed()
	conn1.Serve(frameConfirm(noFilterIdentifier))
	// The re-entry poll fails transiently, parking the walk on `poll-retry`;
	// the socket then dies, so no page is ever accepted at the new cursor.
	h.awaitTimer(timerPollRetry)
	conn1.FailReads(errors.New("connection reset"))
	h.awaitTimer(timerBackoff)
	h.fireTimer(timerBackoff)

	conn2 := h.driveToSubscribed()
	conn2.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	calls := h.polls.Calls()
	if len(calls) != 3 {
		t.Fatalf("poll seam calls = %d, want 3", len(calls))
	}
	if calls[1].Cursor != (eventfeed.Cursor{Since: "now"}) {
		t.Fatalf("re-entry cursor = %+v, want the present-class since=now fallback", calls[1].Cursor)
	}
	if calls[2].Cursor != (eventfeed.Cursor{Since: "now"}) {
		t.Fatalf("reconnect entry cursor = %+v, want the latched present-class reset (the 409 discarded the position)", calls[2].Cursor)
	}
	if len(rejected) != 1 || rejected[0] != eventfeed.PollFilterChanged {
		t.Fatalf("Observer.PositionRejected = %v, want one filter_changed", rejected)
	}
	assertPositions(t, store.Saves(), "pos-1")
}

// TestAuthorizationCounterCountsAllThreeShapes: the shared connection-level
// counter is one counter over three failure shapes — an unauthorized mint, an
// `unauthorized` disconnect at connect, and an unauthorized poll — so a MIXED
// sequence reaches the threshold-3 terminal.
func TestAuthorizationCounterCountsAllThreeShapes(t *testing.T) {
	h := newHarness(t)
	h.minter.ScriptError(&eventfeed.MintError{Kind: eventfeed.MintUnauthorized}) // 1
	h.minter.ScriptTicket(ticket(2))
	h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollUnauthorized, Err: errors.New("401")}) // 2
	h.minter.ScriptTicket(ticket(3))
	h.start()

	h.fireTimer(timerBackoff)

	conn2 := h.driveToSubscribed()
	conn2.Serve(frameConfirm(noFilterIdentifier))
	h.fireTimer(timerBackoff)

	conn3 := h.liveConn()
	conn3.Serve(frameDisconnect("unauthorized", false)) // 3, pre-welcome
	h.join()

	_, terminal, _ := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonAuthorizationFailed {
		t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonAuthorizationFailed)
	}
	if got := h.minter.Calls(); got != 3 {
		t.Fatalf("mint seam calls = %d, want 3", got)
	}
	assertTimers(t, h.clock, map[string]int{})
}

// TestPollRetryIndexGrowsAndResets pins the consecutive-poll-failure index k
// behind the `poll-retry` timer: the full-jitter envelope doubles per
// consecutive failure, and a successful page resets the index — a counter
// separate from the reconnect cycle's.
func TestPollRetryIndexGrowsAndResets(t *testing.T) {
	h := newHarness(t)
	h.conn.SetRand(func() float64 { return 0.5 })
	h.minter.ScriptTicket(ticket(1))
	transient := func() error {
		return &eventfeed.PollError{Kind: eventfeed.PollTransient, Err: errors.New("502")}
	}
	next := testOrigin + "/999/events.json?after=101"
	h.polls.ScriptError(transient()) // k=1
	h.polls.ScriptError(transient()) // k=2
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(101)}, Position: "pos-1", Next: next})
	h.polls.ScriptError(transient()) // k=1 again: the page reset it
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-2"})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))

	if d := h.fireTimer(timerPollRetry); d != 500*time.Millisecond {
		t.Fatalf("poll-retry delay (k=1) = %s, want 0.5 × 1s", d)
	}
	if d := h.fireTimer(timerPollRetry); d != time.Second {
		t.Fatalf("poll-retry delay (k=2) = %s, want 0.5 × 2s", d)
	}
	if d := h.fireTimer(timerPollRetry); d != 500*time.Millisecond {
		t.Fatalf("poll-retry delay after a successful page = %s, want the index reset to k=1", d)
	}
	h.awaitStreaming()
	assertIDs(t, h.deliveredIDs(), 101)
}

// TestResetCursorSurvivesAReconnect is the 409's reset cursor as RECONNECT
// state rather than walk-local state. Transition 19 discards the position, so
// once the socket dies before the re-entry has served a page there is nothing
// left in memory to re-enter from and the entry falls back to the configured
// start mode — the bare present for a resume feed. That silently skips
// everything between the last poll-served id and the present, on the one path
// whose whole purpose was not to skip anything. The cursor is chosen from run
// state (`lastPollServedID`) no later entry re-derives, so it has to be
// latched until a page replaces it.
func TestResetCursorSurvivesAReconnect(t *testing.T) {
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
	h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollFilterChanged})
	h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollTransient, Err: errors.New("502")})
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-2"})
	h.start()

	conn1 := h.driveToSubscribed()
	conn1.Serve(frameConfirm(noFilterIdentifier))
	// The re-entry poll fails transiently, parking the walk on `poll-retry`;
	// the socket then dies, so no page is ever accepted at the reset cursor.
	h.awaitTimer(timerPollRetry)
	conn1.FailReads(errors.New("connection reset"))
	h.awaitTimer(timerBackoff)
	h.fireTimer(timerBackoff)

	conn2 := h.driveToSubscribed()
	conn2.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	calls := h.polls.Calls()
	if len(calls) != 4 {
		t.Fatalf("poll seam calls = %d, want 4", len(calls))
	}
	if calls[2].Cursor != (eventfeed.Cursor{Since: "101"}) {
		t.Fatalf("re-entry cursor = %+v, want the poll-served reset cursor", calls[2].Cursor)
	}
	if calls[3].Cursor != (eventfeed.Cursor{Since: "101"}) {
		t.Fatalf("reconnect cursor = %+v, want the latched reset cursor, not a jump to the present", calls[3].Cursor)
	}
	assertPositions(t, store.Saves(), "pos-1", "pos-2")
}

// TestAcceptedGapResumeSurvivesAReconnect is the same latch from the 410 side,
// plus the discard the Accept implies. The position the 410 refused is
// UNUSABLE, not merely superseded: left in memory it is what the next
// connection's entry selects, drawing the same 410 and re-invoking a gap
// handler that has already been asked and has already answered Accept. The
// accepted resume URL takes its place until a page replaces it, so the
// present-class reset the consumer accepted is what actually continues.
func TestAcceptedGapResumeSurvivesAReconnect(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	ledger := &signalLedger{give: eventfeed.Accept}
	h := storedHarness(t, store, eventfeed.WithSignalHandler(ledger.handler))
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	resume := testOrigin + "/999/events.json?since=now"
	h.polls.ScriptError(gonePoll(resume))
	h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollTransient, Err: errors.New("502")})
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()

	conn1 := h.driveToSubscribed()
	conn1.Serve(frameConfirm(noFilterIdentifier))
	h.awaitTimer(timerPollRetry)
	conn1.FailReads(errors.New("connection reset"))
	h.awaitTimer(timerBackoff)
	h.fireTimer(timerBackoff)

	conn2 := h.driveToSubscribed()
	conn2.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	calls := h.polls.Calls()
	if len(calls) != 3 {
		t.Fatalf("poll seam calls = %d, want 3", len(calls))
	}
	if calls[1].Cursor != (eventfeed.Cursor{PageURL: resume}) {
		t.Fatalf("re-entry cursor = %+v, want the provided resume URL", calls[1].Cursor)
	}
	if calls[2].Cursor != (eventfeed.Cursor{PageURL: resume}) {
		t.Fatalf("reconnect cursor = %+v, want the latched resume URL, not the position the 410 refused", calls[2].Cursor)
	}
	// The handler is asked once, and the accepted disposition is not
	// re-litigated by a reconnect.
	assertInvocations(t, ledger.invocations(), "feedGap/accept")
	assertPositions(t, store.Saves(), "pos-1")
}
