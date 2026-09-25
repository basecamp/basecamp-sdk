package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
	"github.com/basecamp/basecamp-sdk/go/pkg/types"
)

// DefaultSubtaskLimit is the default number of subtasks to return when no
// limit is specified. It matches the 100 a to-do or card embeds under `steps`,
// so a default listing never returns less than the parent already showed.
const DefaultSubtaskLimit = 100

// A subtask is a checklist item under a to-do or a card. It was born as a
// Kanban card step and the wire keeps that history: the payload's `type` is
// "Kanban::Step" permanently and the shape is CardStep, which to-dos and cards
// also embed under Steps. SubtasksService speaks the canonical flat routes bc3
// documents in doc/api/sections/subtasks.md (basecamp/bc3#12659); the
// card-scoped /card_tables/steps spellings CardStepsService speaks stay served
// indefinitely as aliases of the same records.

// CreateSubtaskRequest specifies the parameters for creating a subtask.
type CreateSubtaskRequest struct {
	// Title is the subtask title (required).
	Title string `json:"title"`
	// DueOn is the due date in YYYY-MM-DD form (optional).
	DueOn string `json:"due_on,omitempty"`
	// AssigneeIDs is a list of person IDs to assign this subtask to (optional).
	AssigneeIDs []int64 `json:"assignee_ids,omitempty"`
}

// UpdateSubtaskRequest specifies the parameters for updating a subtask.
//
// The endpoint is a partial update: every omitted parameter is left unchanged,
// so each field here is presence-bearing. DueOn follows UpdateStepRequest: nil
// leaves the due date alone, a pointer to the empty string clears it, and a
// pointer to a date sets it. Use Ptr to build one: Ptr(""), Ptr("2026-09-20").
type UpdateSubtaskRequest struct {
	// Title is the subtask title. Empty leaves it unchanged.
	Title string `json:"title,omitempty"`
	// DueOn is the due date in YYYY-MM-DD form. Nil leaves it unchanged; a
	// pointer to "" clears it.
	DueOn *string `json:"due_on,omitempty"`
	// AssigneeIDs is a list of person IDs to assign this subtask to. Nil leaves
	// assignees unchanged; a non-nil empty slice removes everyone.
	AssigneeIDs []int64 `json:"assignee_ids,omitempty"`
}

// SubtaskListOptions specifies options for listing subtasks.
type SubtaskListOptions struct {
	// Limit is the maximum number of subtasks to return.
	// If 0, uses DefaultSubtaskLimit (100). Use -1 for unlimited.
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination:
	// exactly one request, no Link rel="next" follow (SPEC §8). A positive
	// Limit still trims that page; the per-operation default limit does not
	// apply to it. Use 0 to paginate through all results up to Limit.
	Page int
}

// SubtaskListResult contains the results from listing subtasks.
type SubtaskListResult struct {
	// Subtasks is the list of subtasks returned, in position order.
	Subtasks []CardStep
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// SubtasksService handles subtask operations.
type SubtasksService struct {
	client *AccountClient
}

// NewSubtasksService creates a new SubtasksService.
func NewSubtasksService(client *AccountClient) *SubtasksService {
	return &SubtasksService{client: client}
}

// List returns the subtasks of a recording, in position order. Only to-dos and
// cards hold subtasks; check for SubtasksCount and SubtasksURL on the parent.
//
// By default, returns up to 100 subtasks. Use Limit: -1 for unlimited.
//
// Pagination options:
//   - Limit: maximum number of subtasks to return (0 = 100, -1 = unlimited)
//   - Page: if positive, fetches only that page and disables auto-pagination
//
// The returned SubtaskListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *SubtasksService) List(ctx context.Context, recordingID int64, opts *SubtaskListOptions) (result *SubtaskListResult, err error) {
	op := OperationInfo{
		Service: "Subtasks", Operation: "List",
		ResourceType: "subtask", IsMutation: false,
		ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.ListSubtasksParams
	if opts != nil && opts.Page > 0 {
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params = &generated.ListSubtasksParams{Page: page}
	}

	resp, err := s.client.parent.gen.ListSubtasksWithResponse(ctx, s.client.accountID, recordingID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	totalCount := parseTotalCount(resp.HTTPResponse)

	var subtasks []CardStep
	if resp.JSON200 != nil {
		for _, gs := range *resp.JSON200 {
			subtasks = append(subtasks, cardStepFromGenerated(gs))
		}
	}

	if opts != nil && opts.Page > 0 {
		keep, truncated := pageCap(len(subtasks), opts.Limit, resp.HTTPResponse)
		return &SubtaskListResult{Subtasks: subtasks[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
	}

	limit := DefaultSubtaskLimit
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	if limit > 0 && len(subtasks) >= limit {
		return &SubtaskListResult{Subtasks: subtasks[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(subtasks), limit)}}, nil
	}

	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(subtasks), limit)
	if err != nil {
		return nil, err
	}

	for _, raw := range rawMore {
		var gs generated.CardStep
		if err := json.Unmarshal(raw, &gs); err != nil {
			return nil, fmt.Errorf("failed to parse subtask: %w", err)
		}
		subtasks = append(subtasks, cardStepFromGenerated(gs))
	}

	return &SubtaskListResult{Subtasks: subtasks, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Get returns a subtask by ID.
func (s *SubtasksService) Get(ctx context.Context, subtaskID int64) (result *CardStep, err error) {
	op := OperationInfo{
		Service: "Subtasks", Operation: "Get",
		ResourceType: "subtask", IsMutation: false,
		ResourceID: subtaskID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetSubtaskWithResponse(ctx, s.client.accountID, subtaskID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	subtask := cardStepFromGenerated(*resp.JSON200)
	return &subtask, nil
}

// Create creates a subtask under a to-do or a card. Any other recording answers
// 403 Forbidden. Returns the created subtask.
func (s *SubtasksService) Create(ctx context.Context, recordingID int64, req *CreateSubtaskRequest) (result *CardStep, err error) {
	op := OperationInfo{
		Service: "Subtasks", Operation: "Create",
		ResourceType: "subtask", IsMutation: true,
		ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.Title == "" {
		err = ErrUsage("subtask title is required")
		return nil, err
	}

	body := generated.CreateSubtaskJSONRequestBody{
		Title: req.Title,
	}
	if req.DueOn != "" {
		d, parseErr := types.ParseDate(req.DueOn)
		if parseErr != nil {
			err = ErrUsage("subtask due_on must be in YYYY-MM-DD format")
			return nil, err
		}
		body.DueOn = &d
	}
	if req.AssigneeIDs != nil {
		body.AssigneeIds = &req.AssigneeIDs
	}

	resp, err := s.client.parent.gen.CreateSubtaskWithResponse(ctx, s.client.accountID, recordingID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	subtask := cardStepFromGenerated(*resp.JSON201)
	return &subtask, nil
}

// Update updates a subtask. Omitted fields are left unchanged; see
// UpdateSubtaskRequest for how to clear one. Returns the updated subtask.
func (s *SubtasksService) Update(ctx context.Context, subtaskID int64, req *UpdateSubtaskRequest) (result *CardStep, err error) {
	op := OperationInfo{
		Service: "Subtasks", Operation: "Update",
		ResourceType: "subtask", IsMutation: true,
		ResourceID: subtaskID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil {
		err = ErrUsage("update request is required")
		return nil, err
	}
	if req.Title == "" && req.DueOn == nil && req.AssigneeIDs == nil {
		// bc3 answers an empty body with 400 (ParameterMissing), so refuse it here.
		err = ErrUsage("update request must set at least one field")
		return nil, err
	}

	// Hand-marshaled map, not generated.UpdateSubtaskRequestContent — the
	// SPEC §18 rule 1 carve-out CardStepsService.Update also takes: the
	// "due_on": "" clear is unreachable through *types.Date.
	body := map[string]any{}
	if req.Title != "" {
		body["title"] = req.Title
	}
	if req.AssigneeIDs != nil {
		body["assignee_ids"] = req.AssigneeIDs
	}
	if req.DueOn != nil {
		if *req.DueOn != "" {
			if _, parseErr := types.ParseDate(*req.DueOn); parseErr != nil {
				err = ErrUsage("subtask due_on must be in YYYY-MM-DD format")
				return nil, err
			}
		}
		body["due_on"] = *req.DueOn
	}

	bodyReader, err := marshalBody(body)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.parent.gen.UpdateSubtaskWithBodyWithResponse(ctx, s.client.accountID, subtaskID, "application/json", bodyReader)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	subtask := cardStepFromGenerated(*resp.JSON200)
	return &subtask, nil
}

// Complete marks a subtask as completed.
func (s *SubtasksService) Complete(ctx context.Context, subtaskID int64) (err error) {
	op := OperationInfo{
		Service: "Subtasks", Operation: "Complete",
		ResourceType: "subtask", IsMutation: true,
		ResourceID: subtaskID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.CompleteSubtaskWithResponse(ctx, s.client.accountID, subtaskID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Uncomplete marks a subtask as not completed.
func (s *SubtasksService) Uncomplete(ctx context.Context, subtaskID int64) (err error) {
	op := OperationInfo{
		Service: "Subtasks", Operation: "Uncomplete",
		ResourceType: "subtask", IsMutation: true,
		ResourceID: subtaskID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.UncompleteSubtaskWithResponse(ctx, s.client.accountID, subtaskID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Reposition moves a subtask to a new position among its siblings.
// position is 1-based (1 = top).
func (s *SubtasksService) Reposition(ctx context.Context, subtaskID int64, position int) (err error) {
	op := OperationInfo{
		Service: "Subtasks", Operation: "Reposition",
		ResourceType: "subtask", IsMutation: true,
		ResourceID: subtaskID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if position < 1 || position > math.MaxInt32 {
		err = ErrUsage("position must be between 1 and 2147483647")
		return err
	}

	body := generated.RepositionSubtaskJSONRequestBody{
		Position: int32(position), // #nosec G115 -- bounds checked above
	}

	resp, err := s.client.parent.gen.RepositionSubtaskWithResponse(ctx, s.client.accountID, subtaskID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Delete deletes a subtask. On accounts where deleting is limited to admins
// and the creator, everyone else gets 403 Forbidden.
func (s *SubtasksService) Delete(ctx context.Context, subtaskID int64) (err error) {
	op := OperationInfo{
		Service: "Subtasks", Operation: "Delete",
		ResourceType: "subtask", IsMutation: true,
		ResourceID: subtaskID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DeleteSubtaskWithResponse(ctx, s.client.accountID, subtaskID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}
