package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

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
	CommentsCount    int       `json:"comments_count"`
	CommentsURL      string    `json:"comments_url"`
	Position         int       `json:"position"`
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
	Name             string    `json:"name"`
	Completed        bool      `json:"completed"`
	CompletedRatio   string    `json:"completed_ratio"`
	TodosURL         string    `json:"todos_url"`
	AppTodosURL      string    `json:"app_todos_url"`
}

// CreateTodolistGroupRequest specifies the parameters for creating a todolist group.
type CreateTodolistGroupRequest struct {
	// Name is the group name (required).
	Name string `json:"name"`
}

// UpdateTodolistGroupRequest specifies the parameters for updating a todolist group.
type UpdateTodolistGroupRequest struct {
	// Name is the group name.
	Name string `json:"name,omitempty"`
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
// bucketID is the project ID, todolistID is the todolist ID.
//
// The returned TodolistGroupListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *TodolistGroupsService) List(ctx context.Context, bucketID, todolistID int64) (result *TodolistGroupListResult, err error) {
	op := OperationInfo{
		Service: "TodolistGroups", Operation: "List",
		ResourceType: "todolist_group", IsMutation: false,
		BucketID: bucketID, ResourceID: todolistID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListTodolistGroupsWithResponse(ctx, s.client.accountID, bucketID, todolistID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header
	totalCount := parseTotalCount(resp.HTTPResponse)

	if resp.JSON200 == nil {
		return &TodolistGroupListResult{Groups: nil, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	groups := make([]TodolistGroup, 0, len(*resp.JSON200))
	for _, gg := range *resp.JSON200 {
		groups = append(groups, todolistGroupFromGenerated(gg))
	}

	return &TodolistGroupListResult{Groups: groups, Meta: ListMeta{TotalCount: totalCount}}, nil
}

// Get returns a todolist group by ID.
// bucketID is the project ID, groupID is the group ID.
func (s *TodolistGroupsService) Get(ctx context.Context, bucketID, groupID int64) (result *TodolistGroup, err error) {
	op := OperationInfo{
		Service: "TodolistGroups", Operation: "Get",
		ResourceType: "todolist_group", IsMutation: false,
		BucketID: bucketID, ResourceID: groupID,
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
	resp, err := s.client.parent.gen.GetTodolistOrGroupWithResponse(ctx, s.client.accountID, bucketID, groupID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	// The response is a union type, try to extract as TodolistGroup
	g, err := resp.JSON200.AsTodolistOrGroup1()
	if err != nil {
		err = fmt.Errorf("response is not a todolist group: %w", err)
		return nil, err
	}

	group := todolistGroupFromGenerated(g.Group)
	return &group, nil
}

// Create creates a new group in a todolist.
// bucketID is the project ID, todolistID is the todolist ID.
// Returns the created group.
func (s *TodolistGroupsService) Create(ctx context.Context, bucketID, todolistID int64, req *CreateTodolistGroupRequest) (result *TodolistGroup, err error) {
	op := OperationInfo{
		Service: "TodolistGroups", Operation: "Create",
		ResourceType: "todolist_group", IsMutation: true,
		BucketID: bucketID, ResourceID: todolistID,
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

	resp, err := s.client.parent.gen.CreateTodolistGroupWithResponse(ctx, s.client.accountID, bucketID, todolistID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	group := todolistGroupFromGenerated(*resp.JSON201)
	return &group, nil
}

// Update updates an existing todolist group.
// bucketID is the project ID, groupID is the group ID.
// Returns the updated group.
func (s *TodolistGroupsService) Update(ctx context.Context, bucketID, groupID int64, req *UpdateTodolistGroupRequest) (result *TodolistGroup, err error) {
	op := OperationInfo{
		Service: "TodolistGroups", Operation: "Update",
		ResourceType: "todolist_group", IsMutation: true,
		BucketID: bucketID, ResourceID: groupID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Groups are updated via the todolists endpoint (polymorphic endpoint)
	body := generated.UpdateTodolistOrGroupJSONRequestBody{
		Name: req.Name,
	}

	resp, err := s.client.parent.gen.UpdateTodolistOrGroupWithResponse(ctx, s.client.accountID, bucketID, groupID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	// The response is a union type, try to extract as TodolistGroup
	g, err := resp.JSON200.AsTodolistOrGroup1()
	if err != nil {
		err = fmt.Errorf("response is not a todolist group: %w", err)
		return nil, err
	}

	group := todolistGroupFromGenerated(g.Group)
	return &group, nil
}

// Reposition changes the position of a group within its todolist.
// bucketID is the project ID, groupID is the group ID.
// position is 1-based (1 = first position).
func (s *TodolistGroupsService) Reposition(ctx context.Context, bucketID, groupID int64, position int) (err error) {
	op := OperationInfo{
		Service: "TodolistGroups", Operation: "Reposition",
		ResourceType: "todolist_group", IsMutation: true,
		BucketID: bucketID, ResourceID: groupID,
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
		Position: int32(position),
	}

	resp, err := s.client.parent.gen.RepositionTodolistGroupWithResponse(ctx, s.client.accountID, bucketID, groupID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Trash moves a todolist group to the trash.
// bucketID is the project ID, groupID is the group ID.
// Trashed groups can be recovered from the trash.
func (s *TodolistGroupsService) Trash(ctx context.Context, bucketID, groupID int64) (err error) {
	op := OperationInfo{
		Service: "TodolistGroups", Operation: "Trash",
		ResourceType: "todolist_group", IsMutation: true,
		BucketID: bucketID, ResourceID: groupID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.TrashRecordingWithResponse(ctx, s.client.accountID, bucketID, groupID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
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
		BookmarkURL:      gg.BookmarkUrl,
		SubscriptionURL:  gg.SubscriptionUrl,
		CommentsCount:    int(gg.CommentsCount),
		CommentsURL:      gg.CommentsUrl,
		Position:         int(gg.Position),
		Name:             gg.Name,
		Completed:        gg.Completed,
		CompletedRatio:   gg.CompletedRatio,
		TodosURL:         gg.TodosUrl,
		AppTodosURL:      gg.AppTodosUrl,
		CreatedAt:        gg.CreatedAt,
		UpdatedAt:        gg.UpdatedAt,
	}

	if gg.Id != nil {
		g.ID = *gg.Id
	}

	// Convert nested types
	if gg.Parent.Id != nil || gg.Parent.Title != "" {
		g.Parent = &Parent{
			ID:     derefInt64(gg.Parent.Id),
			Title:  gg.Parent.Title,
			Type:   gg.Parent.Type,
			URL:    gg.Parent.Url,
			AppURL: gg.Parent.AppUrl,
		}
	}

	if gg.Bucket.Id != nil || gg.Bucket.Name != "" {
		g.Bucket = &Bucket{
			ID:   derefInt64(gg.Bucket.Id),
			Name: gg.Bucket.Name,
			Type: gg.Bucket.Type,
		}
	}

	if gg.Creator.Id != nil || gg.Creator.Name != "" {
		g.Creator = &Person{
			ID:           derefInt64(gg.Creator.Id),
			Name:         gg.Creator.Name,
			EmailAddress: gg.Creator.EmailAddress,
			AvatarURL:    gg.Creator.AvatarUrl,
			Admin:        gg.Creator.Admin,
			Owner:        gg.Creator.Owner,
		}
	}

	return g
}
