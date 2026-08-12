package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// TodolistGroupListOptions specifies options for listing todolist groups.
type TodolistGroupListOptions struct {
	// Limit is the maximum number of todolist groups to return.
	// If 0, returns all. Use -1 for unlimited (same as 0).
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination.
	Page int
}

// TodolistGroup is a group inside a to-do list. It is an alias, not a
// separate type: BC3 has no group model at all. A group is a Todolist whose
// parent is a Todolist (Todolist#group?), there is no Todolist::Group class,
// and todolists/groups/{index,show}.json.jbuilder render the very same
// todolists/_todolist.json.jbuilder partial — so a group carries description
// and description_attachments like any list and reports Type "Todolist".
//
// The name is kept because TodolistGroupsService reads better with it, and
// because the group-scoped routes are real even though the shape is not. Every
// value here is a Todolist and can be passed anywhere one is expected.
//
// Discriminate structurally, never on Type:
//
//   - GroupsURL non-empty → a to-do list; Parent is a Todoset.
//   - GroupPositionURL non-empty → a group; Parent is a Todolist.
//
// See Todolist for the field documentation.
type TodolistGroup = Todolist

// CreateTodolistGroupRequest specifies the parameters for creating a todolist group.
type CreateTodolistGroupRequest struct {
	// Name is the group name (required).
	Name string `json:"name"`
}

// ReplaceTodolistGroupRequest specifies the new complete representation of a
// todolist group for TodolistGroupsService.Replace. Omitted fields are
// cleared server-side.
//
// A group is written through the polymorphic todolists endpoint, so BC3
// answers it with TodolistsController#update, which rebuilds the recordable
// from only the permitted params ({name, description}) — a group is just a
// Todolist whose parent is a Todolist, and it carries a description like any
// other. Omitting the description therefore erases it.
//
// Description round-trips: the response is the flat Todolist shape (#544), so
// the TodolistGroup that comes back carries the description that was written.
type ReplaceTodolistGroupRequest struct {
	// Name is the group name (required). It is always sent — the server
	// presence-validates it, so omitting it is a 422, not a preserve.
	Name string `json:"name"`
	// Description is an optional description (can include HTML). Omitting it
	// clears the group's description.
	Description string `json:"description,omitempty"`
}

// TodolistGroupListResult contains the results from listing todolist groups.
type TodolistGroupListResult struct {
	// Groups is the list of todolist groups returned.
	Groups []TodolistGroup
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// TodolistGroupsService handles todolist group operations.
type TodolistGroupsService struct {
	client *AccountClient
}

// NewTodolistGroupsService creates a new TodolistGroupsService.
func NewTodolistGroupsService(client *AccountClient) *TodolistGroupsService {
	return &TodolistGroupsService{client: client}
}

// List returns all groups in a todolist.
//
// Pagination options:
//   - Limit: maximum number of todolist groups to return (0 = all, -1 = unlimited)
//   - Page: if positive, fetches only that page and disables auto-pagination
//
// The returned TodolistGroupListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *TodolistGroupsService) List(ctx context.Context, todolistID int64, opts *TodolistGroupListOptions) (result *TodolistGroupListResult, err error) {
	op := OperationInfo{
		Service: "TodolistGroups", Operation: "List",
		ResourceType: "todolist_group", IsMutation: false,
		ResourceID: todolistID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.ListTodolistGroupsParams
	if opts != nil && opts.Page > 0 {
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params = &generated.ListTodolistGroupsParams{Page: page}
	}

	resp, err := s.client.parent.gen.ListTodolistGroupsWithResponse(ctx, s.client.accountID, todolistID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var groups []TodolistGroup
	if resp.JSON200 != nil {
		for _, gg := range *resp.JSON200 {
			groups = append(groups, todolistFromGenerated(gg))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		keep, truncated := pageCap(len(groups), opts.Limit, resp.HTTPResponse)
		return &TodolistGroupListResult{Groups: groups[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
	}

	// Determine limit: 0 = all (no limit)
	limit := 0
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0 // unlimited
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	// Check if we already have enough items
	if limit > 0 && len(groups) >= limit {
		return &TodolistGroupListResult{Groups: groups[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(groups), limit)}}, nil
	}

	// Follow pagination via Link headers
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(groups), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gg generated.Todolist
		if err := json.Unmarshal(raw, &gg); err != nil {
			return nil, fmt.Errorf("failed to parse todolist group: %w", err)
		}
		groups = append(groups, todolistFromGenerated(gg))
	}

	return &TodolistGroupListResult{Groups: groups, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Get returns a todolist group by ID.
func (s *TodolistGroupsService) Get(ctx context.Context, groupID int64) (result *TodolistGroup, err error) {
	op := OperationInfo{
		Service: "TodolistGroups", Operation: "Get",
		ResourceType: "todolist_group", IsMutation: false,
		ResourceID: groupID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Groups are fetched via the todolists endpoint (polymorphic endpoint)
	resp, err := s.client.parent.gen.GetTodolistOrGroupWithResponse(ctx, s.client.accountID, groupID)
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

	// One flat shape (#544): the group and the list are the same
	// generated.Todolist, so the generated parser's decode is the decode.
	group := todolistFromGenerated(*resp.JSON200)
	return &group, nil
}

// Create creates a new group in a todolist.
// Returns the created group.
func (s *TodolistGroupsService) Create(ctx context.Context, todolistID int64, req *CreateTodolistGroupRequest) (result *TodolistGroup, err error) {
	op := OperationInfo{
		Service: "TodolistGroups", Operation: "Create",
		ResourceType: "todolist_group", IsMutation: true,
		ResourceID: todolistID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req.Name == "" {
		err = ErrUsage("group name is required")
		return nil, err
	}

	body := generated.CreateTodolistGroupJSONRequestBody{
		Name: req.Name,
	}

	resp, err := s.client.parent.gen.CreateTodolistGroupWithResponse(ctx, s.client.accountID, todolistID, body)
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

	group := todolistFromGenerated(*resp.JSON201)
	return &group, nil
}

// Replace sends the request verbatim as the group's new complete
// representation — the server's native PUT semantics. No GET is issued, and
// any field omitted from the request is cleared server-side.
//
// Groups are written through the todolists endpoint (polymorphic endpoint),
// so BC3 answers with TodolistsController#update, which rebuilds the
// recordable from only the permitted params. A missing description therefore
// clears the group's description. Name is required.
//
// There is deliberately no merge-safe Update or Edit on this service, and the
// reason is no longer data loss. Before #544 the group projection modelled no
// description, so a composite built on TodolistGroupsService.Get would have
// read "" for it and PUT that back, erasing it on every call. TodolistGroup is
// now an alias for Todolist and carries the field, so that hazard is gone. The
// composite still is not built because the other five SDKs expose no group
// write of any kind, and Todolists().Update / Todolists().Edit already address
// this exact PUT /{accountId}/todolists/{id} route through the same
// variant-agnostic projection — a sixth spelling of that composite would widen
// a cross-SDK asymmetry rather than close a gap. Merge-safe group writes go
// through Todolists().Update or Todolists().Edit, which preserve a group's
// {name, description} exactly as they do a list's.
//
// Description is offered on the request because the wire body is shared with
// todolists and accepts it: without it a group replace would be
// unconditionally destructive with no caller recourse. It round-trips — the
// returned TodolistGroup carries the description back, because the response is
// the same flat Todolist shape.
// Returns the updated group.
func (s *TodolistGroupsService) Replace(ctx context.Context, groupID int64, req *ReplaceTodolistGroupRequest) (*TodolistGroup, error) {
	return s.replaceGroup(ctx, groupID, func() (generated.UpdateTodolistOrGroupJSONRequestBody, error) {
		if req == nil {
			return generated.UpdateTodolistOrGroupJSONRequestBody{}, ErrUsage("replace request is required")
		}
		if req.Name == "" {
			return generated.UpdateTodolistOrGroupJSONRequestBody{}, ErrUsage("group name is required")
		}
		body := generated.UpdateTodolistOrGroupJSONRequestBody{Name: req.Name}
		if req.Description != "" {
			body.Description = &req.Description
		}
		return body, nil
	})
}

// replaceGroup is the single transport for the UpdateTodolistOrGroup wire
// operation as TodolistGroupsService issues it. It pins the
// TodolistGroups.Replace hook identity; the envelope, the one generated-client
// call site, and the decode live in replaceTodolistOrGroup, shared with
// TodolistsService. Only the hook identity differs — since #544 both services
// project the same flat shape.
func (s *TodolistGroupsService) replaceGroup(ctx context.Context, groupID int64, buildBody func() (generated.UpdateTodolistOrGroupJSONRequestBody, error)) (*TodolistGroup, error) {
	op := OperationInfo{
		Service: "TodolistGroups", Operation: "Replace",
		ResourceType: "todolist_group", IsMutation: true,
		ResourceID: groupID,
	}

	gg, err := replaceTodolistOrGroup(ctx, s.client, op, groupID, buildBody)
	if err != nil {
		return nil, err
	}

	group := todolistFromGenerated(gg)
	return &group, nil
}

// Reposition changes the position of a group within its todolist.
// position is 1-based (1 = first position).
func (s *TodolistGroupsService) Reposition(ctx context.Context, groupID int64, position int) (err error) {
	op := OperationInfo{
		Service: "TodolistGroups", Operation: "Reposition",
		ResourceType: "todolist_group", IsMutation: true,
		ResourceID: groupID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if position < 1 {
		err = ErrUsage("position must be at least 1")
		return err
	}

	body := generated.RepositionTodolistGroupJSONRequestBody{
		Position: int32(position), // #nosec G115 -- position is validated and bounded by API
	}

	resp, err := s.client.parent.gen.RepositionTodolistGroupWithResponse(ctx, s.client.accountID, groupID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Trash moves a todolist group to the trash.
// Trashed groups can be recovered from the trash.
func (s *TodolistGroupsService) Trash(ctx context.Context, groupID int64) (err error) {
	op := OperationInfo{
		Service: "TodolistGroups", Operation: "Trash",
		ResourceType: "todolist_group", IsMutation: true,
		ResourceID: groupID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.TrashRecordingWithResponse(ctx, s.client.accountID, groupID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}
