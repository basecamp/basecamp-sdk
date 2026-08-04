package basecamp

import (
	"context"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
	"github.com/basecamp/basecamp-sdk/go/pkg/types"
)

// ReportsService handles reports operations.
type ReportsService struct {
	client *AccountClient
}

// NewReportsService creates a new ReportsService.
func NewReportsService(client *AccountClient) *ReportsService {
	return &ReportsService{client: client}
}

// AssignablePeople returns people who can be assigned todos.
func (s *ReportsService) AssignablePeople(ctx context.Context) (result []Person, err error) {
	op := OperationInfo{
		Service: "Reports", Operation: "AssignablePeople",
		ResourceType: "person", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListAssignablePeopleWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	people := make([]Person, 0, len(*resp.JSON200))
	for _, gp := range *resp.JSON200 {
		people = append(people, personFromGenerated(gp))
	}

	return people, nil
}

// AssignedTodosOptions specifies options for GetAssignedTodos.
type AssignedTodosOptions struct {
	// GroupBy groups results by "bucket" or "date".
	GroupBy string
}

// AssignedTodosResponse contains the assigned todos for a person.
type AssignedTodosResponse struct {
	Person    *Person `json:"person"`
	GroupedBy string  `json:"grouped_by"`
	Todos     []Todo  `json:"todos"`
}

// AssignedTodos returns todos assigned to a specific person.
func (s *ReportsService) AssignedTodos(ctx context.Context, personID int64, opts *AssignedTodosOptions) (result *AssignedTodosResponse, err error) {
	op := OperationInfo{
		Service: "Reports", Operation: "AssignedTodos",
		ResourceType: "todo", IsMutation: false,
		ResourceID: personID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.GetAssignedTodosParams
	if opts != nil && opts.GroupBy != "" {
		params = &generated.GetAssignedTodosParams{GroupBy: &opts.GroupBy}
	}

	resp, err := s.client.parent.gen.GetAssignedTodosWithResponse(ctx, s.client.accountID, personID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	result = &AssignedTodosResponse{
		GroupedBy: deref(resp.JSON200.GroupedBy),
	}

	if resp.JSON200.Person != nil {
		p := personFromGenerated(*resp.JSON200.Person)
		result.Person = &p
	}

	result.Todos = make([]Todo, 0, len(resp.JSON200.Todos))
	for _, gt := range resp.JSON200.Todos {
		result.Todos = append(result.Todos, todoFromGenerated(gt))
	}

	return result, nil
}

// OverdueTodosResponse contains overdue todos grouped by lateness.
type OverdueTodosResponse struct {
	UnderAWeekLate      []Todo `json:"under_a_week_late"`
	OverAWeekLate       []Todo `json:"over_a_week_late"`
	OverAMonthLate      []Todo `json:"over_a_month_late"`
	OverThreeMonthsLate []Todo `json:"over_three_months_late"`
}

// OverdueTodos returns all overdue todos grouped by lateness.
func (s *ReportsService) OverdueTodos(ctx context.Context) (result *OverdueTodosResponse, err error) {
	op := OperationInfo{
		Service: "Reports", Operation: "OverdueTodos",
		ResourceType: "todo", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetOverdueTodosWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	result = &OverdueTodosResponse{}

	for _, gt := range resp.JSON200.UnderAWeekLate {
		result.UnderAWeekLate = append(result.UnderAWeekLate, todoFromGenerated(gt))
	}
	for _, gt := range resp.JSON200.OverAWeekLate {
		result.OverAWeekLate = append(result.OverAWeekLate, todoFromGenerated(gt))
	}
	for _, gt := range resp.JSON200.OverAMonthLate {
		result.OverAMonthLate = append(result.OverAMonthLate, todoFromGenerated(gt))
	}
	for _, gt := range resp.JSON200.OverThreeMonthsLate {
		result.OverThreeMonthsLate = append(result.OverThreeMonthsLate, todoFromGenerated(gt))
	}

	return result, nil
}

// The upcoming-schedule report renders BC3's reduced calendar partials
// (app/views/api/schedules/calendar/), not the per-resource ones, so its items
// are NOT ScheduleEntry and Todo values with some fields left empty — they are
// different shapes with a different key set. These aliases publish the
// generated projections verbatim.
//
// They are aliases rather than hand-written mirrors on purpose. Converting a
// reduced projection into a full type is what hid the mismatch in the first
// place: a converter compiles happily while it zero-fills every field the
// endpoint never sends, so the missing `created_at`, `title`, `parent` and
// friends read back as "" and nobody learns the response never carried them.
// Aliasing removes the conversion, so the spec is the only place the shape is
// stated. See UpcomingScheduleEntry / UpcomingAssignable in spec/basecamp.smithy.
type (
	// UpcomingScheduleResponse is the report envelope: three arrays, always
	// present, possibly empty.
	UpcomingScheduleResponse = generated.GetUpcomingScheduleResponseContent
	// UpcomingScheduleEntry is a schedule entry as the calendar partial renders
	// it. Notably it carries Recurring, which no other schedule-entry
	// projection has, and it does NOT carry CreatedAt, UpdatedAt, Title,
	// InheritsStatus, Parent or DescriptionAttachments.
	UpcomingScheduleEntry = generated.UpcomingScheduleEntry
	// UpcomingAssignable is a dated to-do, card or step as the calendar partial
	// renders it. Its text is Content, not Title, and its Type is lowercase
	// ("todo", "card", "step").
	UpcomingAssignable = generated.UpcomingAssignable
	// UpcomingScheduleBucket is the project reference both calendar partials
	// emit: Id and Name only, no Type.
	UpcomingScheduleBucket = generated.UpcomingScheduleBucket
	// UpcomingSchedulePerson is the three-field person the calendar partials
	// emit: Id, Name, AvatarUrl.
	UpcomingSchedulePerson = generated.UpcomingSchedulePerson
	// UpcomingAssignableParent is the parent reference an assignable carries:
	// Id and Title only.
	UpcomingAssignableParent = generated.UpcomingAssignableParent
	// UpcomingAssignableCompletion is present only on a completed assignable.
	UpcomingAssignableCompletion = generated.UpcomingAssignableCompletion
)

// UpcomingSchedule returns the schedule entries, recurring occurrences and
// dated assignables falling in a date window. Both bounds are required and must
// be YYYY-MM-DD: BC3 reads them with params.require and answers 400 when either
// is missing, so an unbounded call has never been a thing.
//
// The returned items are the report's reduced calendar projections, not the
// full ScheduleEntry / Todo shapes — see the type aliases above.
func (s *ReportsService) UpcomingSchedule(ctx context.Context, startDate, endDate string) (result *UpcomingScheduleResponse, err error) {
	op := OperationInfo{
		Service: "Reports", Operation: "UpcomingSchedule",
		ResourceType: "schedule_entry", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Both bounds are required, so refuse an empty or malformed one locally
	// rather than spending a round-trip to be told 400.
	if _, parseErr := types.ParseDate(startDate); parseErr != nil {
		err = ErrUsage("window_starts_on is required and must be in YYYY-MM-DD format")
		return nil, err
	}
	if _, parseErr := types.ParseDate(endDate); parseErr != nil {
		err = ErrUsage("window_ends_on is required and must be in YYYY-MM-DD format")
		return nil, err
	}
	params := &generated.GetUpcomingScheduleParams{
		WindowStartsOn: startDate,
		WindowEndsOn:   endDate,
	}

	resp, err := s.client.parent.gen.GetUpcomingScheduleWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}
