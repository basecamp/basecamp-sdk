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
	"slices"
	"strings"
	"sync"
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
		// Authority is not hostname: ":443" is a nonempty url.Host with an
		// empty hostname. CanonicalOrigin already reads Hostname(), so this
		// takes the terminal path; the case is here so it stays that way.
		{"port-only authority", "https://:443/999/events.json"},
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

// startOverflowUnderTheEntryPoll drives a position-resume walk to the moment
// the live buffer fills UNDER an in-flight entry poll: 41 and 42 are admitted
// by the in-flight servicing, 43 cannot be, so its drop is dispatched right
// there — in the goroutine that received it, with the seam call still
// outstanding.
//
// The rendezvous on the raised signal is what makes the ordering deterministic
// rather than sampled: the scripted page is produced only after the poll's
// OnCall returns, so waiting there for the drop removes the state machine's
// race between the queued frames and the finished call. onSignal, when set,
// observes each signal ahead of that rendezvous.
func startOverflowUnderTheEntryPoll(t *testing.T, store *feedtest.Store, onSignal func(eventfeed.Signal), wire func(*harness), opts ...eventfeed.Option) *harness {
	t.Helper()
	store.Stored("pos-0")
	h := storedHarness(t, store, append([]eventfeed.Option{eventfeed.WithLiveBufferCapacity(2)}, opts...)...)
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	dropped := make(chan eventfeed.Signal, 4)
	h.conn.OnSignal(func(s eventfeed.Signal) {
		if onSignal != nil {
			onSignal(s)
		}
		dropped <- s
	})
	if wire != nil {
		wire(h)
	}
	var conn *feedtest.Conn
	h.polls.OnCall(func(feedtest.PollCall) {
		conn.Serve(frameMessage(noFilterIdentifier, 41))
		conn.Serve(frameMessage(noFilterIdentifier, 42))
		conn.Serve(frameMessage(noFilterIdentifier, 43))
		select {
		case s := <-dropped:
			if _, ok := s.(eventfeed.BufferOverflow); !ok {
				t.Errorf("signal = %+v, want the overflowing admission's BufferOverflow", s)
			}
		case <-time.After(watchdog):
			t.Error("the overflowing admission was never dispatched")
		}
	})
	h.start()
	conn = h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	return h
}

// TestOverflowUnderTheEntryPollNeverSavesUnderTheDefaultTerminal is the
// conjunctive save-ordering invariant against the one drop the walk used to
// slip past it. An admission that overflows while the page's poll is in flight
// is a pre-cut loss condition, so its disposition must run BEFORE the page's
// position moves anything durable: with no handler registered that is
// Terminal(buffer_overflow) with the checkpoint exactly where it was, or a
// restart resumes past the dropped event and never serves it.
//
// Dispatching it at a later cut is not merely late, it is unreliable: parked
// until the page boundary the drain would already have emptied the buffer, so
// the admission drops nothing and the signal never fires at all — and parked
// until the call returns, a call that stalls or fails never dispatches it.
// Dispatching where the drop is observed is what makes this invariant
// structural: a disposition that ran strictly earlier cannot run after a save.
func TestOverflowUnderTheEntryPollNeverSavesUnderTheDefaultTerminal(t *testing.T) {
	store := feedtest.NewStore()
	var signals []eventfeed.Signal
	h := startOverflowUnderTheEntryPoll(t, store,
		func(s eventfeed.Signal) { signals = append(signals, s) }, nil)
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

// TestOverflowUnderTheEntryPollSavesAfterTheDisposition is the same ordering
// from the accepting side: the disposition is taken first, the page's position
// saves second, and acceptance is still not license to skip the retained
// events — 42 and 43 are both delivered by the drain that follows.
func TestOverflowUnderTheEntryPollSavesAfterTheDisposition(t *testing.T) {
	store := feedtest.NewStore()
	// The handler runs on the consumer's goroutine, so it reaches the ledger
	// through a target published BEFORE that goroutine exists — the wire hook
	// runs ahead of the iteration, which is the happens-before the plain
	// assign-after-construct spelling does not have.
	var target *harness
	h := startOverflowUnderTheEntryPoll(t, store, nil,
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

	deferred := make(chan struct{}, 4)
	h.conn.OnFrameDeferred(func() { deferred <- struct{}{} })
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
		case <-deferred:
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
	newDrainingHarness := func(t *testing.T, caughtUp *int, opts ...eventfeed.Option) (*harness, *feedtest.Store) {
		t.Helper()
		store := feedtest.NewStore()
		h := storedHarness(t, store, append([]eventfeed.Option{eventfeed.WithObserver(eventfeed.Observer{
			CaughtUp: func() { *caughtUp++ },
		})}, opts...)...)
		h.minter.ScriptTicket(ticket(1))
		h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
		return h, store
	}

	t.Run("deferred by the entry poll's servicing", func(t *testing.T) {
		var caughtUp int
		h, store := newDrainingHarness(t, &caughtUp)
		deferred := make(chan struct{}, 4)
		h.conn.OnFrameDeferred(func() { deferred <- struct{}{} })
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

	// #760: the fatal frame is queued while the deferral slot is ALREADY
	// occupied by a non-fatal socket outcome. The scan used to return the
	// moment it found the slot occupied — "everything the pump has queued
	// arrived behind this one, so the scan stops here either way" — which is a
	// claim about arrival order where the carve-out is a claim about which
	// VERDICT governs. The drain then completed, the held position saved and
	// caught_up announced: the three things §23 says a fatal observed during
	// Draining must prevent.
	//
	// This needs BOTH halves of the two subtests around it, and neither alone
	// reaches it. Deferring during the entry poll is what occupies the slot;
	// queuing the fatal frame mid-drain is what puts it somewhere only the
	// scan can find. Queued any earlier, the ownership cut consumes it first
	// and the carve-out fires without the scan being involved at all — which
	// is how the first draft of this test passed against the un-fixed code.
	//
	// A regression shows up as the watchdog rather than as a failed assertion
	// on the save, and that IS the un-fixed behavior rather than a weak test:
	// the drain completes, the held position saves, caught_up announces, and
	// the recoverable disconnect is then dispatched — so the feed reconnects
	// and parks in Backoff on a clock only the test advances. The iteration
	// never ending is precisely the carve-out having failed.
	t.Run("queued mid-drain behind an occupied deferral slot", func(t *testing.T) {
		var caughtUp int
		h, store := newDrainingHarness(t, &caughtUp)
		h.pauseAfter = 1
		deferred := make(chan struct{}, 4)
		h.conn.OnFrameDeferred(func() { deferred <- struct{}{} })
		handedOff := make(chan struct{}, 32)
		h.conn.OnPumpHandedOff(func(bool) {
			select {
			case handedOff <- struct{}{}:
			default:
			}
		})
		var conn *feedtest.Conn
		h.polls.OnCall(func(feedtest.PollCall) {
			// A reconnecting disconnect is recoverable, so it parks in the slot
			// rather than ending the cycle — and it is still there when the
			// drain begins, because the walk's dispatch points are the page
			// boundary and the walk's END, and this page carries no `next`.
			conn.Serve(frameDisconnect("server_restart", true))
			select {
			case <-deferred:
			case <-time.After(watchdog):
				t.Error("the recoverable disconnect frame was never deferred")
			}
		})
		h.start()
		conn = h.driveToSubscribed()
		h.serveSettled(conn, frameMessage(noFilterIdentifier, 41))
		conn.Serve(frameConfirm(noFilterIdentifier))

		// The consumer parks inside the drain's delivery of the retained event.
		h.waitUntil("the drain parked mid-delivery", func() bool { return len(h.deliveredIDs()) == 1 })
		for len(handedOff) > 0 {
			<-handedOff
		}
		// The fatal frame must be IN the queue before the drain's next scan:
		// that receive is non-blocking by design.
		conn.Serve(frameDisconnect("invalid_event_stream_command", false))
		select {
		case <-handedOff:
		case <-time.After(watchdog):
			t.Error("the fatal frame never reached the pump's queue")
		}
		h.resume()
		h.join()

		assertProtocolFatalDrain(t, h, store, caughtUp)
		assertIDs(t, h.deliveredIDs(), 41)
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

	// The carve-out is about the FRAME, not about how many ordinary frames sit
	// in front of it. The scan's reach is the pump's hand-off queue depth —
	// the only bound that guarantees every frame the pump had queued when the
	// drain began is examined — and deliberately NOT the caller-configurable
	// live-buffer capacity, which measures a different thing entirely (how
	// much the state machine will hold) and can be set as low as 1. Bounding
	// the scan by it lets a single queued ping spend the whole budget, and the
	// fatal frame behind it goes unseen until Streaming — after the held save
	// and the caught_up announcement the carve-out exists to prevent.
	t.Run("behind an ordinary frame under a one-event live buffer", func(t *testing.T) {
		var caughtUp int
		h, store := newDrainingHarness(t, &caughtUp, eventfeed.WithLiveBufferCapacity(1))
		h.pauseAfter = 1
		h.start()
		conn := h.driveToSubscribed()
		h.serveSettled(conn, frameMessage(noFilterIdentifier, 41))
		conn.Serve(frameConfirm(noFilterIdentifier))

		h.waitUntil("the drain parked mid-delivery", func() bool { return len(h.deliveredIDs()) == 1 })
		// A ping is queued AHEAD of the fatal frame: both are in the pump's
		// queue before the drain's next scan, so both were "observed during
		// Draining" by every reading of the carve-out.
		h.serveSettled(conn, framePing(), frameDisconnect("invalid_event_stream_command", false))
		h.resume()
		h.join()

		assertProtocolFatalDrain(t, h, store, caughtUp)
		assertIDs(t, h.deliveredIDs(), 41)
	})
}

// TestOverflowUnderAStalledPollDispatchesWithoutWaiting is SPEC.md §23's
// dispatch-timing rule against the one thing that can postpone a page boundary
// indefinitely: "a semantic signal is dispatched at the first consumer-context
// opportunity after its condition arises, with 'before the next save' as the
// outer bound... an implementation must not defer the signal to a later cut
// that may never come."
//
// The poll here returns only on cancellation, so there is no later cut. The
// goroutine that received the dropping frame IS the consumer's, so drop time is
// the first opportunity — and parking the disposition until the call returns
// postpones the handler forever. Worse, a call that then fails (unauthorized,
// unrecoverable, or a socket death during its retry wait) disposes the attempt
// and clears the parked deferral, so the signal is lost outright.
func TestOverflowUnderAStalledPollDispatchesWithoutWaiting(t *testing.T) {
	// fill drives a position-resume walk into a stalled entry poll and then
	// overflows the live buffer under it: 41 and 42 are admitted by the
	// in-flight servicing, 43 cannot be, and the drop is dispatched where it is
	// observed.
	fill := func(t *testing.T, store *feedtest.Store, opts ...eventfeed.Option) (*harness, *feedtest.Conn) {
		t.Helper()
		store.Stored("pos-0")
		h := storedHarness(t, store, append([]eventfeed.Option{eventfeed.WithLiveBufferCapacity(2)}, opts...)...)
		h.minter.ScriptTicket(ticket(1))
		h.polls.StallNext()
		h.start()
		conn := h.driveToSubscribed()
		conn.Serve(frameConfirm(noFilterIdentifier))
		h.waitUntil("the entry poll is outstanding", func() bool { return h.polls.CallCount() == 1 })
		conn.Serve(frameMessage(noFilterIdentifier, 41))
		conn.Serve(frameMessage(noFilterIdentifier, 42))
		conn.Serve(frameMessage(noFilterIdentifier, 43))
		return h, conn
	}

	t.Run("an accepting handler is invoked while the call is outstanding", func(t *testing.T) {
		store := feedtest.NewStore()
		raised := make(chan eventfeed.Signal, 4)
		h, _ := fill(t, store, eventfeed.WithSignalHandler(func(s eventfeed.Signal) eventfeed.Disposition {
			raised <- s
			return eventfeed.Accept
		}))

		select {
		case s := <-raised:
			ov, ok := s.(eventfeed.BufferOverflow)
			if !ok || ov.DroppedCount != 1 || len(ov.DroppedIDs) != 1 || ov.DroppedIDs[0] != 41 {
				t.Fatalf("signal = %+v, want BufferOverflow{DroppedIDs:[41], DroppedCount:1}", s)
			}
		case <-time.After(watchdog):
			t.Fatal("the drop never reached the handler: the stalled poll postponed the dispatch")
		}
		// The seam call never returned, so nothing durable can have moved — and
		// an Accept keeps awaiting it, exactly as before.
		if got := h.polls.CallCount(); got != 1 {
			t.Fatalf("poll seam calls = %d, want the one stalled call", got)
		}
		assertPositions(t, store.Saves())
	})

	t.Run("a terminating disposition does not wait for the call either", func(t *testing.T) {
		store := feedtest.NewStore()
		h, conn := fill(t, store, eventfeed.WithSignalHandler(
			func(eventfeed.Signal) eventfeed.Disposition { return eventfeed.Terminate }))
		h.join()

		_, terminal, _ := h.snapshot()
		if terminal == nil || terminal.Reason != eventfeed.ReasonBufferOverflow {
			t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonBufferOverflow)
		}
		assertPositions(t, store.Saves())
		if got := fmt.Sprint(h.ledger()); got != "[]" {
			t.Fatalf("ledger = %s, want nothing delivered and nothing saved", got)
		}
		if !conn.Closed() {
			t.Fatal("terminating on the drop must dispose the attempt, which is what returns the abandoned call")
		}
		assertTimers(t, h.clock, map[string]int{})
	})
}

// TestOverflowIsNotSwallowedByAFailingPoll is the other half of the same
// defect, and the reason the dispatch point had to move rather than gain a
// second site. A drop observed while the call is in flight, parked for a cut
// taken only after a SUCCESSFUL page, is lost outright when the call fails:
// every failure edge disposes the attempt, and disposal drops the deferral
// with the socket it belonged to. §23 is unambiguous that it cannot vanish —
// "an unhandled semantic signal cannot disappear" — so with no handler
// registered this is Terminal(buffer_overflow), not a quiet reconnect.
func TestOverflowIsNotSwallowedByAFailingPoll(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store, eventfeed.WithLiveBufferCapacity(2))
	h.minter.ScriptTicket(ticket(1))
	// The call the drop is observed under fails on the edge that rides the
	// reconnect cycle — the quietest of the failure edges, and the one that
	// would otherwise carry on as if nothing had been dropped.
	h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollUnauthorized})
	var signals []eventfeed.Signal
	dropped := make(chan eventfeed.Signal, 4)
	h.conn.OnSignal(func(s eventfeed.Signal) {
		signals = append(signals, s)
		dropped <- s
	})
	var conn *feedtest.Conn
	h.polls.OnCall(func(feedtest.PollCall) {
		conn.Serve(frameMessage(noFilterIdentifier, 41))
		conn.Serve(frameMessage(noFilterIdentifier, 42))
		conn.Serve(frameMessage(noFilterIdentifier, 43))
		select {
		case <-dropped:
		case <-time.After(watchdog):
			t.Error("the drop was never dispatched: the failing call swallowed it")
		}
	})
	h.start()
	conn = h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.join()

	_, terminal, _ := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonBufferOverflow {
		t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonBufferOverflow)
	}
	if len(signals) != 1 {
		t.Fatalf("signals = %v, want exactly one BufferOverflow", signals)
	}
	assertPositions(t, store.Saves())
	// The drop ended the cycle before the poll's own failure could route it
	// into the reconnect lane, so the feed never re-mints.
	if got := h.minter.Calls(); got != 1 {
		t.Fatalf("mint seam calls = %d, want 1 — the terminal is never retried into", got)
	}
}

// TestDrainScanAdmissionIsNotStranded: the drain's protocol-fatal scan admits
// correlated events as it goes, and an admission landing in an iteration that
// took the buffer EMPTY must not end the drain. Streaming never drains the
// buffer, so a stranded event waits for a later repair walk — behind
// `caught_up` and behind the held entry save this drain is what gates.
func TestDrainScanAdmissionIsNotStranded(t *testing.T) {
	store := feedtest.NewStore() // Missing → present-class entry, so the save is HELD
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	// Park the consumer inside the drain's delivery of the one retained event,
	// so the next iteration takes an empty buffer with a frame queued behind it.
	h.pauseAfter = 1
	h.start()

	conn := h.driveToSubscribed()
	h.serveSettled(conn, frameMessage(noFilterIdentifier, 41))
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.waitUntil("the drain parked mid-delivery", func() bool { return len(h.deliveredIDs()) == 1 })
	h.serveSettled(conn, frameMessage(noFilterIdentifier, 42))
	h.resume()
	h.awaitStreaming()

	assertLedger(t, h.ledger(), []string{"event 41", "event 42", "save pos-1"})
	assertIDs(t, h.deliveredIDs(), 41, 42)
	assertPositions(t, store.Saves(), "pos-1")
}

// TestDrainHoldsNoMoreThanTheLiveBufferCapacity: the live buffer's capacity
// is a bound on events HELD AT ONCE — SPEC §23 sizes the connector's whole
// memory ceiling off it, "(pump depth + 2 + EVENT_FEED_LIVE_BUFFER_CAPACITY) ×
// EVENT_FEED_MAX_FRAME_BYTES" — so a drain must not be able to hold a batch
// outside the buffer while the buffer refills to capacity behind it.
//
// The scenario pins it exactly: the buffer is FULL at capacity when Draining
// begins, and 2× capacity more live frames are already queued in the pump, with
// no consumer progress possible in between. Every one of them passes through a
// capacity-sized buffer, so at most `capacity` may survive to delivery and the
// rest must be reported dropped, oldest-id first.
func TestDrainHoldsNoMoreThanTheLiveBufferCapacity(t *testing.T) {
	const capacity = 4
	var dropped []int64
	handler := func(s eventfeed.Signal) eventfeed.Disposition {
		ov, ok := s.(eventfeed.BufferOverflow)
		if !ok {
			t.Errorf("signal = %+v, want a BufferOverflow", s)
			return eventfeed.Terminate
		}
		if ov.DroppedCount != len(ov.DroppedIDs) {
			t.Errorf("signal = %+v: DroppedCount disagrees with DroppedIDs", ov)
		}
		dropped = append(dropped, ov.DroppedIDs...)
		return eventfeed.Accept
	}
	// A position-resume entry: no ownership cut, so the ONLY pass that can
	// dequeue the queued frames is the drain's own.
	h := newHarness(t,
		eventfeed.WithLiveBufferCapacity(capacity),
		eventfeed.WithStart(eventfeed.StartAtPosition("pos-0")),
		eventfeed.WithSignalHandler(handler))
	h.pauseAfter = 1
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(1)}, Position: "pos-1"})
	h.start()

	conn := h.driveToSubscribed()
	// Fill the buffer to capacity before confirmation.
	for id := int64(11); id <= 14; id++ {
		conn.Serve(frameMessage(noFilterIdentifier, id))
	}
	h.serveSettled(conn)
	conn.Serve(frameConfirm(noFilterIdentifier))

	// Park inside the entry page's delivery: the poll has returned and the
	// drain has not begun, so the frames queued now are exactly what the
	// drain's scan will find.
	h.waitUntil("the entry page parked mid-delivery", func() bool { return len(h.deliveredIDs()) == 1 })
	for id := int64(21); id <= 28; id++ {
		conn.Serve(frameMessage(noFilterIdentifier, id))
	}
	h.serveSettled(conn)
	h.resume()
	h.awaitStreaming()
	h.conn.Close()
	h.join()

	var live []int64
	for _, id := range h.deliveredIDs() {
		if id != 1 {
			live = append(live, id)
		}
	}
	if len(live) > capacity {
		t.Fatalf("the drain delivered %d live-buffered events (%v) under a capacity of %d: "+
			"the taken batch escaped the buffer's accounting", len(live), live, capacity)
	}
	assertIDs(t, live, 25, 26, 27, 28)
	assertIDs(t, dropped, 11, 12, 13, 14, 21, 22, 23, 24)
}

// TestRedirectRefusalExposesOnlyTheLocationOrigin: the refused-redirect edge
// promises the rejected Location redacted to its ORIGIN — a hostile
// continuation's path and query are exactly what must not be echoed — so the
// terminal cannot retain the seam error as its cause. PollError.Error and
// PollError.Unwrap both reach the underlying generated error, whose text
// routinely carries the request URL in full.
func TestRedirectRefusalExposesOnlyTheLocationOrigin(t *testing.T) {
	const secret = "/steal?ticket=abc123&next=%2Fadmin"
	h := newHarness(t)
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptError(&eventfeed.PollError{
		Kind:           eventfeed.PollRedirectRefused,
		LocationOrigin: "https://attacker.example.com",
		Err:            errors.New("302 Location: https://attacker.example.com" + secret),
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
		t.Fatalf("terminal message %q should name the refused origin", terminal.Msg)
	}
	// The whole rendering, not just Msg: Error walks the cause chain.
	if rendered := terminal.Error(); strings.Contains(rendered, secret) {
		t.Fatalf("terminal rendering %q leaks the rejected Location's path and query", rendered)
	}
	for err := error(terminal); err != nil; err = errors.Unwrap(err) {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("the terminal's cause chain leaks the rejected Location: %v", err)
		}
	}
	if !conn.Closed() {
		t.Fatal("teardown must close the socket")
	}
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

// TestSupersededPollBoundSurvivesPumpBackPressure is the second half of
// transition 21's bound, and the reason the wait no longer borrows the
// staleness verdict.
//
// Every OTHER socket-open wait in the connector keeps DRAINING the hand-off
// queue while it waits, which is what makes §23's suspension rule sound
// there: a full queue really does prove the peer is outrunning a connector
// that is still consuming. This wait is the one that deliberately stops
// consuming — a frame is already parked in the deferral slot. So the queue it
// stopped draining fills, the pump blocks in hand-off, and the suspension is
// granted against a premise that is false by construction: every firing gets
// disregarded and re-armed, forever, while a compliant PollSource that
// returns only on cancellation holds the consumer's goroutine. A peer that
// keeps sending is then enough to keep an already-dead socket's verdict from
// ever landing.
func TestSupersededPollBoundSurvivesPumpBackPressure(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	h.polls.StallNext()

	deferred := make(chan struct{}, 4)
	h.conn.OnFrameDeferred(func() { deferred <- struct{}{} })
	blocked := make(chan struct{}, 1)
	h.conn.OnPumpBlocked(func() {
		select {
		case blocked <- struct{}{}:
		default:
		}
	})
	// The probe at the superseded boundary handles the whole queue this test
	// fills, in one pass; nothing here awaits handled frames.
	h.drainHandled()
	var conn *feedtest.Conn
	h.polls.OnCall(func(feedtest.PollCall) {
		// The socket speaks its last word while the entry poll is
		// outstanding: the outcome is deferred, and the walk stops draining.
		conn.Serve(frameDisconnect("remote", true))
		select {
		case <-deferred:
		case <-time.After(watchdog):
			t.Error("the disconnect frame was never deferred")
		}
		// A peer that keeps sending now fills the queue nothing is draining.
		for i := 0; i <= eventfeed.ExportPumpDepth; i++ {
			conn.Serve(framePing())
		}
		select {
		case <-blocked:
		case <-time.After(watchdog):
			t.Error("the pump never blocked on a full hand-off queue")
		}
		// One window from the deferral, with the suspension in force.
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
		t.Fatalf("a socket drop mid-walk is never terminal; got %v", terminal)
	}
	assertPositions(t, store.Saves())
}

// TestDeferredProtocolFatalOutranksAFailedPoll dispatches a deferred socket
// outcome before a poll error is classified. Transition 21's
// finish-the-page ordering exists so an accepted page's deliveries and save
// are not stranded by the socket's death — it is about a page that SUCCEEDED.
// A failed poll has no such page, and recoverPoll's terminal and re-entry
// branches dispose the attempt, which discards the deferral: the server's own
// protocol-fatal verdict is replaced by whatever the poll happened to fail
// with.
func TestDeferredProtocolFatalOutranksAFailedPoll(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollUnauthorized})

	deferred := make(chan struct{}, 4)
	h.conn.OnFrameDeferred(func() { deferred <- struct{}{} })
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
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.join()

	_, terminal, _ := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonProtocolFatal {
		t.Fatalf("terminal = %v, want reason %q — the server's verdict was already observed", terminal, eventfeed.ReasonProtocolFatal)
	}
	assertPositions(t, store.Saves())
}

// TestProtocolFatalBehindADeferredRecoverableOutranksIt is the walk's version
// of the hole #760 closed inside the drain, and it is the same hole: a
// protocol-fatal verdict the pump has ALREADY READ loses to whatever
// non-fatal outcome happens to be parked ahead of it.
//
// SPEC §23 is state-generic about this and says so twice — "the protocol-fatal
// disconnect is terminal from EVERY socket-open state", and "a raw
// invalid_event_stream_command frame READ in AwaitingWelcome, CatchingUp, or
// Draining ... Terminal(protocol_fatal), never Backoff. An explicitly
// non-retryable protocol rejection must not reconnect from any state." Reading
// is the boundary the obligation attaches to, not dispatch order.
//
// So the deferral slot holds a `remote` disconnect — recoverable, arrived
// first — and the fatal sits in the pump's queue behind it. dispatchDeferred
// reported the `remote`, ended the cycle as a plain socket failure, and
// reconnected: a second mint, against a server that had already refused the
// command. The fatal died with the socket.
//
// The assertions are the ones the carve-out is about. The terminal reason,
// because reporting the recoverable verdict is the defect itself; and ZERO
// reconnects, because "never Backoff" is the half a reason assertion alone
// would not catch (a connector that reconnects once and then terminates on the
// re-issued frame still ends at protocol_fatal).
//
// There is one case per deferred-disposition boundary the walk has, because
// each is a separate hole: a probe removed at any ONE of them must fail
// exactly its own case, and a case that passes with every probe removed is
// pinning nothing.
func TestProtocolFatalBehindADeferredRecoverableOutranksIt(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []eventfeed.Option
		// arm scripts the poll outcome that decides WHICH deferred-disposition
		// boundary the walk reaches with the slot occupied.
		arm func(h *harness)
		// wedge serves whatever must sit between the deferred recoverable and
		// the fatal, for the boundaries where merely queueing the fatal is not
		// enough to keep an ordinary admission pass from reaching it first.
		wedge func(conn *feedtest.Conn)
		// stalled marks the boundary reached by abandoning a stalled call
		// rather than by the call returning.
		stalled bool
		// wantSaves is what may legitimately have been persisted BEFORE the
		// fatal was observed.
		wantSaves []string
	}{
		// The ordinary page boundary. The page succeeds and its position
		// saves; the walk then loops back to the top with the slot occupied.
		// A ping is wedged ahead of the fatal and the buffer is sized to 1,
		// because otherwise the entry cut's admission pass — bounded by
		// liveBufferCapacity, which defaults large — dequeues the fatal on its
		// own and the boundary's own probe is never what catches it. That
		// bound is caller-configurable down to 1, which is the whole reason
		// the probe carries pumpDepth+1 instead of borrowing it.
		{
			name: "page boundary",
			opts: []eventfeed.Option{eventfeed.WithLiveBufferCapacity(1)},
			arm: func(h *harness) {
				h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1", Next: testOrigin + "/999/events.json?after=1"})
				h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-2"})
			},
			wedge:     func(conn *feedtest.Conn) { conn.Serve(framePing()) },
			wantSaves: []string{"pos-1"},
		},
		// The superseded boundary: the call is abandoned with the deferral
		// still parked, and its teardown is what cancels the call.
		{
			name:    "superseded boundary",
			arm:     func(h *harness) { h.polls.StallNext() },
			stalled: true,
		},
		// The poll-error boundary: recoverPoll's terminal and re-entry
		// branches dispose the attempt, discarding everything the pump read.
		{
			name: "poll error boundary",
			arm: func(h *harness) {
				h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollUnauthorized})
			},
		},
		// The malformed-page boundary: a page with no position is refused,
		// and the refusal is classified after the deferral is dispatched.
		{
			name: "malformed page boundary",
			arm:  func(h *harness) { h.polls.ScriptPage(eventfeed.PollPage{Position: ""}) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := feedtest.NewStore()
			store.Stored("pos-0")
			h := storedHarness(t, store, tc.opts...)
			h.minter.ScriptTicket(ticket(1))
			// The second mint is scripted to fail TERMINALLY. A reconnecting
			// connector therefore ends at Terminal(mint_failed) having made
			// two mint calls, and a correct one ends at
			// Terminal(protocol_fatal) having made one — so both assertions
			// discriminate, and neither outcome strands the run on a socket
			// the test never serves.
			h.minter.ScriptError(&eventfeed.MintError{Kind: eventfeed.MintUnrecoverable})
			tc.arm(h)
			h.drainHandled()

			deferred := make(chan struct{}, 4)
			h.conn.OnFrameDeferred(func() { deferred <- struct{}{} })
			queued := make(chan struct{}, eventfeed.ExportPumpDepth)
			h.conn.OnPumpHandedOff(func(bool) {
				select {
				case queued <- struct{}{}:
				default:
				}
			})
			var conn *feedtest.Conn
			var once sync.Once
			h.polls.OnCall(func(feedtest.PollCall) {
				once.Do(func() {
					// Recoverable FIRST, so it takes the single deferral slot.
					conn.Serve(frameDisconnect("remote", true))
					select {
					case <-deferred:
					case <-time.After(watchdog):
						t.Error("the recoverable disconnect was never deferred")
						return
					}
					if tc.wedge != nil {
						tc.wedge(conn)
					}
					// Fatal LAST, and only once the pump has handed it off:
					// the guarantee is about a frame the pump has READ, so a
					// test that raced the read would prove nothing.
					drain(queued)
					conn.Serve(frameDisconnect("invalid_event_stream_command", false))
					select {
					case <-queued:
					case <-time.After(watchdog):
						t.Error("the fatal frame never reached the hand-off queue")
					}
					if tc.stalled {
						// Lapse the superseded-poll bound so the walk
						// abandons the call it is still inside.
						h.clock.Advance(staleAfter)
					}
				})
			})
			h.start()
			conn = h.driveToSubscribed()
			conn.Serve(frameConfirm(noFilterIdentifier))
			h.awaitEndOrReconnect()
			h.join()

			_, terminal, _ := h.snapshot()
			if terminal == nil || terminal.Reason != eventfeed.ReasonProtocolFatal {
				t.Fatalf("terminal = %v, want reason %q — the server's verdict was already read", terminal, eventfeed.ReasonProtocolFatal)
			}
			if got := h.minter.Calls(); got != 1 {
				t.Fatalf("mint calls = %d, want 1: a protocol-fatal rejection must not reconnect from any state", got)
			}
			assertPositions(t, store.Saves(), tc.wantSaves...)
		})
	}
}

// drain empties a buffered signal channel without blocking.
func drain(ch chan struct{}) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// awaitEndOrReconnect returns once the run has ended, or once a reconnect has
// armed the backoff timer — firing it in that case so the run proceeds to its
// scripted terminal mint failure instead of parking on a virtual clock nothing
// advances. Without it a run that wrongly reconnects hangs, and a hang reports
// as "the watchdog expired" rather than as the defect under test.
func (h *harness) awaitEndOrReconnect() {
	h.t.Helper()
	deadline := time.Now().Add(watchdog)
	for {
		select {
		case <-h.done:
			return
		default:
		}
		if slices.Contains(h.clock.Outstanding(), timerBackoff) {
			h.fireTimer(timerBackoff)
			return
		}
		if time.Now().After(deadline) {
			h.t.Fatal("the run neither ended nor reconnected within the watchdog")
		}
		time.Sleep(time.Millisecond)
	}
}

// TestProtocolFatalBehindTheBlockedHandOffIsImmediate extends the carve-out's
// scan bound to the frame the pump has already READ but not yet handed off.
//
// The drain-start boundary the guarantee is stated over is "every frame the
// pump had already read", and that differs from the queue's contents by
// exactly one: the pump is a single goroutine, so at most one hand-off is in
// flight. With the queue full of ordinary frames and the fatal one held in
// that hand-off, a budget of pumpDepth spends itself on the fatal frame's
// predecessors — Go hands a blocked sender's value into the buffer during the
// very first receive, so the fatal enters at the TAIL and sits exactly one
// dequeue past the budget. The drain then completes, the held entry position
// saves and `caught_up` announces: precisely what the carve-out forbids.
func TestProtocolFatalBehindTheBlockedHandOffIsImmediate(t *testing.T) {
	var caughtUp int
	store := feedtest.NewStore()
	h := storedHarness(t, store, eventfeed.WithObserver(eventfeed.Observer{
		CaughtUp: func() { caughtUp++ },
	}))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	blocked := make(chan struct{}, 1)
	h.conn.OnPumpBlocked(func() {
		select {
		case blocked <- struct{}{}:
		default:
		}
	})
	h.pauseAfter = 1
	h.start()

	conn := h.driveToSubscribed()
	h.serveSettled(conn, frameMessage(noFilterIdentifier, 41))
	conn.Serve(frameConfirm(noFilterIdentifier))

	// The consumer parks inside the drain's delivery of the retained event,
	// so nothing dequeues while the queue fills.
	h.waitUntil("the drain parked mid-delivery", func() bool { return len(h.deliveredIDs()) == 1 })
	// The scan handles a whole queue's worth of frames in one pass, which is
	// more than the harness's frame-handled channel holds; nothing waits on
	// it from here.
	h.drainHandled()
	for i := 0; i < eventfeed.ExportPumpDepth; i++ {
		conn.Serve(framePing())
	}
	conn.Serve(frameDisconnect("invalid_event_stream_command", false))
	select {
	case <-blocked:
	case <-time.After(watchdog):
		t.Fatal("the pump never blocked handing off the fatal frame")
	}
	h.resume()
	h.join()

	assertProtocolFatalDrain(t, h, store, caughtUp)
	assertIDs(t, h.deliveredIDs(), 41)
}

// TestPollPageWithNoPositionIsMalformed is daybreak blocker 2. A page without
// a position is refused before delivery, before any counter reset, and before
// any mutation of the walk's state.
//
// Both entry classes are covered because they fail DIFFERENTLY, and the
// present-class one is the worse of the two — an assertion written only
// against the position-resume path would miss it entirely.
func TestPollPageWithNoPositionIsMalformed(t *testing.T) {
	// Position-resume entry: acceptPosition("") would set l.position = "",
	// and entryCursor selects on `l.position != ""` — so it does not keep the
	// old cursor, it falls through to the StartResume default, which is a bare
	// present entry. The feed would resume at the server's head with every
	// event between the real position and now skipped, and no signal at all.
	t.Run("position-resume entry", func(t *testing.T) {
		store := feedtest.NewStore()
		store.Stored("pos-0")
		h := storedHarness(t, store)
		h.minter.ScriptTicket(ticket(1))
		h.polls.ScriptPage(eventfeed.PollPage{
			Events:   []eventfeed.Event{pollEvent(101)},
			Position: "",
		})
		h.start()

		conn := h.driveToSubscribed()
		conn.Serve(frameConfirm(noFilterIdentifier))
		h.join()

		_, terminal, _ := h.snapshot()
		if terminal == nil || terminal.Reason != eventfeed.ReasonPollFailed {
			t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonPollFailed)
		}
		// Nothing durable moved, and the page's events were never delivered:
		// the refusal precedes both.
		assertPositions(t, store.Saves())
		assertIDs(t, h.deliveredIDs())
		if !conn.Closed() {
			t.Error("teardown must close the socket")
		}
	})

	// Present-class entry, and the reason this subtest exists. `held` uses ""
	// as its sentinel for "the final entry was not present-class", so an empty
	// position does not get saved as empty — it collapses into that sentinel,
	// `held != ""` skips acceptPosition AND saveCheckpoint outright, and
	// caught_up is announced anyway. The entry position is DISCARDED, with the
	// drain's deliveries already handed to the consumer.
	t.Run("present-class entry", func(t *testing.T) {
		store := feedtest.NewStore()
		caughtUp := 0
		h := storedHarness(t, store,
			eventfeed.WithStart(eventfeed.StartPresent()),
			eventfeed.WithObserver(eventfeed.Observer{CaughtUp: func() { caughtUp++ }}),
		)
		h.minter.ScriptTicket(ticket(1))
		h.polls.ScriptPage(eventfeed.PollPage{
			Events:   []eventfeed.Event{pollEvent(101)},
			Position: "",
		})
		h.start()

		conn := h.driveToSubscribed()
		conn.Serve(frameConfirm(noFilterIdentifier))
		h.join()

		_, terminal, _ := h.snapshot()
		if terminal == nil || terminal.Reason != eventfeed.ReasonPollFailed {
			t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonPollFailed)
		}
		assertPositions(t, store.Saves())
		assertIDs(t, h.deliveredIDs())
		if caughtUp != 0 {
			t.Errorf("caught_up announcements = %d, want 0 — the walk never completed", caughtUp)
		}
	})

	// The refusal is not retried. A server that answered without a position
	// answers the same way to the same request, so a poll-retry loop would
	// spin: exactly one seam call is made.
	t.Run("is not retried", func(t *testing.T) {
		store := feedtest.NewStore()
		store.Stored("pos-0")
		h := storedHarness(t, store)
		h.minter.ScriptTicket(ticket(1))
		h.polls.ScriptPage(eventfeed.PollPage{Position: ""})
		h.start()

		conn := h.driveToSubscribed()
		conn.Serve(frameConfirm(noFilterIdentifier))
		h.join()

		if n := len(h.polls.Calls()); n != 1 {
			t.Errorf("poll seam calls = %d, want 1 — a malformed shape must not be retried", n)
		}
	})

	// A page on the SECOND hop of the walk is refused the same way, and the
	// first page's save stands: the refusal is per-page, not a rollback.
	t.Run("mid-walk page", func(t *testing.T) {
		store := feedtest.NewStore()
		store.Stored("pos-0")
		h := storedHarness(t, store)
		h.minter.ScriptTicket(ticket(1))
		next := testOrigin + "/999/events.json?after=101"
		h.polls.ScriptPage(eventfeed.PollPage{
			Events:   []eventfeed.Event{pollEvent(101)},
			Position: "pos-1",
			Next:     next,
		})
		h.polls.ScriptPage(eventfeed.PollPage{
			Events:   []eventfeed.Event{pollEvent(102)},
			Position: "",
		})
		h.start()

		conn := h.driveToSubscribed()
		conn.Serve(frameConfirm(noFilterIdentifier))
		h.join()

		_, terminal, _ := h.snapshot()
		if terminal == nil || terminal.Reason != eventfeed.ReasonPollFailed {
			t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonPollFailed)
		}
		// The first page completed normally; only the malformed one is refused.
		assertPositions(t, store.Saves(), "pos-1")
		assertIDs(t, h.deliveredIDs(), 101)
	})
}

// TestDeferredFatalOutranksAMalformedPage is the precedence half of the
// empty-position guard. A socket outcome deferred while the call was in flight
// is the server's own verdict, and disposal discards the deferral — so
// classifying the malformed page first loses it, and a raw
// invalid_event_stream_command is reported as poll_failed.
//
// This mirrors the failed-poll branch, which already dispatches the deferral
// before classifying, for the same reason and with the same comment.
func TestDeferredFatalOutranksAMalformedPage(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store)
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: ""}) // malformed
	deferred := make(chan struct{}, 4)
	h.conn.OnFrameDeferred(func() { deferred <- struct{}{} })
	var conn *feedtest.Conn
	h.polls.OnCall(func(feedtest.PollCall) {
		conn.Serve(frameDisconnect("invalid_event_stream_command", false))
		select {
		case <-deferred:
		case <-time.After(watchdog):
			t.Error("the fatal frame was never deferred")
		}
	})
	h.start()
	conn = h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.join()

	_, terminal, _ := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonProtocolFatal {
		t.Fatalf("terminal = %v, want reason %q — the server's verdict outranks the malformed page",
			terminal, eventfeed.ReasonProtocolFatal)
	}
	assertPositions(t, store.Saves())
}

// TestProtocolFatalHandedOffDuringADrainIsImmediate pins the carve-out at the
// boundary it can actually hold, and that boundary is narrower than an earlier
// revision of this file claimed.
//
// That revision said the guarantee covered "every frame the pump had ALREADY
// READ", and added an atomic flag the pump set after its read with the scan
// spinning on it. It does not work and cannot. The flag is published AFTER the
// read returns, leaving a pre-publish window; and the scan loads it AFTER its
// own empty select, leaving a second window in which the frame lands and the
// flag clears between the two reads. Two samples do not make a happens-before.
// Closing the window for real needs the read to complete inside a critical
// section the scan can enter, and the read blocks indefinitely on a quiet
// socket, so that lock deadlocks the drain against a peer that stopped talking.
//
// So the mechanism is gone and the boundary is stated where it holds: every
// frame HANDED OFF, plus the one a blocked hand-off is holding. That is what
// §23 means by "observed during Draining" — the state machine observes at
// hand-off — and it is what the pumpDepth+1 budget reaches, because Go moves a
// blocked sender's value into the buffer on the first receive.
//
// This drives the guarantee through the hand-off rendezvous rather than
// through a window no scan can see.
func TestProtocolFatalHandedOffDuringADrainIsImmediate(t *testing.T) {
	var caughtUp int
	store := feedtest.NewStore()
	h := storedHarness(t, store, eventfeed.WithObserver(eventfeed.Observer{
		CaughtUp: func() { caughtUp++ },
	}))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.drainHandled()

	queued := make(chan struct{}, eventfeed.ExportPumpDepth)
	h.conn.OnPumpHandedOff(func(bool) {
		select {
		case queued <- struct{}{}:
		default:
		}
	})
	h.pauseAfter = 1
	h.start()

	conn := h.driveToSubscribed()
	h.serveSettled(conn, frameMessage(noFilterIdentifier, 41))
	conn.Serve(frameConfirm(noFilterIdentifier))

	// The consumer parks inside the drain's delivery of the retained event, so
	// the drain is still in flight when the fatal frame is handed off.
	h.waitUntil("the drain parked mid-delivery", func() bool { return len(h.deliveredIDs()) == 1 })
	drain(queued)
	conn.Serve(frameDisconnect("invalid_event_stream_command", false))
	select {
	case <-queued:
	case <-time.After(watchdog):
		t.Fatal("the fatal frame never reached the hand-off queue")
	}

	// Resumed with the fatal frame observable: the drain must not complete.
	h.resume()
	h.join()

	assertProtocolFatalDrain(t, h, store, caughtUp)
	assertIDs(t, h.deliveredIDs(), 41)
}

// staleDeferralHarness wires the shape every grace-phase test needs: a
// position-resume entry, a stalled entry poll — a PollSource that returns only
// on cancellation — and observation points for the deferral and for the
// StaleConnection age. The socket is SILENT throughout: no frame and no read
// error, which is the whole point. A half-open socket produces neither, so the
// staleness firing is the only evidence the connector will ever get, and until
// #758 the one wait that could observe it did not carry the case.
func staleDeferralHarness(t *testing.T, store *feedtest.Store, ages *[]time.Duration, mu *sync.Mutex) (*harness, *deferralWatch) {
	t.Helper()
	h := storedHarness(t, store, eventfeed.WithObserver(eventfeed.Observer{
		StaleConnection: func(age time.Duration) {
			mu.Lock()
			*ages = append(*ages, age)
			mu.Unlock()
		},
	}))
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	w := &deferralWatch{ch: make(chan struct{}, 4)}
	h.conn.OnFrameDeferred(func() { w.ch <- struct{}{} })
	t.Cleanup(func() { w.report(t) })
	return h, w
}

// deferralWatch is the rendezvous for "the in-flight-poll servicing has parked
// an outcome", waited on from the poll seam call's OWN goroutine — which is the
// only place that can hold the call open across the deferral.
//
// It records a miss instead of failing, and the report is registered as a test
// cleanup, because that goroutine outlives the test: a t.Error raised there
// after the test has finished panics the whole binary, replacing a legible
// assertion failure with a stack dump. The cleanup still runs while the test
// can be failed, so the diagnostic survives.
type deferralWatch struct {
	ch chan struct{}
	mu sync.Mutex
	// missed records a lapsed wait — the shape the defect this file's
	// grace-phase tests cover produces.
	missed bool
}

// await blocks until a deferral lands, bounded by the watchdog, reporting
// whether it did. A caller that gets false must stop rather than advance a
// clock nothing is waiting on.
func (w *deferralWatch) await() bool {
	select {
	case <-w.ch:
		return true
	case <-time.After(watchdog):
		w.mu.Lock()
		w.missed = true
		w.mu.Unlock()
		return false
	}
}

func (w *deferralWatch) report(t *testing.T) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.missed {
		t.Error("the staleness expiry was never deferred while the poll seam call was in flight")
	}
}

// TestStalenessDuringAStalledPollTearsDownAtTheGraceDeadline is #758: a
// silently half-open socket observed while a poll seam call is in flight.
//
// Nothing else can report it. The socket produces no frame and no read error,
// so the `staleness` firing is the only evidence there is; the PollSource
// returns only on cancellation, and the cancel would come from the teardown the
// wait is preventing. Before this, pollPage's select carried neither staleness
// case, the firing sat unconsumed, and CatchingUp waited forever.
//
// The wait terminates within DETECTION WINDOW + GRACE PHASE of the last frame:
// one window to decide the socket is dead, one to bound the abandoned call.
func TestStalenessDuringAStalledPollTearsDownAtTheGraceDeadline(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	var mu sync.Mutex
	var ages []time.Duration
	h, deferral := staleDeferralHarness(t, store, &ages, &mu)
	h.polls.StallNext()

	h.polls.OnCall(func(feedtest.PollCall) {
		// The detection window lapses with the call outstanding.
		h.clock.Advance(staleAfter)
		if !deferral.await() {
			return
		}
		// And then the grace phase.
		h.clock.Advance(staleAfter)
	})
	h.start()
	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))

	h.awaitTimer(timerBackoff)
	if !conn.Closed() {
		t.Fatal("the deferred staleness expiry must dispose the attempt")
	}
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("staleness is never terminal; got %v", terminal)
	}
	assertPositions(t, store.Saves())
	assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
	mu.Lock()
	defer mu.Unlock()
	if len(ages) != 1 || ages[0] != staleAfter {
		t.Fatalf("StaleConnection ages = %v, want exactly one of %s", ages, staleAfter)
	}
}

// TestStalenessDuringAnInFlightPollFinishesThePage is acceptance criterion 1
// made deterministic. TestWalkFailureBetweenPages/staleness_expiry pins the
// same ordering, but its poll returns while the firing is still unobserved, so
// the state machine's select may take either ready case; this one holds the
// seam call open until the deferral has demonstrably landed.
//
// The expiry is a SOCKET outcome, and transition 21 observes those at the page
// boundary: the in-flight page is accepted, delivered and SAVED, and only then
// is the socket torn down. Disposing the attempt where the expiry is observed
// is the obvious remedy and it strands exactly that save.
func TestStalenessDuringAnInFlightPollFinishesThePage(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	var mu sync.Mutex
	var ages []time.Duration
	h, deferral := staleDeferralHarness(t, store, &ages, &mu)
	next := testOrigin + "/999/events.json?after=101"
	h.polls.ScriptPage(eventfeed.PollPage{
		Events:   []eventfeed.Event{pollEvent(101)},
		Position: "pos-1",
		Next:     next,
	})
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(102)}, Position: "pos-2"})

	var once sync.Once
	h.polls.OnCall(func(feedtest.PollCall) {
		once.Do(func() {
			h.clock.Advance(staleAfter)
			deferral.await()
		})
	})
	h.start()
	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))

	h.awaitTimer(timerBackoff)
	if got := h.polls.CallCount(); got != 1 {
		t.Fatalf("poll seam calls = %d, want 1 — the walk must not follow `next` on a stale socket", got)
	}
	if !conn.Closed() {
		t.Fatal("the deferred staleness expiry must dispose the attempt")
	}
	assertLedger(t, h.ledger(), []string{"event 101", "save pos-1"})
	assertPositions(t, store.Saves(), "pos-1")
	assertIDs(t, h.deliveredIDs(), 101)
	assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
	mu.Lock()
	defer mu.Unlock()
	if len(ages) != 1 || ages[0] != staleAfter {
		t.Fatalf("StaleConnection ages = %v, want exactly one of %s", ages, staleAfter)
	}
}

// settleWake bounds the one NEGATIVE rendezvous in this file: "the connector
// did not take another wake". Nothing can satisfy it, so it is deliberately far
// shorter than the watchdog — it is a wall-clock cost the passing path always
// pays, and the price of the assertion having teeth. No virtual time and no
// timing assertion depends on it; it only orders a wake that already happened
// (or did not) ahead of the next clock advance.
const settleWake = 250 * time.Millisecond

// TestGracePhaseIsImmuneToFrameResets is the first half of acceptance
// criterion 3. The grace phase is a fixed deadline read at the deferral, not a
// window the peer can push: it bounds how long the connector waits for an
// abandoned seam call, which has nothing to do with whether the peer is still
// talking.
//
// The distinction is sharper than "does it eventually end", and the sharper
// form is what is asserted. A frame arriving inside the phase re-arms staleness
// and thereby hands the wait a WAKE, and a wake is enough to carry an
// implementation that also moved its deadline to a correct-looking teardown.
// So the test consumes that wake first, and then requires the phase to reach
// its deadline on a wake of its OWN — which only a deadline the frame could not
// move can produce.
func TestGracePhaseIsImmuneToFrameResets(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	var mu sync.Mutex
	var ages []time.Duration
	h, deferral := staleDeferralHarness(t, store, &ages, &mu)
	h.polls.StallNext()
	h.drainHandled()
	queued := make(chan struct{}, eventfeed.ExportPumpDepth)
	h.conn.OnPumpHandedOff(func(bool) {
		select {
		case queued <- struct{}{}:
		default:
		}
	})
	wakes := make(chan struct{}, 8)
	h.conn.OnSupersededWake(func() {
		select {
		case wakes <- struct{}{}:
		default:
		}
	})

	var conn *feedtest.Conn
	h.polls.OnCall(func(feedtest.PollCall) {
		h.clock.Advance(staleAfter)
		if !deferral.await() {
			return
		}
		// Half a window into the grace phase, the peer speaks again.
		h.clock.Advance(staleAfter / 2)
		drain(queued)
		drain(wakes)
		conn.Serve(framePing())
		select {
		case <-queued:
		case <-time.After(watchdog):
			t.Error("the ping never reached the hand-off queue")
			return
		}
		// The ping's re-arm is legitimate and so is any wake it delivers —
		// wakes may be early, and the deadline is what decides. Consuming it
		// here is what leaves the phase nothing but its own wake to finish on.
		select {
		case <-wakes:
		case <-time.After(settleWake):
		}
		// The remaining half still lapses the grace phase: had the ping moved
		// it, nothing fires here and the wait is left holding a window half a
		// window out of reach, with the wake it might have ridden already spent.
		h.clock.Advance(staleAfter / 2)
	})
	h.start()
	conn = h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))

	h.awaitTimer(timerBackoff)
	if !conn.Closed() {
		t.Fatal("a frame inside the grace phase must not extend it")
	}
	assertPositions(t, store.Saves())
	mu.Lock()
	defer mu.Unlock()
	if len(ages) != 1 {
		t.Fatalf("StaleConnection ages = %v, want exactly one", ages)
	}
}

// TestGracePhaseIsImmuneToPumpSuspension is the second half of acceptance
// criterion 3, and the one the previous borrowed bound failed.
//
// A blocked hand-off suspends staleness EVALUATION, on the premise that a full
// queue proves the peer is outrunning a connector that is still consuming. This
// wait is the one that deliberately stops consuming — an outcome is already
// parked in the single deferral slot — so the premise is false by construction
// here, and a bound that borrows the staleness VERDICT is re-armed forever by a
// peer that keeps sending. The grace phase is a deadline instead, and a
// deadline cannot be suspended.
func TestGracePhaseIsImmuneToPumpSuspension(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	var mu sync.Mutex
	var ages []time.Duration
	h, deferral := staleDeferralHarness(t, store, &ages, &mu)
	h.polls.StallNext()
	h.drainHandled()
	blocked := make(chan struct{}, 1)
	h.conn.OnPumpBlocked(func() {
		select {
		case blocked <- struct{}{}:
		default:
		}
	})

	var conn *feedtest.Conn
	h.polls.OnCall(func(feedtest.PollCall) {
		h.clock.Advance(staleAfter)
		if !deferral.await() {
			return
		}
		// A peer that keeps sending fills the queue nothing is draining.
		for i := 0; i <= eventfeed.ExportPumpDepth; i++ {
			conn.Serve(framePing())
		}
		select {
		case <-blocked:
		case <-time.After(watchdog):
			t.Error("the pump never blocked on a full hand-off queue")
			return
		}
		// One grace phase from the deferral, with the suspension in force.
		h.clock.Advance(staleAfter)
	})
	h.start()
	conn = h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))

	h.awaitTimer(timerBackoff)
	if !conn.Closed() {
		t.Fatal("a blocked hand-off must not suspend the grace phase")
	}
	assertPositions(t, store.Saves())
	mu.Lock()
	defer mu.Unlock()
	if len(ages) != 1 {
		t.Fatalf("StaleConnection ages = %v, want exactly one", ages)
	}
}

// TestDeferredFatalOutranksADeferredStalenessExpiry is acceptance criterion 4's
// probe half. The deferral slot's third form carries no frame, and the
// protocol-fatal probe runs at every deferred-disposition boundary — so it must
// read the slot as "not a frame" explicitly rather than by accident of what
// parsing a zero item happens to return, and it must still reach the fatal
// frame the pump queued behind the expiry.
//
// The server's own verdict outranks the connector's: an
// `invalid_event_stream_command` already read is Terminal(protocol_fatal), not
// a staleness reconnect.
func TestDeferredFatalOutranksADeferredStalenessExpiry(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	var mu sync.Mutex
	var ages []time.Duration
	h, deferral := staleDeferralHarness(t, store, &ages, &mu)
	// A second mint that fails terminally: a connector that wrongly reconnects
	// ends at Terminal(mint_failed) with two mint calls, so both assertions
	// below discriminate rather than merely time out.
	h.minter.ScriptError(&eventfeed.MintError{Kind: eventfeed.MintUnrecoverable})
	h.polls.StallNext()
	h.drainHandled()
	queued := make(chan struct{}, eventfeed.ExportPumpDepth)
	h.conn.OnPumpHandedOff(func(bool) {
		select {
		case queued <- struct{}{}:
		default:
		}
	})

	var conn *feedtest.Conn
	h.polls.OnCall(func(feedtest.PollCall) {
		h.clock.Advance(staleAfter)
		if !deferral.await() {
			return
		}
		// The fatal frame is served only once the expiry holds the slot, and
		// the wait is for the pump's hand-off: the guarantee is about a frame
		// the state machine can observe, so a test that raced the read would
		// prove nothing.
		drain(queued)
		conn.Serve(frameDisconnect("invalid_event_stream_command", false))
		select {
		case <-queued:
		case <-time.After(watchdog):
			t.Error("the fatal frame never reached the hand-off queue")
			return
		}
		h.clock.Advance(staleAfter)
	})
	h.start()
	conn = h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitEndOrReconnect()
	h.join()

	_, terminal, _ := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonProtocolFatal {
		t.Fatalf("terminal = %v, want reason %q — the server's verdict was already read",
			terminal, eventfeed.ReasonProtocolFatal)
	}
	if got := h.minter.Calls(); got != 1 {
		t.Fatalf("mint calls = %d, want 1: a protocol-fatal rejection must not reconnect from any state", got)
	}
	assertPositions(t, store.Saves())
	mu.Lock()
	defer mu.Unlock()
	if len(ages) != 0 {
		t.Fatalf("StaleConnection ages = %v, want none — the fatal verdict governs", ages)
	}
}

// TestSuspendedStalenessDuringAnInFlightPollDoesNotDefer is the other half of
// acceptance criterion 1: the new cases in the in-flight-poll select evaluate
// the firing under the SAME rule every other select applies, rather than
// treating any firing as the socket's death.
//
// A firing whose window overlapped a blocked hand-off is not evidence — a full
// queue proves the peer was sending faster than the connector consumed, the
// opposite of a dead peer — so it is disregarded and re-armed, and the call is
// still awaited. Deferring it instead abandons a live socket and a page the
// server had already served.
//
// The arrangement is the only one that holds the suspension open across the
// select: this wait DOES drain the hand-off queue, so a pump blocked while the
// state machine is sitting in the select would be released by the very next
// receive. Parking the state machine inside a frame-handled callback is what
// keeps it from draining while the peer fills the queue.
func TestSuspendedStalenessDuringAnInFlightPollDoesNotDefer(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	var mu sync.Mutex
	var ages []time.Duration
	h, _ := staleDeferralHarness(t, store, &ages, &mu)
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})

	gate := make(chan struct{})
	parked := make(chan struct{})
	var parkOnce sync.Once
	h.conn.OnFrameHandled(func(kind string) {
		if kind != "ping" {
			return
		}
		parkOnce.Do(func() {
			close(parked)
			<-gate
		})
	})
	blocked := make(chan struct{}, 1)
	h.conn.OnPumpBlocked(func() {
		select {
		case blocked <- struct{}{}:
		default:
		}
	})

	var conn *feedtest.Conn
	var once sync.Once
	h.polls.OnCall(func(feedtest.PollCall) {
		once.Do(func() {
			// The state machine parks inside the callback for this ping, so it
			// stops dequeuing while the peer keeps sending.
			conn.Serve(framePing())
			select {
			case <-parked:
			case <-time.After(watchdog):
				t.Error("the state machine never parked in the frame-handled callback")
				return
			}
			for i := 0; i <= eventfeed.ExportPumpDepth; i++ {
				conn.Serve(framePing())
			}
			select {
			case <-blocked:
			case <-time.After(watchdog):
				t.Error("the pump never blocked on a full hand-off queue")
				return
			}
			// The window closes over a hand-off the pump spent blocked.
			h.clock.Advance(staleAfter)
			close(gate)
		})
	})
	h.start()
	conn = h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))

	// The socket survives, the page is served, and the walk reaches its head:
	// a disregarded firing consumes neither the call nor the connection.
	h.awaitStreaming()
	if conn.Closed() {
		t.Fatal("a firing whose window overlapped a blocked hand-off must not tear the socket down")
	}
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("a suspended staleness firing is never terminal; got %v", terminal)
	}
	assertPositions(t, store.Saves(), "pos-1")
	mu.Lock()
	defer mu.Unlock()
	if len(ages) != 0 {
		t.Fatalf("StaleConnection ages = %v, want none — the firing was not evidence", ages)
	}
}
