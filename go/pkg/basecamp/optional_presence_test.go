package basecamp

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// The optional-pointer contract (SPEC.md §10) exists so callers can express
// three states: absent, explicit-empty, and non-empty. A `len(x) > 0` guard
// collapses the first two and silently drops an explicit empty array — the
// exact defect the pointer types were introduced to make impossible.

func TestQuestionScheduleToGenerated_DaysPresenceIsNilNotLength(t *testing.T) {
	if gs, err := questionScheduleToGenerated(&QuestionSchedule{Days: []int{}}); err != nil || gs == nil || gs.Days == nil {
		t.Error("explicit empty Days must reach the wire; got omitted")
	} else if len(*gs.Days) != 0 {
		t.Errorf("explicit empty Days must marshal as []; got %v", *gs.Days)
	}
	if gs, err := questionScheduleToGenerated(&QuestionSchedule{StartDate: "2025-01-01"}); err == nil && gs != nil && gs.Days != nil {
		t.Errorf("nil Days must be omitted; got %v", *gs.Days)
	}
	if gs, err := questionScheduleToGenerated(&QuestionSchedule{Days: []int{1, 3}}); err != nil || gs == nil || gs.Days == nil {
		t.Error("non-empty Days must reach the wire; got omitted")
	}
}

// Hour/Minute are *int on QuestionSchedule precisely so absence survives.
// Dereferencing the generated pointer and re-addressing the local manufactures
// a non-nil pointer to zero, which reads back as "explicitly midnight".
func TestQuestionFromGenerated_AbsentHourMinuteStayNil(t *testing.T) {
	q := questionFromGenerated(generated.Question{
		Schedule: &generated.QuestionSchedule{Frequency: ptr("every_week")},
	})
	if q.Schedule == nil {
		t.Fatal("expected a schedule")
	}
	if q.Schedule.Hour != nil {
		t.Errorf("absent hour must stay nil; got %d", *q.Schedule.Hour)
	}
	if q.Schedule.Minute != nil {
		t.Errorf("absent minute must stay nil; got %d", *q.Schedule.Minute)
	}

	explicit := questionFromGenerated(generated.Question{
		Schedule: &generated.QuestionSchedule{
			Frequency: ptr("every_week"),
			Hour:      ptr(int32(0)),
			Minute:    ptr(int32(0)),
		},
	})
	if explicit.Schedule.Hour == nil || *explicit.Schedule.Hour != 0 {
		t.Error("explicit hour 0 must survive as a non-nil zero")
	}
	if explicit.Schedule.Minute == nil || *explicit.Schedule.Minute != 0 {
		t.Error("explicit minute 0 must survive as a non-nil zero")
	}
}

// Wire-level: an explicit empty participant list must survive to the request
// body. A `len(x) > 0` guard would silently omit it, leaving the caller unable
// to express "no participants" on create.
func TestCreateScheduleEntry_ExplicitEmptyParticipantsReachTheWire(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":1,"type":"Schedule::Entry"}`))
	}))
	t.Cleanup(srv.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	svc := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}).ForAccount("99999").Schedules()

	if _, err := svc.CreateEntry(context.Background(), 1, &CreateScheduleEntryRequest{
		Summary:        "s",
		StartsAt:       "2026-08-01T09:00:00Z",
		EndsAt:         "2026-08-01T10:00:00Z",
		ParticipantIDs: []int64{},
	}); err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}

	v, ok := got["participant_ids"]
	if !ok {
		t.Fatal("explicit empty ParticipantIDs must reach the wire; key was omitted")
	}
	if arr, isArr := v.([]any); !isArr || len(arr) != 0 {
		t.Errorf("expected an empty array on the wire, got %#v", v)
	}
}

// Hill-chart settings distinguish "leave this list alone" (nil, omitted) from
// "make this list empty" (non-nil empty, transmitted as []).
func TestUpdateHillChartSettings_NilOmitsAndEmptyTransmits(t *testing.T) {
	capture := func(tracked, untracked []int64) map[string]any {
		t.Helper()
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&got)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"enabled":true,"stale":false}`))
		}))
		t.Cleanup(srv.Close)

		cfg := DefaultConfig()
		cfg.BaseURL = srv.URL
		svc := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}).ForAccount("99999").HillCharts()
		if _, err := svc.UpdateSettings(context.Background(), 1, tracked, untracked); err != nil {
			t.Fatalf("UpdateSettings: %v", err)
		}
		return got
	}

	omitted := capture(nil, nil)
	if _, ok := omitted["tracked"]; ok {
		t.Error("nil tracked must be omitted")
	}
	if _, ok := omitted["untracked"]; ok {
		t.Error("nil untracked must be omitted")
	}

	explicit := capture([]int64{}, []int64{})
	for _, key := range []string{"tracked", "untracked"} {
		v, ok := explicit[key]
		if !ok {
			t.Errorf("explicit empty %s must reach the wire; key was omitted", key)
			continue
		}
		if arr, isArr := v.([]any); !isArr || len(arr) != 0 {
			t.Errorf("%s: expected an empty array, got %#v", key, v)
		}
	}
}

// UpdateNeedle's Description is tri-state: nil leaves it alone, a pointer to
// "" clears it. A plain string could not express the clear.
func TestUpdateGaugeNeedle_DescriptionIsTriState(t *testing.T) {
	capture := func(desc *string) (map[string]any, bool) {
		t.Helper()
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&got)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":1,"type":"Gauge::Needle"}`))
		}))
		t.Cleanup(srv.Close)

		cfg := DefaultConfig()
		cfg.BaseURL = srv.URL
		svc := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}).ForAccount("99999").Gauges()
		if _, err := svc.UpdateNeedle(context.Background(), 1, &UpdateGaugeNeedleRequest{Description: desc}); err != nil {
			t.Fatalf("UpdateNeedle: %v", err)
		}
		needle, _ := got["gauge_needle"].(map[string]any)
		v, present := needle["description"]
		if !present {
			return nil, false
		}
		return map[string]any{"description": v}, true
	}

	if _, present := capture(nil); present {
		t.Error("nil Description must be omitted (no change)")
	}
	body, present := capture(ptr(""))
	if !present {
		t.Fatal(`Description: ptr("") must reach the wire to clear the description`)
	}
	if body["description"] != "" {
		t.Errorf(`expected an explicit empty string, got %#v`, body["description"])
	}
}

// Column moves send position unconditionally: BC3's Kanban::MovesController
// builds the move with params[:position].to_i, so an absent position is
// nil.to_i == 0 — identical to an explicit 0. Omitting it therefore buys
// nothing and costs the ability to name the first slot.
func TestMoveCardColumn_ZeroPositionIsTransmitted(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	svc := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}).ForAccount("99999").CardColumns()
	if err := svc.Move(context.Background(), 1, &MoveColumnRequest{SourceID: 2, TargetID: 3, Position: 0}); err != nil {
		t.Fatalf("Move: %v", err)
	}
	v, ok := got["position"]
	if !ok {
		t.Fatal("position 0 must reach the wire; key was omitted")
	}
	if f, isNum := v.(float64); !isNum || f != 0 {
		t.Errorf("expected position 0, got %#v", v)
	}
}

// A position past int32 would wrap to a negative column index on the wire.
func TestMoveCardColumn_RejectsOutOfRangePosition(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BaseURL = "http://127.0.0.1:1"
	svc := NewClient(cfg, &StaticTokenProvider{Token: "t"}).ForAccount("99999").CardColumns()
	// Computed at runtime, not as a constant: `math.MaxInt32 + 1` as an untyped
	// constant does not fit in `int` on 32-bit targets and fails to compile
	// there. Incrementing exercises the same guard on both — it exceeds
	// MaxInt32 on 64-bit and wraps negative on 32-bit, and Move rejects each.
	pos := math.MaxInt32
	pos++
	err := svc.Move(context.Background(), 1, &MoveColumnRequest{SourceID: 2, TargetID: 3, Position: pos})
	if err == nil {
		t.Fatal("expected a usage error for an out-of-range position")
	}
	// Asserted as a typed usage error, not by substring: a bare err != nil
	// would also pass on the connection error from the unreachable host.
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeUsage {
		t.Errorf("expected a usage error, got: %v", err)
	}
}

// Card and CardStep expose CompletedAt as *time.Time, so presence is already
// representable — an `!IsZero()` guard on top of the nil check threw that away,
// collapsing a present (if implausible) zero timestamp into "never completed".
func TestCardFromGenerated_PresentZeroCompletedAtSurvives(t *testing.T) {
	var zero time.Time

	card := cardFromGenerated(generated.Card{CompletedAt: &zero})
	if card.CompletedAt == nil {
		t.Error("a present zero completed_at must survive as non-nil")
	}
	step := cardStepFromGenerated(generated.CardStep{CompletedAt: &zero})
	if step.CompletedAt == nil {
		t.Error("a present zero completed_at must survive as non-nil on steps")
	}

	if absent := cardFromGenerated(generated.Card{}); absent.CompletedAt != nil {
		t.Error("an absent completed_at must stay nil")
	}
}

// Person timestamps land in string fields, so presence is carried by "" vs a
// formatted value — an !IsZero() guard collapsed a present zero timestamp into
// the same empty string an absent one produces.
func TestPersonFromGenerated_PresentZeroTimestampsPropagate(t *testing.T) {
	var zero time.Time

	// Distinct values, compared exactly: an empty-vs-non-empty check would pass
	// even if CreatedAt were sourced from UpdatedAt.
	created := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	updated := time.Date(2025, 6, 7, 8, 9, 10, 0, time.UTC)
	const layout = "2006-01-02T15:04:05Z07:00"

	p := personFromGenerated(generated.Person{CreatedAt: &created, UpdatedAt: &updated})
	if p.CreatedAt != created.Format(layout) {
		t.Errorf("CreatedAt: got %q, want %q", p.CreatedAt, created.Format(layout))
	}
	if p.UpdatedAt != updated.Format(layout) {
		t.Errorf("UpdatedAt: got %q, want %q", p.UpdatedAt, updated.Format(layout))
	}

	// And the present-zero case the pointer exists to preserve.
	pz := personFromGenerated(generated.Person{CreatedAt: &zero, UpdatedAt: &zero})
	if pz.CreatedAt == "" || pz.UpdatedAt == "" {
		t.Error("a present zero timestamp must propagate, not read as absent")
	}

	if absent := personFromGenerated(generated.Person{}); absent.CreatedAt != "" || absent.UpdatedAt != "" {
		t.Error("absent timestamps must stay empty")
	}
}

// An unparseable group_on must be reported, not silently dropped — otherwise
// the answer is created with server-default grouping and the caller is never
// told their input was invalid. Matches how the card/todo create paths behave.
func TestCreateAnswer_RejectsUnparseableGroupOn(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BaseURL = "http://127.0.0.1:1"
	svc := NewClient(cfg, &StaticTokenProvider{Token: "t"}).ForAccount("99999").Checkins()

	_, err := svc.CreateAnswer(context.Background(), 1, &CreateAnswerRequest{Content: "c", GroupOn: "not-a-date"})
	if err == nil {
		t.Fatal("expected a usage error for an unparseable group_on")
	}
	apiErr, ok := errors.AsType[*Error](err)
	if !ok || apiErr.Code != CodeUsage {
		t.Errorf("expected a usage error, got: %v", err)
	}
}

// The two halves of the webhook timestamp story, pinned so neither is
// "simplified" into the other: WebhookEvent.CreatedAt is pointer-backed and a
// present zero survives, while Recording's are @required VALUE time.Time where
// presence cannot be recovered and IsZero remains a legacy omission heuristic.
func TestWebhookEvent_PointerZeroSurvives_RecordingZeroOmitted(t *testing.T) {
	var zero time.Time

	ev := webhookEventFromGenerated(generated.WebhookEvent{
		CreatedAt: &zero,
		Recording: &generated.Recording{CreatedAt: zero, UpdatedAt: zero},
	})

	if ev.CreatedAt == "" {
		t.Error("WebhookEvent.CreatedAt is pointer-backed: a present zero must survive")
	}
	if ev.Recording.CreatedAt != "" {
		t.Errorf("Recording.CreatedAt is a value type with no recoverable presence; "+
			"a zero must stay omitted rather than emitting a year-1 timestamp, got %q", ev.Recording.CreatedAt)
	}
}

// The canonical events fixture emits "details": {} for an event with no
// membership changes. Requiring a non-nil member mapped that present-empty
// object to nil, so callers could not tell "no changes recorded" from
// "this event carries no details at all".
func TestEventFromGenerated_PresentEmptyDetailsSurvives(t *testing.T) {
	e := eventFromGenerated(generated.Event{Details: &generated.EventDetails{}})
	if e.Details == nil {
		t.Error("a present but empty details object must survive as non-nil")
	}

	if absent := eventFromGenerated(generated.Event{}); absent.Details != nil {
		t.Error("an absent details object must stay nil")
	}
}

// captureUploadBody runs fn against a server that records the JSON body it sent.
func captureUploadBody(t *testing.T, status int, respBody string, fn func(*UploadsService) error) map[string]any {
	t.Helper()
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	svc := NewClient(cfg, &StaticTokenProvider{Token: "test-token"}).ForAccount("99999").Uploads()
	if err := fn(svc); err != nil {
		t.Fatalf("request: %v", err)
	}
	return got
}

// CreateUploadVersion's Description is presence-aware server-side: omitted carries
// the previous version's description forward, "" clears it. A plain string behind
// omitzero() could not express the clear, because "" would read as unset.
func TestCreateUploadVersionRequest_DescriptionIsTriState(t *testing.T) {
	capture := func(desc *string) (any, bool) {
		t.Helper()
		body := captureUploadBody(t, http.StatusCreated, `{"id":1,"filename":"a.png"}`, func(svc *UploadsService) error {
			_, err := svc.CreateVersion(context.Background(), 1, &CreateUploadVersionRequest{
				AttachableSGID: "sgid", Description: desc,
			})
			return err
		})
		v, present := body["description"]
		return v, present
	}

	if _, present := capture(nil); present {
		t.Error("nil Description must be omitted so the previous version's description carries forward")
	}

	v, present := capture(ptr(""))
	if !present {
		t.Fatal(`Description: ptr("") must reach the wire to clear the description`)
	}
	if v != "" {
		t.Errorf(`expected description "" on the wire, got %#v`, v)
	}

	v, present = capture(ptr("<div>Set</div>"))
	if !present || v != "<div>Set</div>" {
		t.Errorf("expected the supplied description on the wire, got %#v (present=%v)", v, present)
	}
}

// UpdateUpload lands on the same serialized ActionText attribute as
// CreateUploadVersion, so its clear has to be reachable too. Leaving it a plain
// string would put one request type that can clear and one that silently cannot
// inside the same service.
func TestUpdateUploadRequest_DescriptionIsTriState(t *testing.T) {
	capture := func(desc *string) (any, bool) {
		t.Helper()
		body := captureUploadBody(t, http.StatusOK, `{"id":1,"filename":"a.png"}`, func(svc *UploadsService) error {
			_, err := svc.Update(context.Background(), 1, &UpdateUploadRequest{Description: desc})
			return err
		})
		v, present := body["description"]
		return v, present
	}

	if _, present := capture(nil); present {
		t.Error("nil Description must be omitted so the current description is left alone")
	}

	v, present := capture(ptr(""))
	if !present {
		t.Fatal(`Description: ptr("") must reach the wire to clear the description`)
	}
	if v != "" {
		t.Errorf(`expected description "" on the wire, got %#v`, v)
	}
}

// BaseName stays a plain string on both request types, deliberately.
// Upload#base_name= guards on new_base_name.present?, so "" and absent are the
// same write server-side — there is no third state a pointer could express.
func TestUploadRequests_BaseNameHasNoClearState(t *testing.T) {
	for _, tc := range []struct {
		name string
		send func(*UploadsService) error
	}{
		{"CreateVersion", func(svc *UploadsService) error {
			_, err := svc.CreateVersion(context.Background(), 1, &CreateUploadVersionRequest{
				AttachableSGID: "sgid", BaseName: "",
			})
			return err
		}},
		{"Update", func(svc *UploadsService) error {
			_, err := svc.Update(context.Background(), 1, &UpdateUploadRequest{BaseName: ""})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status := http.StatusCreated
			if tc.name == "Update" {
				status = http.StatusOK
			}
			body := captureUploadBody(t, status, `{"id":1,"filename":"a.png"}`, tc.send)
			if _, present := body["base_name"]; present {
				t.Error(`an empty BaseName must stay off the wire; "" and absent are the same server write`)
			}
		})
	}
}

// CreateUploadVersion carries the file reference that UpdateUpload deliberately
// does not — the positive counterpart to
// TestUpdateUploadRequest_HasNoFileReplacementField.
func TestCreateUploadVersionRequest_HasFileReplacementField(t *testing.T) {
	f, ok := reflect.TypeOf(CreateUploadVersionRequest{}).FieldByName("AttachableSGID")
	if !ok {
		t.Fatal("CreateUploadVersionRequest must carry AttachableSGID: it is the sanctioned file-replacement path")
	}
	if got := f.Tag.Get("json"); got != "attachable_sgid" {
		t.Errorf(`expected json tag "attachable_sgid", got %q`, got)
	}
}

// The versions partial renders details through the shared
// recordings/events/_event partial, which emits "details": {} for an event with
// no membership changes. UploadVersion has to keep the same present-empty
// distinction Event does, and must not drop the field outright.
func TestUploadVersionFromGenerated_DetailsSurvives(t *testing.T) {
	present := uploadVersionFromGenerated(generated.UploadVersion{
		Details: &generated.EventDetails{},
	})
	if present.Details == nil {
		t.Error("a present but empty details object must survive as non-nil")
	}

	if absent := uploadVersionFromGenerated(generated.UploadVersion{}); absent.Details != nil {
		t.Error("an absent details object must stay nil")
	}

	populated := uploadVersionFromGenerated(generated.UploadVersion{
		Details: &generated.EventDetails{
			AddedPersonIds:       []int64{1, 2},
			NotifiedRecipientIds: []int64{3},
		},
	})
	if populated.Details == nil {
		t.Fatal("expected details")
	}
	if len(populated.Details.AddedPersonIDs) != 2 {
		t.Errorf("added_person_ids must survive, got %v", populated.Details.AddedPersonIDs)
	}
	if len(populated.Details.NotifiedRecipientIDs) != 1 {
		t.Errorf("notified_recipient_ids must survive, got %v", populated.Details.NotifiedRecipientIDs)
	}
}

// Go has two response handlers: checkResponse for the generated service layer,
// and doRequest for the raw Client.Get/Post/Put/Delete escape hatch. A status
// mapped in one and not the other is a real divergence — the 400/422 arm in
// doRequest exists because exactly that happened with field-keyed errors.
func TestRawClientRequest_MapsInsufficientStorage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInsufficientStorage)
		_, _ = w.Write([]byte(`{"error":"The storage limit for this account has been reached."}`))
	}))
	t.Cleanup(srv.Close)

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})

	_, err := client.Get(context.Background(), "/99999/uploads/1")
	if err == nil {
		t.Fatal("expected an error on 507")
	}

	var bcErr *Error
	if !errors.As(err, &bcErr) {
		t.Fatalf("error is not *basecamp.Error: %T", err)
	}
	if bcErr.Code != CodeLimitExceeded {
		t.Errorf("code = %q, want %q", bcErr.Code, CodeLimitExceeded)
	}
	if bcErr.HTTPStatus != 507 {
		t.Errorf("http status = %d, want 507", bcErr.HTTPStatus)
	}
	if bcErr.Retryable {
		t.Error("an account limit must not be retryable")
	}
	if !strings.Contains(bcErr.Message, "storage limit") {
		t.Errorf("server message must survive, got %q", bcErr.Message)
	}
}
