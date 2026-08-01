package basecamp

import (
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
