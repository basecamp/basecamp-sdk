// Construction-time validation and consumer-surface tests for the connector
// (SPEC.md §23 "Consumer Surface"): usage-coded construction errors with
// zero wire attempts, no I/O in New, single-shot Events, idempotent Close.
package eventfeed_test

import (
	"context"
	"errors"
	"strings"
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
		accountID string
		minter    eventfeed.TicketMinter
		polls     eventfeed.PollSource
		opts      []eventfeed.Option
		wantMsg   string
	}{
		{"empty account id", "", minter, polls, nil, "accountID"},
		{"nil minter", "1", nil, polls, nil, "TicketMinter"},
		{"nil poll source", "1", minter, nil, nil, "PollSource"},
		{"filter type with whitespace", "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithFilters(eventfeed.Filters{Types: []string{"a b"}})},
			"whitespace"},
		{"filter list over the cap", "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithFilters(eventfeed.Filters{Buckets: longIDs})},
			"at most 100"},
		{"non-positive filter id", "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithFilters(eventfeed.Filters{Creators: []int64{0}})},
			"positive"},
		{"zero dedupe capacity", "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithDedupeCapacity(0)},
			"dedupe capacity"},
		{"negative live buffer capacity", "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithLiveBufferCapacity(-1)},
			"live buffer capacity"},
		{"non-positive confirmation deadline", "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithConfirmationDeadline(0)},
			"confirmation deadline"},
		{"non-positive repair interval", "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithRepairInterval(-time.Second)},
			"repair interval"},
		{"store without a consumer namespace", "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithCheckpointStore(nopStore{})},
			"consumer namespace"},
		{"start after a non-positive id", "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithStart(eventfeed.StartAfter(0))},
			"positive event id"},
		{"start at an empty position", "1", minter, polls,
			[]eventfeed.Option{eventfeed.WithStart(eventfeed.StartAtPosition(""))},
			"non-empty position"},
		{"explicit position with a store", "1", minter, polls,
			[]eventfeed.Option{
				eventfeed.WithStart(eventfeed.StartAtPosition("pos-1")),
				eventfeed.WithCheckpointStore(nopStore{}),
				eventfeed.WithConsumerNamespace("agent"),
			},
			"mutually exclusive"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := eventfeed.New(tc.accountID, tc.minter, tc.polls, tc.opts...)
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

	if _, err := eventfeed.New("1", minter, polls); err != nil {
		t.Fatalf("minimal New: %v", err)
	}
	_, err := eventfeed.New("1", minter, polls,
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

// TestNewDoesNoIO: construction is validation only — the first wire attempt
// happens on the first iteration.
func TestNewDoesNoIO(t *testing.T) {
	minter := feedtest.NewMinter()
	tr := feedtest.NewTransport()
	if _, err := eventfeed.New("1", minter, feedtest.NewPolls(),
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
