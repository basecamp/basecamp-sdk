package basecamp

import (
	"context"
	"encoding/json"
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

	// Limit is the maximum number of projects to return.
	// If 0 (default), returns all projects.
	Limit int

	// Page, if non-zero, disables pagination and returns only the first page.
	// NOTE: The page number itself is not yet honored due to OpenAPI client
	// limitations. Use 0 to paginate through all results up to Limit.
	Page int
}

// ProjectListResult contains the results from listing projects.
type ProjectListResult struct {
	// Projects is the list of projects returned.
	Projects []Project
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
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
	client *AccountClient
}

// NewProjectsService creates a new ProjectsService.
func NewProjectsService(client *AccountClient) *ProjectsService {
	return &ProjectsService{client: client}
}

// List returns all projects visible to the current user.
// By default, returns active projects sorted by most recently created first.
//
// Pagination options:
//   - Limit: maximum number of projects to return (0 = all)
//   - Page: if non-zero, disables pagination and returns first page only
//
// The returned ProjectListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *ProjectsService) List(ctx context.Context, opts *ProjectListOptions) (result *ProjectListResult, err error) {
	op := OperationInfo{
		Service: "Projects", Operation: "List",
		ResourceType: "project", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	// Build params for generated client
	params := &generated.ListProjectsParams{}
	if opts != nil && opts.Status != "" {
		params.Status = string(opts.Status)
	}

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListProjectsWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var projects []Project
	if resp.JSON200 != nil {
		for _, gp := range *resp.JSON200 {
			projects = append(projects, projectFromGenerated(gp))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &ProjectListResult{Projects: projects, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Determine limit: 0 = all (default for projects)
	limit := 0
	if opts != nil {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(projects) >= limit {
		return &ProjectListResult{Projects: projects[:limit], Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, err := s.client.parent.FollowPagination(ctx, resp.HTTPResponse, len(projects), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gp generated.Project
		if err := json.Unmarshal(raw, &gp); err != nil {
			return nil, fmt.Errorf("failed to parse project: %w", err)
		}
		projects = append(projects, projectFromGenerated(gp))
	}

	return &ProjectListResult{Projects: projects, Meta: ListMeta{TotalCount: totalCount}}, nil
}

// Get returns a project by ID.
func (s *ProjectsService) Get(ctx context.Context, id int64) (result *Project, err error) {
	op := OperationInfo{
		Service: "Projects", Operation: "Get",
		ResourceType: "project", IsMutation: false,
		ResourceID: id,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetProjectWithResponse(ctx, s.client.accountID, id)
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

	project := projectFromGenerated(*resp.JSON200)
	return &project, nil
}

// Create creates a new project.
// Returns the created project.
func (s *ProjectsService) Create(ctx context.Context, req *CreateProjectRequest) (result *Project, err error) {
	op := OperationInfo{
		Service: "Projects", Operation: "Create",
		ResourceType: "project", IsMutation: true,
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
		err = ErrUsage("project name is required")
		return nil, err
	}

	body := generated.CreateProjectJSONRequestBody{
		Name:        req.Name,
		Description: req.Description,
	}

	resp, err := s.client.parent.gen.CreateProjectWithResponse(ctx, s.client.accountID, body)
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

	project := projectFromGenerated(*resp.JSON201)
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
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

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

	resp, err := s.client.parent.gen.UpdateProjectWithResponse(ctx, s.client.accountID, id, body)
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

	project := projectFromGenerated(*resp.JSON200)
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
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.TrashProjectWithResponse(ctx, s.client.accountID, id)
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

	if gp.Id != 0 {
		p.ID = gp.Id
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
			if gd.Id != 0 {
				di.ID = gd.Id
			}
			if gd.Position != 0 {
				pos := int(gd.Position)
				di.Position = &pos
			}
			p.Dock = append(p.Dock, di)
		}
	}

	// Convert client company
	if gp.ClientCompany.Id != 0 || gp.ClientCompany.Name != "" {
		p.ClientCompany = &ClientCompany{
			ID:   gp.ClientCompany.Id,
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
