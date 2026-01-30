package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Template represents a Basecamp project template.
type Template struct {
	ID          int64     `json:"id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
}

// ProjectConstruction represents the status of a project being created from a template.
type ProjectConstruction struct {
	ID      int64    `json:"id"`
	Status  string   `json:"status"`
	URL     string   `json:"url"`
	Project *Project `json:"project,omitempty"`
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
// The returned TemplateListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *TemplatesService) List(ctx context.Context) (result *TemplateListResult, err error) {
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

	resp, err := s.client.parent.gen.ListTemplatesWithResponse(ctx, s.client.accountID, nil)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header
	totalCount := parseTotalCount(resp.HTTPResponse)

	if resp.JSON200 == nil {
		return &TemplateListResult{Templates: nil, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	templates := make([]Template, 0, len(*resp.JSON200))
	for _, gt := range *resp.JSON200 {
		templates = append(templates, templateFromGenerated(gt))
	}

	return &TemplateListResult{Templates: templates, Meta: ListMeta{TotalCount: totalCount}}, nil
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
	if err = checkResponse(resp.HTTPResponse); err != nil {
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
		Description: req.Description,
	}

	resp, err := s.client.parent.gen.CreateTemplateWithResponse(ctx, s.client.accountID, body)
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
		Name:        req.Name,
		Description: req.Description,
	}

	resp, err := s.client.parent.gen.UpdateTemplateWithResponse(ctx, s.client.accountID, templateID, body)
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
	return checkResponse(resp.HTTPResponse)
}

// CreateProject creates a new project from a template.
// This operation is asynchronous; use GetConstruction to check the status.
func (s *TemplatesService) CreateProject(ctx context.Context, templateID int64, name, description string) (result *ProjectConstruction, err error) {
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

	if name == "" {
		err = ErrUsage("project name is required")
		return nil, err
	}

	body := generated.CreateProjectFromTemplateJSONRequestBody{
		Name:        name,
		Description: description,
	}

	resp, err := s.client.parent.gen.CreateProjectFromTemplateWithResponse(ctx, s.client.accountID, templateID, body)
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
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	construction := projectConstructionFromGenerated(*resp.JSON200)
	return &construction, nil
}

// templateFromGenerated converts a generated Template to our clean type.
func templateFromGenerated(gt generated.Template) Template {
	t := Template{
		Status:      gt.Status,
		CreatedAt:   gt.CreatedAt,
		UpdatedAt:   gt.UpdatedAt,
		Name:        gt.Name,
		Description: gt.Description,
	}

	if gt.Id != nil {
		t.ID = *gt.Id
	}

	return t
}

// projectConstructionFromGenerated converts a generated ProjectConstruction to our clean type.
func projectConstructionFromGenerated(gc generated.ProjectConstruction) ProjectConstruction {
	c := ProjectConstruction{
		Status: gc.Status,
		URL:    gc.Url,
	}

	if gc.Id != nil {
		c.ID = *gc.Id
	}

	if gc.Project.Id != nil || gc.Project.Name != "" {
		c.Project = &Project{
			Name:        gc.Project.Name,
			Description: gc.Project.Description,
			Purpose:     gc.Project.Purpose,
			CreatedAt:   gc.Project.CreatedAt,
			UpdatedAt:   gc.Project.UpdatedAt,
			Status:      gc.Project.Status,
			URL:         gc.Project.Url,
			AppURL:      gc.Project.AppUrl,
		}
		if gc.Project.Id != nil {
			c.Project.ID = *gc.Project.Id
		}
	}

	return c
}
