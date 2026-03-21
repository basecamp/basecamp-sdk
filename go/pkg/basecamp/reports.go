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
		params = &generated.GetAssignedTodosParams{GroupBy: opts.GroupBy}
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
		GroupedBy: resp.JSON200.GroupedBy,
	}

	if resp.JSON200.Person.Id != 0 || resp.JSON200.Person.Name != "" {
		result.Person = &Person{
			ID:           resp.JSON200.Person.Id,
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

// MyAssignmentPerson represents a minimal assignee in the assignments response.
type MyAssignmentPerson struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// MyAssignmentBucket represents the project bucket for an assignment.
type MyAssignmentBucket struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	AppURL string `json:"app_url"`
}

// MyAssignmentParent represents the parent container for an assignment.
type MyAssignmentParent struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	AppURL string `json:"app_url"`
}

// MyAssignment represents an assignment returned from /my/assignments endpoints.
type MyAssignment struct {
	ID                  int64                `json:"id"`
	AppURL              string               `json:"app_url"`
	Content             string               `json:"content"`
	StartsOn            string               `json:"starts_on,omitempty"`
	DueOn               string               `json:"due_on,omitempty"`
	Bucket              *MyAssignmentBucket  `json:"bucket,omitempty"`
	Completed           bool                 `json:"completed"`
	Type                string               `json:"type"`
	Assignees           []MyAssignmentPerson `json:"assignees,omitempty"`
	CommentsCount       int                  `json:"comments_count"`
	HasDescription      bool                 `json:"has_description"`
	PriorityRecordingID *int64               `json:"priority_recording_id,omitempty"`
	Parent              *MyAssignmentParent  `json:"parent,omitempty"`
	Children            []MyAssignment       `json:"children,omitempty"`
}

// AssignmentsResponse contains active assignments split by priority.
type AssignmentsResponse struct {
	Priorities    []MyAssignment `json:"priorities"`
	NonPriorities []MyAssignment `json:"non_priorities"`
}

// DueAssignmentsOptions specifies query parameters for due assignments.
type DueAssignmentsOptions struct {
	// Scope filters by due date range.
	Scope string
}

// Assignments returns the current user's active assignments grouped by priority.
func (s *ReportsService) Assignments(ctx context.Context) (result *AssignmentsResponse, err error) {
	op := OperationInfo{
		Service: "Reports", Operation: "Assignments",
		ResourceType: "assignment", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetAssignmentsWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	result = &AssignmentsResponse{
		Priorities:    make([]MyAssignment, 0, len(resp.JSON200.Priorities)),
		NonPriorities: make([]MyAssignment, 0, len(resp.JSON200.NonPriorities)),
	}

	for _, gm := range resp.JSON200.Priorities {
		result.Priorities = append(result.Priorities, myAssignmentFromGenerated(gm))
	}
	for _, gm := range resp.JSON200.NonPriorities {
		result.NonPriorities = append(result.NonPriorities, myAssignmentFromGenerated(gm))
	}

	return result, nil
}

// CompletedAssignments returns the current user's completed assignments.
func (s *ReportsService) CompletedAssignments(ctx context.Context) (result []MyAssignment, err error) {
	op := OperationInfo{
		Service: "Reports", Operation: "CompletedAssignments",
		ResourceType: "completed_assignment", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetCompletedAssignmentsWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	result = make([]MyAssignment, 0, len(*resp.JSON200))
	for _, gm := range *resp.JSON200 {
		result = append(result, myAssignmentFromGenerated(gm))
	}

	return result, nil
}

// DueAssignments returns the current user's assignments filtered by due-date scope.
func (s *ReportsService) DueAssignments(ctx context.Context, opts *DueAssignmentsOptions) (result []MyAssignment, err error) {
	op := OperationInfo{
		Service: "Reports", Operation: "DueAssignments",
		ResourceType: "due_assignment", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.GetDueAssignmentsParams
	if opts != nil && opts.Scope != "" {
		params = &generated.GetDueAssignmentsParams{Scope: opts.Scope}
	}

	resp, err := s.client.parent.gen.GetDueAssignmentsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	result = make([]MyAssignment, 0, len(*resp.JSON200))
	for _, gm := range *resp.JSON200 {
		result = append(result, myAssignmentFromGenerated(gm))
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
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
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

// myAssignmentFromGenerated converts a generated MyAssignment to our clean type.
func myAssignmentFromGenerated(gm generated.MyAssignment) MyAssignment {
	m := MyAssignment{
		ID:             gm.Id,
		AppURL:         gm.AppUrl,
		Content:        gm.Content,
		Completed:      gm.Completed,
		Type:           gm.Type,
		CommentsCount:  int(gm.CommentsCount),
		HasDescription: gm.HasDescription,
	}

	if !gm.StartsOn.IsZero() {
		m.StartsOn = gm.StartsOn.String()
	}
	if !gm.DueOn.IsZero() {
		m.DueOn = gm.DueOn.String()
	}
	if gm.PriorityRecordingId != nil {
		id := *gm.PriorityRecordingId
		m.PriorityRecordingID = &id
	}

	if gm.Bucket.Id != 0 || gm.Bucket.Name != "" || gm.Bucket.AppUrl != "" {
		m.Bucket = &MyAssignmentBucket{
			ID:     gm.Bucket.Id,
			Name:   gm.Bucket.Name,
			AppURL: gm.Bucket.AppUrl,
		}
	}

	if gm.Parent.Id != 0 || gm.Parent.Title != "" || gm.Parent.AppUrl != "" {
		m.Parent = &MyAssignmentParent{
			ID:     gm.Parent.Id,
			Title:  gm.Parent.Title,
			AppURL: gm.Parent.AppUrl,
		}
	}

	if len(gm.Assignees) > 0 {
		m.Assignees = make([]MyAssignmentPerson, 0, len(gm.Assignees))
		for _, gp := range gm.Assignees {
			m.Assignees = append(m.Assignees, MyAssignmentPerson{
				ID:   gp.Id,
				Name: gp.Name,
			})
		}
	}

	if len(gm.Children) > 0 {
		m.Children = make([]MyAssignment, 0, len(gm.Children))
		for _, gc := range gm.Children {
			m.Children = append(m.Children, myAssignmentFromGenerated(gc))
		}
	}

	return m
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

	if ga.Bucket.Id != 0 || ga.Bucket.Name != "" {
		a.Bucket = &Bucket{
			ID:   ga.Bucket.Id,
			Name: ga.Bucket.Name,
			Type: ga.Bucket.Type,
		}
	}

	if ga.Parent.Id != 0 || ga.Parent.Title != "" {
		a.Parent = &Parent{
			ID:     ga.Parent.Id,
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
