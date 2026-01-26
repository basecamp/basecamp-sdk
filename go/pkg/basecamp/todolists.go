package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

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
	client *Client
}

// NewTodolistsService creates a new TodolistsService.
func NewTodolistsService(client *Client) *TodolistsService {
	return &TodolistsService{client: client}
}

// List returns all todolists in a todoset.
// bucketID is the project ID, todosetID is the todoset ID.
func (s *TodolistsService) List(ctx context.Context, bucketID, todosetID int64, opts *TodolistListOptions) ([]Todolist, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/todosets/%d/todolists.json", bucketID, todosetID)
	if opts != nil && opts.Status != "" {
		path = fmt.Sprintf("/buckets/%d/todosets/%d/todolists.json?status=%s", bucketID, todosetID, opts.Status)
	}

	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	todolists := make([]Todolist, 0, len(results))
	for _, raw := range results {
		var tl Todolist
		if err := json.Unmarshal(raw, &tl); err != nil {
			return nil, fmt.Errorf("failed to parse todolist: %w", err)
		}
		todolists = append(todolists, tl)
	}

	return todolists, nil
}

// Get returns a todolist by ID.
// bucketID is the project ID, todolistID is the todolist ID.
func (s *TodolistsService) Get(ctx context.Context, bucketID, todolistID int64) (*Todolist, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/todolists/%d.json", bucketID, todolistID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var todolist Todolist
	if err := resp.UnmarshalData(&todolist); err != nil {
		return nil, fmt.Errorf("failed to parse todolist: %w", err)
	}

	return &todolist, nil
}

// Create creates a new todolist in a todoset.
// bucketID is the project ID, todosetID is the todoset ID.
// Returns the created todolist.
func (s *TodolistsService) Create(ctx context.Context, bucketID, todosetID int64, req *CreateTodolistRequest) (*Todolist, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req.Name == "" {
		return nil, ErrUsage("todolist name is required")
	}

	path := fmt.Sprintf("/buckets/%d/todosets/%d/todolists.json", bucketID, todosetID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var todolist Todolist
	if err := resp.UnmarshalData(&todolist); err != nil {
		return nil, fmt.Errorf("failed to parse todolist: %w", err)
	}

	return &todolist, nil
}

// Update updates an existing todolist.
// bucketID is the project ID, todolistID is the todolist ID.
// Returns the updated todolist.
func (s *TodolistsService) Update(ctx context.Context, bucketID, todolistID int64, req *UpdateTodolistRequest) (*Todolist, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/todolists/%d.json", bucketID, todolistID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var todolist Todolist
	if err := resp.UnmarshalData(&todolist); err != nil {
		return nil, fmt.Errorf("failed to parse todolist: %w", err)
	}

	return &todolist, nil
}

// Trash moves a todolist to the trash.
// bucketID is the project ID, todolistID is the todolist ID.
// Trashed todolists can be recovered from the trash.
func (s *TodolistsService) Trash(ctx context.Context, bucketID, todolistID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/status/trashed.json", bucketID, todolistID)
	_, err := s.client.Put(ctx, path, nil)
	return err
}
