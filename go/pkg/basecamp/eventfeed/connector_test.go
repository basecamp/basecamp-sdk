// Construction-time validation and consumer-surface tests for the connector
// (SPEC.md §23 "Consumer Surface"): usage-coded construction errors with
// zero wire attempts, no I/O in New, single-shot Events, idempotent Close.
package eventfeed_test

import (
	"context"
	"errors"
	"strings"
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
