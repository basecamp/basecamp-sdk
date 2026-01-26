package basecamp

import (
	"context"
	"fmt"
	"time"
)

// Todoset represents a Basecamp todoset (container for todolists in a project).
// Each project has exactly one todoset in its dock.
type Todoset struct {
	ID                int64     `json:"id"`
	Status            string    `json:"status"`
	VisibleToClients  bool      `json:"visible_to_clients"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	Title             string    `json:"title"`
	InheritsStatus    bool      `json:"inherits_status"`
	Type              string    `json:"type"`
	URL               string    `json:"url"`
	AppURL            string    `json:"app_url"`
	BookmarkURL       string    `json:"bookmark_url"`
	Position          *int      `json:"position,omitempty"`
	Bucket            *Bucket   `json:"bucket,omitempty"`
	Creator           *Person   `json:"creator,omitempty"`
	Name              string    `json:"name"`
	TodolistsCount    int       `json:"todolists_count"`
	TodolistsURL      string    `json:"todolists_url"`
	CompletedRatio    string    `json:"completed_ratio"`
	Completed         bool      `json:"completed"`
	CompletedCount    int       `json:"completed_count"`
	OnScheduleCount   int       `json:"on_schedule_count"`
	OverScheduleCount int       `json:"over_schedule_count"`
	AppTodolistsURL   string    `json:"app_todolists_url"`
}

// TodosetsService handles todoset operations.
type TodosetsService struct {
	client *Client
}

// NewTodosetsService creates a new TodosetsService.
func NewTodosetsService(client *Client) *TodosetsService {
	return &TodosetsService{client: client}
}

// Get returns a todoset by ID.
// bucketID is the project ID, todosetID is the todoset ID.
func (s *TodosetsService) Get(ctx context.Context, bucketID, todosetID int64) (*Todoset, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/todosets/%d.json", bucketID, todosetID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var todoset Todoset
	if err := resp.UnmarshalData(&todoset); err != nil {
		return nil, fmt.Errorf("failed to parse todoset: %w", err)
	}

	return &todoset, nil
}
