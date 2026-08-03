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

// TodolistGroup represents a Basecamp todolist group (organizational folder within a todolist).
type TodolistGroup struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status"`
	Type             string    `json:"type"`
	URL              string    `json:"url"`
	AppURL           string    `json:"app_url"`
	BookmarkURL      string    `json:"bookmark_url"`
	SubscriptionURL  string    `json:"subscription_url"`
	// BubbleUpURL is the URL of the Bubble Up record for this recording. Always
	// present: todolists/_todolist.json.jbuilder renders the shared recording
	// partial with bubbleupable: true unconditionally, and every list, show, and
	// group path renders that partial.
	BubbleUpURL    string  `json:"bubble_up_url"`
	CommentsCount  int     `json:"comments_count"`
	CommentsURL    string  `json:"comments_url"`
	Position       int     `json:"position"`
	Parent         *Parent `json:"parent,omitempty"`
	Bucket         *Bucket `json:"bucket,omitempty"`
	Creator        *Person `json:"creator,omitempty"`
	Name           string  `json:"name"`
	Completed      bool    `json:"completed"`
	CompletedRatio string  `json:"completed_ratio"`
	TodosURL       string  `json:"todos_url"`
	AppTodosURL    string  `json:"app_todos_url"`
}

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
			groups = append(groups, todolistGroupFromGenerated(gg))
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
		var gg generated.TodolistGroup
		if err := json.Unmarshal(raw, &gg); err != nil {
			return nil, fmt.Errorf("failed to parse todolist group: %w", err)
		}
		groups = append(groups, todolistGroupFromGenerated(gg))
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

	// The API returns flat JSON, not the envelope that AsTodolistOrGroup1 expects.
	// Decode resp.Body directly into the generated TodolistGroup type.
	var gg generated.TodolistGroup
	if err := json.Unmarshal(resp.Body, &gg); err != nil {
		return nil, fmt.Errorf("failed to parse todolist group: %w", err)
	}

	group := todolistGroupFromGenerated(gg)
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

	group := todolistGroupFromGenerated(*resp.JSON201)
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
// There is deliberately no merge-safe Update or Edit on this service. The
// TodolistGroup projection does not model description (nor
// description_attachments, boosts_*, or groups_url), so a composite built on
// TodolistGroupsService.Get would read a group with no description and then
// PUT the zero value — erasing the description on every call, which is the
// data loss this triad exists to remove. Callers who want a merge-safe group
// write should use Todolists().Update or Todolists().Edit: they address the
// same PUT /{accountId}/todolists/{id} route through the full Todolist
// projection, which is variant-agnostic and preserves a group's
// {name, description} correctly. Tracked by #544, the flat-shape
// consolidation that would model description on groups and unblock a
// composite here.
//
// Description is still offered on the request because the wire body is shared
// with todolists and accepts it: without it a group replace would be
// unconditionally destructive with no caller recourse. It does not round-trip
// — the returned TodolistGroup will not carry it back — for the same #544
// reason.
// Returns the updated group.
func (s *TodolistGroupsService) Replace(ctx context.Context, groupID int64, req *ReplaceTodolistGroupRequest) (*TodolistGroup, error) {
	return s.replaceGroup(ctx, groupID, func() (map[string]any, error) {
		if req == nil {
			return nil, ErrUsage("replace request is required")
		}
		if req.Name == "" {
			return nil, ErrUsage("group name is required")
		}
		body := map[string]any{"name": req.Name}
		if req.Description != "" {
			body["description"] = req.Description
		}
		return body, nil
	})
}

// replaceGroup is the single transport for the UpdateTodolistOrGroup wire
// operation as TodolistGroupsService issues it. It pins the
// TodolistGroups.Replace hook identity and decodes the group shape; the
// envelope and the one generated-client call site live in
// replaceTodolistOrGroup, shared with TodolistsService.
func (s *TodolistGroupsService) replaceGroup(ctx context.Context, groupID int64, buildBody func() (map[string]any, error)) (*TodolistGroup, error) {
	op := OperationInfo{
		Service: "TodolistGroups", Operation: "Replace",
		ResourceType: "todolist_group", IsMutation: true,
		ResourceID: groupID,
	}

	raw, err := replaceTodolistOrGroup(ctx, s.client, op, groupID, buildBody)
	if err != nil {
		return nil, err
	}

	// The API returns flat JSON, not the envelope that AsTodolistOrGroup1 expects.
	// Decode the body directly into the generated TodolistGroup type.
	var gg generated.TodolistGroup
	if err := json.Unmarshal(raw, &gg); err != nil {
		return nil, fmt.Errorf("failed to parse todolist group: %w", err)
	}

	group := todolistGroupFromGenerated(gg)
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

// todolistGroupFromGenerated converts a generated TodolistGroup to our clean TodolistGroup type.
func todolistGroupFromGenerated(gg generated.TodolistGroup) TodolistGroup {
	g := TodolistGroup{
		Status:           gg.Status,
		VisibleToClients: gg.VisibleToClients,
		Title:            gg.Title,
		InheritsStatus:   gg.InheritsStatus,
		Type:             gg.Type,
		URL:              gg.Url,
		AppURL:           gg.AppUrl,
		BookmarkURL:      deref(gg.BookmarkUrl),
		SubscriptionURL:  deref(gg.SubscriptionUrl),
		BubbleUpURL:      gg.BubbleUpUrl,
		CommentsCount:    int(deref(gg.CommentsCount)),
		CommentsURL:      deref(gg.CommentsUrl),
		Position:         int(deref(gg.Position)),
		Name:             gg.Name,
		Completed:        deref(gg.Completed),
		CompletedRatio:   deref(gg.CompletedRatio),
		TodosURL:         deref(gg.TodosUrl),
		AppTodosURL:      deref(gg.AppTodosUrl),
		CreatedAt:        gg.CreatedAt,
		UpdatedAt:        gg.UpdatedAt,
	}

	if gg.Id != 0 {
		g.ID = gg.Id
	}

	// Convert nested types
	if gg.Parent.Id != 0 || gg.Parent.Title != "" {
		g.Parent = &Parent{
			ID:     gg.Parent.Id,
			Title:  gg.Parent.Title,
			Type:   gg.Parent.Type,
			URL:    gg.Parent.Url,
			AppURL: gg.Parent.AppUrl,
		}
	}

	if gg.Bucket.Id != 0 || gg.Bucket.Name != "" {
		g.Bucket = &Bucket{
			ID:   gg.Bucket.Id,
			Name: gg.Bucket.Name,
			Type: gg.Bucket.Type,
		}
	}

	if gg.Creator.Id != 0 || gg.Creator.Name != "" {
		creator := personFromGenerated(gg.Creator)
		g.Creator = &creator
	}

	return g
}
