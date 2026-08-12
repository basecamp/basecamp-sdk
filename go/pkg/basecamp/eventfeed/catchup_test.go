// Slice-4b acceptance tests: the catch-up walk, the entry boundary's
// ownership cut and conjunctive save ordering, the drain, and streaming
// steady state (SPEC.md §23 transitions 16/20–26 plus the out-of-inventory
// checkpoint_load / invalid_continuation / poll_failed edges). Structured to
// mirror the tier-2 scenario fixtures 01, 12, 19–22, 24, 26, 28, 29 so the
// fixture driver can later subsume them; all time flows through the injected
// feedtest.Clock, and delivery-vs-save ordering is asserted on one ledger
// both sides write from the consumer's goroutine.
package eventfeed_test

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed/feedtest"
)

// pollEvent builds one poll-lane row: the eight always-present keys, and no
// visible_to_clients (poll rows omit it — the presence asymmetry).
func pollEvent(id int64) eventfeed.Event {
	return eventfeed.Event{
		ID:          id,
		Kind:        "message",
		EventType:   "message.created",
		Action:      "created",
		CreatedAt:   time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
		BucketID:    2,
		CreatorID:   3,
		RecordingID: 900,
	}
}

// storedHarness wires a harness to a scripted checkpoint store recording its
// saves onto the shared ledger, so "delivery strictly precedes this page's
// save" is a statement about one ordered list.
func storedHarness(t *testing.T, store *feedtest.Store, opts ...eventfeed.Option) *harness {
	t.Helper()
	base := make([]eventfeed.Option, 0, 2+len(opts))
	base = append(base,
		eventfeed.WithCheckpointStore(store),
		eventfeed.WithConsumerNamespace("agent"))
	h := newHarness(t, append(base, opts...)...)
	store.OnSave(func(position string) { h.record("save " + position) })
	return h
}

func assertLedger(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ledger = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ledger = %v, want %v", got, want)
		}
	}
}

func assertIDs(t *testing.T, got []int64, want ...int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("delivered ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("delivered ids = %v, want %v", got, want)
		}
	}
}

func assertPositions(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("checkpoint ledger = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("checkpoint ledger = %v, want %v", got, want)
		}
	}
}

// TestCatchUpWalkCheckpointsAfterEachPage is fixtures 12 + 01: a
// position-resume entry walks the pages, delivering each page's events BEFORE
// that page's position is saved, follows `next` verbatim, drains the live
// buffer after the walk, and then streams — with live ids never advancing the
// durable position and a re-pushed poll id suppressed by the dedupe LRU.
func TestCatchUpWalkCheckpointsAfterEachPage(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	next := testOrigin + "/999/events.json?after=102"
	h.polls.ScriptPage(eventfeed.PollPage{
		Events:   []eventfeed.Event{pollEvent(101), pollEvent(102)},
		Position: "pos-1",
		Next:     next,
	})
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(103)}, Position: "pos-2"})
	base := runtime.NumGoroutine()
	h.start()

	conn := h.driveToSubscribed()
	h.serveSettled(conn, frameMessage(noFilterIdentifier, 104))
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	assertLedger(t, h.ledger(), []string{
		"event 101", "event 102", "save pos-1",
		"event 103", "save pos-2",
		"event 104",
	})
	assertPositions(t, store.Saves(), "pos-1", "pos-2")

	calls := h.polls.Calls()
	if len(calls) != 2 {
		t.Fatalf("poll seam calls = %d, want 2", len(calls))
	}
	if calls[0].Cursor != (eventfeed.Cursor{Position: "pos-0"}) {
		t.Fatalf("entry cursor = %+v, want the loaded position", calls[0].Cursor)
	}
	if calls[1].Cursor != (eventfeed.Cursor{PageURL: next}) {
		t.Fatalf("continuation cursor = %+v, want the `next` URL verbatim", calls[1].Cursor)
	}
	assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1})

	// Streaming: a fresh live id is delivered; a re-push of a poll-served id
	// is suppressed by delivered-id dedupe; neither saves.
	conn.Serve(frameMessage(noFilterIdentifier, 105))
	h.waitUntil("live delivery", func() bool { return len(h.deliveredIDs()) == 5 })
	h.serveSettled(conn, frameMessage(noFilterIdentifier, 103))
	conn.Serve(frameMessage(noFilterIdentifier, 106))
	h.waitUntil("post-duplicate delivery", func() bool { return len(h.deliveredIDs()) == 6 })
	assertIDs(t, h.deliveredIDs(), 101, 102, 103, 104, 105, 106)
	assertPositions(t, store.Saves(), "pos-1", "pos-2")

	h.conn.Close()
	h.join()
	assertTimers(t, h.clock, map[string]int{})
	assertGoroutinesSettle(t, base)
}

// TestPresentEntryHoldsPositionUntilDrained is fixture 19: on a present-class
// entry the entry poll's position is HELD, the admitted straggler is
// delivered, and only then does the position save — the conjunctive
// save-ordering invariant. A save landing first would fail the ledger.
func TestPresentEntryHoldsPositionUntilDrained(t *testing.T) {
	store := feedtest.NewStore() // Missing → present-class entry
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()

	conn := h.driveToSubscribed()
	h.serveSettled(conn, frameMessage(noFilterIdentifier, 41))
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	assertLedger(t, h.ledger(), []string{"event 41", "save pos-1"})
	assertIDs(t, h.deliveredIDs(), 41)
	assertPositions(t, store.Saves(), "pos-1")
	if calls := h.polls.Calls(); len(calls) != 1 || calls[0].Cursor != (eventfeed.Cursor{}) {
		t.Fatalf("poll calls = %+v, want one bare present entry", calls)
	}
	assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1})
}

// TestOwnershipCutAdmitsFramesArrivingDuringTheEntryPoll pins the cut's
// observation rule: a straggler that arrives AFTER confirmation and while the
// entry poll is in flight counts as observed — admitted into the
// state-machine-owned buffer at or before the cut, whether the state machine
// admits it while the seam call is outstanding or in the bounded pass taken
// once the response is accepted — so it is delivered before the held position
// saves.
func TestOwnershipCutAdmitsFramesArrivingDuringTheEntryPoll(t *testing.T) {
	store := feedtest.NewStore()
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})

	var conn *feedtest.Conn
	handed := make(chan struct{})
	h.polls.OnCall(func(feedtest.PollCall) {
		// Inside the entry poll seam call: serve the straggler plus a
		// trailing ping and wait for the pump to take both, so the frame is
		// queued for the state machine before the response is accepted.
		conn.Serve(frameMessage(noFilterIdentifier, 41))
		conn.Serve(framePing())
		deadline := time.Now().Add(watchdog)
		for conn.Pending() > 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		close(handed)
	})
	h.start()

	conn = h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	select {
	case <-handed:
	case <-time.After(watchdog):
		t.Fatal("the entry poll was never reached")
	}
	h.awaitStreaming()

	assertLedger(t, h.ledger(), []string{"event 41", "save pos-1"})
	assertPositions(t, store.Saves(), "pos-1")
}

// TestPostSnapshotStragglerDeliveredAndDeduped is fixture 20: a lower-id
// straggler arriving after the snapshot and its save is still delivered — the
// LRU tracks delivered ids, never position ordering — a repeat is suppressed,
// and the position never regresses.
func TestPostSnapshotStragglerDeliveredAndDeduped(t *testing.T) {
	store := feedtest.NewStore()
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()
	assertPositions(t, store.Saves(), "pos-1")

	conn.Serve(frameMessage(noFilterIdentifier, 41))
	h.waitUntil("straggler delivered", func() bool { return len(h.deliveredIDs()) == 1 })
	h.serveSettled(conn, frameMessage(noFilterIdentifier, 41))
	conn.Serve(frameMessage(noFilterIdentifier, 42))
	h.waitUntil("second delivery", func() bool { return len(h.deliveredIDs()) == 2 })

	assertIDs(t, h.deliveredIDs(), 41, 42)
	assertPositions(t, store.Saves(), "pos-1")
}

// TestOverflowAcceptedStillDeliversRetained is fixture 22: accepting a
// BufferOverflow is not license to skip retained deliveries — the retained
// pre-cut events complete BEFORE the held entry position saves.
func TestOverflowAcceptedStillDeliversRetained(t *testing.T) {
	var invocations int
	handler := func(s eventfeed.Signal) eventfeed.Disposition {
		ov, ok := s.(eventfeed.BufferOverflow)
		if !ok || ov.DroppedCount != 1 || len(ov.DroppedIDs) != 1 || ov.DroppedIDs[0] != 51 {
			t.Errorf("signal = %+v, want BufferOverflow{DroppedIDs:[51], DroppedCount:1}", s)
		}
		invocations++
		return eventfeed.Accept
	}
	store := feedtest.NewStore()
	h := storedHarness(t, store,
		eventfeed.WithLiveBufferCapacity(2),
		eventfeed.WithSignalHandler(handler))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()

	conn := h.driveToSubscribed()
	h.serveSettled(conn,
		frameMessage(noFilterIdentifier, 51),
		frameMessage(noFilterIdentifier, 52),
		frameMessage(noFilterIdentifier, 53))
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	assertLedger(t, h.ledger(), []string{"event 52", "event 53", "save pos-1"})
	assertPositions(t, store.Saves(), "pos-1")
	if invocations != 1 {
		t.Fatalf("handler invocations = %d, want exactly 1", invocations)
	}
}

// TestOverflowDefaultTerminalNeverSaves is fixture 21: with no handler the
// overflow is terminal — and structurally there is no entry poll and no save.
func TestOverflowDefaultTerminalNeverSaves(t *testing.T) {
	store := feedtest.NewStore()
	h := storedHarness(t, store, eventfeed.WithLiveBufferCapacity(2))
	h.minter.ScriptTicket(ticket(1))
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameMessage(noFilterIdentifier, 51))
	conn.Serve(frameMessage(noFilterIdentifier, 52))
	conn.Serve(frameMessage(noFilterIdentifier, 53))
	h.join()

	_, terminal, _ := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonBufferOverflow {
		t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonBufferOverflow)
	}
	if got := h.polls.CallCount(); got != 0 {
		t.Fatalf("poll seam calls = %d, want 0", got)
	}
	assertPositions(t, store.Saves())
	if !conn.Closed() {
		t.Fatal("teardown must close the socket")
	}
	assertTimers(t, h.clock, map[string]int{})
}

// TestCheckpointLoadFailureIsTerminal is fixture 28: Failed(load) is
// Terminal(checkpoint_load) with ZERO wire attempts, and is deliberately not
// collapsible to Missing (which would silently start at the present).
func TestCheckpointLoadFailureIsTerminal(t *testing.T) {
	sentinel := errors.New("store unreadable")
	store := feedtest.NewStore()
	store.FailLoad(sentinel)
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	h.start()
	h.join()

	_, terminal, elements := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonCheckpointLoad {
		t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonCheckpointLoad)
	}
	if !errors.Is(terminal, sentinel) {
		t.Fatalf("terminal must wrap the store error; got %v", terminal)
	}
	if elements != 1 {
		t.Fatalf("iteration elements = %d, want exactly the one error element", elements)
	}
	if got := h.minter.Calls(); got != 0 {
		t.Fatalf("mint seam calls = %d, want 0 (load precedes the first mint)", got)
	}
	if got := len(h.tr.Dials()); got != 0 {
		t.Fatalf("connect seam calls = %d, want 0", got)
	}
	if got := h.polls.CallCount(); got != 0 {
		t.Fatalf("poll seam calls = %d, want 0", got)
	}
	assertTimers(t, h.clock, map[string]int{})
}

// TestCheckpointIdentityAndEntrySelection: load happens exactly once under
// the full four-part identity, Missing enters present-class, Loaded enters at
// the stored position, and saves carry the same key.
func TestCheckpointIdentityAndEntrySelection(t *testing.T) {
	filters := eventfeed.Filters{Types: []string{"message.created"}}
	wantKey := eventfeed.CheckpointKey{
		Origin:            testOrigin,
		AccountID:         "5951425",
		ConsumerNamespace: "agent",
		FilterKey:         filters.FilterKey(),
	}

	t.Run("missing enters present-class", func(t *testing.T) {
		store := feedtest.NewStore()
		h := storedHarness(t, store, eventfeed.WithFilters(filters))
		h.minter.ScriptTicket(ticket(1))
		h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
		h.start()

		conn := h.driveToSubscribed()
		conn.Serve(frameConfirm(eventfeed.ExportSubscribeIdentifier(filters)))
		b := h.awaitBoundary()
		if b.Entry != (eventfeed.Cursor{}) || !b.PresentClass {
			t.Fatalf("entry = %+v present=%v, want the bare present entry", b.Entry, b.PresentClass)
		}
		h.awaitStreaming()
		loads := store.Loads()
		if len(loads) != 1 || loads[0] != wantKey {
			t.Fatalf("load keys = %+v, want exactly one %+v", loads, wantKey)
		}
		keys := store.SaveKeys()
		if len(keys) != 1 || keys[0] != wantKey {
			t.Fatalf("save keys = %+v, want exactly one %+v", keys, wantKey)
		}
	})

	t.Run("loaded enters at the stored position", func(t *testing.T) {
		store := feedtest.NewStore()
		store.Stored("pos-9")
		h := storedHarness(t, store, eventfeed.WithFilters(filters))
		h.minter.ScriptTicket(ticket(1))
		h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-10"})
		h.start()

		conn := h.driveToSubscribed()
		conn.Serve(frameConfirm(eventfeed.ExportSubscribeIdentifier(filters)))
		b := h.awaitBoundary()
		if b.Entry != (eventfeed.Cursor{Position: "pos-9"}) || b.PresentClass {
			t.Fatalf("entry = %+v present=%v, want the stored position, position-resume class", b.Entry, b.PresentClass)
		}
	})
}

// TestCheckpointSaveFailureContinues is fixture 29's store half: a failed
// save is an observer notification, the feed continues, and the NEXT save is
// still attempted — there is no save circuit breaker.
func TestCheckpointSaveFailureContinues(t *testing.T) {
	sentinel := errors.New("disk full")
	store := feedtest.NewStore()
	store.Stored("pos-0")
	store.FailNextSave(sentinel)

	var saveFailures []error
	var checkpoints []string
	obs := eventfeed.Observer{
		CheckpointSaveFailed: func(err error) { saveFailures = append(saveFailures, err) },
		Checkpoint:           func(position string) { checkpoints = append(checkpoints, position) },
	}
	h := storedHarness(t, store, eventfeed.WithObserver(obs))
	h.minter.ScriptTicket(ticket(1))
	next := testOrigin + "/999/events.json?after=101"
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(101)}, Position: "pos-1", Next: next})
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(102)}, Position: "pos-2"})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	assertIDs(t, h.deliveredIDs(), 101, 102)
	assertPositions(t, store.Saves(), "pos-1", "pos-2")
	if len(saveFailures) != 1 || !errors.Is(saveFailures[0], sentinel) {
		t.Fatalf("CheckpointSaveFailed = %v, want exactly the one store error", saveFailures)
	}
	if len(checkpoints) != 1 || checkpoints[0] != "pos-2" {
		t.Fatalf("Checkpoint = %v, want only the position that durably saved", checkpoints)
	}
}

// TestHostileContinuationIsTerminal is fixture 26: a cross-origin `next`
// terminates with invalid_continuation, no request is issued to the foreign
// origin, and the error carries the rejected URL redacted to its origin.
func TestHostileContinuationIsTerminal(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{
		Events:   []eventfeed.Event{pollEvent(101)},
		Position: "pos-1",
		Next:     "https://attacker.example.com/999/events.json?after=101&token=secret",
	})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.join()

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
	assertIDs(t, h.deliveredIDs(), 101)
	assertPositions(t, store.Saves(), "pos-1")
	if !conn.Closed() {
		t.Fatal("teardown must close the socket")
	}
	assertTimers(t, h.clock, map[string]int{})
}

// TestContinuationValidation covers the rejected shapes (cross-origin,
// protocol downgrade, non-http(s) scheme, relative) and the accepted one (a
// same-origin absolute URL with a different path and query).
func TestContinuationValidation(t *testing.T) {
	rejected := []struct {
		name string
		next string
	}{
		{"cross-origin host", "https://attacker.example.com/events.json"},
		{"protocol downgrade", "http://3.basecampapi.com/999/events.json"},
		{"foreign port", "https://3.basecampapi.com:8443/999/events.json"},
		{"non-http scheme", "file:///etc/passwd"},
		{"relative", "/999/events.json?after=101"},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.minter.ScriptTicket(ticket(1))
			h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1", Next: tc.next})
			h.start()

			conn := h.driveToSubscribed()
			conn.Serve(frameConfirm(noFilterIdentifier))
			h.join()

			_, terminal, _ := h.snapshot()
			if terminal == nil || terminal.Reason != eventfeed.ReasonInvalidContinuation {
				t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonInvalidContinuation)
			}
			if got := h.polls.CallCount(); got != 1 {
				t.Fatalf("poll seam calls = %d, want 1 (the failing URL is never requested)", got)
			}
		})
	}

	t.Run("same-origin continuation is followed", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptTicket(ticket(1))
		next := testOrigin + "/999/events.json?after=101&filter=x"
		h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(101)}, Position: "pos-1", Next: next})
		h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(102)}, Position: "pos-2"})
		h.start()

		conn := h.driveToSubscribed()
		conn.Serve(frameConfirm(noFilterIdentifier))
		h.awaitStreaming()
		assertIDs(t, h.deliveredIDs(), 101, 102)
		if calls := h.polls.Calls(); len(calls) != 2 || calls[1].Cursor.PageURL != next {
			t.Fatalf("poll calls = %+v, want the continuation followed verbatim", calls)
		}
	})
}

// TestPollRetryTiming: transient and throttled poll failures retry inside
// CatchingUp on the `poll-retry` timer at the SAME cursor — never terminal —
// with a server-directed Retry-After waited exactly and the local draw
// bounded by the consecutive-poll-failure index.
func TestPollRetryTiming(t *testing.T) {
	t.Run("transient retries the same cursor", func(t *testing.T) {
		store := feedtest.NewStore()
		store.Stored("pos-0")
		h := storedHarness(t, store)
		h.minter.ScriptTicket(ticket(1))
		h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollTransient, Err: errors.New("502")})
		h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(101)}, Position: "pos-1"})
		h.start()

		conn := h.driveToSubscribed()
		conn.Serve(frameConfirm(noFilterIdentifier))
		h.awaitTimer(timerPollRetry)
		assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerPollRetry: 1})
		if d := h.fireTimer(timerPollRetry); d < 0 || d >= time.Second {
			t.Fatalf("first poll-retry delay = %s, want within [0, 1s) (full jitter, k=1)", d)
		}
		h.awaitStreaming()

		calls := h.polls.Calls()
		if len(calls) != 2 || calls[0].Cursor != calls[1].Cursor {
			t.Fatalf("poll calls = %+v, want the retry at the same cursor", calls)
		}
		assertIDs(t, h.deliveredIDs(), 101)
		if _, terminal, _ := h.snapshot(); terminal != nil {
			t.Fatalf("a transient poll is never terminal; got %v", terminal)
		}
	})

	t.Run("throttled waits the Retry-After exactly", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptTicket(ticket(1))
		h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollThrottled, RetryAfter: 90 * time.Second})
		h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
		h.start()

		conn := h.driveToSubscribed()
		conn.Serve(frameConfirm(noFilterIdentifier))
		if d := h.fireTimer(timerPollRetry); d != 90*time.Second {
			t.Fatalf("poll-retry delay = %s, want the 90s Retry-After exactly (cap-exempt)", d)
		}
		h.awaitStreaming()
	})
}

// TestPollFailureClassification: the terminal poll dispositions this slice
// owns — filter_invalid with the server's message preserved, poll_failed for
// unrecoverable and unclassified outcomes, and invalid_continuation for a
// refused redirect (never poll_failed).
func TestPollFailureClassification(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		reason  eventfeed.TerminalReason
		wantMsg string
	}{
		{
			"filter invalid preserves the server message",
			&eventfeed.PollError{Kind: eventfeed.PollFilterInvalid, Msg: "unknown event types: nope.created"},
			eventfeed.ReasonFilterInvalid,
			"unknown event types: nope.created",
		},
		{
			"unrecoverable is poll_failed",
			&eventfeed.PollError{Kind: eventfeed.PollUnrecoverable, Err: errors.New("404 not found")},
			eventfeed.ReasonPollFailed,
			"",
		},
		{
			"unclassified is poll_failed",
			errors.New("adapter bug: unclassified"),
			eventfeed.ReasonPollFailed,
			"",
		},
		{
			"an unknown kind is poll_failed, never a guessed re-entry",
			&eventfeed.PollError{Err: errors.New("adapter bug: unset kind")},
			eventfeed.ReasonPollFailed,
			"unknown poll error kind 0",
		},
		{
			"redirect refused is invalid_continuation",
			&eventfeed.PollError{Kind: eventfeed.PollRedirectRefused, LocationOrigin: "https://attacker.example.com"},
			eventfeed.ReasonInvalidContinuation,
			"https://attacker.example.com",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.minter.ScriptTicket(ticket(1))
			h.polls.ScriptError(tc.err)
			h.start()

			conn := h.driveToSubscribed()
			conn.Serve(frameConfirm(noFilterIdentifier))
			h.join()

			_, terminal, _ := h.snapshot()
			if terminal == nil || terminal.Reason != tc.reason {
				t.Fatalf("terminal = %v, want reason %q", terminal, tc.reason)
			}
			if tc.wantMsg != "" && !strings.Contains(terminal.Msg, tc.wantMsg) {
				t.Fatalf("terminal message %q should carry %q", terminal.Msg, tc.wantMsg)
			}
			if got := h.minter.Calls(); got != 1 {
				t.Fatalf("mint seam calls = %d, want 1 (terminal, never retried into)", got)
			}
			if !conn.Closed() {
				t.Fatal("teardown must close the socket")
			}
			assertTimers(t, h.clock, map[string]int{})
		})
	}
}

// TestUnauthorizedPollRidesReconnectCycle: an unauthorized poll tears the
// attempt down to Backoff (the fresh mint/token cycle is its recovery path)
// and increments the SHARED authorization counter — while a successful poll
// page is the one thing that resets it.
func TestUnauthorizedPollRidesReconnectCycle(t *testing.T) {
	unauthorizedPoll := func() error {
		return &eventfeed.PollError{Kind: eventfeed.PollUnauthorized, Err: errors.New("401")}
	}

	t.Run("first failure reconnects with a fresh ticket", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptTicket(ticket(1))
		h.minter.ScriptTicket(ticket(2))
		h.polls.ScriptError(unauthorizedPoll())
		h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
		h.start()

		conn1 := h.driveToSubscribed()
		conn1.Serve(frameConfirm(noFilterIdentifier))
		h.awaitTimer(timerBackoff)
		if !conn1.Closed() {
			t.Fatal("an unauthorized poll must tear the attempt down")
		}
		assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
		h.fireTimer(timerBackoff)
		conn2 := h.driveToSubscribed()
		conn2.Serve(frameConfirm(noFilterIdentifier))
		h.awaitStreaming()
		if got := h.minter.Calls(); got != 2 {
			t.Fatalf("mint seam calls = %d, want 2", got)
		}
		if _, terminal, _ := h.snapshot(); terminal != nil {
			t.Fatalf("one unauthorized poll must not be terminal; got %v", terminal)
		}
	})

	t.Run("three consecutive are terminal", func(t *testing.T) {
		h := newHarness(t)
		for i := 1; i <= 3; i++ {
			h.minter.ScriptTicket(ticket(i))
			h.polls.ScriptError(unauthorizedPoll())
		}
		h.start()

		for i := 1; i <= 2; i++ {
			conn := h.driveToSubscribed()
			conn.Serve(frameConfirm(noFilterIdentifier))
			h.fireTimer(timerBackoff)
		}
		conn := h.driveToSubscribed()
		conn.Serve(frameConfirm(noFilterIdentifier))
		h.join()

		_, terminal, _ := h.snapshot()
		if terminal == nil || terminal.Reason != eventfeed.ReasonAuthorizationFailed {
			t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonAuthorizationFailed)
		}
		assertTimers(t, h.clock, map[string]int{})
	})

	t.Run("a successful page resets the shared counter", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptTicket(ticket(1))
		h.polls.ScriptError(unauthorizedPoll()) // failure 1
		h.minter.ScriptTicket(ticket(2))
		h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"}) // resets to 0
		h.minter.ScriptError(&eventfeed.MintError{Kind: eventfeed.MintUnauthorized})
		h.minter.ScriptError(&eventfeed.MintError{Kind: eventfeed.MintUnauthorized})
		h.start()

		conn1 := h.driveToSubscribed()
		conn1.Serve(frameConfirm(noFilterIdentifier))
		h.fireTimer(timerBackoff)
		conn2 := h.driveToSubscribed()
		conn2.Serve(frameConfirm(noFilterIdentifier))
		h.awaitStreaming()

		// Two more unauthorized failures: without the reset this would be the
		// third consecutive and terminal.
		conn2.Serve(frameDisconnect("unauthorized", false))
		h.fireTimer(timerBackoff)
		h.fireTimer(timerBackoff)
		h.waitUntil("both unauthorized mints attempted", func() bool { return h.minter.Calls() == 4 })
		if _, terminal, _ := h.snapshot(); terminal != nil {
			t.Fatalf("the successful page must reset the counter; got %v", terminal)
		}
	})
}

// TestSocketFailureMidWalk is transition 21: a socket failure observed while
// the walk waits on `poll-retry` tears the attempt down to Backoff, keeping
// the per-page checkpoints already saved.
func TestSocketFailureMidWalk(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	next := testOrigin + "/999/events.json?after=101"
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(101)}, Position: "pos-1", Next: next})
	h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollTransient, Err: errors.New("502")})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitTimer(timerPollRetry)
	conn.FailReads(errors.New("connection reset"))
	h.awaitTimer(timerBackoff)

	if !conn.Closed() {
		t.Fatal("the socket failure must dispose the attempt")
	}
	assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
	assertPositions(t, store.Saves(), "pos-1")
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("a mid-walk socket failure is never terminal; got %v", terminal)
	}
}

// TestStreamingFailureEdges: transition 26 (a raw protocol-fatal disconnect
// read while streaming is terminal from every socket-open state) and
// transition 25 (staleness expiry tears the socket down to Backoff).
func TestStreamingFailureEdges(t *testing.T) {
	stream := func(t *testing.T, h *harness) *feedtest.Conn {
		t.Helper()
		h.minter.ScriptTicket(ticket(1))
		h.minter.ScriptTicket(ticket(2))
		h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
		h.start()
		conn := h.driveToSubscribed()
		conn.Serve(frameConfirm(noFilterIdentifier))
		h.awaitStreaming()
		return conn
	}

	t.Run("protocol fatal is terminal", func(t *testing.T) {
		h := newHarness(t)
		conn := stream(t, h)
		conn.Serve(frameDisconnect("invalid_event_stream_command", false))
		h.join()
		_, terminal, _ := h.snapshot()
		if terminal == nil || terminal.Reason != eventfeed.ReasonProtocolFatal {
			t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonProtocolFatal)
		}
		if got := h.minter.Calls(); got != 1 {
			t.Fatalf("mint seam calls = %d, want 1 (never retried into)", got)
		}
		assertTimers(t, h.clock, map[string]int{})
	})

	t.Run("staleness reconnects", func(t *testing.T) {
		h := newHarness(t)
		conn := stream(t, h)
		h.clock.Advance(7500 * time.Millisecond)
		h.awaitTimer(timerBackoff)
		if !conn.Closed() {
			t.Fatal("staleness must tear the socket down")
		}
		assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
		if _, terminal, _ := h.snapshot(); terminal != nil {
			t.Fatalf("staleness is never terminal; got %v", terminal)
		}
	})
}

// TestRepairPollTimerArmedOnStreaming: entry to Streaming arms the jittered
// `repair-poll` timer (60s ± 20%), and every cycle re-arms it once the walk
// that firing drove has returned through Draining.
func TestRepairPollTimerArmedOnStreaming(t *testing.T) {
	h := newHarness(t)
	h.minter.ScriptTicket(ticket(1))
	for _, position := range []string{"pos-1", "pos-2", "pos-3"} {
		h.polls.ScriptPage(eventfeed.PollPage{Position: position})
	}
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	for cycle := 1; cycle <= 2; cycle++ {
		d := h.fireTimer(timerRepairPoll)
		if d < 48*time.Second || d > 72*time.Second {
			t.Fatalf("repair-poll delay (cycle %d) = %s, want within 60s ± 20%%", cycle, d)
		}
		h.awaitStreaming()
	}
	assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1})
}

// TestConsumerBreakSkipsThePagesCheckpoint: a consumer that breaks mid-page
// ends iteration cleanly and that page's position is never saved —
// checkpoint-after-processing, structurally.
func TestConsumerBreakSkipsThePagesCheckpoint(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store)
	h.breakAfter = 1
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{
		Events:   []eventfeed.Event{pollEvent(101), pollEvent(102)},
		Position: "pos-1",
	})
	base := runtime.NumGoroutine()
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.join()

	assertIDs(t, h.deliveredIDs(), 101)
	assertPositions(t, store.Saves())
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("a consumer break ends iteration with no error element; got %v", terminal)
	}
	if !conn.Closed() {
		t.Fatal("a consumer break must dispose the attempt")
	}
	assertTimers(t, h.clock, map[string]int{})
	assertGoroutinesSettle(t, base)
}

// TestNoStoreNeverSaves: with no checkpoint store the walk still delivers and
// advances its in-memory position — the store is write-through durability,
// not the position's authority.
func TestNoStoreNeverSaves(t *testing.T) {
	h := newHarness(t)
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(101)}, Position: "pos-1"})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()
	assertIDs(t, h.deliveredIDs(), 101)
	if got := fmt.Sprint(h.ledger()); got != "[event 101]" {
		t.Fatalf("ledger = %s, want only the delivery", got)
	}
}

// startDeferredOverflowWalk drives a position-resume walk to the moment the
// live buffer fills UNDER an in-flight entry poll: 41 and 42 are admitted by
// the in-flight servicing, 43 cannot be, so it is parked as an OVERFLOW
// deferral while the seam call is still allowed to complete.
//
// The rendezvous on the deferral is what makes the ordering deterministic
// rather than sampled: the scripted page is produced only after the poll's
// OnCall returns, so waiting there for the deferral removes the state
// machine's race between the queued frame and the finished call.
func startDeferredOverflowWalk(t *testing.T, store *feedtest.Store, wire func(*harness), opts ...eventfeed.Option) *harness {
	t.Helper()
	store.Stored("pos-0")
	h := storedHarness(t, store, append([]eventfeed.Option{eventfeed.WithLiveBufferCapacity(2)}, opts...)...)
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	deferred := make(chan bool, 4)
	h.conn.OnFrameDeferred(func(overflow bool) { deferred <- overflow })
	if wire != nil {
		wire(h)
	}
	var conn *feedtest.Conn
	h.polls.OnCall(func(feedtest.PollCall) {
		conn.Serve(frameMessage(noFilterIdentifier, 41))
		conn.Serve(frameMessage(noFilterIdentifier, 42))
		conn.Serve(frameMessage(noFilterIdentifier, 43))
		select {
		case overflow := <-deferred:
			if !overflow {
				t.Error("the deferral was a socket outcome, want the overflowing admission")
			}
		case <-time.After(watchdog):
			t.Error("the overflowing admission was never deferred")
		}
	})
	h.start()
	conn = h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	return h
}

// TestDeferredOverflowNeverSavesUnderTheDefaultTerminal is the conjunctive
// save-ordering invariant against the one drop the walk used to slip past it.
// An admission that overflows while the page's poll is in flight is a pre-cut
// loss condition, so its disposition must run BEFORE the page's position
// moves anything durable: with no handler registered that is
// Terminal(buffer_overflow) with the checkpoint exactly where it was, or a
// restart resumes past the dropped event and never serves it.
//
// Dispatching it at the walk's ordinary point is not merely late, it is
// silent: by then the drain has emptied the buffer, the admission drops
// nothing, and the signal never fires at all.
func TestDeferredOverflowNeverSavesUnderTheDefaultTerminal(t *testing.T) {
	store := feedtest.NewStore()
	var signals []eventfeed.Signal
	h := startDeferredOverflowWalk(t, store, func(h *harness) {
		h.conn.OnSignal(func(s eventfeed.Signal) { signals = append(signals, s) })
	})
	h.join()

	_, terminal, _ := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonBufferOverflow {
		t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonBufferOverflow)
	}
	if len(signals) != 1 {
		t.Fatalf("signals = %v, want exactly one BufferOverflow raised at drop time", signals)
	}
	ov, ok := signals[0].(eventfeed.BufferOverflow)
	if !ok || ov.DroppedCount != 1 || len(ov.DroppedIDs) != 1 || ov.DroppedIDs[0] != 41 {
		t.Fatalf("signal = %+v, want BufferOverflow{DroppedIDs:[41], DroppedCount:1}", signals[0])
	}
	assertPositions(t, store.Saves())
	if got := fmt.Sprint(h.ledger()); got != "[]" {
		t.Fatalf("ledger = %s, want nothing delivered and nothing saved", got)
	}
}

// TestDeferredOverflowAcceptedSavesAfterTheDisposition is the same ordering
// from the accepting side: the disposition is taken first, the page's position
// saves second, and acceptance is still not license to skip the retained
// events — 42 and 43 are both delivered by the drain that follows.
func TestDeferredOverflowAcceptedSavesAfterTheDisposition(t *testing.T) {
	store := feedtest.NewStore()
	// The handler runs on the consumer's goroutine, so it reaches the ledger
	// through a target published BEFORE that goroutine exists — the wire hook
	// runs ahead of the iteration, which is the happens-before the plain
	// assign-after-construct spelling does not have.
	var target *harness
	h := startDeferredOverflowWalk(t, store,
		func(h *harness) { target = h },
		eventfeed.WithSignalHandler(func(eventfeed.Signal) eventfeed.Disposition {
			target.record("overflow")
			return eventfeed.Accept
		}))
	h.awaitStreaming()

	assertLedger(t, h.ledger(), []string{"overflow", "save pos-1", "event 42", "event 43"})
	assertPositions(t, store.Saves(), "pos-1")
}

// TestSocketOutcomeDuringAStalledPollIsBounded is transition 21 against a
// seam call that returns only on cancellation. The socket dies while the entry
// poll is outstanding, so the outcome is deferred and the call is awaited —
// the in-flight page is still the page boundary's to accept — but the wait is
// bounded by the same staleness window every other socket-open wait is.
// Awaiting it unconditionally parks the consumer's goroutine forever behind a
// socket that has already spoken, and the staleness expiry that would tear
// that socket down is unobservable for as long as the call is in flight.
func TestSocketOutcomeDuringAStalledPollIsBounded(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	h.polls.StallNext()

	deferred := make(chan bool, 4)
	h.conn.OnFrameDeferred(func(overflow bool) { deferred <- overflow })
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
	h.polls.OnCall(func(feedtest.PollCall) {
		conn.FailReads(errors.New("connection reset by peer"))
		select {
		case <-errQueued:
		case <-time.After(watchdog):
			t.Error("the pump never handed the socket failure to the state machine")
		}
		select {
		case overflow := <-deferred:
			if overflow {
				t.Error("the deferral was an overflowing admission, want the socket outcome")
			}
		case <-time.After(watchdog):
			t.Error("the socket failure was never deferred")
		}
		// The window now lapses with the seam call still outstanding.
		h.clock.Advance(staleAfter)
	})
	h.start()
	conn = h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))

	h.awaitTimer(timerBackoff)
	if !conn.Closed() {
		t.Fatal("abandoning the superseded call must dispose the attempt")
	}
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("a socket failure mid-walk is never terminal; got %v", terminal)
	}
	assertPositions(t, store.Saves())
	assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
}

// TestProtocolFatalDuringDrainingIsImmediate is SPEC.md §23's carve-out from
// Draining's deferred consumption: "a raw `invalid_event_stream_command`
// observed during Draining is Terminal(`protocol_fatal`) immediately — the
// drain is not completed, the held entry position is NOT saved, and no
// `caught_up` is announced; only recoverable failures defer."
//
// Both subtests are the same frame reaching Draining by the two routes it
// can: parked in the deferral slot by the entry poll's own servicing, and
// queued by the pump while the drain is mid-delivery. A drain that replays
// the buffer without ever looking at the frame queue takes the recoverable
// path in both — held save, `caught_up`, and only then the terminal.
func TestProtocolFatalDuringDrainingIsImmediate(t *testing.T) {
	// A store with nothing in it makes the entry present-class, which is what
	// gives the drain a HELD position to (wrongly) save.
	newDrainingHarness := func(t *testing.T, caughtUp *int) (*harness, *feedtest.Store) {
		t.Helper()
		store := feedtest.NewStore()
		h := storedHarness(t, store, eventfeed.WithObserver(eventfeed.Observer{
			CaughtUp: func() { *caughtUp++ },
		}))
		h.minter.ScriptTicket(ticket(1))
		h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
		return h, store
	}

	t.Run("deferred by the entry poll's servicing", func(t *testing.T) {
		var caughtUp int
		h, store := newDrainingHarness(t, &caughtUp)
		deferred := make(chan bool, 4)
		h.conn.OnFrameDeferred(func(overflow bool) { deferred <- overflow })
		var conn *feedtest.Conn
		h.polls.OnCall(func(feedtest.PollCall) {
			conn.Serve(frameDisconnect("invalid_event_stream_command", false))
			select {
			case <-deferred:
			case <-time.After(watchdog):
				t.Error("the disconnect frame was never deferred")
			}
		})
		h.start()
		conn = h.driveToSubscribed()
		h.serveSettled(conn, frameMessage(noFilterIdentifier, 41))
		conn.Serve(frameConfirm(noFilterIdentifier))
		h.join()

		assertProtocolFatalDrain(t, h, store, caughtUp)
		assertIDs(t, h.deliveredIDs())
	})

	t.Run("queued while the drain is delivering", func(t *testing.T) {
		var caughtUp int
		h, store := newDrainingHarness(t, &caughtUp)
		h.pauseAfter = 1
		h.start()
		conn := h.driveToSubscribed()
		h.serveSettled(conn, frameMessage(noFilterIdentifier, 41))
		conn.Serve(frameConfirm(noFilterIdentifier))

		// The consumer parks inside the drain's delivery of the retained
		// event; the fatal frame is queued while it is parked.
		h.waitUntil("the drain parked mid-delivery", func() bool { return len(h.deliveredIDs()) == 1 })
		h.serveSettled(conn, frameDisconnect("invalid_event_stream_command", false))
		h.resume()
		h.join()

		assertProtocolFatalDrain(t, h, store, caughtUp)
		assertIDs(t, h.deliveredIDs(), 41)
	})
}

// assertProtocolFatalDrain asserts the carve-out's three consequences: the
// typed terminal, no held save, and no caught_up announcement.
func assertProtocolFatalDrain(t *testing.T, h *harness, store *feedtest.Store, caughtUp int) {
	t.Helper()
	_, terminal, _ := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonProtocolFatal {
		t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonProtocolFatal)
	}
	assertPositions(t, store.Saves())
	if caughtUp != 0 {
		t.Fatalf("caught_up announcements = %d, want 0 — the drain never completed", caughtUp)
	}
}
