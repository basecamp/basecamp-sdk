package basecamp

import (
	"context"
	"encoding/json"
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
    },
    {
      "id": 1071915472,
      "kind": "boost_created",
      "action": "created",
      "created_at": "2026-07-14T06:13:00.000Z",
      "event_type": "boost.created",
      "bucket_id": 2085958499,
      "creator_id": 1049715947,
      "performed_by_id": null,
      "recording_id": 1069479766,
      "details": {"boost_id": 502, "boosted_event_id": null, "boosted_event_type": null}
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
	if len(page.Events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(page.Events))
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
	// Details are the server's bytes, verbatim — keys, order and explicit
	// nulls included — never a typed projection.
	if string(boost.Details) != `{"boost_id": 501, "boosted_event_id": 1071915468, "boosted_event_type": "message.created"}` {
		t.Errorf("expected verbatim boost details, got %s", boost.Details)
	}
	var boostDetails struct {
		BoostID          *int64  `json:"boost_id"`
		BoostedEventType *string `json:"boosted_event_type"`
	}
	if err := json.Unmarshal(boost.Details, &boostDetails); err != nil || boostDetails.BoostID == nil || *boostDetails.BoostID != 501 || boostDetails.BoostedEventType == nil {
		t.Errorf("expected boost details to decode, got %+v (%v)", boostDetails, err)
	}
	moved := page.Events[2]
	if string(moved.Details) != `{"column_id": 77, "previous_column_id": 76}` {
		t.Errorf("expected verbatim card.moved details, got %s", moved.Details)
	}
	recordingBoost := page.Events[3]
	if string(recordingBoost.Details) != `{"boost_id": 502, "boosted_event_id": null, "boosted_event_type": null}` {
		t.Errorf("expected the explicit nulls to survive verbatim, got %s", recordingBoost.Details)
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
	if gone.EpochAfterID != 1071915000 {
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

func TestEventFeedService_PollEvents_GoneWithoutEpochStaysCanonical(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(410)
		_, _ = w.Write([]byte(`{"error": "That position predates this feed's epoch.", "resume": "https://3.basecampapi.com/99999/events.json?since=0"}`))
	})
	_, err := svc.PollEvents(context.Background(), &PollEventsOptions{Position: "posOLD"})
	var gone *FeedPositionGoneError
	if errors.As(err, &gone) {
		t.Fatalf("a feed 410 without epoch_after_id must not be typed with a fabricated epoch, got %+v", gone)
	}
	var base *Error
	if !errors.As(err, &base) || base.HTTPStatus != 410 {
		t.Fatalf("expected the canonical 410 *Error, got %v", err)
	}
}

func TestEventFeedService_PollEvents_MalformedPositionWithoutReasonIsUndifferentiated(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error": "Unrecognized position. Resume with since=<id> or since=now."}`))
	})
	_, err := svc.PollEvents(context.Background(), &PollEventsOptions{Position: "garbage"})
	var request *FeedRequestError
	if !errors.As(err, &request) {
		t.Fatalf("expected *FeedRequestError, got %T: %v", err, err)
	}
	if request.Reason != "" {
		t.Errorf("a body without reason must leave Reason empty, got %q", request.Reason)
	}
	var base *Error
	if !errors.As(err, &base) || base.HTTPStatus != 400 || base.Code != "validation" {
		t.Errorf("expected validation/400 underneath, got %+v", base)
	}
	var mismatch *FeedFilterMismatchError
	var gone *FeedPositionGoneError
	if errors.As(err, &mismatch) || errors.As(err, &gone) {
		t.Error("a 400 must not be typed as a feed 409/410")
	}
}

func TestEventFeedService_PollEvents_BadRequestCarriesReason(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error": "The types filter names an unknown type. Fix the filters; a position reset won't help.", "reason": "invalid_filter"}`))
	})
	_, err := svc.PollEvents(context.Background(), &PollEventsOptions{Types: []string{"nope.created"}})
	var request *FeedRequestError
	if !errors.As(err, &request) {
		t.Fatalf("expected *FeedRequestError, got %T: %v", err, err)
	}
	if request.Reason != FeedReasonInvalidFilter {
		t.Errorf("expected reason invalid_filter, got %q", request.Reason)
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
	var gone *InboxPositionGoneError
	if !errors.As(err, &gone) {
		t.Fatalf("expected *InboxPositionGoneError, got %T: %v", err, err)
	}
	var feedGone *FeedPositionGoneError
	if errors.As(err, &feedGone) {
		t.Error("an inbox 410 must never type as the feed's FeedPositionGoneError")
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

	for _, bad := range []string{
		"",
		"/99999/events.json?since=now",
		"https://3.basecampapi.com/99999/events.json?buckets=abc",
		// A malformed pair must be an error, not a silently dropped position.
		"https://3.basecampapi.com/99999/events.json?position=%ZZ&types=message.created",
	} {
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

// The decode is where the poll lanes' bodies are held to their documented
// shape (#915). A malformed one must not arrive as the typed value a consumer
// recovers on: the 409's digests discard a held position and the 400's reason
// keys recover-versus-stop, so each refusal is asserted to reach neither the
// typed error nor a status a fallback can key off.
func TestEventFeedService_PollEvents_ConflictWithoutTwoSrv2DigestsIsNotAFilterChange(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"a digest outside the hex alphabet", `{"error":"conflict","position_digest":"38b223c13c89dc89","filters_digest":"x"}`},
		{"a digest of the wrong length", `{"error":"conflict","position_digest":"38b223c13c89dc89","filters_digest":"44136fa355b3678a0"}`},
		{"an uppercase digest", `{"error":"conflict","position_digest":"38B223C13C89DC89","filters_digest":"44136fa355b3678a"}`},
		{"a missing digest", `{"error":"conflict","position_digest":"38b223c13c89dc89"}`},
		{"a null digest", `{"error":"conflict","position_digest":"38b223c13c89dc89","filters_digest":null}`},
		{"no digests at all", `{"error":"conflict"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(409)
				_, _ = w.Write([]byte(tc.body))
			})
			_, err := svc.PollEvents(context.Background(), &PollEventsOptions{Position: "posAAA"})
			var mismatch *FeedFilterMismatchError
			if errors.As(err, &mismatch) {
				t.Fatalf("a 409 without two srv2 digests must not be typed as a filter change, got %+v", mismatch)
			}
			assertMalformedFeedResponse(t, err)
		})
	}
}

func TestEventFeedService_PollEvents_BadRequestWithAnUnnamedReasonIsNotARequestError(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		// The message is the one bc3 sends for a malformed position on
		// purpose: it is what a consumer's legacy classifier matches, and the
		// refusal has to hold with the classifier's own bait in the body.
		{"a reason the contract does not name", `{"error":"Unrecognized position. Resume with since=<id>.","reason":"invalid_something"}`},
		{"a present but empty reason", `{"error":"Unrecognized position. Resume with since=<id>.","reason":""}`},
		{"a reason in the wrong case", `{"error":"Unrecognized position. Resume with since=<id>.","reason":"INVALID_POSITION"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(400)
				_, _ = w.Write([]byte(tc.body))
			})
			_, err := svc.PollEvents(context.Background(), &PollEventsOptions{Position: "posAAA"})
			var request *FeedRequestError
			if errors.As(err, &request) {
				t.Fatalf("a 400 whose reason the contract does not name must not be typed as a request error, got %+v", request)
			}
			assertMalformedFeedResponse(t, err)
		})
	}
}

func TestEventFeedService_PollInbox_BadRequestWithAnUnnamedReasonIsNotARequestError(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"Unrecognized position. Resume with since=<id>.","reason":"invalid_something"}`))
	})
	_, err := svc.PollInbox(context.Background(), &PollInboxOptions{Position: "posAAA"})
	var request *FeedRequestError
	if errors.As(err, &request) {
		t.Fatalf("the inbox lane shares the feed's decode, got %+v", request)
	}
	assertMalformedFeedResponse(t, err)
}

// The bad byte is in a row rather than in the position on purpose: a cursor
// the decoder substituted is caught by the cursor half of checkPollBody, so a
// case that puts it there would pass with the body scan deleted. Here the
// position round-trips and only the whole-body scan can see the mutation —
// which is the claim, since an event_type the decoder rewrote is one a
// consumer dispatches on while the page's position commits over it.
func TestEventFeedService_PollEvents_RefusesAPageWhoseRowsAreNotValidUTF8(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		head := []byte(`{"events":[{"id":1,"kind":"message_created","action":"created","created_at":"2026-07-14T06:10:00Z","event_type":"message.`)
		tail := []byte(`created","bucket_id":2,"creator_id":3,"performed_by_id":null,"recording_id":9}],"position":"posAAA"}`)
		_, _ = w.Write(append(head, append([]byte{0xff}, tail...)...))
	})
	page, err := svc.PollEvents(context.Background(), nil)
	if page != nil {
		t.Fatalf("a page decoded from bytes that are not UTF-8 must not reach the caller, got %+v", page)
	}
	assertMalformedFeedResponse(t, err)
}

func TestEventFeedService_PollEvents_RefusesAPositionThatDidNotSurviveDecoding(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// A lone surrogate escape: well-formed ASCII on the wire, which the
		// decoder turns into a position the server never issued.
		_, _ = w.Write([]byte(`{"events":[],"position":"pos\ud800AAA"}`))
	})
	page, err := svc.PollEvents(context.Background(), nil)
	if page != nil {
		t.Fatalf("a substituted position must not reach the caller, got %+v", page)
	}
	assertMalformedFeedResponse(t, err)
}

func TestEventFeedService_PollEvents_RefusesAContinuationThatDidNotSurviveDecoding(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"events":[],"position":"posAAA","next":"https://3.basecampapi.com/99999/events.json?position=pos\udfffAAA"}`))
	})
	page, err := svc.PollEvents(context.Background(), nil)
	if page != nil {
		t.Fatalf("a substituted continuation must not reach the caller, got %+v", page)
	}
	assertMalformedFeedResponse(t, err)
}

func TestEventFeedService_PollInbox_RefusesAPositionThatDidNotSurviveDecoding(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"position":"pos\ud800AAA"}`))
	})
	page, err := svc.PollInbox(context.Background(), nil)
	if page != nil {
		t.Fatalf("the inbox lane shares the feed's body check, got %+v", page)
	}
	assertMalformedFeedResponse(t, err)
}

func TestEventFeedService_PollEvents_KeepsAPageWhoseCursorsSurvived(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Non-ASCII that is not a substitution: valid UTF-8 in the body and a
		// cursor that round-trips, so the guard must let the page through.
		_, _ = w.Write([]byte(`{"events":[],"position":"posé","next":"https://3.basecampapi.com/99999/events.json?position=pos%C3%A9"}`))
	})
	page, err := svc.PollEvents(context.Background(), nil)
	if err != nil {
		t.Fatalf("a well-formed page must not be refused: %v", err)
	}
	if page.Position != "posé" {
		t.Errorf("position = %q, want it carried through verbatim", page.Position)
	}
}

// assertMalformedFeedResponse holds a refusal to the shape SPEC §6 gives
// every malformed body: api_error, non-retryable, and statusless. The status
// is what a consumer's own fallbacks key off — the connector's 400 message
// classifier among them — so carrying one here would hand the refused body to
// the recovery it was refused for.
func assertMalformedFeedResponse(t *testing.T, err error) {
	t.Helper()
	var base *Error
	if !errors.As(err, &base) {
		t.Fatalf("expected the canonical *Error, got %T: %v", err, err)
	}
	if base.Code != CodeAPI {
		t.Errorf("code = %q, want %q", base.Code, CodeAPI)
	}
	if base.HTTPStatus != 0 {
		t.Errorf("httpStatus = %d, want 0: a malformed body is not a verdict any status describes", base.HTTPStatus)
	}
	if base.Retryable {
		t.Error("a malformed body is not retryable: the same request draws the same body")
	}
}

func TestEventFeedService_PollEvents_RefusesAResumeThatDidNotSurviveDecoding(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(410)
		// The substitution lands in a preserved filter, which the resume's
		// own fence check cannot see: re-entering on it walks the epoch
		// under filters the position was never minted for.
		_, _ = w.Write([]byte(`{"error":"gone","epoch_after_id":1071915000,"resume":"https://3.basecampapi.com/99999/events.json?since=1071915000&types=message.cr\ud800eated"}`))
	})
	_, err := svc.PollEvents(context.Background(), &PollEventsOptions{Position: "posOLD"})
	var gone *FeedPositionGoneError
	if errors.As(err, &gone) {
		t.Fatalf("a substituted resume URL must not be typed as a gap to re-enter on, got %+v", gone)
	}
	assertMalformedFeedResponse(t, err)
}

func TestEventFeedService_PollInbox_RefusesAResumeThatDidNotSurviveDecoding(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(410)
		_, _ = w.Write([]byte(`{"error":"gone","resume":"https://3.basecampapi.com/99999/inbox.json?since=0&reasons=ment\ud800ioned"}`))
	})
	_, err := svc.PollInbox(context.Background(), &PollInboxOptions{Position: "posOLD"})
	var gone *InboxPositionGoneError
	if errors.As(err, &gone) {
		t.Fatalf("the inbox's 410 shares the feed's decode, got %+v", gone)
	}
	assertMalformedFeedResponse(t, err)
}
