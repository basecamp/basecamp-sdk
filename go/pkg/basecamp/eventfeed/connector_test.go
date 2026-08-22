// Construction-time validation and consumer-surface tests for the connector
// (SPEC.md §23 "Consumer Surface"): usage-coded construction errors with
// zero wire attempts, no I/O in New, single-shot Events, idempotent Close.
package eventfeed_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed/feedtest"
)

// nopStore is a CheckpointStore stub for construction tests.
type nopStore struct{}

func (nopStore) Load(context.Context, eventfeed.CheckpointKey) (string, bool, error) {
	return "", false, nil
}

func (nopStore) Save(context.Context, eventfeed.CheckpointKey, string) error { return nil }

func TestNewValidation(t *testing.T) {
	minter := feedtest.NewMinter()
	polls := feedtest.NewPolls()
	longIDs := make([]int64, 101)
	for i := range longIDs {
		longIDs[i] = int64(i + 1)
	}

	cases := []struct {
		name      string
		origin    string
		accountID string
		minter    eventfeed.TicketMinter
		polls     eventfeed.PollSource
		opts      []eventfeed.Option
		wantMsg   string
	}{
		{"empty origin", "", "1", minter, polls, nil, "base origin"},
		{"unparseable origin", "://nope", "1", minter, polls, nil, "origin"},
		{"origin without a host", "https://", "1", minter, polls, nil, "scheme and host"},
		{"empty account id", testOrigin, "", minter, polls, nil, "accountID"},
		{"nil minter", testOrigin, "1", nil, polls, nil, "TicketMinter"},
		{"nil poll source", testOrigin, "1", minter, nil, nil, "PollSource"},
		{"filter type with whitespace", testOrigin, "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithFilters(eventfeed.Filters{Types: []string{"a b"}})},
			"whitespace"},
		{"filter list over the cap", testOrigin, "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithFilters(eventfeed.Filters{Buckets: longIDs})},
			"at most 100"},
		{"non-positive filter id", testOrigin, "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithFilters(eventfeed.Filters{Creators: []int64{0}})},
			"positive"},
		{"zero dedupe capacity", testOrigin, "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithDedupeCapacity(0)},
			"dedupe capacity"},
		{"negative live buffer capacity", testOrigin, "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithLiveBufferCapacity(-1)},
			"live buffer capacity"},
		{"non-positive confirmation deadline", testOrigin, "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithConfirmationDeadline(0)},
			"confirmation deadline"},
		{"non-positive repair interval", testOrigin, "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithRepairInterval(-time.Second)},
			"repair interval"},
		{"store without a consumer namespace", testOrigin, "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithCheckpointStore(nopStore{})},
			"consumer namespace"},
		{"start after a non-positive id", testOrigin, "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithStart(eventfeed.StartAfter(0))},
			"positive event id"},
		{"start at an empty position", testOrigin, "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithStart(eventfeed.StartAtPosition(""))},
			"non-empty position"},
		{"explicit position with a store", testOrigin, "1", minter, polls,
			[]eventfeed.Option{
				eventfeed.WithStart(eventfeed.StartAtPosition("pos-1")),
				eventfeed.WithCheckpointStore(nopStore{}),
				eventfeed.WithConsumerNamespace("agent"),
			},
			"mutually exclusive"},
		// SPEC §9's transport-security rule (and §23's "the configured base
		// origin is the trust anchor every continuation is validated
		// against"): cleartext is the localhost carve-out only.
		{"cleartext non-loopback origin", "http://api.example", "1", minter, polls, nil, "https"},
		{"cleartext non-loopback origin with a port", "http://api.example:8080", "1", minter, polls, nil, "https"},
		{"non-http origin scheme", "wss://cable.example.com", "1", minter, polls, nil, "http(s)"},
		// Checkpoint-identity components must be valid UTF-8: the identity
		// encoding is one-to-one only over valid UTF-8 (checkpoint.go).
		{"invalid utf-8 account id", testOrigin, "\xff", minter, polls, nil, "UTF-8"},
		{"invalid utf-8 consumer namespace", testOrigin, "1", minter, polls,
			[]eventfeed.Option{
				eventfeed.WithCheckpointStore(nopStore{}),
				eventfeed.WithConsumerNamespace("\xff"),
			},
			"UTF-8"},
		{"invalid utf-8 filter type", testOrigin, "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithFilters(eventfeed.Filters{Types: []string{"message.\xff"}})},
			"UTF-8"},
		// The origin must be validated RAW, before canonicalization:
		// CanonicalOrigin lowercases through strings.ToLower, which rewrites
		// every invalid byte to U+FFFD, so a post-canonical check passes a
		// collapsed identity — these two rows would otherwise canonicalize to
		// ONE origin and share one checkpoint lineage, the exact collapse
		// checkIdentityText exists to refuse (checkpoint.go).
		{"invalid utf-8 origin host (0xff)", "https://\xff.example.com", "1", minter, polls, nil, "UTF-8"},
		{"invalid utf-8 origin host (0xfe)", "https://\xfe.example.com", "1", minter, polls, nil, "UTF-8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := eventfeed.New(tc.origin, tc.accountID, tc.minter, tc.polls, tc.opts...)
			if err == nil {
				t.Fatalf("New succeeded (%v), want a usage-coded construction error", c)
			}
			var te *eventfeed.TerminalError
			if !errors.As(err, &te) || te.Reason != eventfeed.ReasonUsage {
				t.Fatalf("error = %v, want *TerminalError with reason %q", err, eventfeed.ReasonUsage)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error %q does not mention %q", err, tc.wantMsg)
			}
		})
	}
}

func TestNewValidConfigurations(t *testing.T) {
	minter := feedtest.NewMinter()
	polls := feedtest.NewPolls()

	if _, err := eventfeed.New(testOrigin, "1", minter, polls); err != nil {
		t.Fatalf("minimal New: %v", err)
	}
	_, err := eventfeed.New(testOrigin, "1", minter, polls,
		eventfeed.WithFilters(eventfeed.Filters{Types: []string{"message.created"}}),
		eventfeed.WithStart(eventfeed.StartPresent()),
		eventfeed.WithTransport(feedtest.NewTransport()),
		eventfeed.WithClock(feedtest.NewClock()),
		eventfeed.WithCheckpointStore(nopStore{}),
		eventfeed.WithConsumerNamespace("agent"),
		eventfeed.WithConfirmationDeadline(5*time.Second),
		eventfeed.WithRepairInterval(30*time.Second),
		eventfeed.WithDedupeCapacity(100),
		eventfeed.WithLiveBufferCapacity(100),
		eventfeed.WithSignalHandler(func(eventfeed.Signal) eventfeed.Disposition { return eventfeed.Accept }),
		eventfeed.WithObserver(eventfeed.Observer{}),
	)
	if err != nil {
		t.Fatalf("fully-optioned New: %v", err)
	}
}

// TestNewAcceptsTheLoopbackCarveOut: cleartext is permitted for exactly the
// hosts checkCableURL permits ws:// for — SPEC §9's localhost/loopback
// carve-out, same helper, same rule.
func TestNewAcceptsTheLoopbackCarveOut(t *testing.T) {
	for _, origin := range []string{
		"https://api.example",
		"http://localhost:3000",
		"http://127.0.0.1:9292",
		"http://[::1]:3000",
		"http://app.localhost",
		"HTTP://LOCALHOST:3000",
	} {
		if _, err := eventfeed.New(origin, "1", feedtest.NewMinter(), feedtest.NewPolls()); err != nil {
			t.Errorf("New(%q): %v", origin, err)
		}
	}
}

// TestNewRejectsIndistinguishableConsumerNamespaces: 0xff and 0xfe are
// distinct namespaces that the identity encoding renders identically (its
// rune-wise JSON escaping maps every invalid byte to U+FFFD), so two
// independent consumers would otherwise share — and overwrite — one
// checkpoint lineage, each skipping the other's events. Construction refuses
// them rather than silently mangling.
func TestNewRejectsIndistinguishableConsumerNamespaces(t *testing.T) {
	for _, ns := range []string{"\xff", "\xfe", "agent\xc3(1"} {
		_, err := eventfeed.New(testOrigin, "1", feedtest.NewMinter(), feedtest.NewPolls(),
			eventfeed.WithCheckpointStore(nopStore{}), eventfeed.WithConsumerNamespace(ns))
		if err == nil {
			t.Fatalf("New(namespace %q) succeeded, want a usage-coded construction error", ns)
		}
		var te *eventfeed.TerminalError
		if !errors.As(err, &te) || te.Reason != eventfeed.ReasonUsage {
			t.Fatalf("New(namespace %q) error = %v, want *TerminalError with reason %q", ns, err, eventfeed.ReasonUsage)
		}
	}
}

// TestNewDoesNoIO: construction is validation only — the first wire attempt
// happens on the first iteration.
func TestNewDoesNoIO(t *testing.T) {
	minter := feedtest.NewMinter()
	tr := feedtest.NewTransport()
	if _, err := eventfeed.New(testOrigin, "1", minter, feedtest.NewPolls(),
		eventfeed.WithTransport(tr), eventfeed.WithClock(feedtest.NewClock())); err != nil {
		t.Fatal(err)
	}
	if minter.Calls() != 0 || len(tr.Dials()) != 0 {
		t.Fatalf("New performed I/O: %d mints, %d dials", minter.Calls(), len(tr.Dials()))
	}
}

// TestEventsSingleShot: a second consumption yields exactly one
// usage-terminal error element.
func TestEventsSingleShot(t *testing.T) {
	h := newHarness(t)
	h.minter.StallNext()
	h.start()
	h.waitUntil("mint in flight", func() bool { return h.minter.Calls() == 1 })
	h.conn.Close()
	h.join()
	if _, terminal, elements := h.snapshot(); terminal != nil || elements != 0 {
		t.Fatalf("first consumption should end cleanly; got %d elements, terminal %v", elements, terminal)
	}

	var second []error
	for _, err := range h.conn.Events(context.Background()) {
		second = append(second, err)
	}
	if len(second) != 1 {
		t.Fatalf("second consumption yielded %d elements, want exactly 1", len(second))
	}
	var te *eventfeed.TerminalError
	if !errors.As(second[0], &te) || te.Reason != eventfeed.ReasonUsage {
		t.Fatalf("second consumption error = %v, want reason %q", second[0], eventfeed.ReasonUsage)
	}
}

// TestCloseIdempotent: Close is idempotent from any goroutine, and closing
// before the first iteration ends it immediately with no wire attempts.
func TestCloseIdempotent(t *testing.T) {
	h := newHarness(t)
	if err := h.conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := h.conn.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	h.start()
	h.join()
	if _, terminal, elements := h.snapshot(); terminal != nil || elements != 0 {
		t.Fatalf("pre-closed iteration should end cleanly; got %d elements, terminal %v", elements, terminal)
	}
}

// TestCloseFromAnObserverCancelsBeforeItReturns: SPEC §23 makes close() a
// universal edge from EVERY non-absorbing state, so the active run must
// observe cancellation before Close returns — not when some independently
// scheduled goroutine happens to run. Observer.Connecting is the tightest
// statement of the rule: it fires on the iteration goroutine itself, one
// statement before the mint seam call, so a Close taken inside it must stop
// the mint that was about to happen. It must also not deadlock — Close is
// waiting on nothing, least of all on the goroutine that called it.
func TestCloseFromAnObserverCancelsBeforeItReturns(t *testing.T) {
	const runs = 200
	for i := range runs {
		minter := feedtest.NewMinter()
		tr := feedtest.NewTransport()
		var c *eventfeed.Connector
		var err error
		c, err = eventfeed.New(testOrigin, "1", minter, feedtest.NewPolls(),
			eventfeed.WithTransport(tr),
			eventfeed.WithClock(feedtest.NewClock()),
			eventfeed.WithObserver(eventfeed.Observer{
				Connecting: func(int, time.Duration) {
					if cerr := c.Close(); cerr != nil {
						t.Fatalf("run %d: Close: %v", i, cerr)
					}
				},
			}))
		if err != nil {
			t.Fatalf("run %d: New: %v", i, err)
		}
		// Scripted so an un-cancelled mint SUCCEEDS: the assertion must be
		// that the seam was never called, not that it happened to fail.
		minter.ScriptTicket(ticket(1))
		elements := 0
		for range c.Events(context.Background()) {
			elements++
		}
		if minter.Calls() != 0 {
			t.Fatalf("run %d: %d mint seam call(s) after Close returned, want 0", i, minter.Calls())
		}
		if got := len(tr.Dials()); got != 0 {
			t.Fatalf("run %d: %d dial(s) after Close returned, want 0", i, got)
		}
		if elements != 0 {
			t.Fatalf("run %d: iteration yielded %d element(s), want none (the Closed edge)", i, elements)
		}
	}
}

// TestCloseStopsDeliveryAlreadyUnderway: the same universal edge, stated over
// deliveries rather than seam calls. The consumer is parked inside the loop
// body — the state machine is mid-page, with two more rows to yield — when
// Close is called from another goroutine. Nothing may be delivered after it
// returns; the run abandons the rest of the page and ends with no error
// element (the page is re-served from the last usable checkpoint next run).
func TestCloseStopsDeliveryAlreadyUnderway(t *testing.T) {
	h := newHarness(t)
	h.pauseAfter = 1
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{
		Events:   []eventfeed.Event{pollEvent(101), pollEvent(102), pollEvent(103)},
		Position: "pos-1",
	})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(noFilterIdentifier))
	h.waitUntil("the page's first row was delivered", func() bool { return len(h.deliveredIDs()) == 1 })
	if err := h.conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	h.resume()
	h.join()

	assertIDs(t, h.deliveredIDs(), 101)
	if _, terminal, elements := h.snapshot(); terminal != nil || elements != 1 {
		t.Fatalf("iteration yielded %d element(s), terminal %v; want exactly the one pre-Close delivery",
			elements, terminal)
	}
}

// filterMutationHarness drives one full run under caller-held filter slices,
// mutating them at the moment mutate says, and returns everything the run
// derived the filters from: the subscribe frame written to the socket, the
// filters each poll carried, and the checkpoint keys the store saw.
func filterMutationHarness(t *testing.T, original eventfeed.Filters, live eventfeed.Filters, mutate func(), fromConnected bool) (writes [][]byte, polled []eventfeed.Filters, keys []eventfeed.CheckpointKey) {
	t.Helper()
	store := feedtest.NewStore()
	store.Stored("pos-0")
	opts := []eventfeed.Option{eventfeed.WithFilters(live)}
	if fromConnected {
		// The review's exact scenario: the mutation lands from a lifecycle
		// callback, after newLoop has already frozen the subscription
		// identifier — so a retained backing array splits the subscribed
		// filters from the polled and checkpointed ones.
		opts = append(opts, eventfeed.WithObserver(eventfeed.Observer{Connected: mutate}))
	}
	h := storedHarness(t, store, opts...)
	if !fromConnected {
		mutate()
	}
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(101)}, Position: "pos-1"})
	h.start()

	conn := h.driveToSubscribed()
	// Asserted here rather than with the rest: the identifier the connector
	// subscribed under is what the confirm frame below has to correlate
	// against, so a mutated subscription would otherwise surface as a
	// confirmation that never arrives.
	writes = conn.Writes()
	if len(writes) != 1 || string(writes[0]) != string(eventfeed.ExportSubscribeFrame(original)) {
		t.Fatalf("subscribe writes = %q, want exactly %q", writes, eventfeed.ExportSubscribeFrame(original))
	}
	conn.Serve(frameConfirm(eventfeed.ExportSubscribeIdentifier(original)))
	h.awaitStreaming()
	h.conn.Close()
	h.join()

	calls := h.polls.Calls()
	polled = make([]eventfeed.Filters, len(calls))
	for i, c := range calls {
		polled[i] = c.Filters
	}
	return writes, polled, append(store.Loads(), store.SaveKeys()...)
}

// assertFiltersUnaffected asserts every filter-derived artifact of the run is
// the constructed filter set: the subscribe bytes, every poll's filters, and
// every checkpoint key's lineage.
func assertFiltersUnaffected(t *testing.T, original eventfeed.Filters, writes [][]byte, polled []eventfeed.Filters, keys []eventfeed.CheckpointKey) {
	t.Helper()
	want := eventfeed.ExportSubscribeIdentifier(original)
	if len(writes) != 1 || string(writes[0]) != string(eventfeed.ExportSubscribeFrame(original)) {
		t.Fatalf("subscribe writes = %q, want exactly %q", writes, eventfeed.ExportSubscribeFrame(original))
	}
	if len(polled) == 0 {
		t.Fatal("no poll was made; the run did not reach the poll lane")
	}
	for i, f := range polled {
		if got := eventfeed.ExportSubscribeIdentifier(f); got != want {
			t.Errorf("poll %d filters = %s, want %s", i, got, want)
		}
	}
	if len(keys) == 0 {
		t.Fatal("the store saw no key; the run did not reach the checkpoint lane")
	}
	for i, k := range keys {
		if k.FilterKey != original.FilterKey() {
			t.Errorf("checkpoint key %d lineage = %s, want %s", i, k.FilterKey, original.FilterKey())
		}
	}
}

// TestFiltersMutatedBeforeIterationDoNotReachTheWire: Filters carries slices,
// so a caller that keeps its backing arrays could otherwise change the
// subscription, the poll parameters and the checkpoint lineage AFTER the
// constructor validated them — including into a filter set Validate rejects.
// New snapshots them, so the run is unaffected.
func TestFiltersMutatedBeforeIterationDoNotReachTheWire(t *testing.T) {
	types := []string{"message.created"}
	buckets := []int64{11}
	creators := []int64{22}
	original := eventfeed.Filters{
		Types:    []string{"message.created"},
		Buckets:  []int64{11},
		Creators: []int64{22},
	}
	writes, polled, keys := filterMutationHarness(t, original,
		eventfeed.Filters{Types: types, Buckets: buckets, Creators: creators},
		func() {
			// "a b" would not survive Filters.Validate — the fail-closed
			// guarantee is only worth what the connector actually holds.
			types[0] = "a b"
			buckets[0] = 99
			creators[0] = 98
		}, false)
	assertFiltersUnaffected(t, original, writes, polled, keys)
}

// TestFiltersMutatedFromConnectedDoNotSplitTheLineage: the same hazard from
// Observer.Connected, after the subscription identifier is frozen. Without a
// snapshot the cable stays subscribed under the original filters while the
// polls and the checkpoint key move to the mutated ones — a position saved
// under a lineage the feed never subscribed to.
func TestFiltersMutatedFromConnectedDoNotSplitTheLineage(t *testing.T) {
	types := []string{"message.created"}
	buckets := []int64{11}
	creators := []int64{22}
	original := eventfeed.Filters{
		Types:    []string{"message.created"},
		Buckets:  []int64{11},
		Creators: []int64{22},
	}
	writes, polled, keys := filterMutationHarness(t, original,
		eventfeed.Filters{Types: types, Buckets: buckets, Creators: creators},
		func() {
			types[0] = "todo.completed"
			buckets[0] = 99
			creators[0] = 98
		}, true)
	assertFiltersUnaffected(t, original, writes, polled, keys)
}

// TestPollSeamCannotRepointTheLineage: the filters handed to the poll seam
// are a copy too. The seam is host code — an adapter that sorts or dedupes in
// place while building its query would otherwise mutate the connector's own
// storage mid-run, splitting the subscribed filters from the polled and
// checkpointed ones exactly as a retained caller array does.
func TestPollSeamCannotRepointTheLineage(t *testing.T) {
	original := eventfeed.Filters{
		Types:    []string{"message.created"},
		Buckets:  []int64{11},
		Creators: []int64{22},
	}
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store, eventfeed.WithFilters(eventfeed.Filters{
		Types:    []string{"message.created"},
		Buckets:  []int64{11},
		Creators: []int64{22},
	}))
	// The ENTRY poll's seam call mutates what it was handed, in place. Only
	// the first: the assertion below reads the second call's filters, and the
	// seam's own copy is what the fake records, so a mutation on that call
	// would prove nothing either way.
	var polled atomic.Int32
	h.polls.OnCall(func(c feedtest.PollCall) {
		if polled.Add(1) != 1 {
			return
		}
		c.Filters.Types[0] = "todo.completed"
		c.Filters.Buckets[0] = 99
		c.Filters.Creators[0] = 98
	})
	h.minter.ScriptTicket(ticket(1))
	next := testOrigin + "/999/events.json?after=101"
	h.polls.ScriptPage(eventfeed.PollPage{
		Events:   []eventfeed.Event{pollEvent(101)},
		Position: "pos-1",
		Next:     next,
	})
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(102)}, Position: "pos-2"})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(eventfeed.ExportSubscribeIdentifier(original)))
	h.awaitStreaming()
	h.conn.Close()
	h.join()

	calls := h.polls.Calls()
	if len(calls) != 2 {
		t.Fatalf("poll seam calls = %d, want 2 (the entry page and its continuation)", len(calls))
	}
	// The first call's record aliases what the seam mutated; the SECOND call
	// is the one that proves the connector's storage survived it.
	want := eventfeed.ExportSubscribeIdentifier(original)
	if got := eventfeed.ExportSubscribeIdentifier(calls[1].Filters); got != want {
		t.Errorf("continuation poll filters = %s, want %s", got, want)
	}
	for i, k := range append(store.Loads(), store.SaveKeys()...) {
		if k.FilterKey != original.FilterKey() {
			t.Errorf("checkpoint key %d lineage = %s, want %s", i, k.FilterKey, original.FilterKey())
		}
	}
}

// TestConcurrentCloseAllCallersSeeCancellation pins Close's guarantee for
// EVERY caller, not just the first. Events' own comment states it — "there is
// no window in which the run proceeds past a returned Close" — and clearing
// the cancellation field and invoking it outside the lock left one: the first
// caller descheduled between the unlock and the cancel, a concurrent second
// caller observing nil, taking no action, and returning while the run context
// is still live. Its caller has been told the feed is closed while a delivery
// or a seam call can still begin.
//
// It asserts on the run CONTEXT, deliberately. Every in-struct proxy for it —
// a cleared cancel func, the close latch — is set by the broken Close too, so
// asserting on one would pass against un-fixed code.
//
// The interleaving is real rather than designed, so the proof is
// probabilistic: the assertion is exact and the iteration count is sized off
// MEASURED hits rather than guessed. Against un-fixed code the window was hit
// at iterations 92, 142, 169 and 362 across four runs, so the budget is set
// several times the worst observed — a count near the worst hit is how a
// regression test quietly becomes flaky-green.
func TestConcurrentCloseAllCallersSeeCancellation(t *testing.T) {
	for i := 0; i < 1500; i++ {
		h := newHarness(t)
		h.minter.ScriptTicket(ticket(1))
		got := make(chan context.Context, 1)
		h.conn.OnRunContext(func(ctx context.Context) {
			select {
			case got <- ctx:
			default:
			}
		})
		h.start()
		var runCtx context.Context
		select {
		case runCtx = <-got:
		case <-time.After(watchdog):
			t.Fatal("the run context was never registered")
		}

		var wg sync.WaitGroup
		bad := make(chan struct{}, 4)
		for g := 0; g < 4; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = h.conn.Close()
				if runCtx.Err() == nil {
					select {
					case bad <- struct{}{}:
					default:
					}
				}
			}()
		}
		wg.Wait()
		select {
		case <-bad:
			t.Fatalf("iteration %d: Close returned with the run context still live", i)
		default:
		}
		h.cancel()
	}
}

// emptyPositionStore is a custom CheckpointStore that reports FOUND with an
// empty position — the state the built-in FileStore classifies as a store
// failure, and which a custom store is under no obligation to.
type emptyPositionStore struct{ loads int }

func (s *emptyPositionStore) Load(context.Context, eventfeed.CheckpointKey) (string, bool, error) {
	s.loads++
	return "", true, nil
}

func (s *emptyPositionStore) Save(context.Context, eventfeed.CheckpointKey, string) error {
	return nil
}

// TestFoundButEmptyPositionIsALoadFailure closes a SILENT history skip at the
// seam. entryCursor selects on a non-empty position, so an empty one loaded
// under StartResume falls through to the mode's default — a bare present
// entry, which is present-class — and the feed resumes at the server's head
// having skipped everything between the stored position and now, with no
// signal at all.
//
// The built-in store already refuses to return this state ("an empty position
// cannot be told apart from having none"), but that is one implementation of a
// public seam; the invariant has to hold for every store, so it is enforced
// where the load is consumed.
func TestFoundButEmptyPositionIsALoadFailure(t *testing.T) {
	store := &emptyPositionStore{}
	h := newHarness(t,
		eventfeed.WithCheckpointStore(store),
		eventfeed.WithConsumerNamespace("agent"))
	h.minter.ScriptTicket(ticket(1))
	h.start()
	h.join()

	_, terminal, _ := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonCheckpointLoad {
		t.Fatalf("terminal = %v, want reason %q — a found-but-empty position is a store failure", terminal, eventfeed.ReasonCheckpointLoad)
	}
	if store.loads != 1 {
		t.Errorf("store loads = %d, want 1", store.loads)
	}
	// Terminal BEFORE any wire attempt, like every other checkpoint_load.
	if got := h.minter.Calls(); got != 0 {
		t.Errorf("mint seam calls = %d, want 0 — the load fails before any wire attempt", got)
	}
	if got := len(h.tr.Dials()); got != 0 {
		t.Errorf("dials = %d, want 0", got)
	}
}

// blockingStore is a CheckpointStore whose Save parks until released, so a
// test can hold one durable write in flight across a concurrent Close. Load is
// scripted separately.
type blockingStore struct {
	entered     chan struct{}
	release     chan struct{}
	loadErr     error
	loadCtx     chan context.Context
	loadEntered chan struct{}
	loadRelease chan struct{}
	mu          sync.Mutex
	saves       []string
}

func newBlockingStore() *blockingStore {
	return &blockingStore{
		entered: make(chan struct{}, 8),
		release: make(chan struct{}),
		loadCtx: make(chan context.Context, 4),
	}
}

func (s *blockingStore) Load(ctx context.Context, _ eventfeed.CheckpointKey) (string, bool, error) {
	select {
	case s.loadCtx <- ctx:
	default:
	}
	// Parkable, so a test can hold the load in flight and land a Close on it.
	// That window is the only one the cancelled-load classification governs:
	// closing BEFORE the run starts never reaches this function at all, since
	// Events takes the isClosed latch with zero wire attempts.
	if s.loadEntered != nil {
		s.loadEntered <- struct{}{}
		<-s.loadRelease
	}
	if s.loadErr != nil {
		return "", false, s.loadErr
	}
	return "pos-0", true, nil
}

func (s *blockingStore) Save(_ context.Context, _ eventfeed.CheckpointKey, position string) error {
	s.entered <- struct{}{}
	<-s.release
	s.mu.Lock()
	s.saves = append(s.saves, position)
	s.mu.Unlock()
	return nil
}

func (s *blockingStore) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.saves...)
}

// TestCloseDoesNotBlockOnAnInFlightCheckpointSave pins Close's own promise.
//
// Waiting on CheckpointStore.Save was the obvious spelling and is wrong twice:
// a store whose Save calls Close self-deadlocks on the caller's goroutine
// (Close is documented as callable from anywhere), and a store that merely
// stalls blocks EVERY Close for as long as it stalls — contradicting the one
// thing Close promises unconditionally, that cancellation is visible before it
// returns.
//
// So Close cancels and returns. The in-flight save still completes: its
// position was accepted and delivered before Close was called, and abandoning a
// durable write half-way is worse than finishing it.
func TestCloseDoesNotBlockOnAnInFlightCheckpointSave(t *testing.T) {
	store := newBlockingStore()
	h := newHarness(t,
		eventfeed.WithCheckpointStore(store),
		eventfeed.WithConsumerNamespace("agent"))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(101)}, Position: "pos-1"})
	h.start()
	sock := h.driveToSubscribed()
	sock.Serve(frameConfirm(noFilterIdentifier))

	// The save is parked inside the store, on the run goroutine.
	select {
	case <-store.entered:
	case <-time.After(watchdog):
		t.Fatal("the checkpoint save never reached the store")
	}

	closeReturned := make(chan struct{})
	go func() { _ = h.conn.Close(); close(closeReturned) }()
	select {
	case <-closeReturned:
	case <-time.After(watchdog):
		t.Fatal("Close blocked behind an in-flight checkpoint save; a stalled store must not hold up Close")
	}

	close(store.release)
	deadline := time.Now().Add(watchdog)
	for len(store.recorded()) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := store.recorded(); len(got) != 1 || got[0] != "pos-1" {
		t.Errorf("recorded saves = %v, want [pos-1] — a write that already commenced completes", got)
	}
}

// TestAnAcceptedPositionSavesEvenWhenCloseLandsFirst is the other direction,
// and it is the inverse of what this test used to assert.
//
// It used to pin a gate that REFUSED a save once Close had returned. Close is
// taken from inside Observer.PageDelivered, which fires immediately after the
// page's events have been handed to the consumer and immediately before the
// page's position is written — so what the gate actually bought here was a
// silent dropped save for events the consumer had already received. The next
// run then re-delivers them, having no record they ever arrived.
//
// That was the whole of the gate's cost, and against it stood a guarantee it
// did not deliver: the overwrite window it existed to close was narrowed, not
// closed (claiming the gate and writing are not atomic together), and the harm
// it narrowed — a checkpoint moving backward — is bounded replay, the same
// class as the drop it caused. Quiescence is what actually closes it, and the
// iterator returning IS quiescence. See Connector.Wait.
func TestAnAcceptedPositionSavesEvenWhenCloseLandsFirst(t *testing.T) {
	store := newBlockingStore()
	close(store.release) // saves complete immediately
	var h *harness
	h = newHarness(t,
		eventfeed.WithCheckpointStore(store),
		eventfeed.WithConsumerNamespace("agent"),
		eventfeed.WithObserver(eventfeed.Observer{
			PageDelivered: func(int, string) {
				if err := h.conn.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
			},
		}))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{
		Events:   []eventfeed.Event{pollEvent(101)},
		Position: "pos-1",
	})
	h.start()
	sock := h.driveToSubscribed()
	sock.Serve(frameConfirm(noFilterIdentifier))
	h.join()

	if got := store.recorded(); len(got) != 1 || got[0] != "pos-1" {
		t.Errorf("recorded saves = %v, want [pos-1] — event 101 was delivered, so its position must be durable", got)
	}
}

// TestWaitReturnsOnlyAfterTheRunHasExited pins the guarantee that replaced the
// durable gate, and pins it against the thing it must not be confused with.
//
// Close cancels and returns while the run is still unwinding — deliberately,
// since it is callable from inside a callback that runs ON the run's goroutine.
// So Close alone cannot order a second connector over the same store. Wait can,
// because it reports the run's exit, at which point no save can be in flight by
// construction rather than by a window narrow enough to be unlikely.
//
// The save is held INSIDE the store so the two are distinguishable: a Wait that
// merely mirrored Close would return here, with the write still pending.
func TestWaitReturnsOnlyAfterTheRunHasExited(t *testing.T) {
	store := newBlockingStore()
	h := newHarness(t,
		eventfeed.WithCheckpointStore(store),
		eventfeed.WithConsumerNamespace("agent"))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(101)}, Position: "pos-1"})
	h.start()
	sock := h.driveToSubscribed()
	sock.Serve(frameConfirm(noFilterIdentifier))

	// The run goroutine is parked inside CheckpointStore.Save.
	select {
	case <-store.entered:
	case <-time.After(watchdog):
		t.Fatal("the checkpoint save never reached the store")
	}
	if err := h.conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	waited := make(chan struct{})
	go func() { h.conn.Wait(); close(waited) }()
	select {
	case <-waited:
		t.Fatal("Wait returned with a checkpoint save still in flight; it must report the run's EXIT, not Close's return")
	case <-time.After(50 * time.Millisecond):
	}

	close(store.release)
	select {
	case <-waited:
	case <-time.After(watchdog):
		t.Fatal("Wait did not return after the run exited")
	}
	// Quiescence is the point: by the time Wait returns, the write a second
	// connector would race is already durable.
	if got := store.recorded(); len(got) != 1 || got[0] != "pos-1" {
		t.Errorf("recorded saves = %v, want [pos-1] before Wait returns", got)
	}
	h.join()
}

// TestWaitWithNoActiveRunReturnsImmediately covers the two shapes that are not
// a running feed: never consumed, and already finished. Neither may block.
func TestWaitWithNoActiveRunReturnsImmediately(t *testing.T) {
	t.Run("never consumed", func(t *testing.T) {
		h := newHarness(t)
		done := make(chan struct{})
		go func() { h.conn.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(watchdog):
			t.Fatal("Wait blocked on a connector that was never consumed")
		}
	})
	t.Run("run already finished", func(t *testing.T) {
		h := newHarness(t)
		h.minter.ScriptError(&eventfeed.MintError{Kind: eventfeed.MintUnrecoverable})
		h.start()
		h.join()
		done := make(chan struct{})
		go func() { h.conn.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(watchdog):
			t.Fatal("Wait blocked after the run had already exited")
		}
	})
}

// TestCancelledCheckpointLoadIsNotAStoreFailure is the third item. The load
// happens on the first iteration and before the first mint, so it is exactly
// the window a prompt Close lands in. Reporting Terminal(checkpoint_load) for
// it would diagnose the consumer's store for the consumer's own shutdown — and
// §23 says Close ends the iterator with NO error element.
//
// The load is held IN FLIGHT and Close lands on it. Closing before the run
// starts proves nothing: Events takes the isClosed latch with zero wire
// attempts and never calls the store at all. Verified — written that way
// first, it passed with the classification deleted.
func TestCancelledCheckpointLoadIsNotAStoreFailure(t *testing.T) {
	store := newBlockingStore()
	store.loadEntered = make(chan struct{}, 1)
	store.loadRelease = make(chan struct{})
	// The store reports its OWN error type, not a wrapped ctx.Err(), which is
	// what a store is entitled to do and why the classification reads the
	// context rather than the error's shape.
	store.loadErr = errors.New("store: request aborted")
	h := newHarness(t,
		eventfeed.WithCheckpointStore(store),
		eventfeed.WithConsumerNamespace("agent"))
	h.minter.ScriptTicket(ticket(1))
	h.start()

	select {
	case <-store.loadEntered:
	case <-time.After(watchdog):
		t.Fatal("the checkpoint load never reached the store")
	}
	if err := h.conn.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}
	close(store.loadRelease)
	h.join()

	_, terminal, elements := h.snapshot()
	if terminal != nil {
		t.Errorf("terminal = %v, want none — a cancelled load is a shutdown, not a store failure", terminal)
	}
	if elements != 0 {
		t.Errorf("iteration elements = %d, want 0", elements)
	}
	if got := h.minter.Calls(); got != 0 {
		t.Errorf("mint seam calls = %d, want 0", got)
	}
}

// An UNCANCELLED load failure is still Terminal(checkpoint_load): the guard
// above must not swallow the real edge. Without this the previous test would be
// satisfied by deleting the classification entirely.
func TestUncancelledCheckpointLoadFailureIsStillTerminal(t *testing.T) {
	store := feedtest.NewStore()
	store.FailLoad(errors.New("disk on fire"))
	h := newHarness(t,
		eventfeed.WithCheckpointStore(store),
		eventfeed.WithConsumerNamespace("agent"))
	h.minter.ScriptTicket(ticket(1))
	h.start()
	h.join()

	_, terminal, _ := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonCheckpointLoad {
		t.Fatalf("terminal = %v, want reason %q", terminal, eventfeed.ReasonCheckpointLoad)
	}
	if got := h.minter.Calls(); got != 0 {
		t.Errorf("mint seam calls = %d, want 0 (zero wire attempts)", got)
	}
}

// hookedStore is a CheckpointStore that runs a hook inside Load — the one
// callback site that is not an Observer, and the only deterministic way to
// take a Close between the run starting and its first Observer.Connecting.
type hookedStore struct {
	onLoad func()
	mu     sync.Mutex
	saves  []string
}

func (s *hookedStore) Load(context.Context, eventfeed.CheckpointKey) (string, bool, error) {
	if s.onLoad != nil {
		s.onLoad()
	}
	return "pos-0", true, nil
}

func (s *hookedStore) Save(_ context.Context, _ eventfeed.CheckpointKey, position string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saves = append(s.saves, position)
	return nil
}

func (s *hookedStore) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.saves...)
}

// TestCloseOutranksTheNextLifecycleAnnouncement extends to the observer what
// emitTerminal already does for the terminal element: a Close the consumer has
// already taken outranks what the connector says next.
//
// emitTerminal made exactly ONE exit Close-ordered. Every lifecycle
// announcement kept firing, because the nearest runCtx check sat AFTER the
// callback rather than before it — a check placed to catch a Close taken from
// INSIDE the callback, which is a different Close from one taken earlier. So a
// consumer that closed during `confirmed` was still told `catch_up_started`,
// and one that closed during `catch_up_started` was still told a page had been
// delivered.
//
// Each case closes from one callback and names the announcement that must not
// follow it. The pairing is the assertion: "the feed stopped" is satisfied by
// almost any behavior, where "this specific next thing was not announced" is
// satisfied only by the ordering.
func TestCloseOutranksTheNextLifecycleAnnouncement(t *testing.T) {
	for _, tc := range []struct {
		name string
		// observer wires the CLOSING callback and the one that must not fire
		// after it; forbidden records the latter.
		observer func(closeFeed func(), forbidden func(string)) eventfeed.Observer
		// notAfter names the announcement under test, for the failure message.
		notAfter string
		// presentAt enters present-class, which HOLDS its position to the end
		// of the walk instead of saving per page.
		presentAt bool
	}{
		{
			name:     "Connecting after a Close taken during the checkpoint load",
			notAfter: "connecting",
			observer: func(_ func(), forbidden func(string)) eventfeed.Observer {
				return eventfeed.Observer{
					Connecting: func(int, time.Duration) { forbidden("connecting") },
				}
			},
		},
		{
			name:     "CatchUpStarted after a Close taken during Confirmed",
			notAfter: "catch_up_started",
			observer: func(cl func(), forbidden func(string)) eventfeed.Observer {
				return eventfeed.Observer{
					Confirmed:      func() { cl() },
					CatchUpStarted: func(eventfeed.Cursor) { forbidden("catch_up_started") },
				}
			},
		},
		{
			name:     "PageDelivered after a Close taken during CatchUpStarted",
			notAfter: "page_delivered",
			observer: func(cl func(), forbidden func(string)) eventfeed.Observer {
				return eventfeed.Observer{
					CatchUpStarted: func(eventfeed.Cursor) { cl() },
					PageDelivered:  func(int, string) { forbidden("page_delivered") },
				}
			},
		},
		{
			name:     "CaughtUp after a Close taken during PageDelivered",
			notAfter: "caught_up",
			observer: func(cl func(), forbidden func(string)) eventfeed.Observer {
				return eventfeed.Observer{
					PageDelivered: func(int, string) { cl() },
					CaughtUp:      func() { forbidden("caught_up") },
				}
			},
		},
		{
			name:     "CaughtUp after a Close taken during Checkpoint",
			notAfter: "caught_up",
			observer: func(cl func(), forbidden func(string)) eventfeed.Observer {
				return eventfeed.Observer{
					Checkpoint: func(string) { cl() },
					CaughtUp:   func() { forbidden("caught_up") },
				}
			},
		},
		// The case above cannot isolate the check before `caught_up`: on a
		// position-resume entry the save fires per page, so the walk's own
		// boundary check ends things first and either check alone would pass
		// it. A PRESENT-class entry HOLDS its position to the end, so its
		// Checkpoint callback fires after the walk has already returned and
		// after the drain — leaving the check before `caught_up` as the only
		// thing standing between that Close and the announcement.
		{
			name:      "CaughtUp after a Close taken during the HELD save",
			notAfter:  "caught_up",
			presentAt: true,
			observer: func(cl func(), forbidden func(string)) eventfeed.Observer {
				return eventfeed.Observer{
					Checkpoint: func(string) { cl() },
					CaughtUp:   func() { forbidden("caught_up") },
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var seen []string
			forbidden := func(name string) {
				mu.Lock()
				defer mu.Unlock()
				seen = append(seen, name)
			}
			var h *harness
			closeFeed := func() {
				if err := h.conn.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
			}
			store := &hookedStore{}
			if tc.notAfter == "connecting" {
				// The load runs before the first mint and before the first
				// Connecting announcement, so it is the deterministic way to
				// take a Close in that window. Every other case closes from an
				// Observer callback.
				store.onLoad = func() { closeFeed() }
			}
			opts := []eventfeed.Option{
				eventfeed.WithCheckpointStore(store),
				eventfeed.WithConsumerNamespace("agent"),
				eventfeed.WithObserver(tc.observer(closeFeed, forbidden)),
			}
			if tc.presentAt {
				opts = append(opts, eventfeed.WithStart(eventfeed.StartPresent()))
			}
			h = newHarness(t, opts...)
			h.minter.ScriptTicket(ticket(1))
			h.polls.ScriptPage(eventfeed.PollPage{
				Events:   []eventfeed.Event{pollEvent(101)},
				Position: "pos-1",
			})
			h.start()
			if tc.notAfter != "connecting" {
				sock := h.driveToSubscribed()
				sock.Serve(frameConfirm(noFilterIdentifier))
			}
			h.join()

			mu.Lock()
			defer mu.Unlock()
			if len(seen) != 0 {
				t.Errorf("%q fired after Close; §23 makes close() a universal edge from every non-absorbing state", tc.notAfter)
			}
			if _, terminal, _ := h.snapshot(); terminal != nil {
				t.Errorf("terminal = %v, want none — Close ends the iterator with no error element", terminal)
			}
		})
	}
}

// TestCloseDuringPageDeliveredFinishesThePageAndStops is the other half of the
// ordering above, and the half that must NOT be a suppression.
//
// A Close taken from Observer.PageDelivered ends the walk — no second poll,
// whatever `next` says. But the page it interrupted is finished first: its
// events were already handed to the consumer, so its position is written. The
// check sits after the save for exactly that reason, and a check placed before
// it would reintroduce the silent dropped save the durable gate was deleted
// for.
func TestCloseDuringPageDeliveredFinishesThePageAndStops(t *testing.T) {
	store := &hookedStore{}
	var h *harness
	h = newHarness(t,
		eventfeed.WithCheckpointStore(store),
		eventfeed.WithConsumerNamespace("agent"),
		eventfeed.WithObserver(eventfeed.Observer{
			PageDelivered: func(int, string) {
				if err := h.conn.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
			},
		}))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{
		Events:   []eventfeed.Event{pollEvent(101)},
		Position: "pos-1",
		Next:     testOrigin + "/999/events.json?after=101",
	})
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{pollEvent(102)}, Position: "pos-2"})
	h.start()
	sock := h.driveToSubscribed()
	sock.Serve(frameConfirm(noFilterIdentifier))
	h.join()

	if got := store.recorded(); len(got) != 1 || got[0] != "pos-1" {
		t.Errorf("recorded saves = %v, want [pos-1] — event 101 was delivered, so its position must be durable", got)
	}
	if got := h.polls.Calls(); len(got) != 1 {
		t.Errorf("poll seam calls = %d, want 1 — Close must stop the walk before it follows `next`", len(got))
	}
	assertIDs(t, h.deliveredIDs(), 101)
}

// TestCloseDuringPageDeliveredSilencesThePageBoundary is what the page
// boundary's own checks buy that the walk's top-of-loop check does not.
//
// The top-of-loop check is later, and between it and PageDelivered sit two
// passes that DISPATCH what they dequeue: the ownership cut on a present-class
// entry, and socketCheck before following `next`. With a disconnect frame
// queued when the consumer closes, either announces `disconnected` — a
// teardown report for a socket the consumer had already stopped caring about,
// after the universal Closed edge was taken.
//
// Both entry classes, because they reach different passes and a single case
// would leave one of the two checks unexercised. That is not hypothetical: the
// first version of this test used the default harness, which resolves to
// PRESENT-class, so it drove only the ownership cut while claiming to be about
// socketCheck.
func TestCloseDuringPageDeliveredSilencesThePageBoundary(t *testing.T) {
	for _, tc := range []struct {
		name string
		// stored gives the entry a position, which makes it position-resume
		// and routes the page through socketCheck rather than the cut.
		stored bool
	}{
		{name: "present-class entry, at the ownership cut"},
		{name: "position-resume entry, at socketCheck", stored: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var disconnects int
			var h *harness
			var conn *feedtest.Conn
			queued := make(chan struct{}, 8)
			store := feedtest.NewStore()
			if tc.stored {
				store.Stored("pos-0")
			}
			h = newHarness(t,
				eventfeed.WithCheckpointStore(store),
				eventfeed.WithConsumerNamespace("agent"),
				eventfeed.WithObserver(eventfeed.Observer{
					PageDelivered: func(int, string) {
						// Served HERE, not during the poll: a frame arriving
						// while the call is in flight is parked in the
						// deferral slot, which neither pass looks at. This one
						// goes into the pump's QUEUE, which is what they drain.
						//
						// The drain first is load-bearing: welcome and confirm
						// have already handed off, so an undrained channel
						// satisfies the wait below on a STALE token, and Close
						// then lands before the disconnect is queued at all —
						// leaving nothing for the code under test to find.
						drain(queued)
						conn.Serve(frameDisconnect("remote", true))
						select {
						case <-queued:
						case <-time.After(watchdog):
							t.Error("the disconnect frame never reached the hand-off queue")
							return
						}
						if err := h.conn.Close(); err != nil {
							t.Errorf("Close: %v", err)
						}
					},
					Disconnected: func(string, error) {
						mu.Lock()
						defer mu.Unlock()
						disconnects++
					},
				}))
			h.conn.OnPumpHandedOff(func(bool) {
				select {
				case queued <- struct{}{}:
				default:
				}
			})
			h.minter.ScriptTicket(ticket(1))
			h.polls.ScriptPage(eventfeed.PollPage{
				Events:   []eventfeed.Event{pollEvent(101)},
				Position: "pos-1",
				Next:     testOrigin + "/999/events.json?after=101",
			})
			h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-2"})
			h.start()
			conn = h.driveToSubscribed()
			conn.Serve(frameConfirm(noFilterIdentifier))
			h.join()

			mu.Lock()
			defer mu.Unlock()
			if disconnects != 0 {
				t.Errorf("Observer.Disconnected fired %d time(s) after Close; the page boundary must end the walk before its dispatching passes run", disconnects)
			}
		})
	}
}

// TestCloseFromEveryCallbackSiteDoesNotDeadlock pins that Close is callable
// from inside a consumer callback, which is documented as supported and is the
// case that has to be right rather than merely likely. Every callback runs ON
// the run goroutine, and Close takes the connector's own mutex, so a Close that
// waited for anything the run goroutine still owed would deadlock against the
// very callback that made it.
//
// Each site below ends the iteration cleanly with no error element; a deadlock
// fails as the watchdog rather than hanging the package.
func TestCloseFromEveryCallbackSiteDoesNotDeadlock(t *testing.T) {
	sites := []struct {
		name     string
		observer func(closeFeed func()) eventfeed.Observer
		failSave bool
	}{
		{"Checkpoint", func(cl func()) eventfeed.Observer {
			return eventfeed.Observer{Checkpoint: func(string) { cl() }}
		}, false},
		{"CheckpointSaveFailed", func(cl func()) eventfeed.Observer {
			return eventfeed.Observer{CheckpointSaveFailed: func(error) { cl() }}
		}, true},
		{"PageDelivered", func(cl func()) eventfeed.Observer {
			return eventfeed.Observer{PageDelivered: func(int, string) { cl() }}
		}, false},
		{"CaughtUp", func(cl func()) eventfeed.Observer {
			return eventfeed.Observer{CaughtUp: func() { cl() }}
		}, false},
		{"CatchUpStarted", func(cl func()) eventfeed.Observer {
			return eventfeed.Observer{CatchUpStarted: func(eventfeed.Cursor) { cl() }}
		}, false},
		{"Confirmed", func(cl func()) eventfeed.Observer {
			return eventfeed.Observer{Confirmed: func() { cl() }}
		}, false},
	}
	for _, site := range sites {
		t.Run(site.name, func(t *testing.T) {
			store := feedtest.NewStore()
			store.Stored("pos-0")
			if site.failSave {
				store.FailNextSave(errors.New("store unavailable"))
			}
			var h *harness
			h = newHarness(t,
				eventfeed.WithCheckpointStore(store),
				eventfeed.WithConsumerNamespace("agent"),
				eventfeed.WithObserver(site.observer(func() {
					if err := h.conn.Close(); err != nil {
						t.Errorf("Close from %s: %v", site.name, err)
					}
				})))
			h.minter.ScriptTicket(ticket(1))
			h.polls.ScriptPage(eventfeed.PollPage{
				Events:   []eventfeed.Event{pollEvent(101)},
				Position: "pos-1",
			})
			h.start()
			sock := h.driveToSubscribed()
			sock.Serve(frameConfirm(noFilterIdentifier))
			// The watchdog inside join is what reports a deadlock as itself.
			h.join()

			if _, terminal, _ := h.snapshot(); terminal != nil {
				t.Errorf("terminal = %v, want none — Close ends the iterator with no error element", terminal)
			}
		})
	}
}

// TestConcurrentCloseIsSerializedAndIdempotent drives Close from many
// goroutines at once, which is what the mutex across the cancel is for: a
// second Close must not observe a cleared cancelRun and return while the run
// context is still live. Under -race this also covers the durable gate being
// latched by whichever caller gets there.
func TestConcurrentCloseIsSerializedAndIdempotent(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := newHarness(t,
		eventfeed.WithCheckpointStore(store),
		eventfeed.WithConsumerNamespace("agent"))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	h.start()
	sock := h.driveToSubscribed()
	sock.Serve(frameConfirm(noFilterIdentifier))

	const closers = 16
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, closers)
	for range closers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := h.conn.Close(); err != nil {
				errs <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("Close returned %v, want nil from every caller", err)
	}
	h.join()
	if _, terminal, _ := h.snapshot(); terminal != nil {
		t.Errorf("terminal = %v, want none", terminal)
	}
}

// TestCloseFromConnectedOutranksAQueuedFatalFrame is the precedence #763
// reopened. Arming staleness before Observer.Connected also starts the frame
// pump before it, so by the time that callback runs the peer can already have
// queued a fatal frame. If the callback then calls Close, the state machine's
// next select has two ready cases — the frame and the cancellation — and Go
// picks freely.
//
// §23 makes close() a universal edge from every non-absorbing state and ends
// the iterator with NO error element, so the terminal must never win. The guard
// is central (emitTerminal), not per-select: there are many selects and one
// exit, and a rule every future select has to remember is the shape that
// produced this.
//
// Driven repeatedly because the underlying choice is random: with the guard the
// count is deterministically zero, and without it a terminal escapes within a
// handful of rounds.
func TestCloseFromConnectedOutranksAQueuedFatalFrame(t *testing.T) {
	const rounds = 50
	terminals := 0
	for i := range rounds {
		var h *harness
		handedOff := make(chan struct{}, 8)
		h = newHarness(t, eventfeed.WithObserver(eventfeed.Observer{
			Connected: func() {
				// Connected now fires with the pump already running, so the
				// peer's fatal frame can be queued before the consumer decides
				// to stop — and then both are ready at the same select.
				if sock := h.tr.LastConn(); sock != nil {
					sock.Serve(frameDisconnect("invalid_event_stream_command", false))
					// Wait until the pump has actually QUEUED it, so the
					// cancellation below genuinely races a ready frame rather
					// than beating it to the channel.
					select {
					case <-handedOff:
					case <-time.After(watchdog):
					}
				}
				h.conn.Close() //nolint:errcheck // asserted via the element count
			},
		}))
		h.conn.OnPumpHandedOff(func(bool) {
			select {
			case handedOff <- struct{}{}:
			default:
			}
		})
		h.minter.ScriptTicket(ticket(1))
		h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
		h.start()
		h.join()
		if _, terminal, _ := h.snapshot(); terminal != nil {
			terminals++
			if i == 0 || terminals == 1 {
				t.Logf("round %d emitted %v after Close", i, terminal)
			}
		}
	}
	if terminals != 0 {
		t.Errorf("%d/%d rounds emitted a terminal element after Close returned; the Closed edge must win",
			terminals, rounds)
	}
}

// parkingCloseTransport wraps the harness transport so the FIRST socket
// close parks until the test releases it — a deterministic hold inside a
// teardown's disposal, after the terminal outcome is selected and before
// emitTerminal publishes it.
type parkingCloseTransport struct {
	inner   *feedtest.Transport
	parked  chan struct{}
	release chan struct{}
	once    sync.Once
}

func (p *parkingCloseTransport) Dial(ctx context.Context, wsURL string, maxFrameBytes int64) (eventfeed.CableConn, error) {
	conn, err := p.inner.Dial(ctx, wsURL, maxFrameBytes)
	if err != nil {
		return nil, err
	}
	return &parkingCloseConn{CableConn: conn, tr: p}, nil
}

type parkingCloseConn struct {
	eventfeed.CableConn
	tr *parkingCloseTransport
}

func (c *parkingCloseConn) Close(code int, reason string) error {
	c.tr.once.Do(func() { close(c.tr.parked) })
	<-c.tr.release
	return c.CableConn.Close(code, reason)
}

// TestCallerCancellationOutranksATerminalMidTeardown is the caller-cancel
// sibling of TestCloseFromConnectedOutranksAQueuedFatalFrame. §23's universal
// edge reads "close() / cancellation / consumer break", and Events promises
// all three end iteration with NO error element — but the terminal claim
// serializes publication against Close alone, so a caller cancelling ctx
// while a subscription-rejection teardown was still closing the socket was
// handed the subscription_rejected element anyway. The parked Close makes
// the interleaving deterministic rather than a repeated-rounds race: the
// terminal outcome is already selected, the teardown is mid-close, and the
// caller cancels strictly before publication.
func TestCallerCancellationOutranksATerminalMidTeardown(t *testing.T) {
	pt := &parkingCloseTransport{parked: make(chan struct{}), release: make(chan struct{})}
	h := newHarness(t, eventfeed.WithTransport(pt))
	pt.inner = h.tr
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(pt.release) }) }
	// A parked teardown must never outlive the test, however it ended.
	t.Cleanup(release)
	h.minter.ScriptTicket(ticket(1))
	h.start()

	h.driveToSubscribed().Serve(frameReject(noFilterIdentifier))
	select {
	case <-pt.parked:
	case <-time.After(watchdog):
		t.Fatal("the reject teardown never reached the socket close")
	}
	h.cancel()
	release()
	h.join()

	if _, terminal, elements := h.snapshot(); terminal != nil || elements != 0 {
		t.Fatalf("cancellation must end iteration with no error element; got %d elements, terminal %v",
			elements, terminal)
	}
	if sock := h.tr.LastConn(); sock == nil || !sock.Closed() {
		t.Fatal("the rejected socket must still be closed")
	}
	assertTimers(t, h.clock, map[string]int{})
}

// ctxAwareStore is a CheckpointStore that HONORS its context, which the
// interface permits and this package's own tests never exercised: both
// feedtest.Store and the built-in FileCheckpointStore ignore ctx, so a save
// handed a cancelled context still landed and every assertion passed.
type ctxAwareStore struct {
	mu        sync.Mutex
	saves     []string
	valueSeen []any
}

func (s *ctxAwareStore) Load(context.Context, eventfeed.CheckpointKey) (string, bool, error) {
	return "pos-0", true, nil
}

func (s *ctxAwareStore) Save(ctx context.Context, _ eventfeed.CheckpointKey, position string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saves = append(s.saves, position)
	// WithoutCancel drops cancellation and KEEPS values, and the difference
	// matters to real stores: a trace span, a tenant, or a request id rides on
	// the context, and swapping in context.Background() to dodge cancellation
	// would silently strip all of it. Recorded here so the distinction is
	// asserted rather than assumed.
	s.valueSeen = append(s.valueSeen, ctx.Value(saveProbeKey{}))
	return nil
}

// saveProbeKey types the context value the run carries into the store.
type saveProbeKey struct{}

func (s *ctxAwareStore) recorded() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.saves...)
}

func (s *ctxAwareStore) values() []any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]any(nil), s.valueSeen...)
}

// TestAcceptedPositionSavesAgainstAContextHonoringStore is what
// TestAnAcceptedPositionSavesEvenWhenCloseLandsFirst could not see.
//
// Deleting the durable gate was supposed to make an accepted page's position
// durable even when Close lands at the save boundary — its events were already
// delivered, so dropping the write silently re-delivers them. But the save was
// still handed l.runCtx, which Close cancels synchronously. A store that
// ignores ctx (both of the ones this package ships) writes anyway; a store that
// HONORS it returns ctx.Err() and the position is lost — the exact outcome the
// gate was deleted to prevent, reached by a different route and invisible to
// every existing test.
//
// CheckpointStore.Save takes a context and nothing in its contract says to
// ignore it, so honoring it is compliant. The save therefore runs under a
// context detached from the run's cancellation.
func TestAcceptedPositionSavesAgainstAContextHonoringStore(t *testing.T) {
	store := &ctxAwareStore{}
	var h *harness
	h = newHarness(t,
		eventfeed.WithCheckpointStore(store),
		eventfeed.WithConsumerNamespace("agent"),
		eventfeed.WithObserver(eventfeed.Observer{
			PageDelivered: func(int, string) {
				if err := h.conn.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
			},
		}))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{
		Events:   []eventfeed.Event{pollEvent(101)},
		Position: "pos-1",
	})
	h.startCtx(context.WithValue(context.Background(), saveProbeKey{}, "probe-value"))
	sock := h.driveToSubscribed()
	sock.Serve(frameConfirm(noFilterIdentifier))
	h.join()

	if got := store.recorded(); len(got) != 1 || got[0] != "pos-1" {
		t.Errorf("recorded saves = %v, want [pos-1] — event 101 was delivered, so its position must be durable "+
			"even against a store that honors the cancelled run context", got)
	}
	// Detaching must drop CANCELLATION only. context.Background() would also
	// pass the assertion above while stripping every value the caller put on
	// the context, which a store using them would notice and this test would
	// not.
	if got := store.values(); len(got) != 1 || got[0] != "probe-value" {
		t.Errorf("context values seen by the store = %v, want [probe-value] — the save's context must keep "+
			"the run's values and drop only its cancellation", got)
	}
}
