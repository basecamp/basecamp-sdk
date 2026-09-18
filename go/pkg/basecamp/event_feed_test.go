package basecamp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
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

// A feed 410 without its epoch is not typed with a fabricated 0 boundary —
// and it is not handed back as the canonical 410 either. The epoch is
// @required, so a 410 without one is not the documented gap: the arm decides,
// as every arm does. The connector's verdict is unchanged (unrecoverable
// either way); what changes is that a caller cannot key recovery off a 410
// status the body does not support.
func TestEventFeedService_PollEvents_GoneWithoutEpochIsMalformed(t *testing.T) {
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
	assertMalformedFeedResponse(t, err)
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
		// A member of the wrong JSON type fails a typed decode outright. Read
		// one member at a time, that is a fact about the member; read as a
		// struct, it discards the verdict with the body and falls back to the
		// canonical error this refusal exists to displace.
		{"a wrong-typed digest", `{"error":"conflict","position_digest":42,"filters_digest":"44136fa355b3678a"}`},
		{"a digest that is an object", `{"error":"conflict","position_digest":{"hex":"38b223c13c89dc89"},"filters_digest":"44136fa355b3678a"}`},
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
		// The wrong-typed shapes are the ones that reach the classifier by
		// the back door: a typed decode fails on them whole, and the body
		// then arrives as the canonical 400 with the server's own
		// "Unrecognized position" message intact.
		{"a numeric reason", `{"error":"Unrecognized position. Resume with since=<id>.","reason":42}`},
		{"a reason that is an array", `{"error":"Unrecognized position. Resume with since=<id>.","reason":["invalid_position"]}`},
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

// The raw-bytes form of the row mutation, with a clean position: the decoder
// substitutes U+FFFD for the invalid byte in event_type and returns a token
// the server never wrote. Its escape-form sibling is below.
func TestEventFeedService_PollEvents_RefusesARowWhoseBytesAreNotValidUTF8(t *testing.T) {
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

// A null reason is the ABSENT case, not a present value the contract does not
// name: the member is optional, and this is the same reading the feed's 410
// gives a nulled epoch. Undifferentiated is the documented answer for it, so
// the message fallback stays available — refusing it would turn a server's
// explicit "I have no reason" into a malformed response.
func TestEventFeedService_PollEvents_ANullReasonIsTheUndifferentiated400(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"Unrecognized position. Resume with since=<id> or since=now.","reason":null}`))
	})
	_, err := svc.PollEvents(context.Background(), &PollEventsOptions{Position: "garbage"})
	var request *FeedRequestError
	if !errors.As(err, &request) {
		t.Fatalf("expected *FeedRequestError, got %T: %v", err, err)
	}
	if request.Reason != "" {
		t.Errorf("a null reason must read as absent, got %q", request.Reason)
	}
	var base *Error
	if !errors.As(err, &base) || base.HTTPStatus != 400 || base.Code != CodeValidation {
		t.Errorf("expected validation/400 underneath, got %+v", base)
	}
}

// The arms are TOTAL over the body, and this is the test that says so: one
// table of off-contract shapes per status, every one refused, none of them
// enumerated in the code. It is written as a totality claim rather than a
// list of known doors on purpose — the doors are what kept reappearing
// (a null, a number, a missing member, a body that is not an object), and a
// default that refuses is the only thing that closes the ones nobody has
// thought of yet.
//
// Statuses other than these three are untouched: a 500 behind an HTML error
// page is still a retryable 500, because no arm claims it.
func TestEventFeedService_RefusesEveryOffContractErrorBody(t *testing.T) {
	cases := []struct {
		status int
		name   string
		body   string
	}{
		{400, "not an object", `<html><body>Bad request</body></html>`},
		{400, "a top-level null", `null`},
		{400, "a top-level array", `[{"error":"nope"}]`},
		{400, "an empty body", ``},
		{400, "no error member", `{"reason":"invalid_position"}`},
		{400, "a null error member", `{"error":null,"reason":"invalid_position"}`},
		{400, "a wrong-typed error member", `{"error":{"message":"nope"},"reason":"invalid_position"}`},
		// An empty error is not a present one: §6's body parse falls back to
		// a `message` member when `error` is empty, so this would carry a
		// message the declared member never supplied into a consumer's
		// classifier — whose answer is a position reset.
		{400, "an empty error member", `{"error":"","message":"Unrecognized position","reason":null}`},
		// A member NAME the decoder substituted into does not arrive wrong,
		// it arrives missing: `reason` would read as absent and the body as
		// the undifferentiated 400 it is not.
		{400, "a substituted member name", `{"error":"Unrecognized position. Resume with since=<id>.","rea\ud800son":"invalid_something"}`},
		{409, "no error member", `{"position_digest":"38b223c13c89dc89","filters_digest":"44136fa355b3678a"}`},
		{409, "an empty error member", `{"error":"","position_digest":"38b223c13c89dc89","filters_digest":"44136fa355b3678a"}`},
		{409, "a substituted member name", `{"error":"conflict","position_digest":"38b223c13c89dc89","filters\ud800_digest":"44136fa355b3678a"}`},
		{410, "no error member", `{"epoch_after_id":7,"resume":"https://3.basecampapi.com/99999/events.json?since=7"}`},
		{410, "an empty resume", `{"error":"gone","epoch_after_id":7,"resume":""}`},
		{409, "not an object", `<html><body>Conflict</body></html>`},
		{409, "a top-level null", `null`},
		{409, "an empty body", ``},
		{409, "no digests", `{"error":"conflict"}`},
		{409, "a null digest", `{"error":"conflict","position_digest":"38b223c13c89dc89","filters_digest":null}`},
		{410, "not an object", `<html><body>Gone</body></html>`},
		{410, "a top-level null", `null`},
		{410, "no resume", `{"error":"gone","epoch_after_id":7}`},
		{410, "a null resume", `{"error":"gone","epoch_after_id":7,"resume":null}`},
		{410, "a wrong-typed resume", `{"error":"gone","epoch_after_id":7,"resume":42}`},
		{410, "a wrong-typed epoch", `{"error":"gone","epoch_after_id":"7","resume":"https://3.basecampapi.com/99999/events.json?since=7"}`},
		{410, "a null epoch", `{"error":"gone","epoch_after_id":null,"resume":"https://3.basecampapi.com/99999/events.json?since=7"}`},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d %s", tc.status, tc.name), func(t *testing.T) {
			svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			_, err := svc.PollEvents(context.Background(), &PollEventsOptions{Position: "posAAA"})
			var mismatch *FeedFilterMismatchError
			var request *FeedRequestError
			var gone *FeedPositionGoneError
			if errors.As(err, &mismatch) || errors.As(err, &request) || errors.As(err, &gone) {
				t.Fatalf("an off-contract body must not be typed, got %T", err)
			}
			assertMalformedFeedResponse(t, err)
		})
	}
}

// The other half of the same rule, and the reason the totality above is not
// "reject anything unexpected": a member the contract does not declare is not
// a violation of it. The server may grow the body; it may not omit or
// mistype what it promised. (The inbox 410 with a stray epoch_after_id in
// eventfeed/live_test.go is this rule seen from the connector.)
func TestEventFeedService_KeepsABodyThatGrewAMember(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		_, _ = w.Write([]byte(`{"error":"conflict","position_digest":"38b223c13c89dc89","filters_digest":"44136fa355b3678a","digest_scheme":"srv3","retry_after_filters_settle":30}`))
	})
	_, err := svc.PollEvents(context.Background(), &PollEventsOptions{Position: "posAAA"})
	var mismatch *FeedFilterMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("a body that grew a member is not malformed, got %T: %v", err, err)
	}
	if mismatch.PositionDigest != "38b223c13c89dc89" || mismatch.FiltersDigest != "44136fa355b3678a" {
		t.Errorf("unexpected digests: %+v", mismatch)
	}
}

// The escape form of the substitution outside the cursors: the body is
// well-formed ASCII, so the byte scan cannot see it, and the position is
// clean, so the cursor check cannot either. The row's event_type is what a
// consumer dispatches on, and the page's position commits over it — the event
// is not delivered and never comes back.
func TestEventFeedService_PollEvents_RefusesARowTokenThatDidNotSurviveDecoding(t *testing.T) {
	cases := map[string]string{
		"an event_type": `{"events":[{"id":1,"kind":"message_created","action":"created","created_at":"2026-07-14T06:10:00Z","event_type":"message.\ud800created","bucket_id":2,"creator_id":3,"performed_by_id":null,"recording_id":9}],"position":"posAAA"}`,
		"a kind":        `{"events":[{"id":1,"kind":"message_\ud800created","action":"created","created_at":"2026-07-14T06:10:00Z","event_type":"message.created","bucket_id":2,"creator_id":3,"performed_by_id":null,"recording_id":9}],"position":"posAAA"}`,
		"an action":     `{"events":[{"id":1,"kind":"message_created","action":"cre\ud800ated","created_at":"2026-07-14T06:10:00Z","event_type":"message.created","bucket_id":2,"creator_id":3,"performed_by_id":null,"recording_id":9}],"position":"posAAA"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			})
			page, err := svc.PollEvents(context.Background(), nil)
			if page != nil {
				t.Fatalf("a row token the decoder rewrote must not reach the caller, got %+v", page)
			}
			assertMalformedFeedResponse(t, err)
		})
	}
}

func TestEventFeedService_PollInbox_RefusesARowTokenThatDidNotSurviveDecoding(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// The addressing reason is the inbox's own routing token, beside the
		// event's three.
		_, _ = w.Write([]byte(`{"items":[{"addressing_id":991,"reason":"ment\ud800ioned","addressed_at":"2026-07-14T06:10:00Z","event":{"id":1,"kind":"message_created","action":"created","created_at":"2026-07-14T06:10:00Z","event_type":"message.created","bucket_id":2,"creator_id":3,"performed_by_id":null,"recording_id":9}}],"position":"posAAA"}`))
	})
	page, err := svc.PollInbox(context.Background(), nil)
	if page != nil {
		t.Fatalf("a substituted addressing reason must not reach the caller, got %+v", page)
	}
	assertMalformedFeedResponse(t, err)
}

// Details is not among the row strings, and that is a decision rather than an
// omission: it is json.RawMessage, so no substitution can reach it — the
// bytes arrive verbatim in both mutation forms, which is the whole point of
// carrying them raw, and the push decoder's rule is what judges their shape
// where the two lanes are compared. This pins the verbatim part, so a future
// guard that "helpfully" decodes details cannot land quietly.
func TestEventFeedService_PollEvents_CarriesDetailsVerbatimThroughBothMutationForms(t *testing.T) {
	cases := map[string][]byte{
		"a lone surrogate escape": []byte(`{"note":"a\ud800b"}`),
		"a raw invalid byte":      append(append([]byte(`{"note":"a`), 0xff), []byte(`b"}`)...),
	}
	for name, details := range cases {
		t.Run(name, func(t *testing.T) {
			body := append([]byte(`{"events":[{"id":1,"kind":"boost_created","action":"created","created_at":"2026-07-14T06:10:00Z","event_type":"boost.created","bucket_id":2,"creator_id":3,"performed_by_id":null,"recording_id":9,"details":`), details...)
			body = append(body, []byte(`}],"position":"posAAA"}`)...)
			svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(body)
			})
			page, err := svc.PollEvents(context.Background(), nil)
			if err != nil {
				t.Fatalf("details is the push decoder's to judge, not this guard's: %v", err)
			}
			if len(page.Events) != 1 || !bytes.Equal(page.Events[0].Details, details) {
				t.Fatalf("details = %q, want the server's bytes verbatim", page.Events[0].Details)
			}
		})
	}
}

// The substitution guard is only as complete as its enumeration, and an
// enumeration is the kind of thing a new member gets added next to without
// being added to. This drives the real function with a distinct sentinel in
// every string member and asserts each one comes back, so it fails both ways:
// a member added to FeedEvent and not to feedEventStrings, and a member
// dropped from feedEventStrings. Details is exempt in code, with the reason,
// so the exemption is a decision rather than an oversight.
func TestFeedEventStringsHoldsEveryDecodedString(t *testing.T) {
	// Details is json.RawMessage, not a string: nothing decodes it, so no
	// substitution can reach it.
	exempt := map[string]bool{"Details": true}

	event := reflect.New(reflect.TypeOf(FeedEvent{})).Elem()
	want := map[string]string{}
	for i := range event.NumField() {
		f := event.Type().Field(i)
		if f.Type.Kind() != reflect.String || exempt[f.Name] {
			continue
		}
		sentinel := "sentinel-" + f.Name
		event.Field(i).SetString(sentinel)
		want[f.Name] = sentinel
	}
	if len(want) == 0 {
		t.Fatal("no string members found: the reflection walk is not looking at FeedEvent")
	}

	held := map[string]bool{}
	for _, s := range feedEventStrings(nil, event.Interface().(FeedEvent)) {
		held[s] = true
	}
	for name, sentinel := range want {
		if !held[sentinel] {
			t.Errorf("FeedEvent.%s is a decoded string feedEventStrings does not hold: add it, or exempt it here with the reason", name)
		}
	}

	// The page and item envelopes carry the rest, and checkPollPage's own
	// arms hold those: position, next, and the inbox's addressing reason.
	for _, shape := range []struct {
		name string
		typ  reflect.Type
		held map[string]bool
	}{
		{"EventFeedPage", reflect.TypeOf(EventFeedPage{}), map[string]bool{"Position": true, "Next": true}},
		{"InboxPage", reflect.TypeOf(InboxPage{}), map[string]bool{"Position": true, "Next": true}},
		{"InboxItem", reflect.TypeOf(InboxItem{}), map[string]bool{"Reason": true}},
	} {
		for i := range shape.typ.NumField() {
			f := shape.typ.Field(i)
			if f.Type.Kind() == reflect.String && !shape.held[f.Name] {
				t.Errorf("%s.%s is a decoded string no substitution check holds", shape.name, f.Name)
			}
		}
	}
}

// The 200 envelope is held to its declared members the same way the error
// bodies are: an object whose keys survived the decode, carrying the position
// and the lane's rows. A substituted `position` key would otherwise read as
// an empty position — which the connector already calls an unexpected shape,
// so this is the same verdict one layer earlier.
func TestEventFeedService_RefusesEveryOffContractPageEnvelope(t *testing.T) {
	cases := []struct {
		lane string
		name string
		body string
	}{
		{"events", "not an object", `[{"events":[],"position":"p"}]`},
		{"events", "a top-level null", `null`},
		{"events", "no position", `{"events":[]}`},
		{"events", "a null position", `{"events":[],"position":null}`},
		{"events", "an empty position", `{"events":[],"position":""}`},
		{"events", "a wrong-typed position", `{"events":[],"position":42}`},
		{"events", "a substituted position key", `{"events":[],"posi\ud800tion":"posAAA"}`},
		{"events", "no events member", `{"position":"posAAA"}`},
		{"events", "a null events member", `{"events":null,"position":"posAAA"}`},
		{"items", "no position", `{"items":[]}`},
		{"items", "no items member", `{"position":"posAAA"}`},
		{"items", "a substituted items key", `{"it\ud800ems":[],"position":"posAAA"}`},
	}
	for _, tc := range cases {
		t.Run(tc.lane+" "+tc.name, func(t *testing.T) {
			svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			})
			var err error
			if tc.lane == "items" {
				_, err = svc.PollInbox(context.Background(), nil)
			} else {
				_, err = svc.PollEvents(context.Background(), nil)
			}
			assertMalformedFeedResponse(t, err)
		})
	}
}

// A refused body is precisely the response someone has to look up
// server-side, and errors.As stops at the malformed error rather than at its
// cause — so the request id has to be on it.
func TestEventFeedService_MalformedResponseCarriesTheRequestID(t *testing.T) {
	svc := testEventFeedServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req-abc123")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(409)
		_, _ = w.Write([]byte(`{"error":"conflict","position_digest":"38b223c13c89dc89","filters_digest":"x"}`))
	})
	_, err := svc.PollEvents(context.Background(), &PollEventsOptions{Position: "posAAA"})
	var base *Error
	if !errors.As(err, &base) {
		t.Fatalf("expected the canonical *Error, got %T", err)
	}
	if base.RequestID != "req-abc123" {
		t.Errorf("requestID = %q, want the response's — errors.As stops here, not at the cause", base.RequestID)
	}
}
