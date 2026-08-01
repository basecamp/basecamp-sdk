package basecamp

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// The optional-pointer contract (SPEC.md §10) exists so callers can express
// three states: absent, explicit-empty, and non-empty. A `len(x) > 0` guard
// collapses the first two and silently drops an explicit empty array — the
// exact defect the pointer types were introduced to make impossible.

func TestQuestionScheduleToMap_DaysPresenceIsNilNotLength(t *testing.T) {
	if m := questionScheduleToMap(&QuestionSchedule{Days: []int{}}); m["days"] == nil {
		t.Error("explicit empty Days must reach the wire; got omitted")
	}
	if m := questionScheduleToMap(&QuestionSchedule{}); m["days"] != nil {
		t.Errorf("nil Days must be omitted; got %v", m["days"])
	}
	if m := questionScheduleToMap(&QuestionSchedule{Days: []int{1, 3}}); m["days"] == nil {
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
	err := svc.Move(context.Background(), 1, &MoveColumnRequest{SourceID: 2, TargetID: 3, Position: math.MaxInt32 + 1})
	if err == nil {
		t.Fatal("expected a usage error for an out-of-range position")
	}
	if !strings.Contains(err.Error(), "position must be between") {
		t.Errorf("expected a range error, got %v", err)
	}
}
