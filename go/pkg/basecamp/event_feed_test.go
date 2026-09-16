package basecamp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func testEventFeedServer(t *testing.T, handler http.HandlerFunc) *EventFeedService {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token)
	return client.ForAccount("99999").EventFeed()
}

const feedPageBody = `{
  "events": [
    {
      "id": 1071915468,
      "kind": "message_created",
      "action": "created",
      "created_at": "2026-07-14T06:10:00.159Z",
      "event_type": "message.created",
      "bucket_id": 2085958499,
      "creator_id": 1049715945,
      "performed_by_id": null,
      "recording_id": 1069479766
    },
    {
      "id": 1071915470,
      "kind": "boost_created",
      "action": "created",
      "created_at": "2026-07-14T06:11:00.000Z",
      "event_type": "boost.created",
      "bucket_id": 2085958499,
      "creator_id": 1049715946,
      "performed_by_id": 1049715999,
      "recording_id": 1069479766,
      "details": {"boost_id": 501, "boosted_event_id": 1071915468, "boosted_event_type": "message.created"}
    },
    {
      "id": 1071915471,
      "kind": "kanban_card_moved",
      "action": "moved",
      "created_at": "2026-07-14T06:12:00.000Z",
      "event_type": "card.moved",
      "bucket_id": 2085958499,
      "creator_id": 1049715945,
      "performed_by_id": null,
      "recording_id": 1069479800,
      "details": {"column_id": 77, "previous_column_id": 76}
    }
  ],
  "position": "posAAA",
  "next": "https://3.basecampapi.com/99999/events.json?position=posAAA&types=message.created%2Cboost.created&buckets=2085958499"
}`

func TestEventFeedService_PollEvents_DecodesEnvelope(t *testing.T) {
	var gotQuery url.Values
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/99999/events.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(feedPageBody))
	})

	page, err := svc.PollEvents(context.Background(), &PollEventsOptions{
		Since:             SinceEpoch,
		Types:             []string{"message.created", "boost.created"},
		Buckets:           []int64{2085958499, 12},
		ExcludePerformers: []string{"self"},
		ActorTypes:        []string{"agent", "person"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery.Get("since") != "0" {
		t.Errorf("expected since=0, got %q", gotQuery.Get("since"))
	}
	if gotQuery.Get("types") != "message.created,boost.created" {
		t.Errorf("expected comma-joined types, got %q", gotQuery.Get("types"))
	}
	if gotQuery.Get("buckets") != "2085958499,12" {
		t.Errorf("expected comma-joined buckets, got %q", gotQuery.Get("buckets"))
	}
	if gotQuery.Get("exclude_performers") != "self" {
		t.Errorf("expected exclude_performers=self, got %q", gotQuery.Get("exclude_performers"))
	}
	if gotQuery.Get("actor_types") != "agent,person" {
		t.Errorf("expected actor_types, got %q", gotQuery.Get("actor_types"))
	}
	if _, present := gotQuery["position"]; present {
		t.Error("position must not be sent when unset")
	}

	if page.Position != "posAAA" {
		t.Errorf("expected position posAAA, got %q", page.Position)
	}
	if page.Next == "" {
		t.Fatal("expected next continuation URL")
	}
	if len(page.Events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(page.Events))
	}
	first := page.Events[0]
	if first.ID != 1071915468 || first.EventType != "message.created" || first.RecordingID != 1069479766 {
		t.Errorf("unexpected first event: %+v", first)
	}
	if first.PerformedByID != nil {
		t.Errorf("expected nil PerformedByID for a null wire value, got %d", *first.PerformedByID)
	}
	if first.Details != nil {
		t.Error("expected no details on a message.created event")
	}
	if first.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to decode")
	}
	boost := page.Events[1]
	if boost.PerformedByID == nil || *boost.PerformedByID != 1049715999 {
		t.Errorf("expected PerformedByID 1049715999, got %v", boost.PerformedByID)
	}
	if boost.Details == nil || boost.Details.BoostID == nil || *boost.Details.BoostID != 501 {
		t.Fatalf("expected boost details, got %+v", boost.Details)
	}
	if boost.Details.BoostedEventType == nil || *boost.Details.BoostedEventType != "message.created" {
		t.Errorf("expected boosted_event_type, got %v", boost.Details.BoostedEventType)
	}
	if boost.Details.ColumnID != nil {
		t.Error("boost details must not carry column ids")
	}
	moved := page.Events[2]
	if moved.Details == nil || moved.Details.ColumnID == nil || *moved.Details.ColumnID != 77 || *moved.Details.PreviousColumnID != 76 {
		t.Errorf("expected card.moved column details, got %+v", moved.Details)
	}
}

func TestEventFeedService_PollEvents_NilOptionsEntersAtPresent(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("expected no query for a bare present entry, got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"events": [], "position": "posNOW"}`))
	})
	page, err := svc.PollEvents(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Position != "posNOW" || page.Next != "" || len(page.Events) != 0 {
		t.Errorf("unexpected page: %+v", page)
	}
}

func TestEventFeedService_PollEvents_FilterMismatch(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		_, _ = w.Write([]byte(`{"error": "Positions are bound to the filter set they were minted for. Acknowledge the filter change by re-entering with since=<id> or since=now.", "position_digest": "38b223c13c89dc89", "filters_digest": "44136fa355b3678a"}`))
	})
	_, err := svc.PollEvents(context.Background(), &PollEventsOptions{Position: "posAAA"})
	var mismatch *FeedFilterMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected *FeedFilterMismatchError, got %T: %v", err, err)
	}
	if mismatch.PositionDigest != "38b223c13c89dc89" || mismatch.FiltersDigest != "44136fa355b3678a" {
		t.Errorf("unexpected digests: %+v", mismatch)
	}
	var base *Error
	if !errors.As(err, &base) || base.HTTPStatus != 409 || base.Retryable {
		t.Errorf("expected a non-retryable 409 *Error underneath, got %+v", base)
	}
}

func TestEventFeedService_PollEvents_PositionGone(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(410)
		_, _ = w.Write([]byte(`{"error": "That position predates this feed's epoch, so the history behind it can't be served.", "epoch_after_id": 1071915000, "resume": "https://3.basecampapi.com/99999/events.json?since=1071915000&types=message.created"}`))
	})
	_, err := svc.PollEvents(context.Background(), &PollEventsOptions{Position: "posOLD", Types: []string{"message.created"}})
	var gone *FeedPositionGoneError
	if !errors.As(err, &gone) {
		t.Fatalf("expected *FeedPositionGoneError, got %T: %v", err, err)
	}
	if gone.EpochAfterID == nil || *gone.EpochAfterID != 1071915000 {
		t.Errorf("expected epoch_after_id, got %v", gone.EpochAfterID)
	}
	resume, err := PollEventsOptionsFromURL(gone.Resume)
	if err != nil {
		t.Fatalf("resume URL did not parse: %v", err)
	}
	if resume.Since != "1071915000" || resume.Position != "" || len(resume.Types) != 1 || resume.Types[0] != "message.created" {
		t.Errorf("unexpected resume options: %+v", resume)
	}
}

func TestEventFeedService_PollEvents_MalformedPositionIsPlainError(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error": "Unrecognized position. Resume with since=<id> or since=now."}`))
	})
	_, err := svc.PollEvents(context.Background(), &PollEventsOptions{Position: "garbage"})
	var base *Error
	if !errors.As(err, &base) {
		t.Fatalf("expected *Error, got %T", err)
	}
	if base.HTTPStatus != 400 || base.Code != "validation" {
		t.Errorf("expected validation/400, got %s/%d", base.Code, base.HTTPStatus)
	}
	var mismatch *FeedFilterMismatchError
	var gone *FeedPositionGoneError
	if errors.As(err, &mismatch) || errors.As(err, &gone) {
		t.Error("a 400 must not be typed as a feed 409/410")
	}
}

func TestEventFeedService_PollInbox(t *testing.T) {
	var gotQuery url.Values
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/99999/inbox.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "items": [
    {
      "addressing_id": 991,
      "reason": "mentioned",
      "addressed_at": "2026-07-14T06:10:00.159Z",
      "event": {
        "id": 1071915468,
        "kind": "comment_created",
        "action": "created",
        "created_at": "2026-07-14T06:10:00.159Z",
        "event_type": "comment.created",
        "bucket_id": 2085958499,
        "creator_id": 1049715945,
        "performed_by_id": null,
        "recording_id": 1069479766
      }
    }
  ],
  "position": "inboxPos"
}`))
	})
	page, err := svc.PollInbox(context.Background(), &PollInboxOptions{Since: SinceEpoch, Reasons: []string{"mentioned", "assigned"}, Buckets: []int64{2085958499}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery.Get("reasons") != "mentioned,assigned" || gotQuery.Get("buckets") != "2085958499" || gotQuery.Get("since") != "0" {
		t.Errorf("unexpected query: %v", gotQuery)
	}
	if page.Position != "inboxPos" || page.Next != "" {
		t.Errorf("unexpected page: %+v", page)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page.Items))
	}
	item := page.Items[0]
	if item.AddressingID != 991 || item.Reason != "mentioned" || item.AddressedAt.IsZero() {
		t.Errorf("unexpected item: %+v", item)
	}
	if item.Event.ID != 1071915468 || item.Event.EventType != "comment.created" {
		t.Errorf("unexpected nested event: %+v", item.Event)
	}
}

func TestEventFeedService_PollInbox_ForbiddenForPeople(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	})
	_, err := svc.PollInbox(context.Background(), nil)
	var base *Error
	if !errors.As(err, &base) || base.HTTPStatus != 403 || base.Code != "forbidden" {
		t.Fatalf("expected forbidden/403, got %v", err)
	}
}

func TestEventFeedService_PollInbox_RetentionGoneHasNoEpoch(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(410)
		_, _ = w.Write([]byte(`{"error": "That position predates the inbox's retention window, so the items behind it can't be served.", "resume": "https://3.basecampapi.com/99999/inbox.json?since=0&reasons=mentioned"}`))
	})
	_, err := svc.PollInbox(context.Background(), &PollInboxOptions{Position: "stale"})
	var gone *FeedPositionGoneError
	if !errors.As(err, &gone) {
		t.Fatalf("expected *FeedPositionGoneError, got %T: %v", err, err)
	}
	if gone.EpochAfterID != nil {
		t.Errorf("inbox 410 must not carry an epoch, got %d", *gone.EpochAfterID)
	}
	resume, err := PollInboxOptionsFromURL(gone.Resume)
	if err != nil {
		t.Fatalf("resume URL did not parse: %v", err)
	}
	if resume.Since != "0" || len(resume.Reasons) != 1 || resume.Reasons[0] != "mentioned" {
		t.Errorf("unexpected resume options: %+v", resume)
	}
}

func TestEventFeedService_CreateStreamTicket(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/99999/events/stream_ticket.json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.ContentLength > 0 {
			t.Errorf("expected no request body, got %d bytes", r.ContentLength)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ticket": "tkt-opaque", "expires_in": 120, "url": "wss://example.invalid/99999?ticket=tkt-opaque"}`))
	})
	ticket, err := svc.CreateStreamTicket(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ticket.Ticket != "tkt-opaque" || ticket.ExpiresIn != 120 || ticket.URL != "wss://example.invalid/99999?ticket=tkt-opaque" {
		t.Errorf("unexpected ticket: %+v", ticket)
	}
}

func TestEventFeedService_CreateStreamTicket_Unauthorized(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error": "Unauthorized"}`))
	})
	_, err := svc.CreateStreamTicket(context.Background())
	var base *Error
	if !errors.As(err, &base) || base.HTTPStatus != 401 {
		t.Fatalf("expected a 401 *Error, got %v", err)
	}
}

func TestPollEventsOptionsFromURL(t *testing.T) {
	opts, err := PollEventsOptionsFromURL("https://3.basecampapi.com/99999/events.json?position=posAAA&types=message.created%2Cboost.created&buckets=1%2C2&creators=9&performers=self&exclude_performers=7%2C8&actor_types=agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Position != "posAAA" || opts.Since != "" {
		t.Errorf("unexpected entry: %+v", opts)
	}
	if len(opts.Types) != 2 || len(opts.Buckets) != 2 || opts.Buckets[1] != 2 || len(opts.Creators) != 1 || opts.Creators[0] != 9 {
		t.Errorf("unexpected filters: %+v", opts)
	}
	if len(opts.Performers) != 1 || opts.Performers[0] != "self" || len(opts.ExcludePerformers) != 2 || len(opts.ActorTypes) != 1 {
		t.Errorf("unexpected performer filters: %+v", opts)
	}

	for _, bad := range []string{"", "/99999/events.json?since=now", "https://3.basecampapi.com/99999/events.json?buckets=abc"} {
		if _, err := PollEventsOptionsFromURL(bad); err == nil {
			t.Errorf("expected an error for %q", bad)
		}
	}
}

func TestEventFeedService_PollEvents_ReportsOperation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"events": [], "position": "p"}`))
	}))
	t.Cleanup(server.Close)

	hooks := &recordingHooks{}
	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}, WithHooks(hooks))
	if _, err := client.ForAccount("99999").EventFeed().PollEvents(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hooks.opStartCalls) != 1 {
		t.Fatalf("expected one operation, got %d", len(hooks.opStartCalls))
	}
	op := hooks.opStartCalls[0]
	if op.Service != "EventFeed" || op.Operation != "PollEvents" || op.ResourceType != "feed_event" || op.IsMutation {
		t.Errorf("unexpected operation info: %+v", op)
	}
}
