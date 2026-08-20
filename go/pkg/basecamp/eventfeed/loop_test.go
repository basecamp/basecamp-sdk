// Slice-4a acceptance tests for the connector run loop: mint → dial →
// welcome → subscribe → confirm/reject/deadline (SPEC.md §23 transitions
// 1–15 plus the out-of-inventory mint_failed / invalid_cable_url /
// buffer_overflow / usage edges). Structured to mirror the tier-2 scenario
// fixtures (01–07, 17) so the fixture driver can later subsume them: all
// time flows through the injected feedtest.Clock — no wall-clock sleeps
// except bounded watchdog joins — and mint/connect counts count seam calls.
package eventfeed_test

import (
	"context"
	"encoding/json"
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

// watchdog bounds every rendezvous wait; virtual time never depends on it.
const watchdog = 5 * time.Second

// harness wires a Connector to the feedtest fakes and a collector goroutine
// consuming Events.
type harness struct {
	t      *testing.T
	clock  *feedtest.Clock
	tr     *feedtest.Transport
	minter *feedtest.Minter
	polls  *feedtest.Polls
	conn   *eventfeed.Connector

	cancel   context.CancelFunc
	done     chan struct{}
	boundary chan eventfeed.CatchUpBoundary
	handled  chan string
	states   chan string

	// breakAfter, when positive, makes the collector break out of the range
	// after that many delivered events — the consumer-break teardown path.
	breakAfter int
	// pauseAfter, when positive, parks the collector INSIDE the loop body once
	// that many events have been delivered, until resume is called. The loop
	// body is the consumer's processing, so parking there parks the state
	// machine mid-delivery — the serial back-pressure the deferred-consumption
	// and staleness-suspension rules are stated over.
	pauseAfter int
	pause      chan struct{}
	pauseOnce  sync.Once

	mu       sync.Mutex
	events   []eventfeed.Event
	terminal *eventfeed.TerminalError
	elements int
	// log is the single ordered ledger of deliveries and checkpoint calls.
	// Both are produced on the consumer's goroutine (the loop body IS the
	// consumer), so their relative order is total, not sampled.
	log []string
}

// testOrigin is the harness's configured API base origin: the checkpoint
// key's origin and the same-origin reference for continuation validation.
const testOrigin = "https://3.basecampapi.com"

func newHarness(t *testing.T, opts ...eventfeed.Option) *harness {
	t.Helper()
	h := &harness{
		t:        t,
		clock:    feedtest.NewClock(),
		tr:       feedtest.NewTransport(),
		minter:   feedtest.NewMinter(),
		polls:    feedtest.NewPolls(),
		done:     make(chan struct{}),
		pause:    make(chan struct{}),
		boundary: make(chan eventfeed.CatchUpBoundary, 8),
		handled:  make(chan string, 256),
		states:   make(chan string, 256),
	}
	base := []eventfeed.Option{
		eventfeed.WithTransport(h.tr),
		eventfeed.WithClock(h.clock),
	}
	c, err := eventfeed.New(testOrigin, "5951425", h.minter, h.polls, append(base, opts...)...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	h.conn = c
	// A non-blocking send: a reconnect re-enters the boundary, and a test
	// that never reads it must not wedge the loop.
	c.OnCatchUpEntered(func(b eventfeed.CatchUpBoundary) {
		select {
		case h.boundary <- b:
		default:
		}
	})
	c.OnFrameHandled(func(kind string) { h.handled <- kind })
	c.OnStateChanged(func(state string) { h.states <- state })
	return h
}

// record appends one entry to the ordered delivery/checkpoint ledger.
func (h *harness) record(entry string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.log = append(h.log, entry)
}

// ledger returns the ordered delivery/checkpoint log.
func (h *harness) ledger() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.log...)
}

// start ranges Events on a collector goroutine.
func (h *harness) start() {
	h.t.Helper()
	h.startCtx(context.Background())
}

// startCtx starts the iteration under a caller-supplied parent context, for
// the one assertion that needs a context VALUE to travel into a seam.
func (h *harness) startCtx(parent context.Context) {
	h.t.Helper()
	ctx, cancel := context.WithCancel(parent)
	h.cancel = cancel
	h.t.Cleanup(cancel)
	h.t.Cleanup(func() { h.conn.Close() })
	// A parked collector must never outlive the test, however it ended.
	h.t.Cleanup(h.resume)
	go func() {
		defer close(h.done)
		for ev, err := range h.conn.Events(ctx) {
			h.mu.Lock()
			h.elements++
			if err != nil {
				var te *eventfeed.TerminalError
				if errors.As(err, &te) {
					h.terminal = te
				} else {
					h.t.Errorf("iteration yielded a non-terminal error element: %v", err)
				}
			} else {
				h.events = append(h.events, ev)
				h.log = append(h.log, fmt.Sprintf("event %d", ev.ID))
			}
			stop := h.breakAfter > 0 && len(h.events) >= h.breakAfter
			park := err == nil && h.pauseAfter > 0 && len(h.events) == h.pauseAfter
			h.mu.Unlock()
			if park {
				<-h.pause
			}
			if stop {
				break
			}
		}
	}()
}

// resume releases a collector parked by pauseAfter.
func (h *harness) resume() {
	h.pauseOnce.Do(func() { close(h.pause) })
}

// join waits for the iteration to end.
func (h *harness) join() {
	h.t.Helper()
	select {
	case <-h.done:
	case <-time.After(watchdog):
		h.t.Fatalf("iteration did not end within the watchdog")
	}
}

// snapshot returns the collected elements so far.
func (h *harness) snapshot() (events []eventfeed.Event, terminal *eventfeed.TerminalError, elements int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]eventfeed.Event(nil), h.events...), h.terminal, h.elements
}

// awaitTimer blocks until a timer of the given kind is outstanding.
func (h *harness) awaitTimer(kind string) {
	h.t.Helper()
	done := make(chan struct{})
	go func() { h.clock.AwaitTimer(kind); close(done) }()
	select {
	case <-done:
	case <-time.After(watchdog):
		h.t.Fatalf("timer %q was not armed within the watchdog", kind)
	}
}

// fireTimer awaits and fires the named timer without advancing the clock,
// returning its scheduled delay.
func (h *harness) fireTimer(kind string) time.Duration {
	h.t.Helper()
	h.awaitTimer(kind)
	d, ok := h.clock.FireTimer(kind)
	if !ok {
		h.t.Fatalf("no outstanding %q timer to fire", kind)
	}
	return d
}

// liveConn awaits the current socket (staleness arms at socket open) and
// returns it.
func (h *harness) liveConn() *feedtest.Conn {
	h.t.Helper()
	h.awaitTimer(timerStaleness)
	conn := h.tr.LastConn()
	if conn == nil {
		h.t.Fatal("no connection was dialed")
	}
	return conn
}

// awaitBoundary waits for the confirmed subscription to reach the catch-up
// hand-off.
func (h *harness) awaitBoundary() eventfeed.CatchUpBoundary {
	h.t.Helper()
	select {
	case b := <-h.boundary:
		return b
	case <-time.After(watchdog):
		h.t.Fatal("the catch-up boundary was not reached within the watchdog")
		return eventfeed.CatchUpBoundary{}
	}
}

// awaitStreaming consumes state notifications until Streaming is entered —
// the rendezvous for "the walk finished and the buffer drained", after which
// the walk's deliveries, saves, and timer set are all settled.
func (h *harness) awaitStreaming() {
	h.t.Helper()
	deadline := time.After(watchdog)
	for {
		select {
		case s := <-h.states:
			if s == stateStreaming {
				return
			}
		case <-deadline:
			h.t.Fatalf("state %q was not entered within the watchdog", stateStreaming)
		}
	}
}

// deliveredIDs returns the ids delivered so far, in delivery order.
func (h *harness) deliveredIDs() []int64 {
	events, _, _ := h.snapshot()
	ids := make([]int64, len(events))
	for i, ev := range events {
		ids[i] = ev.ID
	}
	return ids
}

// serveSettled serves frames and returns once the pump has taken them all:
// a trailing ping is served last, so an empty conn queue proves every earlier
// frame already reached the state machine's hand-off queue (the pump reads,
// resets staleness, and hands off in that order).
func (h *harness) serveSettled(conn *feedtest.Conn, frames ...[]byte) {
	h.t.Helper()
	for _, f := range frames {
		conn.Serve(f)
	}
	conn.Serve(framePing())
	h.waitUntil("pump took the served frames", func() bool { return conn.Pending() == 0 })
}

// drainHandled consumes frame-handled notifications for the rest of the test.
//
// The notification send is BLOCKING, and the protocol-fatal probe handles a
// whole queue's worth of frames in a single pass — far more than the channel
// buffers. A test that fills the pump's queue and does not itself await
// handled frames must drain them, or the loop wedges inside the probe on a
// send nobody is receiving. That is a property of the harness's notification,
// not of the connector: the wedge shows up as the state machine parked in
// newHarness's OnFrameHandled callback.
func (h *harness) drainHandled() {
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-h.handled:
			case <-done:
				return
			}
		}
	}()
	h.t.Cleanup(func() { close(done) })
}

// awaitFrameHandled consumes handled-frame notifications until the given
// kind is seen.
func (h *harness) awaitFrameHandled(kind string) {
	h.t.Helper()
	deadline := time.After(watchdog)
	for {
		select {
		case k := <-h.handled:
			if k == kind {
				return
			}
		case <-deadline:
			h.t.Fatalf("frame kind %q was not handled within the watchdog", kind)
		}
	}
}

// waitUntil polls cond under a bounded watchdog — a rendezvous on fake-side
// state (never on virtual time).
func (h *harness) waitUntil(desc string, cond func() bool) {
	h.t.Helper()
	deadline := time.Now().Add(watchdog)
	for !cond() {
		if time.Now().After(deadline) {
			h.t.Fatalf("condition %q not reached within the watchdog", desc)
		}
		time.Sleep(time.Millisecond)
	}
}

// assertTimers asserts the exact outstanding timer multiset.
func assertTimers(t *testing.T, clock *feedtest.Clock, want map[string]int) {
	t.Helper()
	got := map[string]int{}
	for _, name := range clock.Outstanding() {
		got[name]++
	}
	if len(got) != len(want) {
		t.Fatalf("outstanding timers = %v, want %v", got, want)
	}
	for name, n := range want {
		if got[name] != n {
			t.Fatalf("outstanding timers = %v, want %v", got, want)
		}
	}
}

// assertGoroutinesSettle waits (bounded) for the goroutine count to return
// to the pre-test base.
func assertGoroutinesSettle(t *testing.T, base int) {
	t.Helper()
	deadline := time.Now().Add(watchdog)
	for {
		if runtime.NumGoroutine() <= base {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("goroutines did not settle: base %d, now %d", base, runtime.NumGoroutine())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Timer kind literals (SPEC.md §23, kebab-case, verbatim).
const (
	timerHandshakeDeadline    = "handshake-deadline"
	timerConfirmationDeadline = "confirmation-deadline"
	timerBackoff              = "backoff"
	timerStaleness            = "staleness"
	timerRepairPoll           = "repair-poll"
	timerPollRetry            = "poll-retry"
)

// stateStreaming is the fixture spelling of the steady state.
const stateStreaming = "streaming"

// ticket builds the nth scripted stream ticket; URLs are distinct per index
// so fresh-ticket assertions have teeth.
func ticket(n int) eventfeed.StreamTicket {
	return eventfeed.StreamTicket{
		Ticket:    fmt.Sprintf("ticket-%d", n),
		ExpiresIn: 120,
		URL:       fmt.Sprintf("wss://cable.example.com/cable?ticket=ticket-%d", n),
	}
}

func frameWelcome() []byte { return []byte(`{"type":"welcome"}`) }

func framePing() []byte { return []byte(`{"type":"ping","message":1690000000}`) }

func frameConfirm(identifier string) []byte {
	b, err := json.Marshal(map[string]string{"type": "confirm_subscription", "identifier": identifier})
	if err != nil {
		panic(err)
	}
	return b
}

func frameReject(identifier string) []byte {
	b, err := json.Marshal(map[string]string{"type": "reject_subscription", "identifier": identifier})
	if err != nil {
		panic(err)
	}
	return b
}

func frameDisconnect(reason string, reconnect bool) []byte {
	b, err := json.Marshal(map[string]any{"type": "disconnect", "reason": reason, "reconnect": reconnect})
	if err != nil {
		panic(err)
	}
	return b
}

func frameMessage(identifier string, id int64) []byte {
	return frameMessageCreatedAt(identifier, id, "2026-08-01T12:00:00Z")
}

// frameMessageCreatedAt builds a correlated broadcast with a caller-chosen
// created_at. Well-formed JSON with no "type" key and the connector's own
// identifier, so it reaches decodeMessageEvent — which is the point when
// createdAt is deliberately unparseable.
func frameMessageCreatedAt(identifier string, id int64, createdAt string) []byte {
	payload := map[string]any{
		"id":                 id,
		"kind":               "message",
		"event_type":         "message.created",
		"action":             "created",
		"created_at":         createdAt,
		"bucket_id":          2,
		"creator_id":         3,
		"recording_id":       900,
		"visible_to_clients": false,
	}
	b, err := json.Marshal(map[string]any{"identifier": identifier, "message": payload})
	if err != nil {
		panic(err)
	}
	return b
}

var noFilterIdentifier = eventfeed.ExportSubscribeIdentifier(eventfeed.Filters{})

// driveToSubscribed mints, dials, serves welcome, and waits for the
// subscribe write (the confirmation deadline arms with it).
func (h *harness) driveToSubscribed() *feedtest.Conn {
	h.t.Helper()
	conn := h.liveConn()
	conn.Serve(frameWelcome())
	h.awaitTimer(timerConfirmationDeadline)
	return conn
}

// TestHappyPathReachesCatchUpBoundary drives mint → dial → welcome →
// subscribe → confirm and asserts the confirmed subscription reaches the
// slice-4b boundary with the right entry cursor, exact subscribe bytes, a
// verbatim dial of the mint's URL, and only the staleness timer surviving
// (fixture 01's lifecycle prefix).
func TestHappyPathReachesCatchUpBoundary(t *testing.T) {
	var mu sync.Mutex
	var lifecycle []string
	obs := eventfeed.Observer{
		Connecting: func(attempt int, delay time.Duration) {
			mu.Lock()
			lifecycle = append(lifecycle, fmt.Sprintf("connecting %d %s", attempt, delay))
			mu.Unlock()
		},
		Connected: func() { mu.Lock(); lifecycle = append(lifecycle, "connected"); mu.Unlock() },
		Confirmed: func() { mu.Lock(); lifecycle = append(lifecycle, "confirmed"); mu.Unlock() },
	}
	h := newHarness(t, eventfeed.WithObserver(obs))
	h.minter.ScriptTicket(ticket(1))
	// The confirmed subscription walks straight into catch-up (catchup.go);
	// this test's subject is the lifecycle prefix, so the walk is one empty
	// page at the frozen head.
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	base := runtime.NumGoroutine()
	h.start()

	conn := h.driveToSubscribed()
	if got, want := h.tr.DialedURLs(), []string{ticket(1).URL}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("dialed URLs = %v, want %v (the mint's url, verbatim)", got, want)
	}
	writes := conn.Writes()
	if len(writes) != 1 || string(writes[0]) != string(eventfeed.ExportSubscribeFrame(eventfeed.Filters{})) {
		t.Fatalf("subscribe writes = %q, want exactly %q", writes, eventfeed.ExportSubscribeFrame(eventfeed.Filters{}))
	}

	conn.Serve(frameConfirm(noFilterIdentifier))
	b := h.awaitBoundary()
	if b.Entry != (eventfeed.Cursor{}) {
		t.Fatalf("entry cursor = %+v, want the zero Cursor (bare present entry)", b.Entry)
	}
	if !b.PresentClass {
		t.Fatal("entry should be present-class")
	}
	if len(b.Buffered) != 0 {
		t.Fatalf("buffered = %v, want none", b.Buffered)
	}
	if got := h.minter.Calls(); got != 1 {
		t.Fatalf("mint seam calls = %d, want 1", got)
	}
	h.awaitStreaming()
	assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1})

	mu.Lock()
	wantLifecycle := []string{"connecting 1 0s", "connected", "confirmed"}
	if len(lifecycle) != len(wantLifecycle) || lifecycle[0] != wantLifecycle[0] || lifecycle[1] != wantLifecycle[1] || lifecycle[2] != wantLifecycle[2] {
		mu.Unlock()
		t.Fatalf("observer lifecycle = %v, want %v", lifecycle, wantLifecycle)
	}
	mu.Unlock()

	h.conn.Close()
	h.join()
	if _, terminal, elements := h.snapshot(); terminal != nil || elements != 0 {
		t.Fatalf("clean close must end iteration with no elements; got %d elements, terminal %v", elements, terminal)
	}
	if !conn.Closed() {
		t.Fatal("the connector must close the socket on Close")
	}
	assertTimers(t, h.clock, map[string]int{})
	assertGoroutinesSettle(t, base)
}

// TestConfirmationGating is acceptance (b) (fixture 02): a message frame
// landing before confirm_subscription is buffered — zero deliveries, zero
// poll seam calls — and the buffered event is carried into the catch-up
// hand-off.
func TestConfirmationGating(t *testing.T) {
	h := newHarness(t)
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameMessage(noFilterIdentifier, 201))
	h.awaitFrameHandled("message")

	if events, terminal, _ := h.snapshot(); len(events) != 0 || terminal != nil {
		t.Fatalf("nothing may be delivered before confirmation; got %v / %v", events, terminal)
	}
	if got := h.polls.CallCount(); got != 0 {
		t.Fatalf("poll seam calls before confirmation = %d, want 0", got)
	}
	assertTimers(t, h.clock, map[string]int{timerConfirmationDeadline: 1, timerStaleness: 1})

	conn.Serve(frameConfirm(noFilterIdentifier))
	b := h.awaitBoundary()
	if len(b.Buffered) != 1 || b.Buffered[0].ID != 201 {
		t.Fatalf("buffered at hand-off = %v, want exactly event 201", b.Buffered)
	}
	// Confirmation is what unblocks the walk: the entry poll happens only
	// now, and the buffered event drains exactly once.
	h.awaitStreaming()
	if got := h.polls.CallCount(); got != 1 {
		t.Fatalf("poll seam calls after confirmation = %d, want the one entry poll", got)
	}
	assertIDs(t, h.deliveredIDs(), 201)

	h.conn.Close()
	h.join()
	if _, terminal, elements := h.snapshot(); terminal != nil || elements != 1 {
		t.Fatalf("clean close must end iteration with no error element; got %d elements, terminal %v", elements, terminal)
	}
}

// TestConfirmationDeadlineTeardown is acceptance (c) (fixture 03): the
// lapsed confirmation deadline disposes the whole attempt — socket closed,
// exactly {backoff} outstanding — and the retry mints a FRESH ticket and
// dials the new URL.
func TestConfirmationDeadlineTeardown(t *testing.T) {
	h := newHarness(t)
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	base := runtime.NumGoroutine()
	h.start()

	conn1 := h.driveToSubscribed()
	if d := h.fireTimer(timerConfirmationDeadline); d != eventfeed.DefaultConfirmationDeadline {
		t.Fatalf("confirmation deadline armed for %s, want %s", d, eventfeed.DefaultConfirmationDeadline)
	}
	h.awaitTimer(timerBackoff)
	if !conn1.Closed() {
		t.Fatal("deadline teardown must close the socket")
	}
	assertTimers(t, h.clock, map[string]int{timerBackoff: 1})

	if d := h.fireTimer(timerBackoff); d < 0 || d > time.Second {
		t.Fatalf("first backoff delay = %s, want within [0, 1s] (full jitter, n=1)", d)
	}
	conn2 := h.driveToSubscribed()
	if got := h.minter.Calls(); got != 2 {
		t.Fatalf("mint seam calls = %d, want 2 (a fresh ticket on every pass)", got)
	}
	urls := h.tr.DialedURLs()
	if len(urls) != 2 || urls[0] != ticket(1).URL || urls[1] != ticket(2).URL {
		t.Fatalf("dialed URLs = %v, want the two mints' URLs in order", urls)
	}
	assertTimers(t, h.clock, map[string]int{timerConfirmationDeadline: 1, timerStaleness: 1})

	h.conn.Close()
	h.join()
	if !conn2.Closed() {
		t.Fatal("Close must close the live socket")
	}
	assertGoroutinesSettle(t, base)
}

// TestTerminalRejection is acceptance (d) (fixture 04): reject_subscription
// is always terminal — the connector explicitly closes the still-open
// socket, surfaces subscription_rejected as the single error element, makes
// zero reconnect seam calls, and leaves the timer set empty.
func TestTerminalRejection(t *testing.T) {
	h := newHarness(t)
	h.minter.ScriptTicket(ticket(1))
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameReject(noFilterIdentifier))
	h.join()

	_, terminal, elements := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonSubscriptionRejected {
		t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonSubscriptionRejected)
	}
	if elements != 1 {
		t.Fatalf("iteration elements = %d, want exactly the one error element", elements)
	}
	if !conn.Closed() {
		t.Fatal("the connector must explicitly close the rejected socket (Action Cable leaves it open)")
	}
	if got := h.minter.Calls(); got != 1 {
		t.Fatalf("mint seam calls = %d, want 1 (ZERO reconnect attempts)", got)
	}
	if got := len(h.tr.Dials()); got != 1 {
		t.Fatalf("connect seam calls = %d, want 1", got)
	}
	assertTimers(t, h.clock, map[string]int{})
}

// TestHandshakeDeadline covers transition 9 (no welcome before the deadline)
// and transition 7's deadline-expiry-mid-dial (the pending dial is
// cancelled); both recover through Backoff with a fresh ticket.
func TestHandshakeDeadline(t *testing.T) {
	t.Run("no welcome", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptTicket(ticket(1))
		h.minter.ScriptTicket(ticket(2))
		h.start()

		conn1 := h.liveConn()
		if d := h.fireTimer(timerHandshakeDeadline); d != 10*time.Second {
			t.Fatalf("handshake deadline armed for %s, want 10s", d)
		}
		h.awaitTimer(timerBackoff)
		if !conn1.Closed() {
			t.Fatal("handshake-deadline teardown must close the socket")
		}
		assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
		h.fireTimer(timerBackoff)
		h.liveConn()
		if got := h.minter.Calls(); got != 2 {
			t.Fatalf("mint seam calls = %d, want 2", got)
		}
	})

	t.Run("mid-dial", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptTicket(ticket(1))
		h.minter.ScriptTicket(ticket(2))
		h.tr.StallNextDial()
		h.start()

		// The deadline arms on entry to Connecting, BEFORE dial.
		h.fireTimer(timerHandshakeDeadline)
		h.awaitTimer(timerBackoff)
		if got := len(h.tr.Dials()); got != 1 {
			t.Fatalf("connect seam calls = %d, want 1 (the stalled dial still counts)", got)
		}
		h.fireTimer(timerBackoff)
		h.liveConn()
		urls := h.tr.DialedURLs()
		if len(urls) != 2 || urls[1] != ticket(2).URL {
			t.Fatalf("dialed URLs = %v, want the second mint's URL on retry", urls)
		}
	})
}

// TestProtocolFatalDisconnect (fixture 06 + the state-generic rule): a raw
// disconnect frame with reason invalid_event_stream_command is
// Terminal(protocol_fatal) from both pre-confirm socket-open states, never
// retried into.
func TestProtocolFatalDisconnect(t *testing.T) {
	for _, tc := range []struct {
		name    string
		welcome bool
	}{
		{"awaiting confirmation", true},
		{"awaiting welcome", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.minter.ScriptTicket(ticket(1))
			h.start()

			var conn *feedtest.Conn
			if tc.welcome {
				conn = h.driveToSubscribed()
			} else {
				conn = h.liveConn()
			}
			conn.Serve(frameDisconnect("invalid_event_stream_command", false))
			h.join()

			_, terminal, _ := h.snapshot()
			if terminal == nil || terminal.Reason != eventfeed.ReasonProtocolFatal {
				t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonProtocolFatal)
			}
			if !conn.Closed() {
				t.Fatal("the connector must close the socket")
			}
			if got := h.minter.Calls(); got != 1 {
				t.Fatalf("mint seam calls = %d, want 1 (zero further mints)", got)
			}
			assertTimers(t, h.clock, map[string]int{})
		})
	}
}

// TestUnauthorizedDisconnectReconnects (fixture 07): the same
// reconnect:false flag as protocol-fatal, the opposite response — dispatch
// is on the REASON string. A pre-welcome `unauthorized` disconnect retries
// with a fresh ticket.
func TestUnauthorizedDisconnectReconnects(t *testing.T) {
	h := newHarness(t)
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	h.start()

	conn1 := h.liveConn()
	conn1.Serve(frameDisconnect("unauthorized", false))
	h.awaitTimer(timerBackoff)
	if !conn1.Closed() {
		t.Fatal("teardown must close the socket")
	}
	assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
	h.fireTimer(timerBackoff)

	h.driveToSubscribed()
	if got := h.minter.Calls(); got != 2 {
		t.Fatalf("mint seam calls = %d, want 2 (fresh-ticket retry, not terminal)", got)
	}
	urls := h.tr.DialedURLs()
	if len(urls) != 2 || urls[1] != ticket(2).URL {
		t.Fatalf("dialed URLs = %v, want the fresh mint's URL second", urls)
	}
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("unauthorized below the threshold must not be terminal; got %v", terminal)
	}
}

// TestRemoteDisconnectRemints (fixture 17's lifecycle half): a `remote` /
// reconnect:true disconnect rides the existing Backoff transitions —
// teardown, backoff, fresh mint, reconnect to the new URL.
func TestRemoteDisconnectRemints(t *testing.T) {
	h := newHarness(t)
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	h.start()

	conn1 := h.driveToSubscribed()
	conn1.Serve(frameDisconnect("remote", true))
	h.awaitTimer(timerBackoff)
	if !conn1.Closed() {
		t.Fatal("teardown must close the socket")
	}
	h.fireTimer(timerBackoff)
	h.driveToSubscribed()
	urls := h.tr.DialedURLs()
	if len(urls) != 2 || urls[1] != ticket(2).URL {
		t.Fatalf("dialed URLs = %v, want a fresh mint's URL on reconnect", urls)
	}
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("`remote` must not be terminal; got %v", terminal)
	}
}

// TestAuthorizationFailureThreshold: the shared connection-level counter —
// unauthorized mints and `unauthorized` disconnects — surfaces
// authorization_failed on the 3rd consecutive failure (transitions 4/5,
// 9/10), including across mixed shapes.
func TestAuthorizationFailureThreshold(t *testing.T) {
	unauthorized := func() error {
		return &eventfeed.MintError{Kind: eventfeed.MintUnauthorized, Err: errors.New("401")}
	}

	t.Run("three unauthorized mints", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptError(unauthorized())
		h.minter.ScriptError(unauthorized())
		h.minter.ScriptError(unauthorized())
		h.start()

		h.fireTimer(timerBackoff) // after mint 1
		h.fireTimer(timerBackoff) // after mint 2
		h.join()                  // mint 3 → terminal

		_, terminal, _ := h.snapshot()
		if terminal == nil || terminal.Reason != eventfeed.ReasonAuthorizationFailed {
			t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonAuthorizationFailed)
		}
		if got := h.minter.Calls(); got != 3 {
			t.Fatalf("mint seam calls = %d, want 3", got)
		}
		if got := len(h.tr.Dials()); got != 0 {
			t.Fatalf("connect seam calls = %d, want 0", got)
		}
		assertTimers(t, h.clock, map[string]int{})
	})

	t.Run("mixed mint and disconnect failures share the counter", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptError(unauthorized()) // failure 1
		h.minter.ScriptTicket(ticket(2))
		h.minter.ScriptError(unauthorized()) // failure 3
		h.start()

		h.fireTimer(timerBackoff)
		conn := h.liveConn()
		conn.Serve(frameDisconnect("unauthorized", false)) // failure 2
		h.fireTimer(timerBackoff)
		h.join() // third failure → terminal

		_, terminal, _ := h.snapshot()
		if terminal == nil || terminal.Reason != eventfeed.ReasonAuthorizationFailed {
			t.Fatalf("terminal = %v, want reason %q (one SHARED counter)", terminal, eventfeed.ReasonAuthorizationFailed)
		}
		if got := h.minter.Calls(); got != 3 {
			t.Fatalf("mint seam calls = %d, want 3", got)
		}
	})
}

// TestMintFailureClassification: transient/throttled ride Backoff
// (Retry-After floors the delay and wins outright past the cap);
// unrecoverable — and unclassified — mint errors are Terminal(mint_failed)
// with the generated error attached.
func TestMintFailureClassification(t *testing.T) {
	t.Run("throttled Retry-After floors the backoff", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptError(&eventfeed.MintError{Kind: eventfeed.MintThrottled, RetryAfter: 30 * time.Second})
		h.start()
		if d := h.fireTimer(timerBackoff); d != 30*time.Second {
			t.Fatalf("backoff delay = %s, want exactly the 30s Retry-After floor", d)
		}
	})

	t.Run("Retry-After beyond the cap wins outright", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptError(&eventfeed.MintError{Kind: eventfeed.MintThrottled, RetryAfter: 120 * time.Second})
		h.start()
		if d := h.fireTimer(timerBackoff); d != 120*time.Second {
			t.Fatalf("backoff delay = %s, want 120s (server-directed waits are cap-exempt)", d)
		}
	})

	t.Run("transient rides backoff to a fresh mint", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptError(&eventfeed.MintError{Kind: eventfeed.MintTransient, Err: errors.New("boom")})
		h.minter.ScriptTicket(ticket(2))
		h.start()
		h.fireTimer(timerBackoff)
		h.liveConn()
		if got := h.minter.Calls(); got != 2 {
			t.Fatalf("mint seam calls = %d, want 2", got)
		}
	})

	t.Run("unrecoverable is terminal with the cause attached", func(t *testing.T) {
		sentinel := errors.New("422 unprocessable")
		h := newHarness(t)
		h.minter.ScriptError(&eventfeed.MintError{Kind: eventfeed.MintUnrecoverable, Err: sentinel})
		h.start()
		h.join()
		_, terminal, _ := h.snapshot()
		if terminal == nil || terminal.Reason != eventfeed.ReasonMintFailed {
			t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonMintFailed)
		}
		if !errors.Is(terminal, sentinel) {
			t.Fatalf("terminal must wrap the generated error; got %v", terminal)
		}
		assertTimers(t, h.clock, map[string]int{})
	})

	t.Run("an unclassified mint error is terminal", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptError(errors.New("adapter bug: unclassified"))
		h.start()
		h.join()
		_, terminal, _ := h.snapshot()
		if terminal == nil || terminal.Reason != eventfeed.ReasonMintFailed {
			t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonMintFailed)
		}
	})
}

// TestInvalidCableURL: a mint URL failing cable-URL policy — and a
// policy-kind dial error — are Terminal(invalid_cable_url), never Backoff;
// a transient dial failure rides Backoff to a fresh mint (transition 7).
func TestInvalidCableURL(t *testing.T) {
	t.Run("non-wss scheme, zero dials", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptTicket(eventfeed.StreamTicket{Ticket: "t", ExpiresIn: 120, URL: "https://cable.example.com/cable"})
		h.start()
		h.join()
		_, terminal, _ := h.snapshot()
		if terminal == nil || terminal.Reason != eventfeed.ReasonInvalidCableURL {
			t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonInvalidCableURL)
		}
		if got := len(h.tr.Dials()); got != 0 {
			t.Fatalf("connect seam calls = %d, want 0 (policy pre-check refuses the dial)", got)
		}
		assertTimers(t, h.clock, map[string]int{})
	})

	t.Run("ws outside loopback, zero dials", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptTicket(eventfeed.StreamTicket{Ticket: "t", ExpiresIn: 120, URL: "ws://cable.example.com/cable"})
		h.start()
		h.join()
		if _, terminal, _ := h.snapshot(); terminal == nil || terminal.Reason != eventfeed.ReasonInvalidCableURL {
			t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonInvalidCableURL)
		}
	})

	t.Run("policy dial error is terminal", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptTicket(ticket(1))
		h.tr.FailNextDial(&eventfeed.DialError{Kind: eventfeed.DialPolicy, Reason: "redirect encountered"})
		h.start()
		h.join()
		if _, terminal, _ := h.snapshot(); terminal == nil || terminal.Reason != eventfeed.ReasonInvalidCableURL {
			t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonInvalidCableURL)
		}
		if got := len(h.tr.Dials()); got != 1 {
			t.Fatalf("connect seam calls = %d, want 1", got)
		}
		assertTimers(t, h.clock, map[string]int{})
	})

	t.Run("transient dial failure rides backoff", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptTicket(ticket(1))
		h.minter.ScriptTicket(ticket(2))
		h.tr.FailNextDial(errors.New("connection refused"))
		h.start()
		h.fireTimer(timerBackoff)
		h.liveConn()
		if got := h.minter.Calls(); got != 2 {
			t.Fatalf("mint seam calls = %d, want 2 (fresh ticket after a failed dial)", got)
		}
		urls := h.tr.DialedURLs()
		if len(urls) != 2 || urls[1] != ticket(2).URL {
			t.Fatalf("dialed URLs = %v, want the fresh mint's URL second", urls)
		}
	})
}

// TestInvalidFramesPreConfirm: the invalid-frame class (unparseable JSON, a
// correlated message failing Event decode) dispatches as a SOCKET FAILURE —
// Backoff, never terminal — while unknown types, pings, and
// foreign-identifier confirm/reject frames are ignored.
func TestInvalidFramesPreConfirm(t *testing.T) {
	t.Run("unparseable frame tears down to backoff", func(t *testing.T) {
		var mu sync.Mutex
		var disconnects []error
		obs := eventfeed.Observer{Disconnected: func(reason string, err error) {
			mu.Lock()
			disconnects = append(disconnects, err)
			mu.Unlock()
		}}
		h := newHarness(t, eventfeed.WithObserver(obs))
		h.minter.ScriptTicket(ticket(1))
		h.minter.ScriptTicket(ticket(2))
		h.start()

		conn := h.liveConn()
		conn.Serve([]byte("not json at all"))
		h.awaitTimer(timerBackoff)
		if !conn.Closed() {
			t.Fatal("an invalid frame must tear the socket down")
		}
		if _, terminal, _ := h.snapshot(); terminal != nil {
			t.Fatalf("an invalid frame is never terminal; got %v", terminal)
		}
		mu.Lock()
		if len(disconnects) != 1 || disconnects[0] == nil || !strings.Contains(disconnects[0].Error(), "invalid inbound frame") {
			mu.Unlock()
			t.Fatalf("Disconnected must carry the invalid-frame indication; got %v", disconnects)
		}
		mu.Unlock()
		h.fireTimer(timerBackoff)
		h.liveConn()
		if got := h.minter.Calls(); got != 2 {
			t.Fatalf("mint seam calls = %d, want 2 (the reconnect cycle recovers)", got)
		}
	})

	t.Run("correlated message failing event decode tears down", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptTicket(ticket(1))
		h.minter.ScriptTicket(ticket(2))
		h.start()

		conn := h.driveToSubscribed()
		bad, err := json.Marshal(map[string]any{"identifier": noFilterIdentifier, "message": map[string]any{"kind": "message"}})
		if err != nil {
			t.Fatal(err)
		}
		conn.Serve(bad)
		h.awaitTimer(timerBackoff)
		if _, terminal, _ := h.snapshot(); terminal != nil {
			t.Fatalf("a decode failure is never terminal; got %v", terminal)
		}
		if !conn.Closed() {
			t.Fatal("teardown must close the socket")
		}
	})

	t.Run("unknown types, pings, and foreign identifiers are ignored", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptTicket(ticket(1))
		h.start()

		conn := h.driveToSubscribed()
		conn.Serve([]byte(`{"type":"transmissions_ahoy"}`))
		conn.Serve(framePing())
		conn.Serve(frameConfirm(`{"channel":"SomeOtherChannel"}`))
		conn.Serve(frameReject(`{"channel":"SomeOtherChannel"}`))
		h.awaitFrameHandled("reject_subscription")
		select {
		case b := <-h.boundary:
			t.Fatalf("a foreign-identifier confirm must not confirm; boundary %v", b)
		default:
		}
		if _, terminal, _ := h.snapshot(); terminal != nil {
			t.Fatalf("a foreign-identifier reject must be ignored; got %v", terminal)
		}
		conn.Serve(frameConfirm(noFilterIdentifier))
		h.awaitBoundary()
	})
}

// TestNullFrameTearsDownToBackoff: a top-level `null` frame is the
// invalid-frame class's parse shape, not an unknown-type frame. Classified as
// unknown it would be liveness-only, so a peer sending nothing but `null`
// could hold the socket open forever — the pump re-arms staleness before the
// frame is parsed — while delivering no protocol traffic. The disposition is
// the class's: socket failure, Backoff, never terminal.
func TestNullFrameTearsDownToBackoff(t *testing.T) {
	var mu sync.Mutex
	var disconnects []error
	obs := eventfeed.Observer{Disconnected: func(_ string, err error) {
		mu.Lock()
		disconnects = append(disconnects, err)
		mu.Unlock()
	}}
	h := newHarness(t, eventfeed.WithObserver(obs))
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	h.start()

	conn := h.liveConn()
	conn.Serve([]byte(`null`))
	h.awaitTimer(timerBackoff)
	if !conn.Closed() {
		t.Fatal("a null frame must tear the socket down")
	}
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("an invalid frame is never terminal; got %v", terminal)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(disconnects) != 1 || disconnects[0] == nil || !eventfeed.ExportIsInvalidFrameError(disconnects[0]) {
		t.Fatalf("Disconnected must carry the invalid-frame indication; got %v", disconnects)
	}
}

// TestFrameDerivedObserverTextIsBounded: both frame-derived strings
// Disconnected can carry — a raw disconnect frame's reason, and the
// invalid-frame rendering of a decoder error that quotes frame bytes — are
// bounded by §9's MAX_ERROR_MESSAGE_LENGTH, which §23's Security Invariants
// apply to any rendering of frame contents. Unbounded, either is a 1 MiB
// attacker-chosen string in the consumer's logs.
func TestFrameDerivedObserverTextIsBounded(t *testing.T) {
	oversized := strings.Repeat("a", 4096)

	t.Run("disconnect reason", func(t *testing.T) {
		var mu sync.Mutex
		var reasons []string
		obs := eventfeed.Observer{Disconnected: func(reason string, _ error) {
			mu.Lock()
			reasons = append(reasons, reason)
			mu.Unlock()
		}}
		h := newHarness(t, eventfeed.WithObserver(obs))
		h.minter.ScriptTicket(ticket(1))
		h.minter.ScriptTicket(ticket(2))
		h.start()

		conn := h.liveConn()
		conn.Serve(frameDisconnect(oversized, true))
		h.awaitTimer(timerBackoff)
		if _, terminal, _ := h.snapshot(); terminal != nil {
			t.Fatalf("an unrecognized reason is a socket drop, never terminal; got %v", terminal)
		}
		mu.Lock()
		defer mu.Unlock()
		if len(reasons) != 1 {
			t.Fatalf("Disconnected calls = %d, want 1", len(reasons))
		}
		if got := len(reasons[0]); got > 500 {
			t.Fatalf("observed reason length = %d bytes, want at most 500", got)
		}
	})

	t.Run("event decode error", func(t *testing.T) {
		var mu sync.Mutex
		var disconnects []error
		obs := eventfeed.Observer{Disconnected: func(_ string, err error) {
			mu.Lock()
			disconnects = append(disconnects, err)
			mu.Unlock()
		}}
		h := newHarness(t, eventfeed.WithObserver(obs))
		h.minter.ScriptTicket(ticket(1))
		h.minter.ScriptTicket(ticket(2))
		h.start()

		conn := h.driveToSubscribed()
		// time.Time's decoder embeds the offending value in its parse error.
		bad, err := json.Marshal(map[string]any{
			"identifier": noFilterIdentifier,
			"message": map[string]any{
				"id": 1, "kind": "message", "event_type": "message.created",
				"action": "created", "created_at": oversized,
				"bucket_id": 2, "creator_id": 3, "recording_id": 4,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		conn.Serve(bad)
		h.awaitTimer(timerBackoff)
		mu.Lock()
		defer mu.Unlock()
		if len(disconnects) != 1 || disconnects[0] == nil {
			t.Fatalf("Disconnected must report the decode failure; got %v", disconnects)
		}
		if got := len(disconnects[0].Error()); got > 500 {
			t.Fatalf("rendered error length = %d bytes, want at most 500", got)
		}
		// Bounding only the outermost rendering is not a fix: the observer
		// holds the error, and errors.Unwrap walks straight back to the raw
		// time.Time parse error carrying the full frame-derived value. An
		// error carrying frame-derived text is FLAT (redactDialErr's
		// precedent) — the whole chain is checked, not just its head.
		for err := disconnects[0]; err != nil; err = errors.Unwrap(err) {
			if strings.Contains(err.Error(), oversized) {
				t.Fatalf("the error chain re-exposes the frame-supplied value at %T", err)
			}
			if got := len(err.Error()); got > 500 {
				t.Fatalf("chained error %T renders %d bytes, want at most 500", err, got)
			}
		}
		// The invalid-frame classification the tier-2 driver matches on must
		// survive the flattening.
		if !eventfeed.ExportIsInvalidFrameError(disconnects[0]) {
			t.Fatalf("Disconnected error %T lost its invalid-frame classification", disconnects[0])
		}
	})
}

// TestDuplicateWelcomeResendsSubscribe: subscribe is sent on each welcome,
// byte-identical (the server absorbs identical retransmits).
func TestDuplicateWelcomeResendsSubscribe(t *testing.T) {
	h := newHarness(t)
	h.minter.ScriptTicket(ticket(1))
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameWelcome())
	h.waitUntil("second subscribe write", func() bool { return len(conn.Writes()) == 2 })
	writes := conn.Writes()
	if string(writes[0]) != string(writes[1]) {
		t.Fatalf("resubscribe must be byte-identical: %q vs %q", writes[0], writes[1])
	}
	assertTimers(t, h.clock, map[string]int{timerConfirmationDeadline: 1, timerStaleness: 1})
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitBoundary()
}

// TestSubscribeWriteFailure: a failed subscribe write takes the socket-
// failure path — Backoff, never terminal.
func TestSubscribeWriteFailure(t *testing.T) {
	h := newHarness(t)
	h.minter.ScriptTicket(ticket(1))
	h.start()

	conn := h.liveConn()
	conn.FailWrites(errors.New("broken pipe"))
	conn.Serve(frameWelcome())
	h.awaitTimer(timerBackoff)
	if !conn.Closed() {
		t.Fatal("teardown must close the socket")
	}
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("a write failure is never terminal; got %v", terminal)
	}
}

// TestStaleness: the staleness timer (armed at socket open, reset pump-side
// per inbound frame of any kind) tears the socket down on expiry — and a
// frame genuinely re-arms it.
func TestStaleness(t *testing.T) {
	t.Run("expiry pre-welcome tears down", func(t *testing.T) {
		var mu sync.Mutex
		var stale []time.Duration
		obs := eventfeed.Observer{StaleConnection: func(since time.Duration) {
			mu.Lock()
			stale = append(stale, since)
			mu.Unlock()
		}}
		h := newHarness(t, eventfeed.WithObserver(obs))
		h.minter.ScriptTicket(ticket(1))
		h.start()

		conn := h.liveConn()
		h.clock.Advance(7500 * time.Millisecond)
		h.awaitTimer(timerBackoff)
		if !conn.Closed() {
			t.Fatal("staleness must tear the socket down")
		}
		assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
		mu.Lock()
		if len(stale) != 1 || stale[0] < 7500*time.Millisecond {
			mu.Unlock()
			t.Fatalf("StaleConnection = %v, want one callback with >= 7.5s of silence", stale)
		}
		mu.Unlock()
	})

	t.Run("frames reset the window", func(t *testing.T) {
		h := newHarness(t, eventfeed.WithConfirmationDeadline(time.Hour))
		h.minter.ScriptTicket(ticket(1))
		h.start()

		conn := h.driveToSubscribed()
		h.clock.Advance(5 * time.Second)
		conn.Serve(framePing())
		h.awaitFrameHandled("ping")
		h.clock.Advance(5 * time.Second)
		// 10s since socket open, but only 5s since the ping: still alive.
		assertTimers(t, h.clock, map[string]int{timerConfirmationDeadline: 1, timerStaleness: 1})
		h.clock.Advance(2500 * time.Millisecond)
		h.awaitTimer(timerBackoff)
		if !conn.Closed() {
			t.Fatal("the reset window's expiry must still tear down")
		}
	})

	// SPEC.md §23 "Cable Protocol Details": the reset happens pump-side, at
	// frame receipt, "so frame-vs-deadline ordering is well-defined at the
	// transport boundary regardless of queue depth or consumer latency: a
	// fired staleness deadline observed on return from a slow delivery is
	// AUTHORITATIVE". A frame received after the deadline fired is not one the
	// pump "received first", so it cannot supersede the firing — and the state
	// machine, parked inside the delivery, has not had a chance to observe it
	// yet. The pump is reading throughout here (the queue never fills), so the
	// suspension rule does not apply and the window testifies.
	t.Run("a frame arriving after the deadline does not supersede it", func(t *testing.T) {
		var mu sync.Mutex
		var stale []time.Duration
		obs := eventfeed.Observer{StaleConnection: func(since time.Duration) {
			mu.Lock()
			stale = append(stale, since)
			mu.Unlock()
		}}
		h := newHarness(t, eventfeed.WithObserver(obs))
		h.pauseAfter = 1
		h.minter.ScriptTicket(ticket(1))
		h.minter.ScriptTicket(ticket(2))
		h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
		h.start()

		conn := h.driveToSubscribed()
		conn.Serve(frameConfirm(noFilterIdentifier))
		h.awaitStreaming()

		// The consumer parks inside the delivery of the first live event, so
		// the deadline fires with the state machine unable to observe it.
		conn.Serve(frameMessage(noFilterIdentifier, 1))
		h.waitUntil("the consumer parked mid-delivery", func() bool { return len(h.deliveredIDs()) == 1 })
		h.clock.Advance(staleAfter)
		// The late frame lands after the firing, before control returns.
		h.serveSettled(conn, framePing())
		h.resume()

		h.awaitTimer(timerBackoff)
		if !conn.Closed() {
			t.Fatal("a frame received after the deadline fired must not erase the expiry")
		}
		assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
		if _, terminal, _ := h.snapshot(); terminal != nil {
			t.Fatalf("staleness is never terminal; got %v", terminal)
		}
		mu.Lock()
		defer mu.Unlock()
		if len(stale) != 1 || stale[0] < staleAfter {
			t.Fatalf("StaleConnection = %v, want one callback with >= %s of silence", stale, staleAfter)
		}
	})
}

// TestBufferOverflowPreConfirm: overflow of the live buffer dispatches the
// BufferOverflow semantic signal at drop time — default-terminal with no
// handler, terminal on Terminate, continuing on Accept with the invocation
// recorded (SPEC.md §23 "Semantic Signals").
func TestBufferOverflowPreConfirm(t *testing.T) {
	serveThree := func(h *harness) *feedtest.Conn {
		conn := h.driveToSubscribed()
		conn.Serve(frameMessage(noFilterIdentifier, 1))
		conn.Serve(frameMessage(noFilterIdentifier, 2))
		conn.Serve(frameMessage(noFilterIdentifier, 3))
		return conn
	}

	t.Run("no handler is terminal", func(t *testing.T) {
		var droppedCounts []int
		var mu sync.Mutex
		obs := eventfeed.Observer{BufferOverflow: func(n int) {
			mu.Lock()
			droppedCounts = append(droppedCounts, n)
			mu.Unlock()
		}}
		h := newHarness(t, eventfeed.WithLiveBufferCapacity(2), eventfeed.WithObserver(obs))
		h.minter.ScriptTicket(ticket(1))
		h.start()
		conn := serveThree(h)
		h.join()
		_, terminal, _ := h.snapshot()
		if terminal == nil || terminal.Reason != eventfeed.ReasonBufferOverflow {
			t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonBufferOverflow)
		}
		if !conn.Closed() {
			t.Fatal("teardown must close the socket")
		}
		mu.Lock()
		if len(droppedCounts) != 1 || droppedCounts[0] != 1 {
			mu.Unlock()
			t.Fatalf("Observer.BufferOverflow = %v, want one callback for 1 drop", droppedCounts)
		}
		mu.Unlock()
		assertTimers(t, h.clock, map[string]int{})
	})

	t.Run("handler Accept continues with the invocation recorded", func(t *testing.T) {
		var mu sync.Mutex
		var signals []eventfeed.Signal
		handler := func(s eventfeed.Signal) eventfeed.Disposition {
			mu.Lock()
			signals = append(signals, s)
			mu.Unlock()
			return eventfeed.Accept
		}
		h := newHarness(t, eventfeed.WithLiveBufferCapacity(2), eventfeed.WithSignalHandler(handler))
		h.minter.ScriptTicket(ticket(1))
		h.start()
		conn := serveThree(h)
		h.awaitFrameHandled("message")
		h.awaitFrameHandled("message")
		h.awaitFrameHandled("message")
		conn.Serve(frameConfirm(noFilterIdentifier))
		b := h.awaitBoundary()
		if len(b.Buffered) != 2 || b.Buffered[0].ID != 2 || b.Buffered[1].ID != 3 {
			t.Fatalf("buffered = %v, want the newest two events (oldest dropped)", b.Buffered)
		}
		mu.Lock()
		defer mu.Unlock()
		if len(signals) != 1 {
			t.Fatalf("handler invocations = %d, want exactly 1 per signal", len(signals))
		}
		ov, ok := signals[0].(eventfeed.BufferOverflow)
		if !ok || ov.DroppedCount != 1 || len(ov.DroppedIDs) != 1 || ov.DroppedIDs[0] != 1 {
			t.Fatalf("signal = %+v, want BufferOverflow{DroppedIDs:[1], DroppedCount:1}", signals[0])
		}
	})

	t.Run("handler Terminate is terminal", func(t *testing.T) {
		var invocations int
		var mu sync.Mutex
		handler := func(s eventfeed.Signal) eventfeed.Disposition {
			mu.Lock()
			invocations++
			mu.Unlock()
			return eventfeed.Terminate
		}
		h := newHarness(t, eventfeed.WithLiveBufferCapacity(2), eventfeed.WithSignalHandler(handler))
		h.minter.ScriptTicket(ticket(1))
		h.start()
		serveThree(h)
		h.join()
		_, terminal, _ := h.snapshot()
		if terminal == nil || terminal.Reason != eventfeed.ReasonBufferOverflow {
			t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonBufferOverflow)
		}
		mu.Lock()
		defer mu.Unlock()
		if invocations != 1 {
			t.Fatalf("handler invocations = %d, want 1 (Terminate is the handler's verdict, not a bypass)", invocations)
		}
	})
}

// TestStartModes: each entry mode maps to its SPEC-defined cursor and
// present-class flag at the catch-up hand-off.
func TestStartModes(t *testing.T) {
	cases := []struct {
		name    string
		start   eventfeed.Start
		cursor  eventfeed.Cursor
		present bool
	}{
		{"resume without a store is the bare present", eventfeed.StartResume(), eventfeed.Cursor{}, true},
		{"present", eventfeed.StartPresent(), eventfeed.Cursor{Since: "now"}, true},
		{"beginning", eventfeed.StartBeginning(), eventfeed.Cursor{Since: "0"}, false},
		{"after id", eventfeed.StartAfter(42), eventfeed.Cursor{Since: "42"}, false},
		{"at position", eventfeed.StartAtPosition("pos-1"), eventfeed.Cursor{Position: "pos-1"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, eventfeed.WithStart(tc.start))
			h.minter.ScriptTicket(ticket(1))
			h.start()
			conn := h.driveToSubscribed()
			conn.Serve(frameConfirm(noFilterIdentifier))
			b := h.awaitBoundary()
			if b.Entry != tc.cursor || b.PresentClass != tc.present {
				t.Fatalf("boundary = {%+v present=%v}, want {%+v present=%v}", b.Entry, b.PresentClass, tc.cursor, tc.present)
			}
		})
	}
}

// TestFilteredSubscribeIdentifier: the subscribe command carries the ordered,
// comma-joined filter params, byte-exact, and confirmation correlates on
// that identifier.
func TestFilteredSubscribeIdentifier(t *testing.T) {
	filters := eventfeed.Filters{Types: []string{"message.created", "todo.completed"}, Buckets: []int64{2, 1}}
	h := newHarness(t, eventfeed.WithFilters(filters))
	h.minter.ScriptTicket(ticket(1))
	h.start()

	conn := h.driveToSubscribed()
	want := string(eventfeed.ExportSubscribeFrame(filters))
	if !strings.Contains(want, `message.created,todo.completed`) || !strings.Contains(want, `2,1`) {
		t.Fatalf("test premise: subscribe frame %q should join filters in configured order", want)
	}
	writes := conn.Writes()
	if len(writes) != 1 || string(writes[0]) != want {
		t.Fatalf("subscribe = %q, want %q", writes, want)
	}
	conn.Serve(frameConfirm(eventfeed.ExportSubscribeIdentifier(filters)))
	h.awaitBoundary()
}

// TestTeardownCancelsInFlight: Close (and caller cancellation) during an
// in-flight mint, a stalled dial, and an awaited confirmation each end
// iteration cleanly — no error element, no surviving timers, no leaked
// goroutines.
func TestTeardownCancelsInFlight(t *testing.T) {
	t.Run("mid-mint", func(t *testing.T) {
		h := newHarness(t)
		h.minter.StallNext()
		base := runtime.NumGoroutine()
		h.start()
		h.waitUntil("mint in flight", func() bool { return h.minter.Calls() == 1 })
		h.conn.Close()
		h.join()
		if _, terminal, elements := h.snapshot(); terminal != nil || elements != 0 {
			t.Fatalf("clean close: got %d elements, terminal %v", elements, terminal)
		}
		if got := len(h.tr.Dials()); got != 0 {
			t.Fatalf("connect seam calls = %d, want 0", got)
		}
		assertTimers(t, h.clock, map[string]int{})
		assertGoroutinesSettle(t, base)
	})

	t.Run("mid-dial", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptTicket(ticket(1))
		h.tr.StallNextDial()
		base := runtime.NumGoroutine()
		h.start()
		h.waitUntil("dial in flight", func() bool { return len(h.tr.Dials()) == 1 })
		h.conn.Close()
		h.join()
		if _, terminal, elements := h.snapshot(); terminal != nil || elements != 0 {
			t.Fatalf("clean close: got %d elements, terminal %v", elements, terminal)
		}
		assertTimers(t, h.clock, map[string]int{})
		assertGoroutinesSettle(t, base)
	})

	t.Run("mid-await-confirmation", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptTicket(ticket(1))
		base := runtime.NumGoroutine()
		h.start()
		conn := h.driveToSubscribed()
		h.conn.Close()
		h.join()
		if !conn.Closed() {
			t.Fatal("Close must close the live socket")
		}
		if _, terminal, elements := h.snapshot(); terminal != nil || elements != 0 {
			t.Fatalf("clean close: got %d elements, terminal %v", elements, terminal)
		}
		assertTimers(t, h.clock, map[string]int{})
		assertGoroutinesSettle(t, base)
	})

	t.Run("caller cancellation mid-await", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptTicket(ticket(1))
		base := runtime.NumGoroutine()
		h.start()
		conn := h.driveToSubscribed()
		h.cancel()
		h.join()
		if !conn.Closed() {
			t.Fatal("cancellation must close the live socket")
		}
		if _, terminal, elements := h.snapshot(); terminal != nil || elements != 0 {
			t.Fatalf("clean cancel: got %d elements, terminal %v", elements, terminal)
		}
		assertTimers(t, h.clock, map[string]int{})
		assertGoroutinesSettle(t, base)
	})
}

// TestStartModesWithAPopulatedStore: only StartResume is defined as "the
// stored position if any" (SPEC.md §23 "Consumer Ergonomics"). StartPresent,
// StartBeginning and StartAfter promise since=now, since=0 and since=<id>,
// and a caller pairing one with a configured store means it — otherwise every
// explicit mode silently becomes resume the moment the store has anything in
// it, and a checkpointed feed can never be deliberately replayed or reset.
// The load still runs under those modes (its failure edge and its lineage
// identity are not mode-dependent); the value simply is not what the entry is
// taken from.
func TestStartModesWithAPopulatedStore(t *testing.T) {
	cases := []struct {
		name    string
		start   eventfeed.Start
		cursor  eventfeed.Cursor
		present bool
	}{
		{"resume takes the stored position", eventfeed.StartResume(), eventfeed.Cursor{Position: "pos-0"}, false},
		{"present ignores it", eventfeed.StartPresent(), eventfeed.Cursor{Since: "now"}, true},
		{"beginning ignores it", eventfeed.StartBeginning(), eventfeed.Cursor{Since: "0"}, false},
		{"after id ignores it", eventfeed.StartAfter(42), eventfeed.Cursor{Since: "42"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := feedtest.NewStore()
			store.Stored("pos-0")
			h := storedHarness(t, store, eventfeed.WithStart(tc.start))
			h.minter.ScriptTicket(ticket(1))
			h.start()
			conn := h.driveToSubscribed()
			conn.Serve(frameConfirm(noFilterIdentifier))
			b := h.awaitBoundary()
			if b.Entry != tc.cursor || b.PresentClass != tc.present {
				t.Fatalf("boundary = {%+v present=%v}, want {%+v present=%v}", b.Entry, b.PresentClass, tc.cursor, tc.present)
			}
			if got := len(store.Loads()); got != 1 {
				t.Fatalf("checkpoint loads = %d, want exactly 1 — the load runs under every mode", got)
			}
		})
	}

	// The other half of the rule: a position ACCEPTED during this run is
	// authoritative within the run, so a reconnect under an explicit mode
	// resumes where the feed actually got to rather than re-entering at the
	// mode's original cursor.
	t.Run("a position accepted during the run wins on reconnect", func(t *testing.T) {
		store := feedtest.NewStore()
		store.Stored("pos-0")
		h := storedHarness(t, store, eventfeed.WithStart(eventfeed.StartPresent()))
		h.minter.ScriptTicket(ticket(1))
		h.minter.ScriptTicket(ticket(2))
		h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
		h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-2"})
		h.start()

		conn := h.driveToSubscribed()
		conn.Serve(frameConfirm(noFilterIdentifier))
		h.awaitStreaming()
		conn.FailReads(errors.New("connection reset"))
		h.awaitTimer(timerBackoff)
		h.fireTimer(timerBackoff)
		conn2 := h.driveToSubscribed()
		conn2.Serve(frameConfirm(noFilterIdentifier))
		h.awaitStreaming()

		calls := h.polls.Calls()
		if len(calls) != 2 {
			t.Fatalf("poll seam calls = %d, want 2", len(calls))
		}
		if calls[0].Cursor != (eventfeed.Cursor{Since: "now"}) {
			t.Fatalf("entry cursor = %+v, want the configured present entry", calls[0].Cursor)
		}
		if calls[1].Cursor != (eventfeed.Cursor{Position: "pos-1"}) {
			t.Fatalf("reconnect cursor = %+v, want the position accepted during this run", calls[1].Cursor)
		}
	})
}

// TestRejectBeforeSubscribeIsIgnored gates transition 12 on the state §23
// draws it from. `reject_subscription` is the connector's single most severe
// verdict — Terminal, ZERO reconnects, no backoff — and ungated it can be
// produced by one unsolicited frame from a peer that has done nothing but
// complete the WebSocket handshake, against zero subscription attempts. The
// neighbouring confirm branch already gates on AwaitingConfirmation; this is
// the same gate on the same transition's other half.
func TestRejectBeforeSubscribeIsIgnored(t *testing.T) {
	h := newHarness(t)
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()

	conn := h.liveConn()
	// The connector's OWN identifier, so correlation is not what saves it,
	// and before any subscribe command has been written.
	conn.Serve(frameReject(noFilterIdentifier))
	h.awaitFrameHandled("reject_subscription")
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("terminal = %v, want none — transition 12 exists only from AwaitingConfirmation", terminal)
	}
	if conn.Closed() {
		t.Fatal("a premature rejection must not tear the socket down")
	}

	// The handshake then proceeds normally on the same socket.
	conn.Serve(frameWelcome())
	h.awaitTimer(timerConfirmationDeadline)
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("terminal = %v, want none", terminal)
	}
}

// TestStalledSubscribeWriteStillLapsesTheHandshakeDeadline bounds the one
// place the state machine hands control to the transport synchronously while
// a deadline governs the phase. A CableConn.WriteFrame that blocks — a peer
// whose receive window has shut, or a seam implementation that returns only
// on cancellation — cannot be interrupted from inside the write: at.ctx is
// cancelled by a teardown the write is itself blocking, so neither the
// handshake nor the confirmation deadline can ever be observed and the feed
// hangs after `welcome` until the consumer closes the connector.
func TestStalledSubscribeWriteStillLapsesTheHandshakeDeadline(t *testing.T) {
	h := newHarness(t)
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	h.start()

	conn := h.liveConn()
	conn.StallWrites()
	conn.Serve(frameWelcome())
	// The frame-handled hook fires before the switch, so past it the only
	// path is the subscribe write: firing the deadline here cannot be taken
	// by the outer select instead.
	h.awaitFrameHandled("welcome")
	h.fireTimer(timerHandshakeDeadline)

	h.awaitTimer(timerBackoff)
	if !conn.Closed() {
		t.Fatal("the lapse must dispose the attempt, closing the socket")
	}
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("a handshake lapse is never terminal; got %v", terminal)
	}
	assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
}

// TestWelcomeRacingAnExpiredHandshakeDeadlineLapses reads Stop's result as
// the ordering discriminator, the same way staleHolder.arm does. When the
// welcome frame and the handshake expiry are both ready the select picks one
// at random; if the frame wins, an unchecked Stop swallows the firing and
// replaces the expired handshake timer with a fresh confirmation window — so
// a welcome that arrived past the deadline is accepted instead of taking
// transition 9, half the time it happens.
//
// The hook fires in exactly that window (write returned, timer not yet
// stopped), which is the only way to make the race deterministic from
// outside: serving the frame and firing the timer from the test leaves the
// select genuinely free to pick either.
func TestWelcomeRacingAnExpiredHandshakeDeadlineLapses(t *testing.T) {
	h := newHarness(t)
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	fired := make(chan struct{}, 1)
	h.conn.OnSubscribeWritten(func() {
		if _, ok := h.clock.FireTimer(timerHandshakeDeadline); ok {
			select {
			case fired <- struct{}{}:
			default:
			}
		}
	})
	h.start()

	conn := h.liveConn()
	conn.Serve(frameWelcome())
	select {
	case <-fired:
	case <-time.After(watchdog):
		t.Fatal("the handshake deadline was never fired inside the write→Stop window")
	}

	h.awaitTimer(timerBackoff)
	if !conn.Closed() {
		t.Fatal("a lapsed handshake must dispose the attempt, closing the socket")
	}
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("a handshake lapse is never terminal; got %v", terminal)
	}
	// The decisive assertion: no confirmation-deadline was armed over the
	// expired handshake timer.
	assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
}

// TestConfirmRacingAnExpiredConfirmationDeadlineLapses is the confirmation
// half of the Stop-as-ordering-discriminator rule, and the more consequential
// half. Where an unchecked Stop on the handshake deadline merely swallowed the
// firing, here the connector would go on to announce Confirmed and enter
// catch-up on a subscription whose confirmation arrived past transition 14's
// deadline — the lapse not just swallowed but overwritten by a successful
// handoff.
//
// The frame-handled hook fires before the switch, so firing the deadline from
// it lands in the window between the frame being classified and the Stop that
// cancels it: the same window the select leaves open when both are ready, made
// deterministic.
func TestConfirmRacingAnExpiredConfirmationDeadlineLapses(t *testing.T) {
	h := newHarness(t)
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	fired := make(chan struct{}, 1)
	h.conn.OnFrameHandled(func(kind string) {
		if kind != "confirm_subscription" {
			return
		}
		if _, ok := h.clock.FireTimer(timerConfirmationDeadline); ok {
			select {
			case fired <- struct{}{}:
			default:
			}
		}
	})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	select {
	case <-fired:
	case <-time.After(watchdog):
		t.Fatal("the confirmation deadline was never fired inside the classify→Stop window")
	}

	h.awaitTimer(timerBackoff)
	if !conn.Closed() {
		t.Fatal("a lapsed confirmation must dispose the attempt, closing the socket")
	}
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("a confirmation lapse is never terminal; got %v", terminal)
	}
	// The decisive assertion: catch-up was never entered, so neither the
	// staleness nor the repair-poll timer survives.
	assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
	if got := h.polls.CallCount(); got != 0 {
		t.Fatalf("poll seam calls = %d, want 0 — the confirmation lapsed", got)
	}
}

// TestStalenessArmsBeforeConnectedObserver is #763. The staleness window must
// begin when the socket opens, not when a host's Connected callback returns.
//
// Observed from INSIDE the callback, which is the only place the ordering is
// visible: newStaleHolder reads the window's origin at construction, so if the
// callback runs first, everything it spends is time the window never counts,
// and a peer that went silent immediately gets that much extra grace before
// staleness can fire. Asserting on the timer set afterwards would pass either
// way — the set is the same, only its origin moves.
func TestStalenessArmsBeforeConnectedObserver(t *testing.T) {
	var armedAtConnected []string
	var connectedCalls int
	// Declared ahead of the option so the callback can read the harness's own
	// clock: the closure captures the variable, and by the time it fires the
	// harness is assigned.
	var h *harness
	h = newHarness(t, eventfeed.WithObserver(eventfeed.Observer{
		Connected: func() {
			connectedCalls++
			armedAtConnected = h.clock.Outstanding()
		},
	}))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	if connectedCalls != 1 {
		t.Fatalf("Connected fired %d time(s), want 1", connectedCalls)
	}
	if !slices.Contains(armedAtConnected, timerStaleness) {
		t.Errorf("timers armed when Connected fired = %v, want the %q window already among them",
			armedAtConnected, timerStaleness)
	}
}

// TestDisconnectObserverNeverEchoesPeerText is the canary for the credential
// boundary's logging half.
//
// Observer.Disconnected is a logging surface, and both of its arguments can be
// fed by the peer: a raw disconnect frame's reason, and a WebSocket close
// reason rendered through the error. §23 declares the ticket an "opaque bearer
// credential; never logged", and the cable server is exactly who knows it — it
// was dialed with it. A server that echoes the URL it was dialed with, whether
// maliciously or by logging its own request line into an error, puts the ticket
// in the host's logs.
//
// Truncation was the old defense and does not work: it bounds the length of a
// leak, not whether there is one. This asserts the property instead of the
// mechanism, so it stays true whatever the rendering becomes — the canary is
// planted in every peer-controlled string the teardown path can carry.
// TestObservableSocketErrorDropsWrapperText pins the half of the closed
// vocabulary that errors.Is cannot enforce on its own.
//
// TestDisconnectObserverNeverEchoesPeerText covers the shapes a peer controls
// directly. This covers the one a conforming-looking SEAM controls: a transport
// that annotates its read failure with the URL it was reading —
// fmt.Errorf("read %s: %w", cableURL, context.Canceled) — produces an error that
// MATCHES a recognized sentinel while carrying a ticket in its text. Returning
// the argument on a match would forward that wrapper to Observer.Disconnected,
// so every arm must return the connector's own value instead.
//
// The typed arms are included to prove the same property holds through the
// other mechanism (errors.As extracting the inner value), because a future
// refactor that unified the two halves onto errors.Is would silently reopen
// exactly this hole.
func TestObservableSocketErrorDropsWrapperText(t *testing.T) {
	const canary = "sekrit-ticket-value"
	cableURL := "wss://28.cable.basecamp.com/cable?ticket=" + canary

	for _, tc := range []struct {
		name string
		err  error
		want error
	}{
		{"wrapped context.Canceled", fmt.Errorf("read %s: %w", cableURL, context.Canceled), context.Canceled},
		{
			"wrapped context.DeadlineExceeded",
			fmt.Errorf("read %s: %w", cableURL, context.DeadlineExceeded),
			context.DeadlineExceeded,
		},
		{
			"wrapped staleness sentinel",
			fmt.Errorf("read %s: %w", cableURL, eventfeed.ExportStaleConnectionErr()),
			eventfeed.ExportStaleConnectionErr(),
		},
		{
			"wrapped conn-closed sentinel",
			fmt.Errorf("read %s: %w", cableURL, eventfeed.ExportCableConnClosedErr()),
			eventfeed.ExportCableConnClosedErr(),
		},
		{
			"wrapped CloseError",
			fmt.Errorf("read %s: %w", cableURL, &eventfeed.CloseError{Code: 1011, Reason: cableURL}),
			nil, // typed arm: identity is not pinned, only the absence of the canary
		},
		// The two types a SEAM can author. Both are exported with exported
		// fields and both render free text — DialError its Reason plus
		// Err.Error(), TerminalError its Msg plus Err.Error() — so recognizing
		// them by type recognized nothing about who wrote them. A
		// CableConn.ReadFrame returning either put its text on this callback
		// verbatim. They must now reduce to the generic sentinel.
		{
			"seam-constructed DialError",
			&eventfeed.DialError{Kind: eventfeed.DialPolicy, Reason: cableURL},
			eventfeed.ExportSocketFailedErr(),
		},
		{
			"seam-constructed DialError with a cause",
			&eventfeed.DialError{Kind: eventfeed.DialTransient, Err: errors.New("dial " + cableURL)},
			eventfeed.ExportSocketFailedErr(),
		},
		{
			"seam-constructed TerminalError",
			&eventfeed.TerminalError{Reason: eventfeed.ReasonPollFailed, Msg: cableURL},
			eventfeed.ExportSocketFailedErr(),
		},
		{
			"seam-constructed TerminalError with a cause",
			&eventfeed.TerminalError{Reason: eventfeed.ReasonMintFailed, Err: errors.New("mint " + cableURL)},
			eventfeed.ExportSocketFailedErr(),
		},
		{
			"seam error wrapping a TerminalError",
			fmt.Errorf("read: %w", &eventfeed.TerminalError{Reason: eventfeed.ReasonPollFailed, Msg: cableURL}),
			eventfeed.ExportSocketFailedErr(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := eventfeed.ExportObservableSocketError(tc.err)
			if got == nil {
				t.Fatal("observableSocketError returned nil for a non-nil cause")
			}
			// The whole chain, not just the top: Error() walks it, and so does
			// any observer that logs %+v.
			for e := got; e != nil; e = errors.Unwrap(e) {
				if strings.Contains(e.Error(), canary) {
					t.Errorf("reduced error echoed the ticket: %q", e.Error())
				}
				if strings.Contains(e.Error(), "ticket=") {
					t.Errorf("reduced error echoed a ticket-bearing query: %q", e.Error())
				}
			}
			// Recognition must survive the reduction, or a consumer that
			// branches on the sentinel silently stops matching.
			if tc.want != nil {
				if got != tc.want { //nolint:errorlint // identity is the assertion
					t.Errorf("reduced to %#v, want the canonical sentinel %#v", got, tc.want)
				}
				if !errors.Is(got, tc.want) {
					t.Errorf("errors.Is no longer matches after reduction: %#v", got)
				}
			}
		})
	}
}

func TestDisconnectObserverNeverEchoesPeerText(t *testing.T) {
	const canary = "sekrit-ticket-value"
	peerText := "wss://28.cable.basecamp.com/cable?ticket=" + canary

	for _, tc := range []struct {
		name string
		//nolint:revive // the fixture drives one teardown shape per case
		drive func(t *testing.T, h *harness, sock *feedtest.Conn)
		// beforeWelcome runs drive against a dialed-but-pre-welcome socket,
		// for the one failure that has to be armed before the subscribe write.
		beforeWelcome bool
		// wantErr, when set, is the exact error Disconnected must carry.
		wantErr error
	}{
		{name: "raw disconnect frame reason", drive: func(_ *testing.T, _ *harness, sock *feedtest.Conn) {
			sock.Serve(frameDisconnect(peerText, false))
		}},
		{name: "peer close reason", drive: func(_ *testing.T, _ *harness, sock *feedtest.Conn) {
			sock.FailReads(&eventfeed.CloseError{Code: 1011, Reason: peerText})
		}},
		{
			name: "raw read error",
			drive: func(_ *testing.T, _ *harness, sock *feedtest.Conn) {
				sock.FailReads(errors.New("read tcp: " + peerText))
			},
			wantErr: eventfeed.ExportSocketFailedErr(),
		},
		// The event-decode shape, which is the ONLY frame path where a decoder
		// quotes peer bytes back (time.Time's UnmarshalJSON does, verbatim).
		// Reaching it takes three things the predecessor of this case got
		// wrong, each of which alone made the canary unobservable: NO "type"
		// key, because a correlated broadcast is the typeless shape and
		// `{"type":"message"}` is merely an unrecognized type; the connector's
		// OWN identifier, because a frame that fails correlation is dropped
		// before decode; and WELL-FORMED JSON, because truncated bytes fail
		// the envelope unmarshal first and never reach the payload. The canary
		// also has to sit in a field something renders — it sat in
		// `identifier`, which nothing does — so it goes in created_at.
		{name: "invalid frame payload", drive: func(_ *testing.T, _ *harness, sock *feedtest.Conn) {
			sock.Serve(frameMessageCreatedAt(noFilterIdentifier, 107, peerText))
		}},
		// A seam authoring one of the two exported error types. Both render
		// free text and neither is the connector's to write, so both must
		// reduce to the generic cause before they reach the callback.
		{
			name: "seam-authored TerminalError from ReadFrame",
			drive: func(_ *testing.T, _ *harness, sock *feedtest.Conn) {
				sock.FailReads(&eventfeed.TerminalError{Reason: eventfeed.ReasonPollFailed, Msg: peerText})
			},
			wantErr: eventfeed.ExportSocketFailedErr(),
		},
		{
			name: "seam-authored DialError from ReadFrame",
			drive: func(_ *testing.T, _ *harness, sock *feedtest.Conn) {
				sock.FailReads(&eventfeed.DialError{Kind: eventfeed.DialPolicy, Reason: peerText})
			},
			wantErr: eventfeed.ExportSocketFailedErr(),
		},
		// The subscribe write is the one WriteFrame the connector makes on this
		// path, and it happens ON welcome — so this case has to arm the failure
		// BEFORE welcome is served, which is what beforeWelcome buys.
		{
			name: "seam-authored TerminalError from WriteFrame",
			drive: func(_ *testing.T, _ *harness, sock *feedtest.Conn) {
				sock.FailWrites(&eventfeed.TerminalError{Reason: eventfeed.ReasonMintFailed, Msg: peerText})
			},
			beforeWelcome: true,
			wantErr:       eventfeed.ExportSocketFailedErr(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var seen []string
			var sawErrs []error
			h := newHarness(t, eventfeed.WithObserver(eventfeed.Observer{
				Disconnected: func(reason string, err error) {
					mu.Lock()
					defer mu.Unlock()
					seen = append(seen, reason)
					sawErrs = append(sawErrs, err)
					if err != nil {
						// The whole chain, not just the top: Error() walks it.
						seen = append(seen, err.Error())
						for e := err; e != nil; e = errors.Unwrap(e) {
							seen = append(seen, e.Error())
						}
					}
				},
			}))
			h.minter.ScriptTicket(ticket(1))
			h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
			h.start()
			var sock *feedtest.Conn
			if tc.beforeWelcome {
				sock = h.liveConn()
				tc.drive(t, h, sock)
				sock.Serve(frameWelcome())
			} else {
				sock = h.driveToSubscribed()
				tc.drive(t, h, sock)
			}
			h.awaitTimer(timerBackoff)

			mu.Lock()
			defer mu.Unlock()
			if len(seen) == 0 {
				t.Fatal("Observer.Disconnected never fired; the canary proves nothing")
			}
			for _, got := range seen {
				if strings.Contains(got, canary) {
					t.Errorf("Observer.Disconnected echoed the peer's ticket: %q", got)
				}
				if strings.Contains(got, "ticket=") {
					t.Errorf("Observer.Disconnected echoed a ticket-bearing query: %q", got)
				}
			}
			// Absence of the canary is satisfied by a callback that fires with
			// a nil err, which is not the guarantee. Where the reduction has a
			// determinate answer, pin it by identity.
			if tc.wantErr != nil {
				found := false
				for _, err := range sawErrs {
					if err == tc.wantErr { //nolint:errorlint // identity is the assertion
						found = true
					}
				}
				if !found {
					t.Errorf("Disconnected never carried the reduced cause %#v; saw %#v", tc.wantErr, sawErrs)
				}
			}
		})
	}
}

// TestSeamAuthoredDialPolicyDoesNotReachTheTerminal is the terminal-channel
// half of the ticket-secrecy rule, and the half an earlier pass exempted.
//
// The reasoning that exempted it was "terminal elements keep their typed
// errors and their SPEC-mandated text", which is true of the ONE case §23
// actually mandates — filter_invalid preserves the server's message — and
// false as a blanket. A DialPolicy verdict is not SPEC-mandated text: it is
// authored by whatever CableTransport the host installed, and CableTransport
// is a documented extension point. Copying its Reason into TerminalError.Msg
// and the error itself into the chain hands that text to every consumer,
// through exactly the channel that was hardened for Observer.Disconnected.
//
// The connector's OWN pre-check keeps its text and is not affected: its two
// interpolations are a URL scheme and a port, both of which url.Parse
// constrains to alphanumerics and digits, so neither can carry a credential.
func TestSeamAuthoredDialPolicyDoesNotReachTheTerminal(t *testing.T) {
	const canary = "sekrit-ticket-value"
	cableURL := "wss://28.cable.basecamp.com/cable?ticket=" + canary

	for _, tc := range []struct {
		name string
		err  error
	}{
		{"reason", &eventfeed.DialError{Kind: eventfeed.DialPolicy, Reason: cableURL}},
		{"cause", &eventfeed.DialError{Kind: eventfeed.DialPolicy, Err: errors.New("dial " + cableURL)}},
		{"wrapped", fmt.Errorf("transport: %w", &eventfeed.DialError{Kind: eventfeed.DialPolicy, Reason: cableURL})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.minter.ScriptTicket(ticket(1))
			h.tr.FailNextDial(tc.err)
			h.start()
			h.join()

			_, terminal, _ := h.snapshot()
			if terminal == nil || terminal.Reason != eventfeed.ReasonInvalidCableURL {
				t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonInvalidCableURL)
			}
			// The whole chain, not just the top: Error() walks it, and so does
			// any consumer that logs %+v or unwraps.
			for err := error(terminal); err != nil; err = errors.Unwrap(err) {
				if strings.Contains(err.Error(), canary) {
					t.Errorf("%T in the terminal chain echoed the ticket: %q", err, err.Error())
				}
				if strings.Contains(err.Error(), "ticket=") {
					t.Errorf("%T in the terminal chain echoed a ticket-bearing query: %q", err, err.Error())
				}
			}
			// A seam-authored DialError must not be recoverable from the
			// chain at all: retaining it would hand back through errors.As
			// exactly what the rendering withheld.
			var de *eventfeed.DialError
			if errors.As(error(terminal), &de) {
				t.Errorf("the terminal retains the seam's *DialError (%+v); the chain is the same leak as the rendering", de)
			}
		})
	}
}

// TestObservableCloseErrorCarriesNoPeerReason: CloseError.Error renders only
// the integer code, which is why the reduction let the value through — but
// Reason is an EXPORTED field holding peer text, and it survived intact. A
// host that logs the error's fields, formats it with %+v, or reads ce.Reason
// gets the peer's string on the surface the closed vocabulary exists to
// protect. Rendering is not the only way out of a struct.
func TestObservableCloseErrorCarriesNoPeerReason(t *testing.T) {
	const canary = "sekrit-ticket-value"
	peerText := "wss://28.cable.basecamp.com/cable?ticket=" + canary

	got := eventfeed.ExportObservableSocketError(&eventfeed.CloseError{Code: 1011, Reason: peerText})
	var ce *eventfeed.CloseError
	if !errors.As(got, &ce) {
		t.Fatalf("reduced to %T, want a *CloseError — the code is diagnostic and must survive", got)
	}
	if ce.Reason != "" {
		t.Errorf("the reduced CloseError still carries the peer's reason %q", ce.Reason)
	}
	if ce.Code != 1011 {
		t.Errorf("the reduced CloseError lost its code: %d", ce.Code)
	}
	if fmt.Sprintf("%+v", ce) != fmt.Sprintf("%+v", &eventfeed.CloseError{Code: 1011}) {
		t.Errorf("field-wise rendering still differs from a code-only close error: %+v", ce)
	}
}

// nilDialTransport returns (nil, nil) from Dial exactly once — a seam
// violating its own contract — and then delegates. Nothing in feedtest can
// express this, deliberately: it is not a shape a compliant transport
// produces, and the connector's handling of it is a defensive edge rather
// than a scripted one.
type nilDialTransport struct {
	inner eventfeed.CableTransport
	once  sync.Once
}

func (t *nilDialTransport) Dial(ctx context.Context, url string, maxFrameBytes int64) (eventfeed.CableConn, error) {
	first := false
	t.once.Do(func() { first = true })
	if first {
		return nil, nil
	}
	return t.inner.Dial(ctx, url, maxFrameBytes)
}

// TestNilDialResultLeavesBackoffsTimerSetExact: a (nil, nil) dial takes the
// transient failure edge, and must arrive in Backoff with the SAME exact timer
// set every other transient failure produces.
//
// The handshake deadline is armed before the dial and every other exit from
// this select stops it. This branch did not, so the deadline rode into Backoff
// — whose exact set §23 pins at {backoff} — and a transport returning (nil,
// nil) repeatedly accumulated one ghost handshake-deadline per cycle, each
// firing later into a state with no edge for it.
//
// The assertion is the exact SET rather than "no panic", because the panic
// this branch prevents was never the interesting part: a defensive edge that
// leaves a timer behind has traded a crash for a leak.
func TestNilDialResultLeavesBackoffsTimerSetExact(t *testing.T) {
	h := newHarness(t)
	h.tr = feedtest.NewTransport()
	tr := &nilDialTransport{inner: h.tr}
	h2 := newHarness(t, eventfeed.WithTransport(tr))
	h2.minter.ScriptTicket(ticket(1))
	h2.minter.ScriptTicket(ticket(2))
	h2.start()

	// The dial returns (nil, nil), the cycle fails, and Backoff is entered.
	h2.awaitTimer(timerBackoff)
	assertTimers(t, h2.clock, map[string]int{timerBackoff: 1})
}
