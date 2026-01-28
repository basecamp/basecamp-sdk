package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
	"github.com/basecamp/basecamp-sdk/go/pkg/types"
)

// Todo represents a Basecamp todo item.
type Todo struct {
	ID          int64      `json:"id"`
	Status      string     `json:"status"`
	VisibleTo   []int64    `json:"visible_to"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Title       string     `json:"title"`
	InheritsVis bool       `json:"inherits_status"`
	Type        string     `json:"type"`
	URL         string     `json:"url"`
	AppURL      string     `json:"app_url"`
	BookmarkURL string     `json:"bookmark_url"`
	Parent      *Parent    `json:"parent,omitempty"`
	Bucket      *Bucket    `json:"bucket,omitempty"`
	Creator     *Person    `json:"creator,omitempty"`
	Content     string     `json:"content"`
	Description string     `json:"description"`
	StartsOn    string     `json:"starts_on,omitempty"`
	DueOn       string     `json:"due_on,omitempty"`
	Completed   bool       `json:"completed"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Completer   *Person    `json:"completer,omitempty"`
	Assignees   []Person   `json:"assignees,omitempty"`
	Position    int        `json:"position"`
}

// Person represents a Basecamp user.
type Person struct {
	ID                int64          `json:"id"`
	AttachableSGID    string         `json:"attachable_sgid,omitempty"`
	Name              string         `json:"name"`
	EmailAddress      string         `json:"email_address,omitempty"`
	PersonableType    string         `json:"personable_type,omitempty"`
	Title             string         `json:"title,omitempty"`
	Bio               string         `json:"bio,omitempty"`
	Location          string         `json:"location,omitempty"`
	CreatedAt         string         `json:"created_at,omitempty"`
	UpdatedAt         string         `json:"updated_at,omitempty"`
	Admin             bool           `json:"admin,omitempty"`
	Owner             bool           `json:"owner,omitempty"`
	Client            bool           `json:"client,omitempty"`
	Employee          bool           `json:"employee,omitempty"`
	TimeZone          string         `json:"time_zone,omitempty"`
	AvatarURL         string         `json:"avatar_url,omitempty"`
	CanPing           bool           `json:"can_ping,omitempty"`
	Company           *PersonCompany `json:"company,omitempty"`
	CanManageProjects bool           `json:"can_manage_projects,omitempty"`
	CanManagePeople   bool           `json:"can_manage_people,omitempty"`
}

// PersonCompany represents a company associated with a person.
type PersonCompany struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Parent represents the parent object of a todo.
type Parent struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Type   string `json:"type"`
	URL    string `json:"url"`
	AppURL string `json:"app_url"`
}

// Bucket represents the project (bucket) containing a todo.
type Bucket struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// TodoListOptions specifies options for listing todos.
type TodoListOptions struct {
	// Status filters by completion status.
	// "completed" returns completed todos, "pending" returns pending todos.
	// Empty returns all todos.
	Status string
}

// CreateTodoRequest specifies the parameters for creating a todo.
type CreateTodoRequest struct {
	// Content is the todo text (required).
	Content string `json:"content"`
	// Description is an optional extended description (can include HTML).
	Description string `json:"description,omitempty"`
	// AssigneeIDs is a list of person IDs to assign this todo to.
	AssigneeIDs []int64 `json:"assignee_ids,omitempty"`
	// CompletionSubscriberIDs is a list of person IDs to notify on completion.
	CompletionSubscriberIDs []int64 `json:"completion_subscriber_ids,omitempty"`
	// Notify when true, will notify assignees.
	Notify bool `json:"notify,omitempty"`
	// DueOn is the due date in ISO 8601 format (YYYY-MM-DD).
	DueOn string `json:"due_on,omitempty"`
	// StartsOn is the start date in ISO 8601 format (YYYY-MM-DD).
	StartsOn string `json:"starts_on,omitempty"`
}

// UpdateTodoRequest specifies the parameters for updating a todo.
type UpdateTodoRequest struct {
	// Content is the todo text.
	Content string `json:"content,omitempty"`
	// Description is an optional extended description (can include HTML).
	Description string `json:"description,omitempty"`
	// AssigneeIDs is a list of person IDs to assign this todo to.
	AssigneeIDs []int64 `json:"assignee_ids,omitempty"`
	// CompletionSubscriberIDs is a list of person IDs to notify on completion.
	CompletionSubscriberIDs []int64 `json:"completion_subscriber_ids,omitempty"`
	// Notify when true, will notify assignees.
	Notify bool `json:"notify,omitempty"`
	// DueOn is the due date in ISO 8601 format (YYYY-MM-DD).
	DueOn string `json:"due_on,omitempty"`
	// StartsOn is the start date in ISO 8601 format (YYYY-MM-DD).
	StartsOn string `json:"starts_on,omitempty"`
}

// TodosService handles todo operations.
type TodosService struct {
	client *Client
}

// NewTodosService creates a new TodosService.
func NewTodosService(client *Client) *TodosService {
	return &TodosService{client: client}
}

// List returns all todos in a todolist.
// bucketID is the project ID, todolistID is the todolist ID.
func (s *TodosService) List(ctx context.Context, bucketID, todolistID int64, opts *TodoListOptions) (result []Todo, err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "List",
		ResourceType: "todo", IsMutation: false,
		BucketID: bucketID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	// Only pass params when there are actual filters to avoid serializing zero values
	var params *generated.ListTodosParams
	if opts != nil && opts.Status != "" {
		params = &generated.ListTodosParams{Status: opts.Status}
	}

	resp, err := s.client.gen.ListTodosWithResponse(ctx, bucketID, todolistID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	todos := make([]Todo, 0, len(*resp.JSON200))
	for _, gt := range *resp.JSON200 {
		todos = append(todos, todoFromGenerated(gt))
	}

	return todos, nil
}

// Get returns a todo by ID.
// bucketID is the project ID, todoID is the todo ID.
func (s *TodosService) Get(ctx context.Context, bucketID, todoID int64) (result *Todo, err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "Get",
		ResourceType: "todo", IsMutation: false,
		BucketID: bucketID, ResourceID: todoID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetTodoWithResponse(ctx, bucketID, todoID)
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

	todo := todoFromGenerated(resp.JSON200.Todo)
	return &todo, nil
}

// Create creates a new todo in a todolist.
// bucketID is the project ID, todolistID is the todolist ID.
// Returns the created todo.
func (s *TodosService) Create(ctx context.Context, bucketID, todolistID int64, req *CreateTodoRequest) (result *Todo, err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "Create",
		ResourceType: "todo", IsMutation: true,
		BucketID: bucketID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req.Content == "" {
		err = ErrUsage("todo content is required")
		return nil, err
	}

	body := generated.CreateTodoJSONRequestBody{
		Content:                 req.Content,
		Description:             req.Description,
		AssigneeIds:             req.AssigneeIDs,
		CompletionSubscriberIds: req.CompletionSubscriberIDs,
		Notify:                  req.Notify,
	}
	// Parse date strings to types.Date for the generated client
	if req.DueOn != "" {
		d, parseErr := types.ParseDate(req.DueOn)
		if parseErr != nil {
			err = ErrUsage("todo due_on must be in YYYY-MM-DD format")
			return nil, err
		}
		body.DueOn = d
	}
	if req.StartsOn != "" {
		d, parseErr := types.ParseDate(req.StartsOn)
		if parseErr != nil {
			err = ErrUsage("todo starts_on must be in YYYY-MM-DD format")
			return nil, err
		}
		body.StartsOn = d
	}

	resp, err := s.client.gen.CreateTodoWithResponse(ctx, bucketID, todolistID, body)
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

	todo := todoFromGenerated(resp.JSON200.Todo)
	return &todo, nil
}

// Update updates an existing todo.
// bucketID is the project ID, todoID is the todo ID.
// Returns the updated todo.
func (s *TodosService) Update(ctx context.Context, bucketID, todoID int64, req *UpdateTodoRequest) (result *Todo, err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "Update",
		ResourceType: "todo", IsMutation: true,
		BucketID: bucketID, ResourceID: todoID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	body := generated.UpdateTodoJSONRequestBody{
		Content:                 req.Content,
		Description:             req.Description,
		AssigneeIds:             req.AssigneeIDs,
		CompletionSubscriberIds: req.CompletionSubscriberIDs,
		Notify:                  req.Notify,
	}
	// Parse date strings to types.Date for the generated client
	if req.DueOn != "" {
		d, parseErr := types.ParseDate(req.DueOn)
		if parseErr != nil {
			err = ErrUsage("todo due_on must be in YYYY-MM-DD format")
			return nil, err
		}
		body.DueOn = d
	}
	if req.StartsOn != "" {
		d, parseErr := types.ParseDate(req.StartsOn)
		if parseErr != nil {
			err = ErrUsage("todo starts_on must be in YYYY-MM-DD format")
			return nil, err
		}
		body.StartsOn = d
	}

	resp, err := s.client.gen.UpdateTodoWithResponse(ctx, bucketID, todoID, body)
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

	todo := todoFromGenerated(resp.JSON200.Todo)
	return &todo, nil
}

// Trash moves a todo to the trash.
// bucketID is the project ID, todoID is the todo ID.
// Trashed todos can be recovered from the trash.
func (s *TodosService) Trash(ctx context.Context, bucketID, todoID int64) (err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "Trash",
		ResourceType: "todo", IsMutation: true,
		BucketID: bucketID, ResourceID: todoID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.TrashTodoWithResponse(ctx, bucketID, todoID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Complete marks a todo as completed.
// bucketID is the project ID, todoID is the todo ID.
func (s *TodosService) Complete(ctx context.Context, bucketID, todoID int64) (err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "Complete",
		ResourceType: "todo", IsMutation: true,
		BucketID: bucketID, ResourceID: todoID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.CompleteTodoWithResponse(ctx, bucketID, todoID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Uncomplete marks a completed todo as incomplete (reopens it).
// bucketID is the project ID, todoID is the todo ID.
func (s *TodosService) Uncomplete(ctx context.Context, bucketID, todoID int64) (err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "Uncomplete",
		ResourceType: "todo", IsMutation: true,
		BucketID: bucketID, ResourceID: todoID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.UncompleteTodoWithResponse(ctx, bucketID, todoID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Reposition changes the position of a todo within its todolist.
// bucketID is the project ID, todoID is the todo ID.
// position is 1-based (1 = first position).
func (s *TodosService) Reposition(ctx context.Context, bucketID, todoID int64, position int) (err error) {
	op := OperationInfo{
		Service: "Todos", Operation: "Reposition",
		ResourceType: "todo", IsMutation: true,
		BucketID: bucketID, ResourceID: todoID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	if position < 1 {
		err = ErrUsage("position must be at least 1")
		return err
	}

	body := generated.RepositionTodoJSONRequestBody{
		Position: int32(position),
	}
	resp, err := s.client.gen.RepositionTodoWithResponse(ctx, bucketID, todoID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// todoFromGenerated converts a generated Todo to our clean Todo type.
func todoFromGenerated(gt generated.Todo) Todo {
	t := Todo{
		Status:      gt.Status,
		Title:       gt.Title,
		Type:        gt.Type,
		URL:         gt.Url,
		AppURL:      gt.AppUrl,
		BookmarkURL: gt.BookmarkUrl,
		Content:     gt.Content,
		Description: gt.Description,
		Completed:   gt.Completed,
		Position:    int(gt.Position),
		CreatedAt:   gt.CreatedAt,
		UpdatedAt:   gt.UpdatedAt,
		InheritsVis: gt.InheritsStatus,
	}

	if gt.Id != nil {
		t.ID = *gt.Id
	}

	// Convert date fields to strings
	if !gt.StartsOn.IsZero() {
		t.StartsOn = gt.StartsOn.String()
	}
	if !gt.DueOn.IsZero() {
		t.DueOn = gt.DueOn.String()
	}

	// Convert nested types
	if gt.Parent.Id != nil || gt.Parent.Title != "" {
		t.Parent = &Parent{
			ID:     derefInt64(gt.Parent.Id),
			Title:  gt.Parent.Title,
			Type:   gt.Parent.Type,
			URL:    gt.Parent.Url,
			AppURL: gt.Parent.AppUrl,
		}
	}

	if gt.Bucket.Id != nil || gt.Bucket.Name != "" {
		t.Bucket = &Bucket{
			ID:   derefInt64(gt.Bucket.Id),
			Name: gt.Bucket.Name,
			Type: gt.Bucket.Type,
		}
	}

	if gt.Creator.Id != nil || gt.Creator.Name != "" {
		t.Creator = &Person{
			ID:           derefInt64(gt.Creator.Id),
			Name:         gt.Creator.Name,
			EmailAddress: gt.Creator.EmailAddress,
			AvatarURL:    gt.Creator.AvatarUrl,
			Admin:        gt.Creator.Admin,
			Owner:        gt.Creator.Owner,
		}
	}

	// Convert assignees
	if len(gt.Assignees) > 0 {
		t.Assignees = make([]Person, 0, len(gt.Assignees))
		for _, ga := range gt.Assignees {
			t.Assignees = append(t.Assignees, personFromGenerated(ga))
		}
	}

	return t
}
