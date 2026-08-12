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
	ctx, cancel := context.WithCancel(context.Background())
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
	payload := map[string]any{
		"id":                 id,
		"kind":               "message",
		"event_type":         "message.created",
		"action":             "created",
		"created_at":         "2026-08-01T12:00:00Z",
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
