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

func frameRenewed(deadline time.Time) []byte {
	return mustJSON(map[string]any{"type": "renewed", "stream_expires_at": deadline.UTC().Format(time.RFC3339Nano)})
}

func frameRenewRefused() []byte {
	return mustJSON(map[string]any{"type": "renew_refused", "reason": "expiry_not_later"})
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
			name: "renewed without a usable deadline", kind: "invalid_reply",
			script: func(h *harness, n int) { h.minter.ScriptTicket(ticket(n)) },
			answer: func(t *testing.T, h *harness, conn *feedtest.Conn, _ time.Time, writes int) {
				awaitWrite(t, h, conn, writes)
				conn.Serve([]byte(`{"type":"renewed","stream_expires_at":1793070000}`))
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
// before the client observes its write complete, and that answer is the
// attempt's — attributing it only after the write's completion would discard
// a renewal the server granted.
func TestRenewalReplyBeforeTheWriteCompletes(t *testing.T) {
	log := newRenewLog()
	h := newHarness(t, eventfeed.WithObserver(log.observer()))
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
	log.await(t, "renewed")
	h.awaitTimer(timerRenewDue)
	assertTimers(t, h.clock, map[string]int{timerStaleness: 1, timerRepairPoll: 1, timerRenewDue: 1})
	if conn.Closed() {
		t.Fatal("an acknowledged renewal must leave the socket open")
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
// the renewal's.
type panickingRenewalMinter struct {
	mu    sync.Mutex
	calls int
}

func (m *panickingRenewalMinter) MintStreamTicket(context.Context) (eventfeed.StreamTicket, error) {
	m.mu.Lock()
	m.calls++
	n := m.calls
	m.mu.Unlock()
	if n == 1 {
		return ticket(1), nil
	}
	panic("host minter panicked during a renewal")
}

// TestRenewalMintPanicPropagates: the renewal's mint runs on its own
// goroutine, and a panic there is re-raised on the consumer's, as every other
// seam's is — with the attempt, renewal timers included, disposed on the way.
func TestRenewalMintPanicPropagates(t *testing.T) {
	clock := feedtest.NewClock()
	tr := feedtest.NewTransport()
	polls := feedtest.NewPolls()
	polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	c, err := eventfeed.New(testOrigin, "5951425", &panickingRenewalMinter{}, polls,
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
