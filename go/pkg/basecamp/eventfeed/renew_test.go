// Tier-3 tests for the in-band renewal (SPEC.md §23 "In-Band Renewal"): what
// the scenario fixtures (36–41) cannot express — the jitter formula under a
// deterministic rand, each failure kind and its observer record, the write's
// two socket-failure edges, a reply that beats its own write's completion,
// the repair cadence held across an attempt, Close from inside every renewal
// callback, the floor after a renewal, and the credential never reaching a
// logging surface.
package eventfeed_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed/feedtest"
)

// Renewal timer kinds (SPEC.md §23, kebab-case, verbatim).
const (
	timerRenewDue         = "renew-due"
	timerRenewAckDeadline = "renew-ack-deadline"
)

// welcomeAdvertising is a welcome offering in-band renewal against deadline.
func welcomeAdvertising(deadline time.Time) []byte {
	return mustJSON(map[string]any{"type": "welcome", "stream_expires_at": deadline.UTC().Format(time.RFC3339Nano)})
}

// frameRenewed is the server's acknowledgement, on the subscription's own
// identifier as bc3 sends it.
func frameRenewed(deadline time.Time) []byte {
	return mustJSON(map[string]any{"identifier": noFilterIdentifier, "type": "renewed", "stream_expires_at": deadline.UTC().Format(time.RFC3339Nano)})
}

func frameRenewRefused() []byte {
	return mustJSON(map[string]any{"identifier": noFilterIdentifier, "type": "renew_refused", "reason": "not_later"})
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// renewLog records the renewal's three observer callbacks, and every other
// callback's arguments as text, in order.
type renewLog struct {
	mu      sync.Mutex
	entries []string
	// texts is everything any callback was handed, rendered — what a host
	// logging the observer would print.
	texts   []string
	changed chan struct{}
	// onEntry, when set, runs on the consumer's goroutine after an entry is
	// recorded (the Close-from-a-callback cases).
	onEntry func(string)
}

func newRenewLog() *renewLog { return &renewLog{changed: make(chan struct{}, 64)} }

func (r *renewLog) record(entry string, args ...any) {
	r.mu.Lock()
	r.entries = append(r.entries, entry)
	for _, a := range args {
		r.texts = append(r.texts, fmt.Sprintf("%v %+v", a, a))
	}
	hook := r.onEntry
	r.mu.Unlock()
	select {
	case r.changed <- struct{}{}:
	default:
	}
	if hook != nil {
		hook(entry)
	}
}

func (r *renewLog) observer() eventfeed.Observer {
	return eventfeed.Observer{
		StreamDeadline: func(at time.Time) { r.record("stream_deadline "+at.UTC().Format(time.RFC3339), at) },
		Renewed:        func(at time.Time) { r.record("renewed "+at.UTC().Format(time.RFC3339), at) },
		RenewFailed: func(kind eventfeed.RenewFailure, retrying bool) {
			r.record(fmt.Sprintf("renew_failed %s %t", kind, retrying), kind, retrying)
		},
		Disconnected: func(reason string, err error) { r.record("disconnected", reason, err) },
	}
}

func (r *renewLog) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.entries...)
}

func (r *renewLog) rendered() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return strings.Join(r.texts, "\n")
}

// await waits for an entry with the given prefix to be recorded.
func (r *renewLog) await(t *testing.T, prefix string) {
	t.Helper()
	deadline := time.After(watchdog)
	for {
		for _, e := range r.snapshot() {
			if strings.HasPrefix(e, prefix) {
				return
			}
		}
		select {
		case <-r.changed:
		case <-deadline:
			t.Fatalf("no %q observer record within the watchdog; got %v", prefix, r.snapshot())
		}
	}
}

func (r *renewLog) count(prefix string) int {
	n := 0
	for _, e := range r.snapshot() {
		if strings.HasPrefix(e, prefix) {
			n++
		}
	}
	return n
}

// streamAdvertised drives a harness to Streaming on a socket whose welcome
// advertised deadline, its entry walk one empty present-class page.
func streamAdvertised(t *testing.T, h *harness, deadline time.Time) *feedtest.Conn {
	t.Helper()
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()
	conn := h.liveConn()
	conn.Serve(welcomeAdvertising(deadline))
	h.awaitTimer(timerConfirmationDeadline)
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()
	return conn
}

// awaitWrite waits for the connection's nth write (1-based) and returns it.
func awaitWrite(t *testing.T, h *harness, conn *feedtest.Conn, n int) []byte {
	t.Helper()
	h.waitUntil(fmt.Sprintf("write %d", n), func() bool { return len(conn.Writes()) >= n })
	return conn.Writes()[n-1]
}

// renewOnce fires a socket's first renew-due and waits for the renew command
// the scripted mint lets it write — the socket's second write, after the
// subscribe.
func renewOnce(t *testing.T, h *harness, conn *feedtest.Conn) {
	t.Helper()
	h.fireTimer(timerRenewDue)
	awaitWrite(t, h, conn, 2)
}

// TestRenewalScheduleIsTheLeadLessJitter pins the formula: renew-due targets
// the advertised deadline − 3 min + uniform(−30 s, +30 s), as a delay from the
// clock's reading.
func TestRenewalScheduleIsTheLeadLessJitter(t *testing.T) {
	for _, tc := range []struct {
		rand float64
		want time.Duration
	}{
		{0, time.Hour - 3*time.Minute - 30*time.Second},
		{0.5, time.Hour - 3*time.Minute},
		{0.75, time.Hour - 3*time.Minute + 15*time.Second},
	} {
		t.Run(fmt.Sprint(tc.rand), func(t *testing.T) {
			h := newHarness(t)
			h.conn.SetRand(func() float64 { return tc.rand })
			streamAdvertised(t, h, h.clock.Now().Add(time.Hour))
			assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1, timerRenewDue: 1})
			if got := h.fireTimer(timerRenewDue); got != tc.want {
				t.Fatalf("renew-due delay = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestRenewalRenewsInBand: one renewal, end to end — the observer's records,
// the command's exact bytes (the subscription's identifier and the FRESH
// ticket), no second dial, and the next renew-due drawn from the new
// deadline.
func TestRenewalRenewsInBand(t *testing.T) {
	log := newRenewLog()
	h := newHarness(t, eventfeed.WithObserver(log.observer()))
	h.conn.SetRand(func() float64 { return 0.5 })
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	start := h.clock.Now()
	deadline := start.Add(time.Hour)
	conn := streamAdvertised(t, h, deadline)

	h.fireTimer(timerRenewDue)
	h.awaitTimer(timerRenewAckDeadline)
	if got, want := string(awaitWrite(t, h, conn, 2)), string(eventfeed.ExportRenewCommand(noFilterIdentifier, ticket(2).Ticket)); got != want {
		t.Fatalf("renew command = %s, want %s", got, want)
	}
	conn.Serve(frameRenewed(deadline.Add(time.Hour)))
	log.await(t, "renewed")
	h.awaitTimer(timerRenewDue)
	assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1, timerRenewDue: 1})
	if got, want := h.fireTimer(timerRenewDue), 2*time.Hour-3*time.Minute; got != want {
		t.Fatalf("the next renew-due delay = %s, want %s (drawn from the new deadline)", got, want)
	}
	if got := log.snapshot()[:2]; got[0] != "stream_deadline "+deadline.Format(time.RFC3339) ||
		got[1] != "renewed "+deadline.Add(time.Hour).Format(time.RFC3339) {
		t.Fatalf("observer records = %v", log.snapshot())
	}
	if got := len(h.tr.DialedURLs()); got != 1 {
		t.Fatalf("dials = %d, want 1 — a renewal never re-dials", got)
	}
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Fatalf("terminal = %v", terminal)
	}
}

// TestRenewalFailuresRetryOnceThenGiveUp: every failure kind is retried once,
// exactly 30 s later, and the second failure gives the deadline up — never a
// terminal, never a teardown, never a second dial, and the settled set falls
// back to exactly {staleness, repair-poll}.
func TestRenewalFailuresRetryOnceThenGiveUp(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		// script queues attempt n's mint outcome (n is the ticket index).
		script func(h *harness, n int)
		// answer drives a started attempt to its failure; writes is the
		// index the attempt's renew command lands at, when it writes one.
		answer func(t *testing.T, h *harness, conn *feedtest.Conn, deadline time.Time, writes int)
	}{
		{
			name: "refused", kind: "refused",
			script: func(h *harness, n int) { h.minter.ScriptTicket(ticket(n)) },
			answer: func(t *testing.T, h *harness, conn *feedtest.Conn, _ time.Time, writes int) {
				awaitWrite(t, h, conn, writes)
				conn.Serve(frameRenewRefused())
			},
		},
		{
			name: "declined with reject_subscription", kind: "refused",
			script: func(h *harness, n int) { h.minter.ScriptTicket(ticket(n)) },
			answer: func(t *testing.T, h *harness, conn *feedtest.Conn, _ time.Time, writes int) {
				awaitWrite(t, h, conn, writes)
				conn.Serve(frameReject(noFilterIdentifier))
			},
		},
		{
			name: "renewed without a usable deadline", kind: "invalid_reply",
			script: func(h *harness, n int) { h.minter.ScriptTicket(ticket(n)) },
			answer: func(t *testing.T, h *harness, conn *feedtest.Conn, _ time.Time, writes int) {
				awaitWrite(t, h, conn, writes)
				conn.Serve(mustJSON(map[string]any{"identifier": noFilterIdentifier, "type": "renewed", "stream_expires_at": 1793070000}))
			},
		},
		{
			name: "renewed to a deadline no later", kind: "invalid_reply",
			script: func(h *harness, n int) { h.minter.ScriptTicket(ticket(n)) },
			answer: func(t *testing.T, h *harness, conn *feedtest.Conn, deadline time.Time, writes int) {
				awaitWrite(t, h, conn, writes)
				conn.Serve(frameRenewed(deadline))
			},
		},
		{
			name: "unanswered", kind: "no_reply",
			script: func(h *harness, n int) { h.minter.ScriptTicket(ticket(n)) },
			answer: func(t *testing.T, h *harness, conn *feedtest.Conn, _ time.Time, writes int) {
				awaitWrite(t, h, conn, writes)
				if d := h.fireTimer(timerRenewAckDeadline); d != 10*time.Second {
					t.Fatalf("renew-ack-deadline = %s, want 10s", d)
				}
			},
		},
		{
			name: "transient mint", kind: "mint_failed",
			script: func(h *harness, _ int) {
				h.minter.ScriptError(&eventfeed.MintError{Kind: eventfeed.MintTransient, Err: errors.New("503")})
			},
			answer: func(*testing.T, *harness, *feedtest.Conn, time.Time, int) {},
		},
		{
			name: "unauthorized mint", kind: "mint_failed",
			script: func(h *harness, _ int) {
				h.minter.ScriptError(&eventfeed.MintError{Kind: eventfeed.MintUnauthorized, Err: errors.New("401")})
			},
			answer: func(*testing.T, *harness, *feedtest.Conn, time.Time, int) {},
		},
		{
			name: "mint outstanding at the deadline", kind: "mint_failed",
			script: func(h *harness, _ int) { h.minter.StallNext() },
			answer: func(t *testing.T, h *harness, _ *feedtest.Conn, _ time.Time, _ int) {
				h.fireTimer(timerRenewAckDeadline)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := newRenewLog()
			h := newHarness(t, eventfeed.WithObserver(log.observer()))
			deadline := h.clock.Now().Add(time.Hour)
			conn := streamAdvertised(t, h, deadline)

			tc.script(h, 2)
			h.fireTimer(timerRenewDue)
			tc.answer(t, h, conn, deadline, 2)
			log.await(t, "renew_failed "+tc.kind+" true")
			h.awaitTimer(timerRenewDue)
			assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1, timerRenewDue: 1})

			tc.script(h, 3)
			if d := h.fireTimer(timerRenewDue); d != 30*time.Second {
				t.Fatalf("the retry's renew-due = %s, want exactly 30s", d)
			}
			tc.answer(t, h, conn, deadline, 3)
			log.await(t, "renew_failed "+tc.kind+" false")
			assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1})

			if got := len(h.tr.DialedURLs()); got != 1 {
				t.Fatalf("dials = %d, want 1 — a failed renewal never reconnects", got)
			}
			if conn.Closed() {
				t.Fatal("a failed renewal must not tear the socket down")
			}
			if _, terminal, _ := h.snapshot(); terminal != nil {
				t.Fatalf("a failed renewal is never terminal; got %v", terminal)
			}
			if got := log.count("disconnected"); got != 0 {
				t.Fatalf("disconnected records = %d, want none", got)
			}
			if got := log.count("renew_failed"); got != 2 {
				t.Fatalf("renew_failed records = %d, want exactly 2: %v", got, log.snapshot())
			}
		})
	}
}

// TestRenewalWriteFailureIsASocketFailure: the renew command's write is the
// socket's, so its failure takes transition 25 — and the failure the host's
// observer sees is the connector's own value, never the seam's text, which
// here carries the very ticket the command was writing.
func TestRenewalWriteFailureIsASocketFailure(t *testing.T) {
	const canary = "sekrit-renewal-ticket"
	log := newRenewLog()
	h := newHarness(t, eventfeed.WithObserver(log.observer()))
	conn := streamAdvertised(t, h, h.clock.Now().Add(time.Hour))
	h.minter.ScriptTicket(eventfeed.StreamTicket{Ticket: canary, ExpiresIn: 120, URL: ticket(2).URL})
	conn.FailWrites(fmt.Errorf("write ticket=%s: broken pipe", canary))

	h.fireTimer(timerRenewDue)
	h.awaitTimer(timerBackoff)
	assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
	if !conn.Closed() {
		t.Fatal("a failed renew write must tear the socket down")
	}
	log.await(t, "disconnected")
	if strings.Contains(log.rendered(), canary) {
		t.Fatalf("the renewal's ticket reached the observer:\n%s", log.rendered())
	}
	if got := log.count("renew_failed"); got != 0 {
		t.Fatalf("a socket failure is not a failed renewal; renew_failed records = %d", got)
	}
}

// TestRenewalWriteOutstandingAtTheDeadlineIsASocketFailure: a renew write
// still blocked when renew-ack-deadline lapses means the peer is not reading
// — the socket's failure, reported with the connector's own sentinel.
func TestRenewalWriteOutstandingAtTheDeadlineIsASocketFailure(t *testing.T) {
	var mu sync.Mutex
	var causes []error
	h := newHarness(t, eventfeed.WithObserver(eventfeed.Observer{
		Disconnected: func(_ string, err error) { mu.Lock(); causes = append(causes, err); mu.Unlock() },
	}))
	sent := make(chan struct{}, 1)
	h.conn.OnRenewSent(func() { sent <- struct{}{} })
	conn := streamAdvertised(t, h, h.clock.Now().Add(time.Hour))
	h.minter.ScriptTicket(ticket(2))
	conn.StallWrites()

	h.fireTimer(timerRenewDue)
	select {
	case <-sent:
	case <-time.After(watchdog):
		t.Fatal("the renew write did not begin")
	}
	h.fireTimer(timerRenewAckDeadline)
	h.awaitTimer(timerBackoff)
	assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
	mu.Lock()
	defer mu.Unlock()
	if len(causes) != 1 || !errors.Is(causes[0], eventfeed.ExportRenewWriteLapsedErr()) {
		t.Fatalf("disconnect causes = %v, want exactly the renew-write-lapsed sentinel", causes)
	}
}

// TestRenewalReplyBeforeTheWriteCompletes: the server can answer a frame
// before the client observes its write complete. That answer is the
// attempt's, but the attempt is not over until the write's outcome is in, so
// the reply is parked behind it: taken once the write lands, discarded when the
// write fails — the socket's failure, whatever the reply said — and when the
// write never lands, the deadline finds a socket that is not reading.
func TestRenewalReplyBeforeTheWriteCompletes(t *testing.T) {
	for _, tc := range []struct {
		name string
		// land settles the held write, after the reply was parked.
		land func(h *harness, conn *feedtest.Conn)
		// renewed is whether the parked reply is taken.
		renewed bool
		// cause is the teardown cause, when the socket goes down.
		cause error
	}{
		{"the write lands", func(_ *harness, conn *feedtest.Conn) { conn.ReleaseWrites() }, true, nil},
		{"the write fails", func(_ *harness, conn *feedtest.Conn) {
			conn.FailWrites(errors.New("broken pipe"))
			conn.ReleaseWrites()
		}, false, eventfeed.ExportSocketFailedErr()},
		{"the write never lands", func(h *harness, _ *feedtest.Conn) {
			h.fireTimer(timerRenewAckDeadline)
		}, false, eventfeed.ExportRenewWriteLapsedErr()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := newRenewLog()
			var mu sync.Mutex
			var causes []error
			obs := log.observer()
			obs.Disconnected = func(_ string, err error) { mu.Lock(); causes = append(causes, err); mu.Unlock() }
			h := newHarness(t, eventfeed.WithObserver(obs))
			sent := make(chan struct{}, 1)
			h.conn.OnRenewSent(func() { sent <- struct{}{} })
			deadline := h.clock.Now().Add(time.Hour)
			conn := streamAdvertised(t, h, deadline)
			h.minter.ScriptTicket(ticket(2))
			conn.StallWrites()

			h.fireTimer(timerRenewDue)
			select {
			case <-sent:
			case <-time.After(watchdog):
				t.Fatal("the renew write did not begin")
			}
			conn.Serve(frameRenewed(deadline.Add(time.Hour)))
			h.serveSettled(conn)
			h.awaitFrameHandled("renewed")
			h.awaitFrameHandled("ping")
			if got := log.count("renewed"); got != 0 {
				t.Fatalf("a reply was taken before its write's outcome was in: %v", log.snapshot())
			}
			assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1, timerRenewAckDeadline: 1})

			tc.land(h, conn)
			if tc.renewed {
				log.await(t, "renewed")
				h.awaitTimer(timerRenewDue)
				assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1, timerRenewDue: 1})
				if conn.Closed() {
					t.Fatal("an acknowledged renewal must leave the socket open")
				}
				return
			}
			h.awaitTimer(timerBackoff)
			assertTimers(t, h.clock, map[string]int{timerBackoff: 1})
			if got := log.count("renewed"); got != 0 {
				t.Fatalf("a parked reply was taken over the write's failure: %v", log.snapshot())
			}
			mu.Lock()
			defer mu.Unlock()
			if len(causes) != 1 || !errors.Is(causes[0], tc.cause) {
				t.Fatalf("disconnect causes = %v, want exactly %v", causes, tc.cause)
			}
		})
	}
}

// parkInDelivery serves one live event with the collector set to park inside
// its delivery, and returns once it has: the state machine is then held, so
// whatever the test makes ready next is ready TOGETHER when resume lets the
// select run again — the both-ready interleavings no other ordering can force.
func parkInDelivery(t *testing.T, h *harness, conn *feedtest.Conn, id int64) {
	t.Helper()
	h.mu.Lock()
	h.pauseAfter = len(h.events) + 1
	h.mu.Unlock()
	conn.Serve(frameMessage(noFilterIdentifier, id))
	h.waitUntil("the consumer to park in its delivery", func() bool {
		events, _, _ := h.snapshot()
		return len(events) == int(id)
	})
}

// gatedMinter answers through the harness's scripted minter, holding one call
// of the test's choosing until the test opens its gate — how a mint's outcome
// is made ready at a moment the test picks.
type gatedMinter struct {
	inner *feedtest.Minter

	mu       sync.Mutex
	n, done  int
	gateCall int
	gate     chan struct{}
}

// newGatedHarness is newHarness over a gatedMinter wrapping its scripted
// minter.
func newGatedHarness(t *testing.T, opts ...eventfeed.Option) (*harness, *gatedMinter) {
	t.Helper()
	gm := &gatedMinter{}
	h := newHarnessWithSeams(t, gm, nil, opts...)
	gm.inner = h.minter
	return h, gm
}

// hold gates the nth mint call (1-based) and returns the function that opens
// the gate.
func (m *gatedMinter) hold(n int) func() {
	gate := make(chan struct{})
	m.mu.Lock()
	m.gateCall, m.gate = n, gate
	m.mu.Unlock()
	return func() { close(gate) }
}

func (m *gatedMinter) calls() int    { m.mu.Lock(); defer m.mu.Unlock(); return m.n }
func (m *gatedMinter) returned() int { m.mu.Lock(); defer m.mu.Unlock(); return m.done }

func (m *gatedMinter) MintStreamTicket(ctx context.Context) (eventfeed.StreamTicket, error) {
	m.mu.Lock()
	m.n++
	gated := m.n == m.gateCall
	gate := m.gate
	m.mu.Unlock()
	if gated {
		select {
		case <-gate:
		case <-ctx.Done():
		}
	}
	ticket, err := m.inner.MintStreamTicket(ctx)
	m.mu.Lock()
	m.done++
	m.mu.Unlock()
	return ticket, err
}

// TestRenewalDeadlineOutranksWhatIsReadyWithIt: when the acknowledgement
// deadline and an attempt's next step are ready together, the select picks one
// at random, and the deadline must win either way — a reply, a mint's ticket
// or a write's completion that the deadline fired ahead of is the deadline's.
// Each case is run repeatedly, so an implementation that lets the select
// decide fails on most runs rather than half of one.
func TestRenewalDeadlineOutranksWhatIsReadyWithIt(t *testing.T) {
	for _, tc := range []struct {
		name string
		// arrange starts the attempt and holds its next step back.
		arrange func(t *testing.T, h *harness, gm *gatedMinter, conn *feedtest.Conn, sent <-chan struct{}) (release func())
		want    string
	}{
		{
			name: "a reply",
			arrange: func(t *testing.T, h *harness, _ *gatedMinter, conn *feedtest.Conn, _ <-chan struct{}) func() {
				h.minter.ScriptTicket(ticket(2))
				renewOnce(t, h, conn)
				return func() { conn.Serve(frameRenewed(h.clock.Now().Add(2 * time.Hour))); h.serveSettled(conn) }
			},
			want: "renew_failed no_reply true",
		},
		{
			name: "a minted ticket",
			arrange: func(t *testing.T, h *harness, gm *gatedMinter, _ *feedtest.Conn, _ <-chan struct{}) func() {
				h.minter.ScriptTicket(ticket(2))
				open := gm.hold(2)
				h.fireTimer(timerRenewDue)
				h.waitUntil("the renewal mint to be in flight", func() bool { return gm.calls() == 2 })
				return func() {
					open()
					h.waitUntil("the mint's outcome to be ready", func() bool { return gm.returned() == 2 })
				}
			},
			want: "renew_failed mint_failed true",
		},
		{
			name: "a completed write",
			arrange: func(t *testing.T, h *harness, _ *gatedMinter, conn *feedtest.Conn, sent <-chan struct{}) func() {
				h.minter.ScriptTicket(ticket(2))
				conn.StallWrites()
				h.fireTimer(timerRenewDue)
				select {
				case <-sent:
				case <-time.After(watchdog):
					t.Fatal("the renew write did not begin")
				}
				return func() {
					conn.ReleaseWrites()
					h.waitUntil("the write to land", func() bool { return len(conn.Writes()) == 2 })
				}
			},
			want: "renew_failed no_reply true",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for run := range 8 {
				t.Run(fmt.Sprint(run), func(t *testing.T) {
					log := newRenewLog()
					h, gm := newGatedHarness(t, eventfeed.WithObserver(log.observer()))
					sent := make(chan struct{}, 1)
					h.conn.OnRenewSent(func() { sent <- struct{}{} })
					conn := streamAdvertised(t, h, h.clock.Now().Add(time.Hour))
					release := tc.arrange(t, h, gm, conn, sent)

					parkInDelivery(t, h, conn, 1)
					release()
					h.fireTimer(timerRenewAckDeadline)
					h.resume()

					log.await(t, "renew_failed")
					if got := log.snapshot(); got[len(got)-1] != tc.want {
						t.Fatalf("observer records = %v, want the deadline's %q last", got, tc.want)
					}
					if got := log.count("renewed"); got != 0 {
						t.Fatalf("the deadline lost to what was ready with it: %v", log.snapshot())
					}
					if conn.Closed() {
						t.Fatal("a lapsed renewal must leave a healthy socket open")
					}
					if got := len(conn.Writes()); tc.name == "a minted ticket" && got != 1 {
						t.Fatalf("writes = %d, want only the subscribe — a mint the deadline fired ahead of starts no write", got)
					}
				})
			}
		})
	}
}

// TestRenewalCloseOutranksTheDeadline: a Close taken while the deadline's
// firing is ready ends the run with no renewal reported — no failure, no
// retry scheduled for a feed the consumer ended.
func TestRenewalCloseOutranksTheDeadline(t *testing.T) {
	for run := range 8 {
		t.Run(fmt.Sprint(run), func(t *testing.T) {
			log := newRenewLog()
			h := newHarness(t, eventfeed.WithObserver(log.observer()))
			conn := streamAdvertised(t, h, h.clock.Now().Add(time.Hour))
			h.minter.ScriptTicket(ticket(2))
			renewOnce(t, h, conn)

			parkInDelivery(t, h, conn, 1)
			h.fireTimer(timerRenewAckDeadline)
			h.conn.Close()
			h.resume()
			h.join()
			if got := log.count("renew_failed"); got != 0 {
				t.Fatalf("a renewal failure was reported after Close: %v", log.snapshot())
			}
			assertTimers(t, h.clock, map[string]int{})
		})
	}
}

// TestRenewalHoldsTheRepairCadence: while an attempt is in flight a fired
// repair-poll is HELD — a reply is only ever awaited from Streaming — and is
// taken the moment the attempt ends. The hold is pinned against the select's
// fairness: a fired cadence left selectable would be taken within a few of
// the sixteen turns the served pings force.
func TestRenewalHoldsTheRepairCadence(t *testing.T) {
	log := newRenewLog()
	h := newHarness(t, eventfeed.WithObserver(log.observer()))
	deadline := h.clock.Now().Add(time.Hour)
	conn := streamAdvertised(t, h, deadline)
	h.minter.ScriptTicket(ticket(2))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-2"})

	renewOnce(t, h, conn)
	h.fireTimer(timerRepairPoll)
	for range 16 {
		conn.Serve(framePing())
		h.awaitFrameHandled("ping")
	}
	if got := h.polls.CallCount(); got != 1 {
		t.Fatalf("poll seam calls = %d, want 1 — the repair cadence must wait out the renewal", got)
	}
	conn.Serve(frameRenewed(deadline.Add(time.Hour)))
	log.await(t, "renewed")
	h.waitUntil("the held repair walk to run", func() bool { return h.polls.CallCount() == 2 })
	h.awaitStreaming()
}

// TestRenewalCloseFromItsCallbacks: each renewal callback is a supported Close
// site, and a Close taken inside one ends the run at once — nothing armed,
// nothing minted, nothing announced after it.
func TestRenewalCloseFromItsCallbacks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prefix string
		drive  func(t *testing.T, h *harness, conn *feedtest.Conn, deadline time.Time)
	}{
		{"stream_deadline", "stream_deadline", func(*testing.T, *harness, *feedtest.Conn, time.Time) {}},
		{"renewed", "renewed", func(t *testing.T, h *harness, conn *feedtest.Conn, deadline time.Time) {
			h.minter.ScriptTicket(ticket(2))
			renewOnce(t, h, conn)
			conn.Serve(frameRenewed(deadline.Add(time.Hour)))
		}},
		{"renew_failed", "renew_failed", func(t *testing.T, h *harness, conn *feedtest.Conn, _ time.Time) {
			h.minter.ScriptTicket(ticket(2))
			renewOnce(t, h, conn)
			conn.Serve(frameRenewRefused())
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := newRenewLog()
			h := newHarness(t, eventfeed.WithObserver(log.observer()))
			log.onEntry = func(entry string) {
				if strings.HasPrefix(entry, tc.prefix) {
					h.conn.Close()
				}
			}
			deadline := h.clock.Now().Add(time.Hour)
			h.minter.ScriptTicket(ticket(1))
			h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
			h.start()
			conn := h.liveConn()
			conn.Serve(welcomeAdvertising(deadline))
			if tc.name != "stream_deadline" {
				h.awaitTimer(timerConfirmationDeadline)
				conn.Serve(frameConfirm(noFilterIdentifier))
				h.awaitStreaming()
			}
			tc.drive(t, h, conn, deadline)
			h.join()
			assertTimers(t, h.clock, map[string]int{})
			if !conn.Closed() {
				t.Fatal("Close from a renewal callback must dispose the socket")
			}
			if _, terminal, _ := h.snapshot(); terminal != nil {
				t.Fatalf("Close ends iteration with no error element; got %v", terminal)
			}
		})
	}
}

// TestRenewalAfterARenewalIsFlooredAtTheMinimumInterval: a target already
// behind the clock fires at once, and the renewal after one is never sooner
// than 30 s — the server refuses a second renewal inside that window, so a
// deadline granted close to the present must not become a renewal loop.
func TestRenewalAfterARenewalIsFlooredAtTheMinimumInterval(t *testing.T) {
	log := newRenewLog()
	h := newHarness(t, eventfeed.WithObserver(log.observer()))
	now := h.clock.Now()
	conn := streamAdvertised(t, h, now.Add(time.Minute))
	h.minter.ScriptTicket(ticket(2))

	if d := h.fireTimer(timerRenewDue); d != 0 {
		t.Fatalf("a renew-due whose target has passed = %s, want 0", d)
	}
	awaitWrite(t, h, conn, 2)
	conn.Serve(frameRenewed(now.Add(2 * time.Minute)))
	log.await(t, "renewed")
	if d := h.fireTimer(timerRenewDue); d != 30*time.Second {
		t.Fatalf("the renew-due after a renewal = %s, want the 30s floor", d)
	}
}

// TestRenewalLateReplyIsNotAdopted: a reply arriving after its attempt ended
// answers nothing — the attempt was already counted, so the retry stands
// exactly as scheduled and no renewal is reported.
func TestRenewalLateReplyIsNotAdopted(t *testing.T) {
	log := newRenewLog()
	h := newHarness(t, eventfeed.WithObserver(log.observer()))
	deadline := h.clock.Now().Add(time.Hour)
	conn := streamAdvertised(t, h, deadline)
	h.minter.ScriptTicket(ticket(2))

	renewOnce(t, h, conn)
	h.fireTimer(timerRenewAckDeadline)
	log.await(t, "renew_failed no_reply true")
	conn.Serve(frameRenewed(deadline.Add(time.Hour)))
	h.serveSettled(conn)
	h.awaitFrameHandled("renewed")
	// The ping serveSettled served behind it is handled only once the
	// reply's own dispatch has returned: the state machine is one goroutine.
	h.awaitFrameHandled("ping")
	if got := log.count("renewed"); got != 0 {
		t.Fatalf("a late reply was adopted: %v", log.snapshot())
	}
	if d := h.fireTimer(timerRenewDue); d != 30*time.Second {
		t.Fatalf("the retry's renew-due = %s, want the 30s it was scheduled with", d)
	}
}

// TestRenewalDuplicateWelcomeAdvertisesNothing: only the connection's FIRST
// welcome offers renewal; a duplicate is liveness, whatever it carries.
func TestRenewalDuplicateWelcomeAdvertisesNothing(t *testing.T) {
	log := newRenewLog()
	h := newHarness(t, eventfeed.WithObserver(log.observer()))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()
	conn := h.driveToSubscribed()
	conn.Serve(welcomeAdvertising(h.clock.Now().Add(time.Hour)))
	h.awaitFrameHandled("welcome")
	h.awaitFrameHandled("welcome")
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()
	assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1})
	if got := log.count("stream_deadline"); got != 0 {
		t.Fatalf("a duplicate welcome's advert was taken: %v", log.snapshot())
	}
}

// TestRenewalScheduleSurvivesARepairWalk: renew-due is Streaming's alone — a
// repair walk runs with CatchingUp's own set — and comes back armed for the
// REMAINDER of the target it held, never a redraw: the jitter source answers
// 0.5 for the welcome's draw and 0 for everything after, so a redraw is 30 s
// off the remainder.
func TestRenewalScheduleSurvivesARepairWalk(t *testing.T) {
	// The staleness window and the repair cadence are a day long, so the
	// ten-minute advance below fires nothing.
	h := newHarness(t, eventfeed.WithRepairInterval(24*time.Hour))
	draws := 0
	h.conn.SetRand(func() float64 {
		draws++
		if draws == 1 {
			return 0.5
		}
		return 0
	})
	h.conn.SetStaleAfter(24 * time.Hour)
	var mu sync.Mutex
	var during [][]string
	h.polls.OnCall(func(feedtest.PollCall) {
		mu.Lock()
		during = append(during, h.clock.Outstanding())
		mu.Unlock()
	})
	streamAdvertised(t, h, h.clock.Now().Add(time.Hour))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-2"})

	if due := h.clock.DueWithin(10 * time.Minute); len(due) != 0 {
		t.Fatalf("the advance would fire %v", due)
	}
	h.clock.Advance(10 * time.Minute)
	h.fireTimer(timerRepairPoll)
	h.waitUntil("the repair walk", func() bool { return h.polls.CallCount() == 2 })
	h.awaitStreaming()
	mu.Lock()
	walk := during[len(during)-1]
	mu.Unlock()
	if len(walk) != 1 || walk[0] != timerStaleness {
		t.Fatalf("timers during the repair walk = %v, want exactly [staleness]", walk)
	}
	if got, want := h.fireTimer(timerRenewDue), 47*time.Minute; got != want {
		t.Fatalf("renew-due after the walk = %s, want the remainder %s", got, want)
	}
}

// panickingRenewalMinter answers the first mint and panics on the second —
// the renewal's — at once, or only once its context is cancelled.
type panickingRenewalMinter struct {
	onCancel bool

	mu    sync.Mutex
	calls int
}

func (m *panickingRenewalMinter) MintStreamTicket(ctx context.Context) (eventfeed.StreamTicket, error) {
	m.mu.Lock()
	m.calls++
	n := m.calls
	m.mu.Unlock()
	if n == 1 {
		return ticket(1), nil
	}
	if m.onCancel {
		<-ctx.Done()
	}
	panic("host minter panicked during a renewal")
}

// TestRenewalMintPanicPropagates: the renewal's mint runs on its own
// goroutine, and a panic there is re-raised on the consumer's, as every other
// seam's is — with the attempt, renewal timers included, disposed on the way.
// That holds for a mint that panics only when the acknowledgement deadline
// cancels it, too: the lapse joins the cancelled mint rather than abandoning
// it.
func TestRenewalMintPanicPropagates(t *testing.T) {
	for _, onCancel := range []bool{false, true} {
		t.Run(fmt.Sprintf("on cancellation %t", onCancel), func(t *testing.T) {
			renewalMintPanicPropagates(t, onCancel)
		})
	}
}

func renewalMintPanicPropagates(t *testing.T, onCancel bool) {
	clock := feedtest.NewClock()
	tr := feedtest.NewTransport()
	polls := feedtest.NewPolls()
	polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	c, err := eventfeed.New(testOrigin, "5951425", &panickingRenewalMinter{onCancel: onCancel}, polls,
		eventfeed.WithTransport(tr), eventfeed.WithClock(clock))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	streaming := make(chan struct{}, 1)
	c.OnStateChanged(func(s string) {
		if s == stateStreaming {
			select {
			case streaming <- struct{}{}:
			default:
			}
		}
	})
	recovered := make(chan any, 1)
	go func() {
		defer func() { recovered <- recover() }()
		for range c.Events(context.Background()) {
		}
	}()
	clock.AwaitTimer(timerStaleness)
	conn := tr.LastConn()
	conn.Serve(welcomeAdvertising(clock.Now().Add(time.Hour)))
	clock.AwaitTimer(timerConfirmationDeadline)
	conn.Serve(frameConfirm(noFilterIdentifier))
	select {
	case <-streaming:
	case <-time.After(watchdog):
		t.Fatal("Streaming was not entered")
	}
	clock.AwaitTimer(timerRenewDue)
	clock.FireTimer(timerRenewDue)
	if onCancel {
		clock.AwaitTimer(timerRenewAckDeadline)
		clock.FireTimer(timerRenewAckDeadline)
	}
	select {
	case r := <-recovered:
		if s, _ := r.(string); s != "host minter panicked during a renewal" {
			t.Fatalf("recovered %v, want the minter's own panic value", r)
		}
	case <-time.After(watchdog):
		t.Fatal("the iteration neither panicked nor returned")
	}
	if !conn.Closed() {
		t.Fatal("the live socket was not disposed during panic unwinding")
	}
	assertTimers(t, clock, map[string]int{})
}

// TestRenewalReplyReadBeforeTheCommandIsNotItsAnswer: a reply the pump read
// before an attempt's command went out cannot answer that attempt, however
// late the state machine dequeues it. Here the first attempt goes unanswered
// and its refusal arrives late: read while the consumer is held, with the
// retry's mint finished behind it, so the select can start the retry's write
// before it dequeues the refusal. The retry's own acknowledgement must still
// count — judged at dequeue, the stale refusal would be the retry's second
// failure, and the deadline would be given up on a renewal the server granted.
func TestRenewalReplyReadBeforeTheCommandIsNotItsAnswer(t *testing.T) {
	for run := range 8 {
		t.Run(fmt.Sprint(run), func(t *testing.T) {
			log := newRenewLog()
			h, gm := newGatedHarness(t, eventfeed.WithObserver(log.observer()))
			deadline := h.clock.Now().Add(time.Hour)
			conn := streamAdvertised(t, h, deadline)
			h.minter.ScriptTicket(ticket(2))
			renewOnce(t, h, conn)
			h.fireTimer(timerRenewAckDeadline)
			log.await(t, "renew_failed no_reply true")

			h.minter.ScriptTicket(ticket(3))
			open := gm.hold(3)
			h.fireTimer(timerRenewDue)
			h.waitUntil("the retry's mint to be in flight", func() bool { return gm.calls() == 3 })
			parkInDelivery(t, h, conn, 1)
			conn.Serve(frameRenewRefused())
			h.serveSettled(conn)
			open()
			h.waitUntil("the retry's mint to finish", func() bool { return gm.returned() == 3 })
			h.resume()

			awaitWrite(t, h, conn, 3)
			conn.Serve(frameRenewed(deadline.Add(time.Hour)))
			log.await(t, "renewed")
			if got := log.count("renew_failed"); got != 1 {
				t.Fatalf("renew_failed records = %d, want only the first attempt's: %v", got, log.snapshot())
			}
		})
	}
}

// closingRenewalMinter answers the first mint, and closes the connector from
// inside the second — the renewal's — once the acknowledgement deadline
// cancels it: a Close taken while the lapse joins the mint.
type closingRenewalMinter struct {
	conn *eventfeed.Connector

	mu    sync.Mutex
	calls int
}

func (m *closingRenewalMinter) MintStreamTicket(ctx context.Context) (eventfeed.StreamTicket, error) {
	m.mu.Lock()
	m.calls++
	n := m.calls
	m.mu.Unlock()
	if n == 1 {
		return ticket(1), nil
	}
	<-ctx.Done()
	m.conn.Close()
	return eventfeed.StreamTicket{}, ctx.Err()
}

// TestRenewalCloseDuringTheMintJoinOutranksTheFailure: the lapse joins a
// cancelled mint, and the join is host code's time — a Close taken during it
// ends the run, and no failure or retry is reported for the feed it ended.
func TestRenewalCloseDuringTheMintJoinOutranksTheFailure(t *testing.T) {
	log := newRenewLog()
	minter := &closingRenewalMinter{}
	h := newHarnessWithSeams(t, minter, nil, eventfeed.WithObserver(log.observer()))
	minter.conn = h.conn
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()
	conn := h.liveConn()
	conn.Serve(welcomeAdvertising(h.clock.Now().Add(time.Hour)))
	h.awaitTimer(timerConfirmationDeadline)
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitStreaming()

	h.fireTimer(timerRenewDue)
	h.fireTimer(timerRenewAckDeadline)
	h.join()
	if got := log.count("renew_failed"); got != 0 {
		t.Fatalf("a renewal failure was reported after Close: %v", log.snapshot())
	}
	assertTimers(t, h.clock, map[string]int{})
}

// cancelPanickingWriteTransport hands out conns whose renew write blocks
// until it is cancelled or the conn is closed, and then panics.
type cancelPanickingWriteTransport struct{ inner *feedtest.Transport }

func (p *cancelPanickingWriteTransport) Dial(ctx context.Context, wsURL string, maxFrameBytes int64) (eventfeed.CableConn, error) {
	conn, err := p.inner.Dial(ctx, wsURL, maxFrameBytes)
	if err != nil {
		return nil, err
	}
	return &cancelPanickingWriteConn{CableConn: conn, closed: make(chan struct{})}, nil
}

type cancelPanickingWriteConn struct {
	eventfeed.CableConn
	once   sync.Once
	closed chan struct{}
}

func (c *cancelPanickingWriteConn) WriteFrame(ctx context.Context, data []byte) error {
	if !strings.Contains(string(data), `"command":"renew"`) {
		return c.CableConn.WriteFrame(ctx, data)
	}
	select {
	case <-ctx.Done():
	case <-c.closed:
	}
	panic("host connection panicked as its renew write was abandoned")
}

func (c *cancelPanickingWriteConn) Close(code int, reason string) error {
	c.once.Do(func() { close(c.closed) })
	return c.CableConn.Close(code, reason)
}

// TestRenewalWritePanicOnTeardownPropagates: a renew write still blocked at
// the deadline is the socket's failure, and the teardown that returns it is
// followed by a join — a panic the write raised on the way out reaches the
// consumer, as the mint's does.
func TestRenewalWritePanicOnTeardownPropagates(t *testing.T) {
	base := newHarness(t)
	h := newHarness(t, eventfeed.WithTransport(&cancelPanickingWriteTransport{inner: base.tr}))
	h.tr = base.tr
	h.minter.ScriptTicket(ticket(1))
	h.minter.ScriptTicket(ticket(2))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	sent := make(chan struct{}, 1)
	h.conn.OnRenewSent(func() { sent <- struct{}{} })
	recovered := make(chan any, 1)
	go func() {
		defer func() { recovered <- recover() }()
		for range h.conn.Events(context.Background()) {
		}
	}()
	h.awaitTimer(timerStaleness)
	conn := h.tr.LastConn()
	conn.Serve(welcomeAdvertising(h.clock.Now().Add(time.Hour)))
	h.awaitTimer(timerConfirmationDeadline)
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.awaitTimer(timerRenewDue)
	h.fireTimer(timerRenewDue)
	select {
	case <-sent:
	case <-time.After(watchdog):
		t.Fatal("the renew write did not begin")
	}
	h.fireTimer(timerRenewAckDeadline)
	select {
	case r := <-recovered:
		if s, _ := r.(string); s != "host connection panicked as its renew write was abandoned" {
			t.Fatalf("recovered %v, want the write's own panic value", r)
		}
	case <-time.After(watchdog):
		t.Fatal("the iteration neither panicked nor returned")
	}
	assertTimers(t, h.clock, map[string]int{})
}

// TestRenewalReplyOnAnotherIdentifierAnswersNothing: the server answers a
// renewal on the subscription's own identifier, so a reply — or a reject — on
// any other is liveness, and the attempt runs on to its deadline.
func TestRenewalReplyOnAnotherIdentifierAnswersNothing(t *testing.T) {
	foreign := eventfeed.ExportSubscribeIdentifier(eventfeed.Filters{Types: []string{"todo.created"}})
	for _, reply := range [][]byte{
		mustJSON(map[string]any{"identifier": foreign, "type": "renewed", "stream_expires_at": "2026-01-01T03:00:00.000Z"}),
		mustJSON(map[string]any{"type": "renewed", "stream_expires_at": "2026-01-01T03:00:00.000Z"}),
		mustJSON(map[string]any{"identifier": 42, "type": "renewed", "stream_expires_at": "2026-01-01T03:00:00.000Z"}),
		mustJSON(map[string]any{"identifier": foreign, "type": "renew_refused", "reason": "not_later"}),
		frameReject(foreign),
	} {
		t.Run(string(reply), func(t *testing.T) {
			log := newRenewLog()
			h := newHarness(t, eventfeed.WithObserver(log.observer()))
			conn := streamAdvertised(t, h, h.clock.Now().Add(time.Hour))
			h.minter.ScriptTicket(ticket(2))
			renewOnce(t, h, conn)
			conn.Serve(reply)
			h.serveSettled(conn)
			h.awaitFrameHandled("ping")
			if got := log.snapshot(); len(got) != 1 {
				t.Fatalf("a reply on another identifier answered the renewal: %v", got)
			}
			assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1, timerRenewAckDeadline: 1})
			h.fireTimer(timerRenewAckDeadline)
			log.await(t, "renew_failed no_reply true")
			if conn.Closed() {
				t.Fatal("a reply on another identifier must not tear the socket down")
			}
		})
	}
}
