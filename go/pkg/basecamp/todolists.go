package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Note: Todolists default to fetching all (no limit) since they are structural
// indices, not high-volume content. Use Limit to cap results if needed.

// Todolist represents a Basecamp todolist.
type Todolist struct {
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
	Description      string    `json:"description"`
	Completed        bool      `json:"completed"`
	CompletedRatio   string    `json:"completed_ratio"`
	Name             string    `json:"name"`
	TodosURL         string    `json:"todos_url"`
	GroupsURL        string    `json:"groups_url"`
	AppTodosURL      string    `json:"app_todos_url"`
}

// TodolistListOptions specifies options for listing todolists.
type TodolistListOptions struct {
	// Status filters by status: "archived" or "trashed".
	// Empty returns active todolists.
	Status string

	// Limit is the maximum number of todolists to return.
	// If 0 (default), returns all todolists. Use a positive value to cap results.
	Limit int

	// Page fetches a specific page only (1-indexed).
	// If 0 (default), fetches all pages up to Limit.
	Page int
}

// CreateTodolistRequest specifies the parameters for creating a todolist.
type CreateTodolistRequest struct {
	// Name is the todolist name (required).
	Name string `json:"name"`
	// Description is an optional description (can include HTML).
	Description string `json:"description,omitempty"`
}

// UpdateTodolistRequest specifies the parameters for updating a todolist.
type UpdateTodolistRequest struct {
	// Name is the todolist name.
	Name string `json:"name,omitempty"`
	// Description is an optional description (can include HTML).
	Description string `json:"description,omitempty"`
}

// TodolistsService handles todolist operations.
type TodolistsService struct {
	client *AccountClient
}

// NewTodolistsService creates a new TodolistsService.
func NewTodolistsService(client *AccountClient) *TodolistsService {
	return &TodolistsService{client: client}
}

// List returns todolists in a todoset.
// bucketID is the project ID, todosetID is the todoset ID.
//
// By default, returns all todolists (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of todolists to return (0 = all)
//   - Page: fetch a specific page only (1-indexed, 0 = all pages)
func (s *TodolistsService) List(ctx context.Context, bucketID, todosetID int64, opts *TodolistListOptions) (result []Todolist, err error) {
	op := OperationInfo{
		Service: "Todolists", Operation: "List",
		ResourceType: "todolist", IsMutation: false,
		BucketID: bucketID, ResourceID: todosetID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Build path with query parameters
	path := fmt.Sprintf("/buckets/%d/todosets/%d/todolists.json", bucketID, todosetID)
	if opts != nil && opts.Status != "" {
		path += "?status=" + opts.Status
	}

	// Handle single page fetch
	if opts != nil && opts.Page > 0 {
		params := &generated.ListTodolistsParams{}
		if opts.Status != "" {
			params.Status = opts.Status
		}
		resp, err := s.client.parent.gen.ListTodolistsWithResponse(ctx, s.client.accountID, bucketID, todosetID, params)
		if err != nil {
			return nil, err
		}
		if err = checkResponse(resp.HTTPResponse); err != nil {
			return nil, err
		}
		if resp.JSON200 == nil {
			return nil, nil
		}
		todolists := make([]Todolist, 0, len(*resp.JSON200))
		for _, gtl := range *resp.JSON200 {
			todolists = append(todolists, todolistFromGenerated(gtl))
		}
		return todolists, nil
	}

	// Determine limit: 0 = all (default for todolists), >0 = specific limit
	limit := 0 // default to all for todolists (structural index, not high-volume)
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	rawResults, err := s.client.GetAllWithLimit(ctx, path, limit)
	if err != nil {
		return nil, err
	}

	todolists := make([]Todolist, 0, len(rawResults))
	for _, raw := range rawResults {
		var gtl generated.Todolist
		if err := json.Unmarshal(raw, &gtl); err != nil {
			return nil, fmt.Errorf("failed to parse todolist: %w", err)
		}
		todolists = append(todolists, todolistFromGenerated(gtl))
	}

	return todolists, nil
}

// Get returns a todolist by ID.
// bucketID is the project ID, todolistID is the todolist ID.
func (s *TodolistsService) Get(ctx context.Context, bucketID, todolistID int64) (result *Todolist, err error) {
	op := OperationInfo{
		Service: "Todolists", Operation: "Get",
		ResourceType: "todolist", IsMutation: false,
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

	resp, err := s.client.parent.gen.GetTodolistOrGroupWithResponse(ctx, s.client.accountID, bucketID, todolistID)
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

	// The response is a union type, try to extract as Todolist
	tl, err := resp.JSON200.AsTodolistOrGroup0()
	if err != nil {
		err = fmt.Errorf("response is not a todolist: %w", err)
		return nil, err
	}

	todolist := todolistFromGenerated(tl.Todolist)
	return &todolist, nil
}

// Create creates a new todolist in a todoset.
// bucketID is the project ID, todosetID is the todoset ID.
// Returns the created todolist.
func (s *TodolistsService) Create(ctx context.Context, bucketID, todosetID int64, req *CreateTodolistRequest) (result *Todolist, err error) {
	op := OperationInfo{
		Service: "Todolists", Operation: "Create",
		ResourceType: "todolist", IsMutation: true,
		BucketID: bucketID, ResourceID: todosetID,
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
		err = ErrUsage("todolist name is required")
		return nil, err
	}

	body := generated.CreateTodolistJSONRequestBody{
		Name:        req.Name,
		Description: req.Description,
	}

	resp, err := s.client.parent.gen.CreateTodolistWithResponse(ctx, s.client.accountID, bucketID, todosetID, body)
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

	todolist := todolistFromGenerated(resp.JSON200.Todolist)
	return &todolist, nil
}

// Update updates an existing todolist.
// bucketID is the project ID, todolistID is the todolist ID.
// Returns the updated todolist.
func (s *TodolistsService) Update(ctx context.Context, bucketID, todolistID int64, req *UpdateTodolistRequest) (result *Todolist, err error) {
	op := OperationInfo{
		Service: "Todolists", Operation: "Update",
		ResourceType: "todolist", IsMutation: true,
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

	body := generated.UpdateTodolistOrGroupJSONRequestBody{
		Name:        req.Name,
		Description: req.Description,
	}

	resp, err := s.client.parent.gen.UpdateTodolistOrGroupWithResponse(ctx, s.client.accountID, bucketID, todolistID, body)
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

	// The response is a union type, try to extract as Todolist
	tl, err := resp.JSON200.Result.AsTodolistOrGroup0()
	if err != nil {
		err = fmt.Errorf("response is not a todolist: %w", err)
		return nil, err
	}

	todolist := todolistFromGenerated(tl.Todolist)
	return &todolist, nil
}

// Trash moves a todolist to the trash.
// bucketID is the project ID, todolistID is the todolist ID.
// Trashed todolists can be recovered from the trash.
func (s *TodolistsService) Trash(ctx context.Context, bucketID, todolistID int64) (err error) {
	op := OperationInfo{
		Service: "Todolists", Operation: "Trash",
		ResourceType: "todolist", IsMutation: true,
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

	resp, err := s.client.parent.gen.TrashRecordingWithResponse(ctx, s.client.accountID, bucketID, todolistID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// todolistFromGenerated converts a generated Todolist to our clean Todolist type.
func todolistFromGenerated(gtl generated.Todolist) Todolist {
	tl := Todolist{
		Status:           gtl.Status,
		VisibleToClients: gtl.VisibleToClients,
		Title:            gtl.Title,
		InheritsStatus:   gtl.InheritsStatus,
		Type:             gtl.Type,
		URL:              gtl.Url,
		AppURL:           gtl.AppUrl,
		BookmarkURL:      gtl.BookmarkUrl,
		SubscriptionURL:  gtl.SubscriptionUrl,
		CommentsCount:    int(gtl.CommentsCount),
		CommentsURL:      gtl.CommentsUrl,
		Position:         int(gtl.Position),
		Description:      gtl.Description,
		Completed:        gtl.Completed,
		CompletedRatio:   gtl.CompletedRatio,
		Name:             gtl.Name,
		TodosURL:         gtl.TodosUrl,
		GroupsURL:        gtl.GroupsUrl,
		AppTodosURL:      gtl.AppTodosUrl,
		CreatedAt:        gtl.CreatedAt,
		UpdatedAt:        gtl.UpdatedAt,
	}

	if gtl.Id != nil {
		tl.ID = *gtl.Id
	}

	// Convert nested types
	if gtl.Parent.Id != nil || gtl.Parent.Title != "" {
		tl.Parent = &Parent{
			ID:     derefInt64(gtl.Parent.Id),
			Title:  gtl.Parent.Title,
			Type:   gtl.Parent.Type,
			URL:    gtl.Parent.Url,
			AppURL: gtl.Parent.AppUrl,
		}
	}

	if gtl.Bucket.Id != nil || gtl.Bucket.Name != "" {
		tl.Bucket = &Bucket{
			ID:   derefInt64(gtl.Bucket.Id),
			Name: gtl.Bucket.Name,
			Type: gtl.Bucket.Type,
		}
	}

	if gtl.Creator.Id != nil || gtl.Creator.Name != "" {
		tl.Creator = &Person{
			ID:           derefInt64(gtl.Creator.Id),
			Name:         gtl.Creator.Name,
			EmailAddress: gtl.Creator.EmailAddress,
			AvatarURL:    gtl.Creator.AvatarUrl,
			Admin:        gtl.Creator.Admin,
			Owner:        gtl.Creator.Owner,
		}
	}

	return tl
}
