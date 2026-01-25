package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
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

// TodolistGroupsService handles todolist group operations.
type TodolistGroupsService struct {
	client *Client
}

// NewTodolistGroupsService creates a new TodolistGroupsService.
func NewTodolistGroupsService(client *Client) *TodolistGroupsService {
	return &TodolistGroupsService{client: client}
}

// List returns all groups in a todolist.
// bucketID is the project ID, todolistID is the todolist ID.
func (s *TodolistGroupsService) List(ctx context.Context, bucketID, todolistID int64) ([]TodolistGroup, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/todolists/%d/groups.json", bucketID, todolistID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	groups := make([]TodolistGroup, 0, len(results))
	for _, raw := range results {
		var g TodolistGroup
		if err := json.Unmarshal(raw, &g); err != nil {
			return nil, fmt.Errorf("failed to parse todolist group: %w", err)
		}
		groups = append(groups, g)
	}

	return groups, nil
}

// Get returns a todolist group by ID.
// bucketID is the project ID, groupID is the group ID.
func (s *TodolistGroupsService) Get(ctx context.Context, bucketID, groupID int64) (*TodolistGroup, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	// Groups are fetched via the todolists endpoint
	path := fmt.Sprintf("/buckets/%d/todolists/%d.json", bucketID, groupID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var group TodolistGroup
	if err := resp.UnmarshalData(&group); err != nil {
		return nil, fmt.Errorf("failed to parse todolist group: %w", err)
	}

	return &group, nil
}

// Create creates a new group in a todolist.
// bucketID is the project ID, todolistID is the todolist ID.
// Returns the created group.
func (s *TodolistGroupsService) Create(ctx context.Context, bucketID, todolistID int64, req *CreateTodolistGroupRequest) (*TodolistGroup, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req.Name == "" {
		return nil, ErrUsage("group name is required")
	}

	path := fmt.Sprintf("/buckets/%d/todolists/%d/groups.json", bucketID, todolistID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var group TodolistGroup
	if err := resp.UnmarshalData(&group); err != nil {
		return nil, fmt.Errorf("failed to parse todolist group: %w", err)
	}

	return &group, nil
}

// Update updates an existing todolist group.
// bucketID is the project ID, groupID is the group ID.
// Returns the updated group.
func (s *TodolistGroupsService) Update(ctx context.Context, bucketID, groupID int64, req *UpdateTodolistGroupRequest) (*TodolistGroup, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	// Groups are updated via the todolists endpoint
	path := fmt.Sprintf("/buckets/%d/todolists/%d.json", bucketID, groupID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var group TodolistGroup
	if err := resp.UnmarshalData(&group); err != nil {
		return nil, fmt.Errorf("failed to parse todolist group: %w", err)
	}

	return &group, nil
}

// Reposition changes the position of a group within its todolist.
// bucketID is the project ID, groupID is the group ID.
// position is 1-based (1 = first position).
func (s *TodolistGroupsService) Reposition(ctx context.Context, bucketID, groupID int64, position int) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	if position < 1 {
		return ErrUsage("position must be at least 1")
	}

	path := fmt.Sprintf("/buckets/%d/todolists/%d/position.json", bucketID, groupID)
	body := map[string]int{"position": position}
	_, err := s.client.Put(ctx, path, body)
	return err
}

// Trash moves a todolist group to the trash.
// bucketID is the project ID, groupID is the group ID.
// Trashed groups can be recovered from the trash.
func (s *TodolistGroupsService) Trash(ctx context.Context, bucketID, groupID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/status/trashed.json", bucketID, groupID)
	_, err := s.client.Put(ctx, path, nil)
	return err
}
