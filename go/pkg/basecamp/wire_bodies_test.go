package basecamp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/types"
)

// This file pins the exact bytes the six former body-map sites put on the
// wire (#653). Every expectation was captured from the hand-marshaled map
// implementations before the sites moved to the generated request types: the
// tests are the invariant, the swap is the change. Byte-level equality is
// deliberate — key order, explicit-empty values ("" clears, [] lists), and
// the presence or absence of every omitted member are the contract being
// preserved. Shapes cover every branch of the old map-building code.

// captureBodies wires a handler that records the raw body of every non-GET
// request and answers GETs with getBody and writes with putBody.
func captureBodies(t *testing.T, bodies *[]string, getBody, putBody string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(getBody))
			return
		}
		b, _ := io.ReadAll(r.Body)
		*bodies = append(*bodies, string(b))
		w.WriteHeader(200)
		_, _ = w.Write([]byte(putBody))
	}
}

func TestTodosReplaceBodyBytes(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(svc *TodosService) error
		want string
	}{
		{
			// Replace: every optional branch of the sharp builder set.
			name: "replace full",
			call: func(svc *TodosService) error {
				_, err := svc.Replace(context.Background(), 42, &ReplaceTodoRequest{
					Content:                 "Ship it",
					Description:             "Do the thing",
					AssigneeIDs:             []int64{1, 2},
					CompletionSubscriberIDs: []int64{3},
					Notify:                  true,
					DueOn:                   "2025-04-01",
					StartsOn:                "2025-03-01",
				})
				return err
			},
			want: `{"assignee_ids":[1,2],"completion_subscriber_ids":[3],"content":"Ship it","description":"Do the thing","due_on":"2025-04-01","notify":true,"starts_on":"2025-03-01"}`,
		},
		{
			// Replace: every optional branch omitted.
			name: "replace minimal",
			call: func(svc *TodosService) error {
				_, err := svc.Replace(context.Background(), 42, &ReplaceTodoRequest{Content: "Ship it"})
				return err
			},
			want: `{"content":"Ship it"}`,
		},
		{
			// Edit clearing everything: fullBody must send description "" and
			// both ID lists as explicit [] (clears survive the PUT), and omit
			// the empty dates and false notify.
			name: "edit clear all",
			call: func(svc *TodosService) error {
				_, err := svc.Edit(context.Background(), 42, func(f *TodoFields) error {
					f.Content = "Keep"
					f.Description = ""
					f.AssigneeIDs = nil
					f.CompletionSubscriberIDs = nil
					f.DueOn = ""
					f.StartsOn = ""
					f.Notify = false
					return nil
				})
				return err
			},
			want: `{"assignee_ids":[],"completion_subscriber_ids":[],"content":"Keep","description":""}`,
		},
		{
			// Edit setting the date/notify branches of fullBody.
			name: "edit set dates and notify",
			call: func(svc *TodosService) error {
				_, err := svc.Edit(context.Background(), 42, func(f *TodoFields) error {
					f.Content = "C"
					f.Description = "d"
					f.AssigneeIDs = []int64{1}
					f.DueOn = "2025-04-01"
					f.StartsOn = "2025-03-01"
					f.Notify = true
					return nil
				})
				return err
			},
			want: `{"assignee_ids":[1],"completion_subscriber_ids":[],"content":"C","description":"d","due_on":"2025-04-01","notify":true,"starts_on":"2025-03-01"}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var bodies []string
			svc := testTodosServer(t, captureBodies(t, &bodies,
				`{"id":42,"content":"Old","description":"old desc"}`,
				`{"id":42,"content":"x"}`))
			if err := tc.call(svc); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(bodies) != 1 {
				t.Fatalf("expected 1 write, got %d", len(bodies))
			}
			if bodies[0] != tc.want {
				t.Errorf("body = %s, want %s", bodies[0], tc.want)
			}
		})
	}
}

func TestDocumentsReplaceBodyBytes(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(svc *DocumentsService) error
		want string
	}{
		{
			name: "replace title only",
			call: func(svc *DocumentsService) error {
				title := "T2"
				_, err := svc.Replace(context.Background(), 7, &ReplaceDocumentRequest{Title: &title})
				return err
			},
			want: `{"title":"T2"}`,
		},
		{
			name: "replace content only",
			call: func(svc *DocumentsService) error {
				content := "New words"
				_, err := svc.Replace(context.Background(), 7, &ReplaceDocumentRequest{Content: &content})
				return err
			},
			want: `{"content":"New words"}`,
		},
		{
			// Edit clearing content: fullBody always sends both fields,
			// empties included — "" is the clear on a full-replace endpoint.
			name: "edit clear content",
			call: func(svc *DocumentsService) error {
				_, err := svc.Edit(context.Background(), 7, func(f *DocumentFields) error {
					f.Content = ""
					return nil
				})
				return err
			},
			want: `{"content":"","title":"Doc"}`,
		},
		{
			// Update overlay: read-back content resent verbatim.
			name: "update overlay",
			call: func(svc *DocumentsService) error {
				_, err := svc.Update(context.Background(), 7, &UpdateDocumentRequest{Title: "New"})
				return err
			},
			want: `{"content":"old","title":"New"}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var bodies []string
			svc := testDocumentsServer(t, captureBodies(t, &bodies,
				`{"id":7,"title":"Doc","content":"old"}`,
				`{"id":7,"title":"t","content":"c"}`))
			if err := tc.call(svc); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(bodies) != 1 {
				t.Fatalf("expected 1 write, got %d", len(bodies))
			}
			if bodies[0] != tc.want {
				t.Errorf("body = %s, want %s", bodies[0], tc.want)
			}
		})
	}
}

func TestSchedulesReplaceEntryBodyBytes(t *testing.T) {
	strp := func(s string) *string { return &s }
	boolp := func(b bool) *bool { return &b }

	for _, tc := range []struct {
		name string
		call func(svc *SchedulesService) error
		want string
	}{
		{
			name: "replace minimal",
			call: func(svc *SchedulesService) error {
				_, err := svc.ReplaceEntry(context.Background(), 9, &ReplaceScheduleEntryRequest{
					StartsAt: strp("2026-06-01"),
					EndsAt:   strp("2026-06-02"),
				})
				return err
			},
			want: `{"ends_at":"2026-06-02","starts_at":"2026-06-01"}`,
		},
		{
			// Every optional branch of the sharp builder set, including a
			// false highlighted (an explicit write, not an omission).
			name: "replace full",
			call: func(svc *SchedulesService) error {
				ids := []int64{1, 2}
				_, err := svc.ReplaceEntry(context.Background(), 9, &ReplaceScheduleEntryRequest{
					Summary:        strp("S"),
					StartsAt:       strp("2026-06-01"),
					EndsAt:         strp("2026-06-02"),
					Description:    strp("d"),
					AllDay:         boolp(true),
					ParticipantIDs: &ids,
					Notify:         boolp(true),
					URL:            strp("https://call.example"),
					Highlighted:    boolp(false),
				})
				return err
			},
			want: `{"all_day":true,"description":"d","ends_at":"2026-06-02","highlighted":false,"notify":true,"participant_ids":[1,2],"starts_at":"2026-06-01","summary":"S","url":"https://call.example"}`,
		},
		{
			// A pointer to a nil slice is the explicit "remove everyone", and
			// it must reach the wire as [] rather than null.
			name: "replace explicit empty participants",
			call: func(svc *SchedulesService) error {
				var ids []int64
				_, err := svc.ReplaceEntry(context.Background(), 9, &ReplaceScheduleEntryRequest{
					StartsAt:       strp("2026-06-01"),
					EndsAt:         strp("2026-06-02"),
					ParticipantIDs: &ids,
				})
				return err
			},
			want: `{"ends_at":"2026-06-02","participant_ids":[],"starts_at":"2026-06-01"}`,
		},
		{
			// EditEntry: fullBody sends the five full-state fields always,
			// empties included, plus exactly the carve-outs the callback
			// touched — here an [] participants clear and a "" url clear.
			name: "edit touched carve-outs",
			call: func(svc *SchedulesService) error {
				_, err := svc.EditEntry(context.Background(), 9, func(f *ScheduleEntryFields) error {
					f.Description = ""
					f.SetParticipantIDs(nil)
					f.SetURL("")
					return nil
				})
				return err
			},
			want: `{"all_day":false,"description":"","ends_at":"2026-06-01T10:00:00Z","participant_ids":[],"starts_at":"2026-06-01T09:00:00Z","summary":"S","url":""}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var bodies []string
			svc := testSchedulesServer(t, captureBodies(t, &bodies,
				`{"id":9,"summary":"S","starts_at":"2026-06-01T09:00:00Z","ends_at":"2026-06-01T10:00:00Z","description":"ignored","all_day":false}`,
				`{"id":9,"summary":"S"}`))
			if err := tc.call(svc); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(bodies) != 1 {
				t.Fatalf("expected 1 write, got %d", len(bodies))
			}
			if bodies[0] != tc.want {
				t.Errorf("body = %s, want %s", bodies[0], tc.want)
			}
		})
	}
}

func TestTodolistsReplaceBodyBytes(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(t *testing.T, bodies *[]string) error
		want string
	}{
		{
			name: "replace name only",
			call: func(t *testing.T, bodies *[]string) error {
				svc := testTodolistsServer(t, captureBodies(t, bodies,
					`{"id":3,"name":"List","description":"old"}`,
					`{"id":3,"name":"L"}`))
				_, err := svc.Replace(context.Background(), 3, &ReplaceTodolistRequest{Name: "L"})
				return err
			},
			want: `{"name":"L"}`,
		},
		{
			name: "replace with description",
			call: func(t *testing.T, bodies *[]string) error {
				svc := testTodolistsServer(t, captureBodies(t, bodies,
					`{"id":3,"name":"List","description":"old"}`,
					`{"id":3,"name":"L"}`))
				_, err := svc.Replace(context.Background(), 3, &ReplaceTodolistRequest{Name: "L", Description: "d"})
				return err
			},
			want: `{"description":"d","name":"L"}`,
		},
		{
			// Edit clearing the description: fullBody always sends both
			// fields, so the "" clear reaches the wire present-and-empty.
			name: "edit clear description",
			call: func(t *testing.T, bodies *[]string) error {
				svc := testTodolistsServer(t, captureBodies(t, bodies,
					`{"id":3,"name":"List","description":"old"}`,
					`{"id":3,"name":"List"}`))
				_, err := svc.Edit(context.Background(), 3, func(f *TodolistFields) error {
					f.Description = ""
					return nil
				})
				return err
			},
			want: `{"description":"","name":"List"}`,
		},
		{
			name: "group replace with description",
			call: func(t *testing.T, bodies *[]string) error {
				svc := testTodolistGroupsServer(t, captureBodies(t, bodies,
					`{"id":4,"name":"Group","description":"old"}`,
					`{"id":4,"name":"G"}`))
				_, err := svc.Replace(context.Background(), 4, &ReplaceTodolistGroupRequest{Name: "G", Description: "gd"})
				return err
			},
			want: `{"description":"gd","name":"G"}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var bodies []string
			if err := tc.call(t, &bodies); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(bodies) != 1 {
				t.Fatalf("expected 1 write, got %d", len(bodies))
			}
			if bodies[0] != tc.want {
				t.Errorf("body = %s, want %s", bodies[0], tc.want)
			}
		})
	}
}

func TestPeopleUpdateMyProfileBodyBytes(t *testing.T) {
	strp := func(s string) *string { return &s }

	for _, tc := range []struct {
		name string
		req  *UpdateMyProfileRequest
		want string
	}{
		{
			// Every branch set, including an explicit "" clear for bio: a
			// non-nil pointer to the empty string must reach the wire.
			name: "full with explicit empty bio",
			req: &UpdateMyProfileRequest{
				Name:         strp("N"),
				EmailAddress: strp("e@x.co"),
				Title:        strp("Dev"),
				Bio:          strp(""),
				Location:     strp("Chi"),
				TimeZoneName: strp("America/Chicago"),
				FirstWeekDay: func() *FirstWeekDay { d := FirstWeekDayMonday; return &d }(),
				TimeFormat:   strp("12h"),
			},
			want: `{"bio":"","email_address":"e@x.co","first_week_day":"Monday","location":"Chi","name":"N","time_format":"12h","time_zone_name":"America/Chicago","title":"Dev"}`,
		},
		{
			name: "sparse",
			req:  &UpdateMyProfileRequest{Name: strp("N")},
			want: `{"name":"N"}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var bodies []string
			svc := testPeopleServer(t, captureBodies(t, &bodies, `{}`, `{}`))
			if err := svc.UpdateMyProfile(context.Background(), tc.req); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(bodies) != 1 {
				t.Fatalf("expected 1 write, got %d", len(bodies))
			}
			if bodies[0] != tc.want {
				t.Errorf("body = %s, want %s", bodies[0], tc.want)
			}
		})
	}
}

func TestUpdateAnswerBodyBytes(t *testing.T) {
	var bodies []string
	svc := testCheckinsServer(t, captureBodies(t, &bodies, `{}`, `{}`))
	err := svc.UpdateAnswer(context.Background(), 5, &UpdateAnswerRequest{
		Content: "Done",
		GroupOn: "2025-03-01",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bodies) != 1 {
		t.Fatalf("expected 1 write, got %d", len(bodies))
	}
	const want = `{"content":"Done","group_on":"2025-03-01"}`
	if bodies[0] != want {
		t.Errorf("body = %s, want %s", bodies[0], want)
	}
}

// TestDatePointerCannotSpellEmptyDueOnClear proves the inexpressibility that
// keeps CardsService.UpdateVerbatim and CardStepsService.Update on hand-
// marshaled maps (SPEC §18 rule 1 carve-out): their explicit due-date clear
// is spelled `"due_on": ""` — the one clear encoding all six SDKs can produce
// identically, since body compaction (SPEC §18) strips JSON nulls in five of
// them, and BC3 blank-casts "" to nil (basecamp/bc3#12521). A `*types.Date`
// member has exactly three spellings — absent (nil pointer), null (zero
// Date), and a real date — so `""` is unreachable through the generated
// request type, and switching the wire to null would both violate the body-
// compaction rule and change the pinned cross-SDK contract.
func TestDatePointerCannotSpellEmptyDueOnClear(t *testing.T) {
	type body struct {
		DueOn *types.Date `json:"due_on,omitempty"`
	}

	if b, _ := json.Marshal(body{}); string(b) != `{}` {
		t.Errorf("nil pointer: got %s, want {}", b)
	}
	if b, _ := json.Marshal(body{DueOn: &types.Date{}}); string(b) != `{"due_on":null}` {
		t.Errorf("zero Date: got %s, want {\"due_on\":null}", b)
	}
	d, err := types.ParseDate("2025-04-01")
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := json.Marshal(body{DueOn: &d}); string(b) != `{"due_on":"2025-04-01"}` {
		t.Errorf("parsed Date: got %s, want {\"due_on\":\"2025-04-01\"}", b)
	}
	// The three cases above are exhaustive over *types.Date's states; none of
	// them is `{"due_on":""}`, which is the encoding the card sites must send.
}
