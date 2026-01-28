package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
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
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Position *int   `json:"position"`
	URL      string `json:"url"`
	AppURL   string `json:"app_url"`
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
func (s *ProjectsService) List(ctx context.Context, opts *ProjectListOptions) (result []Project, err error) {
	op := OperationInfo{
		Service: "Projects", Operation: "List",
		ResourceType: "project", IsMutation: false,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	params := &generated.ListProjectsParams{}
	if opts != nil && opts.Status != "" {
		params.Status = string(opts.Status)
	}

	resp, err := s.client.gen.ListProjectsWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	projects := make([]Project, 0, len(*resp.JSON200))
	for _, gp := range *resp.JSON200 {
		projects = append(projects, projectFromGenerated(gp))
	}

	return projects, nil
}

// Get returns a project by ID.
func (s *ProjectsService) Get(ctx context.Context, id int64) (result *Project, err error) {
	op := OperationInfo{
		Service: "Projects", Operation: "Get",
		ResourceType: "project", IsMutation: false,
		ResourceID: id,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetProjectWithResponse(ctx, id)
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

	project := projectFromGenerated(resp.JSON200.Project)
	return &project, nil
}

// Create creates a new project.
// Returns the created project.
func (s *ProjectsService) Create(ctx context.Context, req *CreateProjectRequest) (result *Project, err error) {
	op := OperationInfo{
		Service: "Projects", Operation: "Create",
		ResourceType: "project", IsMutation: true,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req.Name == "" {
		err = ErrUsage("project name is required")
		return nil, err
	}

	body := generated.CreateProjectJSONRequestBody{
		Name:        req.Name,
		Description: req.Description,
	}

	resp, err := s.client.gen.CreateProjectWithResponse(ctx, body)
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

	project := projectFromGenerated(resp.JSON200.Project)
	return &project, nil
}

// Update updates an existing project.
// Returns the updated project.
func (s *ProjectsService) Update(ctx context.Context, id int64, req *UpdateProjectRequest) (result *Project, err error) {
	op := OperationInfo{
		Service: "Projects", Operation: "Update",
		ResourceType: "project", IsMutation: true,
		ResourceID: id,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req.Name == "" {
		err = ErrUsage("project name is required")
		return nil, err
	}

	body := generated.UpdateProjectJSONRequestBody{
		Name:        req.Name,
		Description: req.Description,
		Admissions:  req.Admissions,
	}
	if req.ScheduleAttributes != nil {
		body.ScheduleAttributes = generated.ScheduleAttributes{
			StartDate: req.ScheduleAttributes.StartDate,
			EndDate:   req.ScheduleAttributes.EndDate,
		}
	}

	resp, err := s.client.gen.UpdateProjectWithResponse(ctx, id, body)
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

	project := projectFromGenerated(resp.JSON200.Project)
	return &project, nil
}

// Trash moves a project to the trash.
// Trashed projects are deleted after 30 days.
func (s *ProjectsService) Trash(ctx context.Context, id int64) (err error) {
	op := OperationInfo{
		Service: "Projects", Operation: "Trash",
		ResourceType: "project", IsMutation: true,
		ResourceID: id,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.TrashProjectWithResponse(ctx, id)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// projectFromGenerated converts a generated Project to our clean Project type.
func projectFromGenerated(gp generated.Project) Project {
	p := Project{
		Status:         gp.Status,
		Name:           gp.Name,
		Description:    gp.Description,
		Purpose:        gp.Purpose,
		ClientsEnabled: gp.ClientsEnabled,
		BookmarkURL:    gp.BookmarkUrl,
		URL:            gp.Url,
		AppURL:         gp.AppUrl,
		Bookmarked:     gp.Bookmarked,
		CreatedAt:      gp.CreatedAt,
		UpdatedAt:      gp.UpdatedAt,
	}

	if gp.Id != nil {
		p.ID = *gp.Id
	}

	// Convert dock items
	if len(gp.Dock) > 0 {
		p.Dock = make([]DockItem, 0, len(gp.Dock))
		for _, gd := range gp.Dock {
			di := DockItem{
				Title:   gd.Title,
				Name:    gd.Name,
				Enabled: gd.Enabled,
				URL:     gd.Url,
				AppURL:  gd.AppUrl,
			}
			if gd.Id != nil {
				di.ID = *gd.Id
			}
			if gd.Position != 0 {
				pos := int(gd.Position)
				di.Position = &pos
			}
			p.Dock = append(p.Dock, di)
		}
	}

	// Convert client company
	if gp.ClientCompany.Id != nil || gp.ClientCompany.Name != "" {
		p.ClientCompany = &ClientCompany{
			ID:   derefInt64(gp.ClientCompany.Id),
			Name: gp.ClientCompany.Name,
		}
	}

	// Convert clientside
	if gp.Clientside.Url != "" || gp.Clientside.AppUrl != "" {
		p.Clientside = &Clientside{
			URL:    gp.Clientside.Url,
			AppURL: gp.Clientside.AppUrl,
		}
	}

	return p
}
