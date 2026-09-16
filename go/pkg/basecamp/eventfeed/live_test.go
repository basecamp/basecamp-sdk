package eventfeed_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
)

// The Layer-1 adapters against a real generated client and a loopback API:
// every seam call is one governed operation, every outcome maps onto exactly
// one seam error kind, and a continuation is followed through the operation
// rather than fetched raw — including the redirect the tier-2 family assigns
// to this layer (conformance/event-feed README, row 15).

type liveFixture struct {
	server   *httptest.Server
	live     *eventfeed.Live
	requests atomic.Int32
	last     atomic.Pointer[http.Request]
}

func newLiveFixture(t *testing.T, lane eventfeed.Lane, handler http.HandlerFunc) *liveFixture {
	t.Helper()
	return newLiveFixtureWith(t, lane, handler)
}

func newLiveFixtureWith(t *testing.T, lane eventfeed.Lane, handler http.HandlerFunc, extra ...basecamp.ClientOption) *liveFixture {
	t.Helper()
	f := &liveFixture{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		clone := r.Clone(r.Context())
		f.last.Store(clone)
		handler(w, r)
	}))
	t.Cleanup(f.server.Close)
	cfg := basecamp.DefaultConfig()
	cfg.BaseURL = f.server.URL
	opts := append([]basecamp.ClientOption{basecamp.WithMaxRetries(0), basecamp.WithBaseDelay(time.Millisecond)}, extra...)
	live, err := eventfeed.NewLive(cfg, &basecamp.StaticTokenProvider{Token: "test-token"}, "99999", lane, opts...)
	if err != nil {
		t.Fatalf("NewLive: %v", err)
	}
	f.live = live
	return f
}

func jsonResponse(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

const liveFeedPage = `{"events":[
  {"id":101,"kind":"message_created","action":"created","created_at":"2026-07-14T06:10:00.159Z","event_type":"message.created","bucket_id":2,"creator_id":3,"performed_by_id":null,"recording_id":900},
  {"id":102,"kind":"kanban_card_moved","action":"moved","created_at":"2026-07-14T06:12:00Z","event_type":"card.moved","bucket_id":2,"creator_id":3,"performed_by_id":9007199254740993,"recording_id":901,"details":{"column_id":77,"previous_column_id":76}}
],"position":"posAAA","next":"https://3.basecampapi.com/99999/events.json?position=posAAA&types=message.created%2Ccard.moved&buckets=2"}`

func TestLiveMinter(t *testing.T) {
	t.Run("mints through CreateStreamTicket", func(t *testing.T) {
		f := newLiveFixture(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || r.URL.Path != "/99999/events/stream_ticket.json" {
				t.Errorf("mint hit %s %s", r.Method, r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("mint carried Authorization %q", r.Header.Get("Authorization"))
			}
			jsonResponse(w, 200, `{"ticket":"tkt-1","expires_in":120,"url":"wss://cable.example.test/99999?ticket=tkt-1"}`)
		})
		ticket, err := f.live.Minter().MintStreamTicket(context.Background())
		if err != nil {
			t.Fatalf("MintStreamTicket: %v", err)
		}
		if ticket.Ticket != "tkt-1" || ticket.ExpiresIn != 120 || ticket.URL != "wss://cable.example.test/99999?ticket=tkt-1" {
			t.Fatalf("ticket = %+v", ticket)
		}
	})
	cases := []struct {
		name   string
		status int
		header string
		body   string
		kind   eventfeed.MintErrorKind
		wait   time.Duration
	}{
		{"401 is unauthorized", 401, "", `{"error":"nope"}`, eventfeed.MintUnauthorized, 0},
		{"403 is unauthorized", 403, "", ``, eventfeed.MintUnauthorized, 0},
		{"429 with Retry-After is throttled", 429, "7", `{"error":"slow down"}`, eventfeed.MintThrottled, 7 * time.Second},
		{"503 is transient", 503, "", ``, eventfeed.MintTransient, 0},
		{"404 is unrecoverable", 404, "", `{"error":"gone"}`, eventfeed.MintUnrecoverable, 0},
		{"a Retry-After on a 404 is still unrecoverable", 404, "5", `{"error":"gone"}`, eventfeed.MintUnrecoverable, 0},
		{"a 200 that does not decode is unrecoverable", 200, "", `{"ticket": `, eventfeed.MintUnrecoverable, 0},
		{"a malformed success is unrecoverable", 200, "", `{"ticket":"","expires_in":120,"url":""}`, eventfeed.MintUnrecoverable, 0},
		{"a non-positive lifetime is a malformed success", 200, "", `{"ticket":"t","expires_in":0,"url":"wss://cable.example.test/1?ticket=t"}`, eventfeed.MintUnrecoverable, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newLiveFixture(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
				if tc.header != "" {
					w.Header().Set("Retry-After", tc.header)
				}
				jsonResponse(w, tc.status, tc.body)
			})
			_, err := f.live.Minter().MintStreamTicket(context.Background())
			var me *eventfeed.MintError
			if !errors.As(err, &me) || me.Kind != tc.kind {
				t.Fatalf("error = %v, want MintError kind %s", err, tc.kind)
			}
			if me.RetryAfter != tc.wait {
				t.Fatalf("RetryAfter = %v, want %v", me.RetryAfter, tc.wait)
			}
		})
	}
}

func TestLivePolls_EventsPageAndQuery(t *testing.T) {
	f := newLiveFixture(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, liveFeedPage)
	})
	page, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{
		Types:             []string{"message.created", "card.moved"},
		Buckets:           []int64{2, 1},
		Creators:          []int64{3},
		Performers:        []int64{9},
		ExcludePerformers: []int64{77},
		ActorTypes:        []string{eventfeed.ActorTypeAgent},
	})
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	req := f.last.Load()
	if req.Method != http.MethodGet || req.URL.Path != "/99999/events.json" {
		t.Fatalf("poll hit %s %s", req.Method, req.URL.Path)
	}
	want := url.Values{
		"position": {"pos-0"}, "types": {"message.created,card.moved"}, "buckets": {"2,1"}, "creators": {"3"},
		"performers": {"9"}, "exclude_performers": {"77"}, "actor_types": {"agent"},
	}
	if got := req.URL.Query(); got.Encode() != want.Encode() {
		t.Fatalf("query = %v, want %v", got, want)
	}
	if page.Position != "posAAA" || !strings.HasPrefix(page.Next, "https://3.basecampapi.com/99999/events.json?position=posAAA") {
		t.Fatalf("page = %+v", page)
	}
	if len(page.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(page.Events))
	}
	if page.Events[0].PerformedByID != nil || page.Events[0].Details != nil || page.Events[0].Addressing != nil {
		t.Errorf("event 101 = %+v, want a direct action with no details and no addressing", page.Events[0])
	}
	ev := page.Events[1]
	if ev.PerformedByID == nil || *ev.PerformedByID != 9007199254740993 {
		t.Errorf("event 102 PerformedByID = %v, want 9007199254740993", ev.PerformedByID)
	}
	var details struct {
		ColumnID         int64 `json:"column_id"`
		PreviousColumnID int64 `json:"previous_column_id"`
	}
	if err := json.Unmarshal(ev.Details, &details); err != nil || details.ColumnID != 77 || details.PreviousColumnID != 76 {
		t.Errorf("event 102 Details = %s (%v), want the columns", ev.Details, err)
	}
}

func TestLivePolls_FollowsAContinuationThroughTheOperation(t *testing.T) {
	f := newLiveFixture(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, `{"events":[],"position":"posBBB"}`)
	})
	next := f.server.URL + "/99999/events.json?position=posAAA&types=message.created%2Ccard.moved&buckets=2"
	page, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{PageURL: next}, eventfeed.Filters{Types: []string{"ignored"}})
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	req := f.last.Load()
	// The query the server wrote into `next` is what goes back — never the
	// configured filters, which the continuation already carries canonically.
	want := url.Values{"position": {"posAAA"}, "types": {"message.created,card.moved"}, "buckets": {"2"}}
	if got := req.URL.Query(); got.Encode() != want.Encode() {
		t.Fatalf("continuation query = %v, want %v", got, want)
	}
	if page.Position != "posBBB" || page.Next != "" {
		t.Fatalf("page = %+v, want the walk's end", page)
	}
}

func TestLivePolls_ErrorMatrix(t *testing.T) {
	cases := []struct {
		name   string
		lane   eventfeed.Lane
		status int
		header string
		body   string
		check  func(t *testing.T, pe *eventfeed.PollError)
	}{
		{"400 position is position_invalid", eventfeed.AccountLane, 400, "", `{"error":"Unrecognized position. Resume with since=<id> or since=now."}`,
			func(t *testing.T, pe *eventfeed.PollError) {
				if pe.Kind != eventfeed.PollPositionInvalid {
					t.Fatalf("kind = %s", pe.Kind)
				}
			}},
		{"400 filter is filter_invalid with the message", eventfeed.AccountLane, 400, "", `{"error":"Unknown type nope.created. Fix the filters; a position reset won't help."}`,
			func(t *testing.T, pe *eventfeed.PollError) {
				if pe.Kind != eventfeed.PollFilterInvalid || !strings.HasPrefix(pe.Msg, "Unknown type nope.created") {
					t.Fatalf("pe = %+v", pe)
				}
			}},
		{"409 is filter_changed with both digests", eventfeed.AccountLane, 409, "", `{"error":"Positions are bound to the filter set they were minted for.","position_digest":"38b223c13c89dc89","filters_digest":"44136fa355b3678a"}`,
			func(t *testing.T, pe *eventfeed.PollError) {
				if pe.Kind != eventfeed.PollFilterChanged || pe.PositionDigest != "38b223c13c89dc89" || pe.FiltersDigest != "44136fa355b3678a" {
					t.Fatalf("pe = %+v", pe)
				}
			}},
		{"410 is gone with the epoch and resume", eventfeed.AccountLane, 410, "", `{"error":"That position predates this feed's epoch.","epoch_after_id":1071915000,"resume":"https://3.basecampapi.com/99999/events.json?since=1071915000"}`,
			func(t *testing.T, pe *eventfeed.PollError) {
				if pe.Kind != eventfeed.PollGone || pe.EpochAfterID != 1071915000 || pe.ResumeURL != "https://3.basecampapi.com/99999/events.json?since=1071915000" {
					t.Fatalf("pe = %+v", pe)
				}
			}},
		{"inbox 410 is gone with no epoch", eventfeed.InboxLane, 410, "", `{"error":"That position predates the inbox's retention window.","resume":"https://3.basecampapi.com/99999/inbox.json?since=0"}`,
			func(t *testing.T, pe *eventfeed.PollError) {
				if pe.Kind != eventfeed.PollGone || pe.EpochAfterID != 0 || pe.ResumeURL != "https://3.basecampapi.com/99999/inbox.json?since=0" {
					t.Fatalf("pe = %+v", pe)
				}
			}},
		{"401 is unauthorized", eventfeed.AccountLane, 401, "", `{"error":"nope"}`,
			func(t *testing.T, pe *eventfeed.PollError) {
				if pe.Kind != eventfeed.PollUnauthorized {
					t.Fatalf("kind = %s", pe.Kind)
				}
			}},
		{"inbox 403 for a person is unauthorized", eventfeed.InboxLane, 403, "", ``,
			func(t *testing.T, pe *eventfeed.PollError) {
				if pe.Kind != eventfeed.PollUnauthorized {
					t.Fatalf("kind = %s", pe.Kind)
				}
			}},
		{"429 with Retry-After is throttled", eventfeed.AccountLane, 429, "3", `{"error":"slow down"}`,
			func(t *testing.T, pe *eventfeed.PollError) {
				if pe.Kind != eventfeed.PollThrottled || pe.RetryAfter != 3*time.Second {
					t.Fatalf("pe = %+v", pe)
				}
			}},
		{"503 is transient", eventfeed.AccountLane, 503, "", ``,
			func(t *testing.T, pe *eventfeed.PollError) {
				if pe.Kind != eventfeed.PollTransient {
					t.Fatalf("kind = %s", pe.Kind)
				}
			}},
		{"404 is unrecoverable", eventfeed.AccountLane, 404, "", `{"error":"no such feed"}`,
			func(t *testing.T, pe *eventfeed.PollError) {
				if pe.Kind != eventfeed.PollUnrecoverable {
					t.Fatalf("kind = %s", pe.Kind)
				}
			}},
		{"a Retry-After on a 404 does not make it throttled", eventfeed.AccountLane, 404, "5", `{"error":"no such feed"}`,
			func(t *testing.T, pe *eventfeed.PollError) {
				if pe.Kind != eventfeed.PollUnrecoverable || pe.RetryAfter != 0 {
					t.Fatalf("pe = %+v, want unrecoverable with no wait", pe)
				}
			}},
		{"a 200 that does not decode is unrecoverable", eventfeed.AccountLane, 200, "", `{"events": [`,
			func(t *testing.T, pe *eventfeed.PollError) {
				if pe.Kind != eventfeed.PollUnrecoverable {
					t.Fatalf("kind = %s, want unrecoverable (re-polling draws the same body)", pe.Kind)
				}
			}},
		{"a row missing required members is unrecoverable", eventfeed.AccountLane, 200, "", `{"events":[{"id":0,"kind":"","event_type":"message.created","action":"created","created_at":"2026-07-14T06:10:00Z","bucket_id":2,"creator_id":3,"performed_by_id":null,"recording_id":900}],"position":"p"}`,
			func(t *testing.T, pe *eventfeed.PollError) {
				if pe.Kind != eventfeed.PollUnrecoverable {
					t.Fatalf("kind = %s, want unrecoverable", pe.Kind)
				}
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newLiveFixture(t, tc.lane, func(w http.ResponseWriter, r *http.Request) {
				if tc.header != "" {
					w.Header().Set("Retry-After", tc.header)
				}
				jsonResponse(w, tc.status, tc.body)
			})
			_, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{})
			var pe *eventfeed.PollError
			if !errors.As(err, &pe) {
				t.Fatalf("error = %T (%v), want *PollError", err, err)
			}
			tc.check(t, pe)
			if got := f.requests.Load(); got != 1 {
				t.Fatalf("requests = %d, want exactly one (no retry outside the seam's own budget)", got)
			}
		})
	}
}

// TestLivePolls_RefusesAContinuationWhoseQueryDoesNotParse: a `next` whose
// query holds a malformed pair would have that pair silently dropped by
// url.URL.Query — and if it is `position`, the re-issued poll becomes a bare
// present entry that skips the history it was following.
func TestLivePolls_RefusesAContinuationWhoseQueryDoesNotParse(t *testing.T) {
	f := newLiveFixture(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, `{"events":[],"position":"posBBB"}`)
	})
	next := f.server.URL + "/99999/events.json?position=pos%ZZ&types=message.created"
	_, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{PageURL: next}, eventfeed.Filters{})
	var pe *eventfeed.PollError
	if !errors.As(err, &pe) || pe.Kind != eventfeed.PollUnrecoverable {
		t.Fatalf("error = %v, want unrecoverable", err)
	}
	if f.requests.Load() != 0 {
		t.Fatalf("requests = %d, want none for a continuation that does not parse whole", f.requests.Load())
	}
	if strings.Contains(pe.Error(), "pos%ZZ") {
		t.Fatalf("PollError renders the continuation: %s", pe.Error())
	}
}

// TestLivePolls_RefusesAContinuationWithoutACursor: a followed URL that
// carries filters but no position or since would re-issue as a bare present
// entry; a value the wrapper cannot parse yields a fixed cause, never the value.
func TestLivePolls_RefusesAContinuationWithoutACursor(t *testing.T) {
	f := newLiveFixture(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, `{"events":[],"position":"p"}`)
	})
	for name, next := range map[string]string{
		"no cursor":     f.server.URL + "/99999/events.json?types=message.created",
		"repeated key":  f.server.URL + "/99999/events.json?position=pos-new&position=pos-old",
		"both cursors":  f.server.URL + "/99999/events.json?position=p&since=now",
		"bad filter id": f.server.URL + "/99999/events.json?position=p&buckets=leaked-secret",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{PageURL: next}, eventfeed.Filters{})
			var pe *eventfeed.PollError
			if !errors.As(err, &pe) || pe.Kind != eventfeed.PollUnrecoverable {
				t.Fatalf("error = %v, want unrecoverable", err)
			}
			if strings.Contains(pe.Error(), "leaked-secret") {
				t.Fatalf("PollError renders the continuation's value: %s", pe.Error())
			}
		})
	}
	if f.requests.Load() != 0 {
		t.Fatalf("requests = %d, want none", f.requests.Load())
	}
}

// TestLiveClient_DownloadsKeepTheirDispatchingRedirect: the guard answers
// only the seams' own calls, so the client NewLive built stays a full client
// for the host — a download's authenticated first hop still 302s to the
// signed URL and the SDK follows its own dispatch.
func TestLiveClient_DownloadsKeepTheirDispatchingRedirect(t *testing.T) {
	var f *liveFixture
	f = newLiveFixture(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/99999/blobs/1/download":
			http.Redirect(w, r, f.server.URL+"/signed/blob", http.StatusFound)
		case "/signed/blob":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("PNGDATA"))
		default:
			http.NotFound(w, r)
		}
	})
	result, err := f.live.Client().ForAccount("99999").DownloadURL(context.Background(), f.server.URL+"/99999/blobs/1/download")
	if err != nil {
		t.Fatalf("DownloadURL through the feed's client: %v", err)
	}
	defer result.Body.Close()
	data, _ := io.ReadAll(result.Body)
	if string(data) != "PNGDATA" {
		t.Fatalf("downloaded %q, want the signed hop's bytes", data)
	}
}

// TestLivePolls_A410OfTheOtherLanesShapeIsMalformed: the feed's 410 names
// its epoch and the inbox's carries none; a body of the other lane's shape
// is unrecoverable rather than a gap with a fence that does not exist.
func TestLivePolls_A410OfTheOtherLanesShapeIsMalformed(t *testing.T) {
	const withEpoch = `{"error":"gone","epoch_after_id":500,"resume":"https://3.basecampapi.com/99999/events.json?since=500"}`
	const withoutEpoch = `{"error":"gone","resume":"https://3.basecampapi.com/99999/inbox.json?since=0"}`
	for _, tc := range []struct {
		name string
		lane eventfeed.Lane
		body string
		kind eventfeed.PollErrorKind
	}{
		{"feed with epoch", eventfeed.AccountLane, withEpoch, eventfeed.PollGone},
		{"feed without epoch", eventfeed.AccountLane, withoutEpoch, eventfeed.PollUnrecoverable},
		{"inbox without epoch", eventfeed.InboxLane, withoutEpoch, eventfeed.PollGone},
		{"inbox with epoch", eventfeed.InboxLane, withEpoch, eventfeed.PollUnrecoverable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newLiveFixture(t, tc.lane, func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 410, tc.body)
			})
			_, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{})
			var pe *eventfeed.PollError
			if !errors.As(err, &pe) || pe.Kind != tc.kind {
				t.Fatalf("error = %v, want %s", err, tc.kind)
			}
		})
	}
}

// TestLivePolls_TheGuardRecognizesRoutesBeneathABasePath: a base URL with a
// path prefix puts the feed routes beneath it; the guard still answers the
// 3xx, and the foreign origin sees nothing.
func TestLivePolls_TheGuardRecognizesRoutesBeneathABasePath(t *testing.T) {
	var sentinelHits atomic.Int32
	sentinel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sentinelHits.Add(1)
		jsonResponse(w, 200, `{"events":[],"position":"stolen"}`)
	}))
	t.Cleanup(sentinel.Close)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/99999/events.json" {
			t.Errorf("request path %q, want the route beneath the base path", r.URL.Path)
		}
		w.Header().Set("Location", sentinel.URL+"/99999/events.json?position=pos-0&token=leak")
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(server.Close)
	cfg := basecamp.DefaultConfig()
	cfg.BaseURL = server.URL + "/api/v1"
	live, err := eventfeed.NewLive(cfg, &basecamp.StaticTokenProvider{Token: "t"}, "99999", eventfeed.AccountLane, basecamp.WithMaxRetries(0))
	if err != nil {
		t.Fatalf("NewLive: %v", err)
	}
	_, err = live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{})
	var pe *eventfeed.PollError
	if !errors.As(err, &pe) || pe.Kind != eventfeed.PollRedirectRefused || pe.LocationOrigin != sentinel.URL {
		t.Fatalf("error = %v, want redirect_refused carrying %q", err, sentinel.URL)
	}
	if sentinelHits.Load() != 0 {
		t.Fatalf("the foreign origin received %d request(s), want zero egress", sentinelHits.Load())
	}
}

// TestRedirectGuard_MatchesOnlyTheFeedRoutes: the guard claims exactly the
// three feed routes beneath the configured base path — never a recording's
// audit trail, whose path also ends in {id}/events.json, and never a route
// outside the base path.
func TestRedirectGuard_MatchesOnlyTheFeedRoutes(t *testing.T) {
	for _, tc := range []struct {
		base, path string
		want       bool
	}{
		{"/", "/99999/events.json", true},
		{"/", "/99999/inbox.json", true},
		{"/", "/99999/events/stream_ticket.json", true},
		{"/api/v1/", "/api/v1/99999/events.json", true},
		{"/", "/api/v1/99999/events.json", false},
		{"/api/v1/", "/99999/events.json", false},
		{"/", "/99999/recordings/12345/events.json", false},
		{"/", "/99999/buckets/2085958499/recordings/12345/events.json", false},
		{"/", "/abc/events.json", false},
		{"/", "/99999/events.json/extra", false},
	} {
		if got := eventfeed.ExportIsFeedOperationPath(tc.base, tc.path); got != tc.want {
			t.Errorf("isFeedOperationPath(%q, %q) = %v, want %v", tc.base, tc.path, got, tc.want)
		}
	}
}

// TestLive_AResetBodyIsTransient: a body whose read fails after the headers
// — whatever error type the HTTP stack chose — is a transport failure on
// both seams, told apart from a whole body that did not decode.
func TestLive_AResetBodyIsTransient(t *testing.T) {
	f := newLiveFixtureWith(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, `{"events":[],"position":"p"}`)
	}, basecamp.WithTransport(resettingBodyTransport{inner: http.DefaultTransport}))
	_, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{})
	var pe *eventfeed.PollError
	if !errors.As(err, &pe) || pe.Kind != eventfeed.PollTransient {
		t.Fatalf("poll error = %v, want transient for a reset body", err)
	}
	_, err = f.live.Minter().MintStreamTicket(context.Background())
	var me *eventfeed.MintError
	if !errors.As(err, &me) || me.Kind != eventfeed.MintTransient {
		t.Fatalf("mint error = %v, want transient for a reset body", err)
	}
}

// resettingBodyTransport answers every request with a body whose read fails
// with a plain error after the headers — the shape of an HTTP/2 stream
// reset, which is neither a url.Error nor a net.Error.
type resettingBodyTransport struct{ inner http.RoundTripper }

func (t resettingBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(resetReader{})
	return resp, nil
}

type resetReader struct{}

func (resetReader) Read([]byte) (int, error) { return 0, errors.New("stream error: RST_STREAM") }

// TestLivePolls_AContinuationOutOfOrderIsMalformed: strict order holds
// across a walk's pages — a continuation whose first row does not follow the
// previous page's last is unrecoverable, while a fresh cursor starts over.
func TestLivePolls_AContinuationOutOfOrderIsMalformed(t *testing.T) {
	const row = `"kind":"message_created","event_type":"message.created","action":"created","created_at":"2026-08-01T12:00:00Z","bucket_id":2,"creator_id":3,"performed_by_id":null,"recording_id":900`
	var f *liveFixture
	f = newLiveFixture(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("position") {
		case "pos-0":
			jsonResponse(w, 200, `{"events":[{"id":100,`+row+`},{"id":102,`+row+`}],"position":"pos-1","next":"`+f.server.URL+`/99999/events.json?position=pos-cont"}`)
		default:
			jsonResponse(w, 200, `{"events":[{"id":101,`+row+`}],"position":"pos-2"}`)
		}
	})
	page, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{})
	if err != nil || page.Next == "" {
		t.Fatalf("first page = %+v, %v", page, err)
	}
	_, err = f.live.Polls().Poll(context.Background(), eventfeed.Cursor{PageURL: page.Next}, eventfeed.Filters{})
	var pe *eventfeed.PollError
	if !errors.As(err, &pe) || pe.Kind != eventfeed.PollUnrecoverable {
		t.Fatalf("continuation error = %v, want unrecoverable for a row behind the previous page", err)
	}
	if _, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-fresh"}, eventfeed.Filters{}); err != nil {
		t.Fatalf("a fresh cursor after the refused continuation: %v", err)
	}
}

// TestLivePolls_APageOutOfOrderIsMalformed: rows arrive in strict order of
// the lane's identity; a page that repeats or reorders keys is unrecoverable
// on either lane.
func TestLivePolls_APageOutOfOrderIsMalformed(t *testing.T) {
	const row = `"kind":"message_created","event_type":"message.created","action":"created","created_at":"2026-08-01T12:00:00Z","bucket_id":2,"creator_id":3,"performed_by_id":null,"recording_id":900`
	for name, tc := range map[string]struct {
		lane eventfeed.Lane
		body string
	}{
		"feed reordered":  {eventfeed.AccountLane, `{"events":[{"id":102,` + row + `},{"id":101,` + row + `}],"position":"p"}`},
		"feed duplicated": {eventfeed.AccountLane, `{"events":[{"id":101,` + row + `},{"id":101,` + row + `}],"position":"p"}`},
		"inbox reordered": {eventfeed.InboxLane, `{"items":[{"addressing_id":12,"reason":"mentioned","addressed_at":"2026-08-01T12:00:01Z","event":{"id":101,` + row + `}},{"addressing_id":11,"reason":"assigned","addressed_at":"2026-08-01T12:00:01Z","event":{"id":102,` + row + `}}],"position":"p"}`},
	} {
		t.Run(name, func(t *testing.T) {
			f := newLiveFixture(t, tc.lane, func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, tc.body)
			})
			_, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{})
			var pe *eventfeed.PollError
			if !errors.As(err, &pe) || pe.Kind != eventfeed.PollUnrecoverable {
				t.Fatalf("error = %v, want unrecoverable for a page out of order", err)
			}
		})
	}
}

// TestLivePolls_TheGuardSurvivesAHookThatReplacesTheContext: a host hook that
// returns a fresh context from OnRequestStart drops the seam's per-call
// record; the guard still answers the 3xx, because it recognizes the seam
// call by its route, and the foreign origin still sees nothing.
func TestLivePolls_TheGuardSurvivesAHookThatReplacesTheContext(t *testing.T) {
	var sentinelHits atomic.Int32
	sentinel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sentinelHits.Add(1)
		jsonResponse(w, 200, `{"events":[],"position":"stolen"}`)
	}))
	t.Cleanup(sentinel.Close)
	f := newLiveFixtureWith(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", sentinel.URL+"/99999/events.json?position=pos-0&token=leak")
		w.WriteHeader(http.StatusFound)
	}, basecamp.WithHooks(contextDroppingHooks{}))
	_, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{})
	var pe *eventfeed.PollError
	if !errors.As(err, &pe) || pe.Kind != eventfeed.PollRedirectRefused {
		t.Fatalf("error = %v, want redirect_refused", err)
	}
	if strings.Contains(pe.Error(), "leak") {
		t.Fatalf("PollError renders the refused Location: %s", pe.Error())
	}
	if sentinelHits.Load() != 0 {
		t.Fatalf("the foreign origin received %d request(s), want zero egress", sentinelHits.Load())
	}
}

// contextDroppingHooks returns a fresh context from every start hook — the
// misuse a guard keyed on context values would not survive.
type contextDroppingHooks struct{ basecamp.NoopHooks }

func (contextDroppingHooks) OnOperationStart(context.Context, basecamp.OperationInfo) context.Context {
	return context.Background()
}

func (contextDroppingHooks) OnRequestStart(context.Context, basecamp.RequestInfo) context.Context {
	return context.Background()
}

// TestLive_ATruncatedBodyIsTransient: a connection that ends after the
// headers, mid-body, is a transport failure to retry — on the mint and on the
// poll — not a malformed response to terminate on.
func TestLive_ATruncatedBodyIsTransient(t *testing.T) {
	truncate := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "4096")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"events":[`))
	}
	f := newLiveFixture(t, eventfeed.AccountLane, truncate)
	_, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{})
	var pe *eventfeed.PollError
	if !errors.As(err, &pe) || pe.Kind != eventfeed.PollTransient {
		t.Fatalf("poll error = %v, want transient for a truncated body", err)
	}
	_, err = f.live.Minter().MintStreamTicket(context.Background())
	var me *eventfeed.MintError
	if !errors.As(err, &me) || me.Kind != eventfeed.MintTransient {
		t.Fatalf("mint error = %v, want transient for a truncated body", err)
	}
}

// TestLivePolls_DetailsMatchThePushLaneByteForByte: a row's details object
// reaches the connector as the bytes the server sent on both lanes — an
// explicit null member and a member of a type this SDK does not model
// survive the poll adapter exactly as they survive the push decoder — and a
// details value that is not an object is refused as the push lane refuses it.
func TestLivePolls_DetailsMatchThePushLaneByteForByte(t *testing.T) {
	const row = `{"id":7001,"kind":"boost_created","event_type":"boost.created","action":"created","created_at":"2026-08-01T12:00:00Z","bucket_id":2,"creator_id":3,"performed_by_id":null,"recording_id":900,"actor_type":"user","visible_to_clients":true,"details":{"boost_id":5,"boosted_event_id":null,"boosted_event_type":null,"future_member":{"nested":[1,"two",null]}}}`
	f := newLiveFixture(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, `{"events":[`+row+`],"position":"pos-1"}`)
	})
	page, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{})
	if err != nil || len(page.Events) != 1 {
		t.Fatalf("Poll = %+v, %v", page, err)
	}
	pushed, err := eventfeed.ExportDecodePushEvent([]byte(row))
	if err != nil {
		t.Fatalf("push decode: %v", err)
	}
	if !bytes.Equal(page.Events[0].Details, pushed.Details) {
		t.Fatalf("poll details %s\nwant the push lane's %s", page.Events[0].Details, pushed.Details)
	}
	if !strings.Contains(string(page.Events[0].Details), `"boosted_event_id":null`) || !strings.Contains(string(page.Events[0].Details), `"future_member"`) {
		t.Fatalf("details lost a null or an unmodeled member: %s", page.Events[0].Details)
	}
	for name, details := range map[string]string{"a null is absent": "null", "absent is absent": ""} {
		t.Run(name, func(t *testing.T) {
			body := strings.Replace(row, `,"details":{"boost_id":5,"boosted_event_id":null,"boosted_event_type":null,"future_member":{"nested":[1,"two",null]}}`, "", 1)
			if details != "" {
				body = strings.Replace(body, `"recording_id":900`, `"recording_id":900,"details":`+details, 1)
			}
			f := newLiveFixture(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
				jsonResponse(w, 200, `{"events":[`+body+`],"position":"pos-1"}`)
			})
			page, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{})
			if err != nil || len(page.Events) != 1 || page.Events[0].Details != nil {
				t.Fatalf("Poll = %+v, %v; want one event with no details", page, err)
			}
		})
	}
	t.Run("a non-object is refused", func(t *testing.T) {
		body := strings.Replace(row, `{"boost_id":5,"boosted_event_id":null,"boosted_event_type":null,"future_member":{"nested":[1,"two",null]}}`, `7`, 1)
		f := newLiveFixture(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
			jsonResponse(w, 200, `{"events":[`+body+`],"position":"pos-1"}`)
		})
		_, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{})
		var pe *eventfeed.PollError
		if !errors.As(err, &pe) || pe.Kind != eventfeed.PollUnrecoverable {
			t.Fatalf("error = %v, want unrecoverable for non-object details", err)
		}
	})
}

// TestLivePolls_AMalformedLocationIsARefusedHop: a 3xx whose Location does
// not parse is answered by the guard before net/http's redirect loop would
// parse it into a url.Error; the seam classifies it as a refused hop, not a
// transport failure to retry into, and no rendering — the seam's error or the
// operation hooks' — carries the header.
func TestLivePolls_AMalformedLocationIsARefusedHop(t *testing.T) {
	hooks := &recordingHooks{}
	f := newLiveFixtureWith(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "http://[::1]:namedport/leak")
		w.WriteHeader(http.StatusFound)
	}, basecamp.WithHooks(hooks))
	_, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{})
	var pe *eventfeed.PollError
	if !errors.As(err, &pe) || pe.Kind != eventfeed.PollRedirectRefused || pe.LocationOrigin != "unparsable" {
		t.Fatalf("error = %v, want redirect_refused with the unparsable token", err)
	}
	if strings.Contains(pe.Error(), "leak") {
		t.Fatalf("PollError renders the malformed Location: %s", pe.Error())
	}
	hooks.assertNoLeak(t, "leak")
}

// recordingHooks captures what the operation hooks are handed, so a test can
// assert that the refused Location reached none of them.
type recordingHooks struct {
	basecamp.NoopHooks
	mu         sync.Mutex
	renderings []string
}

func (h *recordingHooks) OnOperationEnd(ctx context.Context, op basecamp.OperationInfo, err error, d time.Duration) {
	if err != nil {
		h.mu.Lock()
		h.renderings = append(h.renderings, err.Error())
		h.mu.Unlock()
	}
}

func (h *recordingHooks) OnRequestEnd(ctx context.Context, info basecamp.RequestInfo, result basecamp.RequestResult) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if result.Error != nil {
		h.renderings = append(h.renderings, result.Error.Error())
	}
	h.renderings = append(h.renderings, fmt.Sprintf("%+v", result))
}

func (h *recordingHooks) assertNoLeak(t *testing.T, token string) {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.renderings) == 0 {
		t.Fatalf("the hooks recorded nothing; the test observes no rendering")
	}
	for _, r := range h.renderings {
		if strings.Contains(r, token) {
			t.Fatalf("a hook rendering carries the refused Location: %s", r)
		}
	}
}

// TestLivePolls_TheClientsOwnTimeoutIsTransient: the HTTP client's timeout
// surfaces as a wrapped DeadlineExceeded while the connector's context is
// live; that is a transport failure to retry, not the connector's cancellation.
func TestLivePolls_TheClientsOwnTimeoutIsTransient(t *testing.T) {
	f := &liveFixture{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(f.server.Close)
	cfg := basecamp.DefaultConfig()
	cfg.BaseURL = f.server.URL
	live, err := eventfeed.NewLive(cfg, &basecamp.StaticTokenProvider{Token: "t"}, "99999", eventfeed.AccountLane,
		basecamp.WithMaxRetries(0), basecamp.WithTimeout(50*time.Millisecond))
	if err != nil {
		t.Fatalf("NewLive: %v", err)
	}
	_, err = live.Polls().Poll(context.Background(), eventfeed.Cursor{Since: "now"}, eventfeed.Filters{})
	var pe *eventfeed.PollError
	if !errors.As(err, &pe) || pe.Kind != eventfeed.PollTransient {
		t.Fatalf("error = %v, want transient for the client's own timeout", err)
	}
}

func TestLivePolls_GateRefusalsAreTransient(t *testing.T) {
	f := newLiveFixture(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, `{"events":[],"position":"p"}`)
	})
	// The seam's mapping is exercised directly: a gate refusal never reaches
	// the wire, so there is no response to script.
	for _, sentinel := range []error{basecamp.ErrCircuitOpen, basecamp.ErrBulkheadFull, basecamp.ErrRateLimited} {
		if kind := eventfeed.ExportMapPollErrorKind(sentinel); kind != eventfeed.PollTransient {
			t.Errorf("%v mapped to %s, want transient", sentinel, kind)
		}
		if kind := eventfeed.ExportMapMintErrorKind(sentinel); kind != eventfeed.MintTransient {
			t.Errorf("%v mapped to %s, want transient", sentinel, kind)
		}
	}
	_ = f
}

func TestLivePolls_InboxItems(t *testing.T) {
	f := newLiveFixture(t, eventfeed.InboxLane, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, `{"items":[{"addressing_id":991,"reason":"mentioned","addressed_at":"2026-07-14T06:10:00Z","event":{"id":101,"kind":"comment_created","action":"created","created_at":"2026-07-14T06:10:00Z","event_type":"comment.created","bucket_id":2,"creator_id":3,"performed_by_id":null,"recording_id":900}}],"position":"ipos-1"}`)
	})
	page, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Since: "0"}, eventfeed.Filters{
		Reasons: []string{"mentioned", "assigned"}, Types: []string{"comment.created"}, Buckets: []int64{2},
	})
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	req := f.last.Load()
	if req.URL.Path != "/99999/inbox.json" {
		t.Fatalf("inbox poll hit %s", req.URL.Path)
	}
	want := url.Values{"since": {"0"}, "reasons": {"mentioned,assigned"}, "types": {"comment.created"}, "buckets": {"2"}}
	if got := req.URL.Query(); got.Encode() != want.Encode() {
		t.Fatalf("query = %v, want %v", got, want)
	}
	if len(page.Events) != 1 || page.Events[0].Addressing == nil || page.Events[0].Addressing.ID != 991 ||
		page.Events[0].Addressing.Reason != "mentioned" || page.Events[0].Key() != 991 || page.Events[0].ID != 101 {
		t.Fatalf("items = %+v, want one item keyed by addressing id 991 over event 101", page.Events)
	}
}

func TestLivePolls_RefusesAnInboxItemMissingItsEnvelope(t *testing.T) {
	f := newLiveFixture(t, eventfeed.InboxLane, func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, `{"items":[{"reason":"mentioned","addressed_at":"2026-07-14T06:10:00Z","event":{"id":101,"kind":"comment_created","action":"created","created_at":"2026-07-14T06:10:00Z","event_type":"comment.created","bucket_id":2,"creator_id":3,"performed_by_id":null,"recording_id":900}}],"position":"ipos-1"}`)
	})
	_, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Since: "0"}, eventfeed.Filters{})
	var pe *eventfeed.PollError
	if !errors.As(err, &pe) || pe.Kind != eventfeed.PollUnrecoverable {
		t.Fatalf("error = %v, want unrecoverable for an item with no addressing_id", err)
	}
}

// TestLivePolls_RefusesACrossOriginRedirectWithZeroEgress is the Layer-1 302
// test the tier-2 family assigns to this layer: a validated same-origin
// continuation answers 302 with a foreign Location. The guard answers the
// hop at the wire — the sentinel server behind the Location never sees a
// request — and the seam reports redirect_refused carrying the Location's
// origin, while the operation hooks see neither the Location nor its query.
func TestLivePolls_RefusesACrossOriginRedirectWithZeroEgress(t *testing.T) {
	var sentinelHits atomic.Int32
	sentinel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sentinelHits.Add(1)
		jsonResponse(w, 200, `{"events":[],"position":"stolen"}`)
	}))
	t.Cleanup(sentinel.Close)
	hooks := &recordingHooks{}
	f := newLiveFixtureWith(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", sentinel.URL+"/99999/events.json?position=pos-0&token=leak")
		jsonResponse(w, http.StatusFound, `{"error":"moved to leak"}`)
	}, basecamp.WithHooks(hooks))
	_, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{})
	var pe *eventfeed.PollError
	if !errors.As(err, &pe) || pe.Kind != eventfeed.PollRedirectRefused {
		t.Fatalf("error = %v, want redirect_refused", err)
	}
	if pe.LocationOrigin != sentinel.URL {
		t.Fatalf("LocationOrigin = %q, want the refused origin %q", pe.LocationOrigin, sentinel.URL)
	}
	if strings.Contains(pe.Error(), "leak") || strings.Contains(pe.Error(), sentinel.URL) {
		t.Fatalf("PollError renders the refused Location: %s", pe.Error())
	}
	if sentinelHits.Load() != 0 {
		t.Fatalf("the foreign origin received %d request(s), want zero egress", sentinelHits.Load())
	}
	hooks.assertNoLeak(t, "leak")
}

// TestLivePolls_ABare3xxIsRefusedToo: a 3xx with no Location is equally not
// a page: the seam reports redirect_refused with the fixed unparsable token.
func TestLivePolls_ABare3xxIsRefusedToo(t *testing.T) {
	f := newLiveFixture(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
	})
	_, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{})
	var pe *eventfeed.PollError
	if !errors.As(err, &pe) || pe.Kind != eventfeed.PollRedirectRefused || pe.LocationOrigin != "unparsable" {
		t.Fatalf("error = %v, want redirect_refused with the unparsable token", err)
	}
}

// TestLivePolls_ASameOriginRedirectIsRefusedToo: the API never redirects a
// feed call, and a continuation is followed by re-issuing the operation, so
// even a same-origin hop is answered by the guard — one request, no follow —
// and the seam reports the API origin as the refused one.
func TestLivePolls_ASameOriginRedirectIsRefusedToo(t *testing.T) {
	var f *liveFixture
	f = newLiveFixture(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("position") == "pos-0" {
			http.Redirect(w, r, f.server.URL+"/99999/events.json?position=pos-moved", http.StatusFound)
			return
		}
		jsonResponse(w, 200, `{"events":[],"position":"pos-1"}`)
	})
	_, err := f.live.Polls().Poll(context.Background(), eventfeed.Cursor{Position: "pos-0"}, eventfeed.Filters{})
	var pe *eventfeed.PollError
	if !errors.As(err, &pe) || pe.Kind != eventfeed.PollRedirectRefused || pe.LocationOrigin != f.server.URL {
		t.Fatalf("error = %v, want redirect_refused carrying %q", err, f.server.URL)
	}
	if f.requests.Load() != 1 {
		t.Fatalf("requests = %d, want the one the hop was refused on", f.requests.Load())
	}
}

// TestLiveMinter_ARedirectIsUnrecoverable: a mint that answers 3xx is out of
// contract and a fresh mint answers the same way; the guard strips the
// Location so nothing — the seam's error or the hooks' — renders it.
func TestLiveMinter_ARedirectIsUnrecoverable(t *testing.T) {
	hooks := &recordingHooks{}
	f := newLiveFixtureWith(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://evil.example.test/mint?token=leak")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}, basecamp.WithHooks(hooks))
	_, err := f.live.Minter().MintStreamTicket(context.Background())
	var me *eventfeed.MintError
	if !errors.As(err, &me) || me.Kind != eventfeed.MintUnrecoverable {
		t.Fatalf("error = %v, want MintUnrecoverable", err)
	}
	if strings.Contains(me.Error(), "leak") || strings.Contains(me.Error(), "evil") {
		t.Fatalf("MintError renders the refused Location: %s", me.Error())
	}
	hooks.assertNoLeak(t, "leak")
}

func TestLivePolls_CancellationPassesThrough(t *testing.T) {
	f := newLiveFixture(t, eventfeed.AccountLane, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := f.live.Polls().Poll(ctx, eventfeed.Cursor{Since: "now"}, eventfeed.Filters{})
		done <- err
	}()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled passed through", err)
		}
		var pe *eventfeed.PollError
		if errors.As(err, &pe) {
			t.Fatalf("a cancelled call was classified as %s; the connector reads the context", pe.Kind)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the cancelled poll did not return promptly")
	}
}

func TestNewLiveValidatesAndConnects(t *testing.T) {
	cfg := basecamp.DefaultConfig()
	cfg.BaseURL = "http://localhost:3001"
	live, err := eventfeed.NewLive(cfg, &basecamp.StaticTokenProvider{Token: "t"}, "1", eventfeed.InboxLane)
	if err != nil {
		t.Fatalf("NewLive: %v", err)
	}
	if live.Origin() != "http://localhost:3001" || live.Client() == nil {
		t.Fatalf("live = origin %q client %v", live.Origin(), live.Client())
	}
	if _, err := live.Connect(eventfeed.WithFilters(eventfeed.Filters{Reasons: []string{"mentioned"}})); err != nil {
		t.Fatalf("Connect on the inbox lane: %v", err)
	}
	if _, err := live.Connect(eventfeed.WithFilters(eventfeed.Filters{Creators: []int64{1}})); err == nil {
		t.Fatal("Connect accepted creators on the inbox lane")
	}
	var te *eventfeed.TerminalError
	if _, err := live.Connect(eventfeed.WithLane(eventfeed.AccountLane)); !errors.As(err, &te) || te.Reason != eventfeed.ReasonUsage {
		t.Fatalf("Connect(WithLane(AccountLane)) on an inbox binding = %v, want a usage error, not a silent override", err)
	}
	if _, err := live.Connect(eventfeed.WithLane(eventfeed.InboxLane)); err != nil {
		t.Fatalf("Connect(WithLane(InboxLane)) on an inbox binding: %v", err)
	}
	for name, tc := range map[string]struct {
		cfg  *basecamp.Config
		id   string
		lane eventfeed.Lane
	}{
		"nil config":     {nil, "1", eventfeed.AccountLane},
		"unknown lane":   {cfg, "1", eventfeed.Lane(9)},
		"cleartext base": {&basecamp.Config{BaseURL: "http://api.example.test"}, "1", eventfeed.AccountLane},
		"nonnumeric id":  {cfg, "abc", eventfeed.AccountLane},
		"userinfo base":  {&basecamp.Config{BaseURL: "https://user:s3cret-leak@api.example.test"}, "1", eventfeed.AccountLane},
		"invalid utf-8":  {&basecamp.Config{BaseURL: "https://exa\xffmple.test"}, "1", eventfeed.AccountLane},
		"no origin":      {&basecamp.Config{BaseURL: "/s3cret-leak"}, "1", eventfeed.AccountLane},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := eventfeed.NewLive(tc.cfg, &basecamp.StaticTokenProvider{Token: "t"}, tc.id, tc.lane)
			var te *eventfeed.TerminalError
			if !errors.As(err, &te) || te.Reason != eventfeed.ReasonUsage {
				t.Fatalf("NewLive error = %v, want a usage-coded construction error", err)
			}
			if strings.Contains(err.Error(), "s3cret-leak") {
				t.Fatalf("the construction error renders the base URL: %v", err)
			}
		})
	}
}
