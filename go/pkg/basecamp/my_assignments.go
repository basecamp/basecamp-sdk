package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// MyAssignment represents a single assignment item.
type MyAssignment struct {
	ID                  int64                  `json:"id"`
	Type                string                 `json:"type,omitempty"`
	Content             string                 `json:"content,omitempty"`
	Completed           bool                   `json:"completed,omitempty"`
	HasDescription      bool                   `json:"has_description,omitempty"`
	DueOn               string                 `json:"due_on,omitempty"`
	StartsOn            string                 `json:"starts_on,omitempty"`
	CommentsCount       int32                  `json:"comments_count,omitempty"`
	AppURL              string                 `json:"app_url,omitempty"`
	Assignees           []MyAssignmentAssignee `json:"assignees,omitempty"`
	Bucket              MyAssignmentBucket     `json:"bucket,omitempty"`
	Parent              MyAssignmentParent     `json:"parent,omitempty"`
	Children            []MyAssignment         `json:"children,omitempty"`
	PriorityRecordingID *int64                 `json:"priority_recording_id,omitempty"`
}

// MyAssignmentAssignee represents an assignee on an assignment.
type MyAssignmentAssignee struct {
	ID        int64  `json:"id"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// MyAssignmentBucket represents the project bucket for an assignment.
type MyAssignmentBucket struct {
	ID     int64  `json:"id"`
	Name   string `json:"name,omitempty"`
	AppURL string `json:"app_url,omitempty"`
}

// MyAssignmentParent represents the parent of an assignment.
type MyAssignmentParent struct {
	ID     int64  `json:"id"`
	Title  string `json:"title,omitempty"`
	AppURL string `json:"app_url,omitempty"`
}

// MyAssignmentsResult contains priorities and non-priority assignments.
type MyAssignmentsResult struct {
	Priorities    []MyAssignment `json:"priorities,omitempty"`
	NonPriorities []MyAssignment `json:"non_priorities,omitempty"`
}

// MyAssignmentsService handles assignment operations for the current user.
type MyAssignmentsService struct {
	client *AccountClient
}

// NewMyAssignmentsService creates a new MyAssignmentsService.
func NewMyAssignmentsService(client *AccountClient) *MyAssignmentsService {
	return &MyAssignmentsService{client: client}
}

// Get returns active assignments (priorities and non-priorities) for the current user.
func (s *MyAssignmentsService) Get(ctx context.Context) (result *MyAssignmentsResult, err error) {
	op := OperationInfo{
		Service: "MyAssignments", Operation: "Get",
		ResourceType: "my_assignment", IsMutation: false,
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

	var assignments MyAssignmentsResult
	if err = json.Unmarshal(resp.Body, &assignments); err != nil {
		return nil, fmt.Errorf("failed to parse assignments: %w", err)
	}

	return &assignments, nil
}

// Completed returns completed assignments for the current user.
func (s *MyAssignmentsService) Completed(ctx context.Context) (result []MyAssignment, err error) {
	op := OperationInfo{
		Service: "MyAssignments", Operation: "Completed",
		ResourceType: "my_assignment", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetMyCompletedAssignmentsWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var assignments []MyAssignment
	if err = json.Unmarshal(resp.Body, &assignments); err != nil {
		return nil, fmt.Errorf("failed to parse completed assignments: %w", err)
	}

	return assignments, nil
}

// Due returns due assignments for the current user, optionally filtered by scope.
// Valid scope values: overdue, due_today, due_tomorrow, due_later_this_week, due_next_week, due_later.
func (s *MyAssignmentsService) Due(ctx context.Context, scope string) (result []MyAssignment, err error) {
	op := OperationInfo{
		Service: "MyAssignments", Operation: "Due",
		ResourceType: "my_assignment", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.GetMyDueAssignmentsParams
	if scope != "" {
		params = &generated.GetMyDueAssignmentsParams{
			Scope: scope,
		}
	}

	resp, err := s.client.parent.gen.GetMyDueAssignmentsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var assignments []MyAssignment
	if err = json.Unmarshal(resp.Body, &assignments); err != nil {
		return nil, fmt.Errorf("failed to parse due assignments: %w", err)
	}

	return assignments, nil
}
