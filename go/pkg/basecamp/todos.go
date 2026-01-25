package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Todo represents a Basecamp todo item.
type Todo struct {
	ID          int64     `json:"id"`
	Status      string    `json:"status"`
	VisibleTo   []int64   `json:"visible_to"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Title       string    `json:"title"`
	InheritsVis bool      `json:"inherits_status"`
	Type        string    `json:"type"`
	URL         string    `json:"url"`
	AppURL      string    `json:"app_url"`
	BookmarkURL string    `json:"bookmark_url"`
	Parent      *Parent   `json:"parent,omitempty"`
	Bucket      *Bucket   `json:"bucket,omitempty"`
	Creator     *Person   `json:"creator,omitempty"`
	Content     string    `json:"content"`
	Description string    `json:"description"`
	StartsOn    string    `json:"starts_on,omitempty"`
	DueOn       string    `json:"due_on,omitempty"`
	Completed   bool      `json:"completed"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Completer   *Person   `json:"completer,omitempty"`
	Assignees   []Person  `json:"assignees,omitempty"`
	Position    int       `json:"position"`
}

// Person represents a Basecamp user.
type Person struct {
	ID                 int64          `json:"id"`
	AttachableSGID     string         `json:"attachable_sgid,omitempty"`
	Name               string         `json:"name"`
	EmailAddress       string         `json:"email_address,omitempty"`
	PersonableType     string         `json:"personable_type,omitempty"`
	Title              string         `json:"title,omitempty"`
	Bio                string         `json:"bio,omitempty"`
	Location           string         `json:"location,omitempty"`
	CreatedAt          string         `json:"created_at,omitempty"`
	UpdatedAt          string         `json:"updated_at,omitempty"`
	Admin              bool           `json:"admin,omitempty"`
	Owner              bool           `json:"owner,omitempty"`
	Client             bool           `json:"client,omitempty"`
	Employee           bool           `json:"employee,omitempty"`
	TimeZone           string         `json:"time_zone,omitempty"`
	AvatarURL          string         `json:"avatar_url,omitempty"`
	CanPing            bool           `json:"can_ping,omitempty"`
	Company            *PersonCompany `json:"company,omitempty"`
	CanManageProjects  bool           `json:"can_manage_projects,omitempty"`
	CanManagePeople    bool           `json:"can_manage_people,omitempty"`
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
func (s *TodosService) List(ctx context.Context, bucketID, todolistID int64, opts *TodoListOptions) ([]Todo, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/todolists/%d/todos.json", bucketID, todolistID)
	if opts != nil && opts.Status != "" {
		path = fmt.Sprintf("/buckets/%d/todolists/%d/todos.json?status=%s", bucketID, todolistID, opts.Status)
	}

	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	todos := make([]Todo, 0, len(results))
	for _, raw := range results {
		var t Todo
		if err := json.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("failed to parse todo: %w", err)
		}
		todos = append(todos, t)
	}

	return todos, nil
}

// Get returns a todo by ID.
// bucketID is the project ID, todoID is the todo ID.
func (s *TodosService) Get(ctx context.Context, bucketID, todoID int64) (*Todo, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/todos/%d.json", bucketID, todoID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var todo Todo
	if err := resp.UnmarshalData(&todo); err != nil {
		return nil, fmt.Errorf("failed to parse todo: %w", err)
	}

	return &todo, nil
}

// Create creates a new todo in a todolist.
// bucketID is the project ID, todolistID is the todolist ID.
// Returns the created todo.
func (s *TodosService) Create(ctx context.Context, bucketID, todolistID int64, req *CreateTodoRequest) (*Todo, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req.Content == "" {
		return nil, ErrUsage("todo content is required")
	}

	path := fmt.Sprintf("/buckets/%d/todolists/%d/todos.json", bucketID, todolistID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var todo Todo
	if err := resp.UnmarshalData(&todo); err != nil {
		return nil, fmt.Errorf("failed to parse todo: %w", err)
	}

	return &todo, nil
}

// Update updates an existing todo.
// bucketID is the project ID, todoID is the todo ID.
// Returns the updated todo.
func (s *TodosService) Update(ctx context.Context, bucketID, todoID int64, req *UpdateTodoRequest) (*Todo, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/todos/%d.json", bucketID, todoID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var todo Todo
	if err := resp.UnmarshalData(&todo); err != nil {
		return nil, fmt.Errorf("failed to parse todo: %w", err)
	}

	return &todo, nil
}

// Trash moves a todo to the trash.
// bucketID is the project ID, todoID is the todo ID.
// Trashed todos can be recovered from the trash.
func (s *TodosService) Trash(ctx context.Context, bucketID, todoID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/status/trashed.json", bucketID, todoID)
	_, err := s.client.Put(ctx, path, nil)
	return err
}

// Complete marks a todo as completed.
// bucketID is the project ID, todoID is the todo ID.
func (s *TodosService) Complete(ctx context.Context, bucketID, todoID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/todos/%d/completion.json", bucketID, todoID)
	_, err := s.client.Post(ctx, path, nil)
	return err
}

// Uncomplete marks a completed todo as incomplete (reopens it).
// bucketID is the project ID, todoID is the todo ID.
func (s *TodosService) Uncomplete(ctx context.Context, bucketID, todoID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/todos/%d/completion.json", bucketID, todoID)
	_, err := s.client.Delete(ctx, path)
	return err
}

// Reposition changes the position of a todo within its todolist.
// bucketID is the project ID, todoID is the todo ID.
// position is 1-based (1 = first position).
func (s *TodosService) Reposition(ctx context.Context, bucketID, todoID int64, position int) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	if position < 1 {
		return ErrUsage("position must be at least 1")
	}

	path := fmt.Sprintf("/buckets/%d/todos/%d/position.json", bucketID, todoID)
	body := map[string]int{"position": position}
	_, err := s.client.Put(ctx, path, body)
	return err
}
