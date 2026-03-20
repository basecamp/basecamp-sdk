package basecamp

import (
	"context"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// MyAssignmentsService handles my assignment operations.
type MyAssignmentsService struct {
	client *AccountClient
}

// NewMyAssignmentsService creates a new MyAssignmentsService.
func NewMyAssignmentsService(client *AccountClient) *MyAssignmentsService {
	return &MyAssignmentsService{client: client}
}

// MyAssignment represents an assignable item in the current user's assignments.
type MyAssignment struct {
	ID                  int64            `json:"id"`
	AppURL              string           `json:"app_url"`
	Content             string           `json:"content"`
	StartsOn            string           `json:"starts_on,omitempty"`
	DueOn               string           `json:"due_on,omitempty"`
	Completed           bool             `json:"completed"`
	Type                string           `json:"type"`
	CommentsCount       int              `json:"comments_count"`
	HasDescription      bool             `json:"has_description"`
	PriorityRecordingID *int64           `json:"priority_recording_id,omitempty"`
	Bucket              *MyAssignmentRef `json:"bucket,omitempty"`
	Parent              *MyAssignmentRef `json:"parent,omitempty"`
	Assignees           []Person         `json:"assignees,omitempty"`
	Children            []MyAssignment   `json:"children,omitempty"`
}

// MyAssignmentRef represents a bucket or parent reference on an assignment.
type MyAssignmentRef struct {
	ID     int64  `json:"id"`
	Name   string `json:"name,omitempty"`
	Title  string `json:"title,omitempty"`
	AppURL string `json:"app_url,omitempty"`
}

// MyAssignmentsResponse contains the current user's assignments.
type MyAssignmentsResponse struct {
	Priorities    []MyAssignment `json:"priorities"`
	NonPriorities []MyAssignment `json:"non_priorities"`
}

// List returns the current user's assignments grouped into priorities and non-priorities.
func (s *MyAssignmentsService) List(ctx context.Context) (result *MyAssignmentsResponse, err error) {
	op := OperationInfo{
		Service: "MyAssignments", Operation: "List",
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

	resp, err := s.client.parent.gen.GetMyAssignmentsWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	result = &MyAssignmentsResponse{}
	for _, ga := range resp.JSON200.Priorities {
		result.Priorities = append(result.Priorities, myAssignmentFromGenerated(ga))
	}
	for _, ga := range resp.JSON200.NonPriorities {
		result.NonPriorities = append(result.NonPriorities, myAssignmentFromGenerated(ga))
	}

	return result, nil
}

// Completed returns the current user's completed assignments.
func (s *MyAssignmentsService) Completed(ctx context.Context) (result []MyAssignment, err error) {
	op := OperationInfo{
		Service: "MyAssignments", Operation: "Completed",
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

	resp, err := s.client.parent.gen.GetMyAssignmentsCompletedWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	for _, ga := range *resp.JSON200 {
		result = append(result, myAssignmentFromGenerated(ga))
	}

	return result, nil
}

// DueOptions specifies options for listing due assignments.
type DueOptions struct {
	// Scope filters by due scope: overdue, due_today, due_tomorrow,
	// due_later_this_week, due_next_week, due_later.
	Scope string
}

// Due returns the current user's due assignments filtered by scope.
func (s *MyAssignmentsService) Due(ctx context.Context, opts *DueOptions) (result []MyAssignment, err error) {
	op := OperationInfo{
		Service: "MyAssignments", Operation: "Due",
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

	var params *generated.GetMyAssignmentsDueParams
	if opts != nil && opts.Scope != "" {
		params = &generated.GetMyAssignmentsDueParams{Scope: opts.Scope}
	}

	resp, err := s.client.parent.gen.GetMyAssignmentsDueWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	for _, ga := range *resp.JSON200 {
		result = append(result, myAssignmentFromGenerated(ga))
	}

	return result, nil
}

// myAssignmentFromGenerated converts a generated MyAssignment to our clean type.
func myAssignmentFromGenerated(ga generated.MyAssignment) MyAssignment {
	a := MyAssignment{
		ID:             ga.Id,
		AppURL:         ga.AppUrl,
		Content:        ga.Content,
		Completed:      ga.Completed,
		Type:           ga.Type,
		CommentsCount:  int(ga.CommentsCount),
		HasDescription: ga.HasDescription,
	}

	if ga.PriorityRecordingId != nil {
		a.PriorityRecordingID = ga.PriorityRecordingId
	}

	if !ga.StartsOn.IsZero() {
		a.StartsOn = ga.StartsOn.String()
	}
	if !ga.DueOn.IsZero() {
		a.DueOn = ga.DueOn.String()
	}

	if ga.Bucket.Id != 0 || ga.Bucket.Name != "" {
		a.Bucket = &MyAssignmentRef{
			ID:     ga.Bucket.Id,
			Name:   ga.Bucket.Name,
			AppURL: ga.Bucket.AppUrl,
		}
	}

	if ga.Parent.Id != 0 || ga.Parent.Title != "" {
		a.Parent = &MyAssignmentRef{
			ID:     ga.Parent.Id,
			Title:  ga.Parent.Title,
			AppURL: ga.Parent.AppUrl,
		}
	}

	for _, gp := range ga.Assignees {
		a.Assignees = append(a.Assignees, Person{
			ID:        gp.Id,
			Name:      gp.Name,
			AvatarURL: gp.AvatarUrl,
		})
	}

	for _, gc := range ga.Children {
		a.Children = append(a.Children, myAssignmentFromGenerated(gc))
	}

	return a
}
