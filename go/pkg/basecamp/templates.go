package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// TemplateListOptions specifies options for listing templates.
type TemplateListOptions struct {
	// Limit is the maximum number of templates to return.
	// If 0, returns all. Use -1 for unlimited (same as 0).
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination.
	Page int
}

// Template represents a Basecamp project template.
type Template struct {
	ID          int64      `json:"id"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	URL         string     `json:"url,omitempty"`
	AppURL      string     `json:"app_url,omitempty"`
	Dock        []DockItem `json:"dock,omitempty"`
}

// ProjectConstruction represents the status of a project being created from a template.
type ProjectConstruction struct {
	ID      int64    `json:"id"`
	Status  string   `json:"status"`
	URL     string   `json:"url"`
	Project *Project `json:"project,omitempty"`
}

// CreateProjectOptions specifies optional parameters for creating a project from a template.
type CreateProjectOptions struct {
	// StartDate is the date the new project starts on (ISO 8601, e.g. "2026-09-01").
	// Template dates are anchored to the Sunday on or before it. Empty anchors them
	// to the week in which the asynchronous construction is processed.
	StartDate string
}

// TemplateLibraryTodolists contains the account's to-do list templates.
type TemplateLibraryTodolists struct {
	Bucket    Bucket     `json:"bucket"`
	Todoset   Parent     `json:"todoset"`
	Todolists []Todolist `json:"todolists"`
}

// TemplateLibraryCardTables contains the account's card table templates.
type TemplateLibraryCardTables struct {
	Bucket Bucket `json:"bucket"`
	// Nil until the first card table template is created.
	KanbanBoardset *Parent     `json:"kanban_boardset"`
	CardTables     []Recording `json:"card_tables"`
}

// TemplateLibraryConfirmationPerson identifies a person whose project access requires confirmation.
type TemplateLibraryConfirmationPerson struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// TemplateLibraryCopy represents the current state of an asynchronous template copy.
type TemplateLibraryCopy struct {
	ID                  int64  `json:"id"`
	Status              string `json:"status"`
	SourceRecordingID   int64  `json:"source_recording_id"`
	DestinationParentID int64  `json:"destination_parent_id"`
	URL                 string `json:"url"`
	// Exactly one destination is set once the copy completes, and neither before.
	DestinationTodolist  *Todolist  `json:"destination_todolist,omitempty"`
	DestinationCardTable *CardTable `json:"destination_card_table,omitempty"`
}

// CreateTemplateLibraryCopyRequest specifies where to copy a template.
type CreateTemplateLibraryCopyRequest struct {
	TemplateRecordingID int64 `json:"template_recording_id"`
	// DestinationProjectID lets Basecamp resolve the container from the template's
	// kind. Supply this or DestinationParentID, not both.
	DestinationProjectID int64 `json:"destination_project_id,omitempty"`
	// DestinationParentID names the container directly, for a caller holding one.
	DestinationParentID   int64 `json:"destination_parent_id,omitempty"`
	AddingPeopleConfirmed bool  `json:"adding_people_confirmed,omitempty"`
}

// CreateTemplateRequest specifies the parameters for creating a template.
type CreateTemplateRequest struct {
	// Name is the template name (required).
	Name string `json:"name"`
	// Description is an optional template description.
	Description string `json:"description,omitempty"`
}

// UpdateTemplateRequest specifies the parameters for updating a template.
type UpdateTemplateRequest struct {
	// Name is the template name (required for update).
	Name string `json:"name"`
	// Description is an optional template description.
	Description string `json:"description,omitempty"`
}

// CreateProjectFromTemplateRequest specifies the parameters for creating a project from a template.
type CreateProjectFromTemplateRequest struct {
	// Name is the project name (required).
	Name string `json:"name"`
	// Description is an optional project description.
	Description string `json:"description,omitempty"`
}

// TemplateListResult contains the results from listing templates.
type TemplateListResult struct {
	// Templates is the list of templates returned.
	Templates []Template
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// TemplatesService handles template operations.
type TemplatesService struct {
	client *AccountClient
}

// NewTemplatesService creates a new TemplatesService.
func NewTemplatesService(client *AccountClient) *TemplatesService {
	return &TemplatesService{client: client}
}

// List returns all templates visible to the current user.
//
// Pagination options:
//   - Limit: maximum number of templates to return (0 = all, -1 = unlimited)
//   - Page: if positive, fetches only that page and disables auto-pagination
//
// The returned TemplateListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *TemplatesService) List(ctx context.Context, opts *TemplateListOptions) (result *TemplateListResult, err error) {
	op := OperationInfo{
		Service: "Templates", Operation: "List",
		ResourceType: "template", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.ListTemplatesParams
	if opts != nil && opts.Page > 0 {
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params = &generated.ListTemplatesParams{Page: page}
	}

	resp, err := s.client.parent.gen.ListTemplatesWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var templates []Template
	if resp.JSON200 != nil {
		for _, gt := range *resp.JSON200 {
			templates = append(templates, templateFromGenerated(gt))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		keep, truncated := pageCap(len(templates), opts.Limit, resp.HTTPResponse)
		return &TemplateListResult{Templates: templates[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
	}

	// Determine limit: 0 = all (no limit)
	limit := 0
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0 // unlimited
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	// Check if we already have enough items
	if limit > 0 && len(templates) >= limit {
		return &TemplateListResult{Templates: templates[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(templates), limit)}}, nil
	}

	// Follow pagination via Link headers
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(templates), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gt generated.Template
		if err := json.Unmarshal(raw, &gt); err != nil {
			return nil, fmt.Errorf("failed to parse template: %w", err)
		}
		templates = append(templates, templateFromGenerated(gt))
	}

	return &TemplateListResult{Templates: templates, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Get returns a template by ID.
func (s *TemplatesService) Get(ctx context.Context, templateID int64) (result *Template, err error) {
	op := OperationInfo{
		Service: "Templates", Operation: "Get",
		ResourceType: "template", IsMutation: false,
		ResourceID: templateID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetTemplateWithResponse(ctx, s.client.accountID, templateID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	template := templateFromGenerated(*resp.JSON200)
	return &template, nil
}

// Create creates a new template.
// Returns the created template.
func (s *TemplatesService) Create(ctx context.Context, req *CreateTemplateRequest) (result *Template, err error) {
	op := OperationInfo{
		Service: "Templates", Operation: "Create",
		ResourceType: "template", IsMutation: true,
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
		err = ErrUsage("template name is required")
		return nil, err
	}

	body := generated.CreateTemplateJSONRequestBody{
		Name:        req.Name,
		Description: omitzero(req.Description),
	}

	resp, err := s.client.parent.gen.CreateTemplateWithResponse(ctx, s.client.accountID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	template := templateFromGenerated(*resp.JSON201)
	return &template, nil
}

// Update updates an existing template.
// Returns the updated template.
func (s *TemplatesService) Update(ctx context.Context, templateID int64, req *UpdateTemplateRequest) (result *Template, err error) {
	op := OperationInfo{
		Service: "Templates", Operation: "Update",
		ResourceType: "template", IsMutation: true,
		ResourceID: templateID,
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
		err = ErrUsage("template name is required")
		return nil, err
	}

	body := generated.UpdateTemplateJSONRequestBody{
		Name: &req.Name,
	}
	if req.Description != "" {
		body.Description = &req.Description
	}

	resp, err := s.client.parent.gen.UpdateTemplateWithResponse(ctx, s.client.accountID, templateID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	template := templateFromGenerated(*resp.JSON200)
	return &template, nil
}

// Delete deletes a template.
func (s *TemplatesService) Delete(ctx context.Context, templateID int64) (err error) {
	op := OperationInfo{
		Service: "Templates", Operation: "Delete",
		ResourceType: "template", IsMutation: true,
		ResourceID: templateID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DeleteTemplateWithResponse(ctx, s.client.accountID, templateID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// CreateProject creates a new project from a template.
// This operation is asynchronous; use GetConstruction to check the status.
func (s *TemplatesService) CreateProject(ctx context.Context, templateID int64, name, description string, opts ...*CreateProjectOptions) (result *ProjectConstruction, err error) {
	op := OperationInfo{
		Service: "Templates", Operation: "CreateProject",
		ResourceType: "project_construction", IsMutation: true,
		ResourceID: templateID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if len(opts) > 1 {
		err = ErrUsage("CreateProject accepts at most one CreateProjectOptions argument")
		return nil, err
	}

	if name == "" {
		err = ErrUsage("project name is required")
		return nil, err
	}

	body := generated.CreateProjectFromTemplateJSONRequestBody{
		Project: generated.ProjectConstructionAttributes{
			Name:        name,
			Description: omitzero(description),
		},
	}
	if len(opts) > 0 && opts[0] != nil {
		body.Project.StartDate = omitzero(opts[0].StartDate)
	}

	resp, err := s.client.parent.gen.CreateProjectFromTemplateWithResponse(ctx, s.client.accountID, templateID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	construction := projectConstructionFromGenerated(*resp.JSON201)
	return &construction, nil
}

// GetConstruction returns the status of a project construction.
func (s *TemplatesService) GetConstruction(ctx context.Context, templateID, constructionID int64) (result *ProjectConstruction, err error) {
	op := OperationInfo{
		Service: "Templates", Operation: "GetConstruction",
		ResourceType: "project_construction", IsMutation: false,
		ResourceID: constructionID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetProjectConstructionWithResponse(ctx, s.client.accountID, templateID, constructionID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	construction := projectConstructionFromGenerated(*resp.JSON200)
	return &construction, nil
}

// GetLibraryTodolists returns the account's to-do list templates.
func (s *TemplatesService) GetLibraryTodolists(ctx context.Context) (result *TemplateLibraryTodolists, err error) {
	op := OperationInfo{
		Service: "Templates", Operation: "GetLibraryTodolists",
		ResourceType: "template_library", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetTemplateLibraryTodolistsWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	library := templateLibraryTodolistsFromGenerated(*resp.JSON200)
	return &library, nil
}

// GetLibraryCardTables returns the account's card table templates.
func (s *TemplatesService) GetLibraryCardTables(ctx context.Context) (result *TemplateLibraryCardTables, err error) {
	op := OperationInfo{
		Service: "Templates", Operation: "GetLibraryCardTables",
		ResourceType: "template_library", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetTemplateLibraryCardTablesWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	library := templateLibraryCardTablesFromGenerated(*resp.JSON200)
	return &library, nil
}

// CreateLibraryCardTable creates a card table template with the default columns.
// name is write-only: the returned CardTable carries it as Title.
func (s *TemplatesService) CreateLibraryCardTable(ctx context.Context, name string) (result *CardTable, err error) {
	op := OperationInfo{
		Service: "Templates", Operation: "CreateLibraryCardTable",
		ResourceType: "template_library_card_table", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if name == "" {
		err = ErrUsage("card table template name is required")
		return nil, err
	}

	body := generated.CreateTemplateLibraryCardTableJSONRequestBody{Name: name}
	resp, err := s.client.parent.gen.CreateTemplateLibraryCardTableWithResponse(ctx, s.client.accountID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	cardTable := cardTableFromGenerated(*resp.JSON201)
	return &cardTable, nil
}

// CreateLibraryCopy starts copying a to-do list template into a project.
func (s *TemplatesService) CreateLibraryCopy(ctx context.Context, req *CreateTemplateLibraryCopyRequest) (result *TemplateLibraryCopy, err error) {
	op := OperationInfo{
		Service: "Templates", Operation: "CreateLibraryCopy",
		ResourceType: "template_library_copy", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil {
		err = ErrUsage("template library copy request is required")
		return nil, err
	}

	body := generated.CreateTemplateLibraryCopyJSONRequestBody{
		TemplateRecordingId:   req.TemplateRecordingID,
		DestinationProjectId:  omitzero(req.DestinationProjectID),
		DestinationParentId:   omitzero(req.DestinationParentID),
		AddingPeopleConfirmed: omitzero(req.AddingPeopleConfirmed),
	}
	resp, err := s.client.parent.gen.CreateTemplateLibraryCopyWithResponse(ctx, s.client.accountID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	templateCopy := templateLibraryCopyFromGenerated(*resp.JSON201)
	return &templateCopy, nil
}

// GetLibraryCopy returns the current state of a to-do list template copy.
func (s *TemplatesService) GetLibraryCopy(ctx context.Context, copyID int64) (result *TemplateLibraryCopy, err error) {
	op := OperationInfo{
		Service: "Templates", Operation: "GetLibraryCopy",
		ResourceType: "template_library_copy", IsMutation: false,
		ResourceID: copyID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetTemplateLibraryCopyWithResponse(ctx, s.client.accountID, copyID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	templateCopy := templateLibraryCopyFromGenerated(*resp.JSON200)
	return &templateCopy, nil
}

// templateFromGenerated converts a generated Template to our clean type.
func templateFromGenerated(gt generated.Template) Template {
	t := Template{
		Status:      deref(gt.Status),
		CreatedAt:   gt.CreatedAt,
		UpdatedAt:   gt.UpdatedAt,
		Name:        gt.Name,
		Description: deref(gt.Description),
		URL:         deref(gt.Url),
		AppURL:      deref(gt.AppUrl),
	}

	if gt.Id != 0 {
		t.ID = gt.Id
	}

	if len(gt.Dock) > 0 {
		t.Dock = make([]DockItem, 0, len(gt.Dock))
		for _, gd := range gt.Dock {
			t.Dock = append(t.Dock, dockItemFromGenerated(gd))
		}
	}

	return t
}

// projectConstructionFromGenerated converts a generated ProjectConstruction to our clean type.
func projectConstructionFromGenerated(gc generated.ProjectConstruction) ProjectConstruction {
	c := ProjectConstruction{
		Status: gc.Status,
		URL:    deref(gc.Url),
	}

	if gc.Id != 0 {
		c.ID = gc.Id
	}

	if gc.Project != nil {
		project := projectFromGenerated(*gc.Project)
		c.Project = &project
	}

	return c
}

func templateLibraryTodolistsFromGenerated(gl generated.TemplateLibraryTodolists) TemplateLibraryTodolists {
	library := TemplateLibraryTodolists{
		Bucket: Bucket{
			ID:   gl.Bucket.Id,
			Name: gl.Bucket.Name,
			Type: gl.Bucket.Type,
		},
		Todoset: Parent{
			ID:     gl.Todoset.Id,
			Title:  gl.Todoset.Title,
			Type:   gl.Todoset.Type,
			URL:    gl.Todoset.Url,
			AppURL: gl.Todoset.AppUrl,
		},
		Todolists: make([]Todolist, 0, len(gl.Todolists)),
	}
	for _, todolist := range gl.Todolists {
		library.Todolists = append(library.Todolists, todolistFromGenerated(todolist))
	}
	return library
}

func templateLibraryCardTablesFromGenerated(gl generated.TemplateLibraryCardTables) TemplateLibraryCardTables {
	library := TemplateLibraryCardTables{
		Bucket: Bucket{
			ID:   gl.Bucket.Id,
			Name: gl.Bucket.Name,
			Type: gl.Bucket.Type,
		},
		CardTables: make([]Recording, 0, len(gl.CardTables)),
	}

	if gl.KanbanBoardset != nil {
		library.KanbanBoardset = &Parent{
			ID:     gl.KanbanBoardset.Id,
			Title:  gl.KanbanBoardset.Title,
			Type:   gl.KanbanBoardset.Type,
			URL:    gl.KanbanBoardset.Url,
			AppURL: gl.KanbanBoardset.AppUrl,
		}
	}

	for _, cardTable := range gl.CardTables {
		library.CardTables = append(library.CardTables, recordingFromGenerated(cardTable))
	}
	return library
}

func templateLibraryCopyFromGenerated(gc generated.TemplateLibraryCopy) TemplateLibraryCopy {
	templateCopy := TemplateLibraryCopy{
		ID:                  gc.Id,
		Status:              gc.Status,
		SourceRecordingID:   gc.SourceRecordingId,
		DestinationParentID: gc.DestinationParentId,
		URL:                 gc.Url,
	}
	if gc.DestinationTodolist != nil {
		todolist := todolistFromGenerated(*gc.DestinationTodolist)
		templateCopy.DestinationTodolist = &todolist
	}
	if gc.DestinationCardTable != nil {
		cardTable := cardTableFromGenerated(*gc.DestinationCardTable)
		templateCopy.DestinationCardTable = &cardTable
	}
	return templateCopy
}
