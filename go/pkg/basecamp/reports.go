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
	if err = checkResponse(resp.HTTPResponse); err != nil {
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
		params = &generated.GetAssignedTodosParams{GroupBy: opts.GroupBy}
	}

	resp, err := s.client.parent.gen.GetAssignedTodosWithResponse(ctx, s.client.accountID, personID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	result = &AssignedTodosResponse{
		GroupedBy: resp.JSON200.GroupedBy,
	}

	if derefInt64(resp.JSON200.Person.Id) != 0 || resp.JSON200.Person.Name != "" {
		result.Person = &Person{
			ID:           derefInt64(resp.JSON200.Person.Id),
			Name:         resp.JSON200.Person.Name,
			EmailAddress: resp.JSON200.Person.EmailAddress,
			AvatarURL:    resp.JSON200.Person.AvatarUrl,
			Admin:        resp.JSON200.Person.Admin,
			Owner:        resp.JSON200.Person.Owner,
		}
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
	if err = checkResponse(resp.HTTPResponse); err != nil {
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

// Assignable represents an assignable item (todo or schedule entry).
type Assignable struct {
	ID        int64    `json:"id"`
	Title     string   `json:"title"`
	Type      string   `json:"type"`
	URL       string   `json:"url"`
	AppURL    string   `json:"app_url"`
	Bucket    *Bucket  `json:"bucket,omitempty"`
	Parent    *Parent  `json:"parent,omitempty"`
	DueOn     string   `json:"due_on,omitempty"`
	StartsOn  string   `json:"starts_on,omitempty"`
	Assignees []Person `json:"assignees,omitempty"`
}

// UpcomingScheduleResponse contains upcoming schedule entries.
type UpcomingScheduleResponse struct {
	ScheduleEntries      []ScheduleEntry `json:"schedule_entries"`
	RecurringOccurrences []ScheduleEntry `json:"recurring_schedule_entry_occurrences"`
	Assignables          []Assignable    `json:"assignables"`
}

// UpcomingSchedule returns schedule entries within a date window.
// startDate and endDate should be in YYYY-MM-DD format.
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

	var params *generated.GetUpcomingScheduleParams
	if startDate != "" || endDate != "" {
		params = &generated.GetUpcomingScheduleParams{}
		if startDate != "" {
			// Validate date format
			if _, parseErr := types.ParseDate(startDate); parseErr != nil {
				err = ErrUsage("window_starts_on must be in YYYY-MM-DD format")
				return nil, err
			}
			params.WindowStartsOn = startDate
		}
		if endDate != "" {
			// Validate date format
			if _, parseErr := types.ParseDate(endDate); parseErr != nil {
				err = ErrUsage("window_ends_on must be in YYYY-MM-DD format")
				return nil, err
			}
			params.WindowEndsOn = endDate
		}
	}

	resp, err := s.client.parent.gen.GetUpcomingScheduleWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	result = &UpcomingScheduleResponse{}

	for _, gs := range resp.JSON200.ScheduleEntries {
		result.ScheduleEntries = append(result.ScheduleEntries, scheduleEntryFromGenerated(gs))
	}
	for _, gs := range resp.JSON200.RecurringScheduleEntryOccurrences {
		result.RecurringOccurrences = append(result.RecurringOccurrences, scheduleEntryFromGenerated(gs))
	}
	for _, ga := range resp.JSON200.Assignables {
		result.Assignables = append(result.Assignables, assignableFromGenerated(ga))
	}

	return result, nil
}

// assignableFromGenerated converts a generated Assignable to our clean type.
func assignableFromGenerated(ga generated.Assignable) Assignable {
	a := Assignable{
		Title:  ga.Title,
		Type:   ga.Type,
		URL:    ga.Url,
		AppURL: ga.AppUrl,
	}

	if ga.Id != nil {
		a.ID = *ga.Id
	}

	// Convert date fields to strings
	if !ga.DueOn.IsZero() {
		a.DueOn = ga.DueOn.String()
	}
	if !ga.StartsOn.IsZero() {
		a.StartsOn = ga.StartsOn.String()
	}

	if derefInt64(ga.Bucket.Id) != 0 || ga.Bucket.Name != "" {
		a.Bucket = &Bucket{
			ID:   derefInt64(ga.Bucket.Id),
			Name: ga.Bucket.Name,
			Type: ga.Bucket.Type,
		}
	}

	if derefInt64(ga.Parent.Id) != 0 || ga.Parent.Title != "" {
		a.Parent = &Parent{
			ID:     derefInt64(ga.Parent.Id),
			Title:  ga.Parent.Title,
			Type:   ga.Parent.Type,
			URL:    ga.Parent.Url,
			AppURL: ga.Parent.AppUrl,
		}
	}

	for _, gp := range ga.Assignees {
		a.Assignees = append(a.Assignees, personFromGenerated(gp))
	}

	return a
}
