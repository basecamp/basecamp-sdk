package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Project represents a Basecamp project.
type Project struct {
	ID             int64          `json:"id"`
	Status         string         `json:"status"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Purpose        string         `json:"purpose"`
	ClientsEnabled bool           `json:"clients_enabled"`
	BookmarkURL    string         `json:"bookmark_url"`
	URL            string         `json:"url"`
	AppURL         string         `json:"app_url"`
	Dock           []DockItem     `json:"dock,omitempty"`
	Bookmarked     bool           `json:"bookmarked"`
	ClientCompany  *ClientCompany `json:"client_company,omitempty"`
	Clientside     *Clientside    `json:"clientside,omitempty"`
}

// DockItem represents a tool in a project's dock.
type DockItem struct {
	ID       int64   `json:"id"`
	Title    string  `json:"title"`
	Name     string  `json:"name"`
	Enabled  bool    `json:"enabled"`
	Position *int    `json:"position"`
	URL      string  `json:"url"`
	AppURL   string  `json:"app_url"`
}

// ClientCompany represents a client company associated with a project.
type ClientCompany struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Clientside represents the client-facing portion of a project.
type Clientside struct {
	URL    string `json:"url"`
	AppURL string `json:"app_url"`
}

// ProjectStatus represents valid project statuses.
type ProjectStatus string

const (
	ProjectStatusActive   ProjectStatus = "active"
	ProjectStatusArchived ProjectStatus = "archived"
	ProjectStatusTrashed  ProjectStatus = "trashed"
)

// ProjectListOptions specifies options for listing projects.
type ProjectListOptions struct {
	// Status filters by project status (active, archived, trashed).
	// If empty, defaults to active projects.
	Status ProjectStatus
}

// CreateProjectRequest specifies the parameters for creating a project.
type CreateProjectRequest struct {
	// Name is the project name (required).
	Name string `json:"name"`
	// Description is an optional project description.
	Description string `json:"description,omitempty"`
}

// UpdateProjectRequest specifies the parameters for updating a project.
type UpdateProjectRequest struct {
	// Name is the project name (required for update).
	Name string `json:"name"`
	// Description is an optional project description.
	Description string `json:"description,omitempty"`
	// Admissions specifies access policy (invite, employee, team).
	Admissions string `json:"admissions,omitempty"`
	// ScheduleAttributes sets project start and end dates.
	ScheduleAttributes *ScheduleAttributes `json:"schedule_attributes,omitempty"`
}

// ScheduleAttributes specifies project schedule dates.
type ScheduleAttributes struct {
	// StartDate is the project start date (ISO 8601 format, e.g., "2022-01-01").
	StartDate string `json:"start_date"`
	// EndDate is the project end date (ISO 8601 format).
	EndDate string `json:"end_date"`
}

// ProjectsService handles project operations.
type ProjectsService struct {
	client *Client
}

// NewProjectsService creates a new ProjectsService.
func NewProjectsService(client *Client) *ProjectsService {
	return &ProjectsService{client: client}
}

// List returns all projects visible to the current user.
// By default, returns active projects sorted by most recently created first.
func (s *ProjectsService) List(ctx context.Context, opts *ProjectListOptions) ([]Project, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := "/projects.json"
	if opts != nil && opts.Status != "" {
		path = fmt.Sprintf("/projects.json?status=%s", opts.Status)
	}

	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	projects := make([]Project, 0, len(results))
	for _, raw := range results {
		var p Project
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("failed to parse project: %w", err)
		}
		projects = append(projects, p)
	}

	return projects, nil
}

// Get returns a project by ID.
func (s *ProjectsService) Get(ctx context.Context, id int64) (*Project, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/projects/%d.json", id)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var project Project
	if err := resp.UnmarshalData(&project); err != nil {
		return nil, fmt.Errorf("failed to parse project: %w", err)
	}

	return &project, nil
}

// Create creates a new project.
// Returns the created project.
func (s *ProjectsService) Create(ctx context.Context, req *CreateProjectRequest) (*Project, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req.Name == "" {
		return nil, ErrUsage("project name is required")
	}

	resp, err := s.client.Post(ctx, "/projects.json", req)
	if err != nil {
		return nil, err
	}

	var project Project
	if err := resp.UnmarshalData(&project); err != nil {
		return nil, fmt.Errorf("failed to parse project: %w", err)
	}

	return &project, nil
}

// Update updates an existing project.
// Returns the updated project.
func (s *ProjectsService) Update(ctx context.Context, id int64, req *UpdateProjectRequest) (*Project, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req.Name == "" {
		return nil, ErrUsage("project name is required")
	}

	path := fmt.Sprintf("/projects/%d.json", id)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var project Project
	if err := resp.UnmarshalData(&project); err != nil {
		return nil, fmt.Errorf("failed to parse project: %w", err)
	}

	return &project, nil
}

// Trash moves a project to the trash.
// Trashed projects are deleted after 30 days.
func (s *ProjectsService) Trash(ctx context.Context, id int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/projects/%d.json", id)
	_, err := s.client.Delete(ctx, path)
	return err
}
