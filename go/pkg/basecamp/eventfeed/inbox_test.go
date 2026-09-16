package eventfeed_test

import (
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed/feedtest"
)

// The inbox lane (SPEC.md §23 "The Inbox Lane"): the same protocol over the
// principal's addressed items, with the addressing id as the lane's identity.

// inboxItem builds one inbox row: the poll-shaped event under an Addressing.
func inboxItem(addressingID, eventID int64, reason string) eventfeed.Event {
	ev := pollEvent(eventID)
	ev.Addressing = &eventfeed.Addressing{
		ID:          addressingID,
		Reason:      reason,
		AddressedAt: time.Date(2026, 8, 1, 12, 0, 1, 0, time.UTC),
	}
	return ev
}

// frameInboxItem builds a correlated broadcast carrying an inbox item in the
// documented poll shape (no transport-only push keys).
func frameInboxItem(identifier string, addressingID, eventID int64, reason string) []byte {
	quoted, err := json.Marshal(identifier)
	if err != nil {
		panic(err)
	}
	return []byte(`{"identifier":` + string(quoted) + `,"message":{"addressing_id":` + strconv.FormatInt(addressingID, 10) +
		`,"reason":"` + reason + `","addressed_at":"2026-08-01T12:00:01Z","event":{"id":` + strconv.FormatInt(eventID, 10) +
		`,"kind":"comment_created","event_type":"comment.created","action":"created","created_at":"2026-08-01T12:00:00Z","bucket_id":2,"creator_id":3,"performed_by_id":null,"recording_id":900}}}`)
}

func TestNewValidatesTheLaneDimensions(t *testing.T) {
	cases := []struct {
		name string
		opts []eventfeed.Option
	}{
		{"reasons on the account lane", []eventfeed.Option{eventfeed.WithFilters(eventfeed.Filters{Reasons: []string{"mentioned"}})}},
		{"creators on the inbox lane", []eventfeed.Option{eventfeed.WithLane(eventfeed.InboxLane), eventfeed.WithFilters(eventfeed.Filters{Creators: []int64{1}})}},
		{"performers on the inbox lane", []eventfeed.Option{eventfeed.WithLane(eventfeed.InboxLane), eventfeed.WithFilters(eventfeed.Filters{Performers: []int64{1}})}},
		{"exclude_performers on the inbox lane", []eventfeed.Option{eventfeed.WithLane(eventfeed.InboxLane), eventfeed.WithFilters(eventfeed.Filters{ExcludePerformers: []int64{1}})}},
		{"actor_types on the inbox lane", []eventfeed.Option{eventfeed.WithLane(eventfeed.InboxLane), eventfeed.WithFilters(eventfeed.Filters{ActorTypes: []string{eventfeed.ActorTypeAgent}})}},
		{"unknown lane", []eventfeed.Option{eventfeed.WithLane(eventfeed.Lane(7))}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := eventfeed.New(testOrigin, "1", feedtest.NewMinter(), feedtest.NewPolls(), tc.opts...)
			var te *eventfeed.TerminalError
			if !errors.As(err, &te) || te.Reason != eventfeed.ReasonUsage {
				t.Fatalf("New error = %v, want *TerminalError with reason %q", err, eventfeed.ReasonUsage)
			}
		})
	}
	// The inbox's own three dimensions construct.
	if _, err := eventfeed.New(testOrigin, "1", feedtest.NewMinter(), feedtest.NewPolls(),
		eventfeed.WithLane(eventfeed.InboxLane),
		eventfeed.WithFilters(eventfeed.Filters{Reasons: []string{"mentioned"}, Types: []string{"comment.created"}, Buckets: []int64{7}})); err != nil {
		t.Fatalf("New(inbox filters) = %v, want nil", err)
	}
}

func TestCheckpointKeyCarriesTheLane(t *testing.T) {
	base := eventfeed.CheckpointKey{Origin: "https://3.basecampapi.com", AccountID: "5951425", ConsumerNamespace: "openclaw", FilterKey: "srv2-44136fa355b3678a"}
	if got, want := base.FlatKey(), `["https://3.basecampapi.com","5951425","openclaw","srv2-44136fa355b3678a"]`; got != want {
		t.Errorf("account flat key = %s, want %s", got, want)
	}
	inbox := base
	inbox.Lane = "inbox"
	if got, want := inbox.FlatKey(), `["https://3.basecampapi.com","5951425","openclaw","srv2-44136fa355b3678a","inbox"]`; got != want {
		t.Errorf("inbox flat key = %s, want %s", got, want)
	}
}

// TestInboxLaneSubscribesPollsAndDedupesByAddressingID drives the lane end to
// end: the subscribe identifier carries `inbox:true` and the reasons; the
// store lineage carries the lane; the entry walk delivers items; a repeated
// EVENT id under a second addressing id is a second delivery, while a
// repeated addressing id is suppressed; and the live item that a poll then
// re-serves is delivered exactly once.
func TestInboxLaneSubscribesPollsAndDedupesByAddressingID(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	filters := eventfeed.Filters{Reasons: []string{"mentioned", "assigned"}}
	h := storedHarness(t, store,
		eventfeed.WithLane(eventfeed.InboxLane),
		eventfeed.WithFilters(filters))
	h.minter.ScriptTicket(ticket(1))
	// Two items address the principal through ONE event (id 100): both
	// deliver. The third repeats addressing id 11: suppressed.
	h.polls.ScriptPage(eventfeed.PollPage{
		Events:   []eventfeed.Event{inboxItem(11, 100, "mentioned"), inboxItem(12, 100, "assigned"), inboxItem(11, 100, "mentioned")},
		Position: "pos-1",
	})
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{inboxItem(13, 101, "mentioned")}, Position: "pos-2"})
	h.start()

	identifier := eventfeed.ExportInboxSubscribeIdentifier(filters)
	conn := h.driveToSubscribed()
	if writes := conn.Writes(); len(writes) == 0 || string(writes[0]) != string(eventfeed.ExportInboxSubscribeFrame(filters)) {
		t.Fatalf("subscribe writes = %q, want the inbox identifier's frame first", writes)
	}
	// A live item for addressing id 13 arrives before confirmation: buffered,
	// delivered on the drain, and then re-served by the repair poll.
	h.serveSettled(conn, frameInboxItem(identifier, 13, 101, "mentioned"))
	conn.Serve(frameConfirm(identifier))
	h.awaitStreaming()

	if loads := store.Loads(); len(loads) != 1 || loads[0].Lane != "inbox" {
		t.Fatalf("store loads = %+v, want one load keyed to the inbox lane", loads)
	}
	events, _, _ := h.snapshot()
	keys := make([]int64, 0, len(events))
	for _, ev := range events {
		if ev.Addressing == nil {
			t.Fatalf("delivered event %d carries no Addressing on the inbox lane", ev.ID)
		}
		keys = append(keys, ev.Key())
	}
	assertIDs(t, keys, 11, 12, 13)
	assertLedger(t, h.ledger(), []string{"event 100", "event 100", "save pos-1", "event 101"})
	assertPositions(t, store.Saves(), "pos-1")

	h.fireTimer(timerRepairPoll)
	h.awaitStreaming()
	events, _, _ = h.snapshot()
	if len(events) != 3 {
		t.Fatalf("delivered = %d events after the repair poll re-served item 13, want 3 (suppressed by addressing id)", len(events))
	}
	assertPositions(t, store.Saves(), "pos-1", "pos-2")
}

// TestInboxLaneResetCursorIsTheItemID: a 400-position re-entry on the inbox
// lane re-enters at since=<last poll-served ITEM id>, never the event id —
// `since` walks item ids there.
func TestInboxLaneResetCursorIsTheItemID(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store, eventfeed.WithLane(eventfeed.InboxLane))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptPage(eventfeed.PollPage{Events: []eventfeed.Event{inboxItem(42, 9000, "boosted")}, Position: "pos-1", Next: testOrigin + "/999/inbox.json?position=pos-1"})
	h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollPositionInvalid})
	h.polls.ScriptPage(eventfeed.PollPage{Position: "pos-2"})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(eventfeed.ExportInboxSubscribeIdentifier(eventfeed.Filters{})))
	h.awaitStreaming()

	calls := h.polls.Calls()
	if len(calls) != 3 {
		t.Fatalf("poll seam calls = %d, want 3", len(calls))
	}
	if calls[2].Cursor != (eventfeed.Cursor{Since: "42"}) {
		t.Fatalf("re-entry cursor = %+v, want since=42 (the item id, not event id 9000)", calls[2].Cursor)
	}
}

// TestInboxLaneGapNamesRetention: the inbox's 410 is the retention window,
// and the default-terminal message says so.
func TestInboxLaneGapNamesRetention(t *testing.T) {
	store := feedtest.NewStore()
	store.Stored("pos-0")
	h := storedHarness(t, store, eventfeed.WithLane(eventfeed.InboxLane))
	h.minter.ScriptTicket(ticket(1))
	h.polls.ScriptError(&eventfeed.PollError{Kind: eventfeed.PollGone, ResumeURL: testOrigin + "/999/inbox.json?since=0"})
	h.start()

	conn := h.driveToSubscribed()
	conn.Serve(frameConfirm(eventfeed.ExportInboxSubscribeIdentifier(eventfeed.Filters{})))
	h.join()
	_, terminal, _ := h.snapshot()
	if terminal == nil || terminal.Reason != eventfeed.ReasonFeedGap || terminal.Msg != "the inbox's retained items behind the held position are gone" {
		t.Fatalf("terminal = %v, want feed_gap naming the retention window", terminal)
	}
}
