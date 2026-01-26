package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
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

// CreateProjectRequest specifies the parameters for creating a project from a template.
type CreateProjectFromTemplateRequest struct {
	// Name is the project name (required).
	Name string `json:"name"`
	// Description is an optional project description.
	Description string `json:"description,omitempty"`
}

// TemplatesService handles template operations.
type TemplatesService struct {
	client *Client
}

// NewTemplatesService creates a new TemplatesService.
func NewTemplatesService(client *Client) *TemplatesService {
	return &TemplatesService{client: client}
}

// List returns all templates visible to the current user.
func (s *TemplatesService) List(ctx context.Context) ([]Template, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	results, err := s.client.GetAll(ctx, "/templates.json")
	if err != nil {
		return nil, err
	}

	templates := make([]Template, 0, len(results))
	for _, raw := range results {
		var t Template
		if err := json.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("failed to parse template: %w", err)
		}
		templates = append(templates, t)
	}

	return templates, nil
}

// Get returns a template by ID.
func (s *TemplatesService) Get(ctx context.Context, templateID int64) (*Template, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/templates/%d.json", templateID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var template Template
	if err := resp.UnmarshalData(&template); err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return &template, nil
}

// Create creates a new template.
// Returns the created template.
func (s *TemplatesService) Create(ctx context.Context, req *CreateTemplateRequest) (*Template, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req.Name == "" {
		return nil, ErrUsage("template name is required")
	}

	resp, err := s.client.Post(ctx, "/templates.json", req)
	if err != nil {
		return nil, err
	}

	var template Template
	if err := resp.UnmarshalData(&template); err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return &template, nil
}

// Update updates an existing template.
// Returns the updated template.
func (s *TemplatesService) Update(ctx context.Context, templateID int64, req *UpdateTemplateRequest) (*Template, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req.Name == "" {
		return nil, ErrUsage("template name is required")
	}

	path := fmt.Sprintf("/templates/%d.json", templateID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var template Template
	if err := resp.UnmarshalData(&template); err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return &template, nil
}

// Delete deletes a template.
func (s *TemplatesService) Delete(ctx context.Context, templateID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/templates/%d.json", templateID)
	_, err := s.client.Delete(ctx, path)
	return err
}

// CreateProject creates a new project from a template.
// This operation is asynchronous; use GetConstruction to check the status.
func (s *TemplatesService) CreateProject(ctx context.Context, templateID int64, name, description string) (*ProjectConstruction, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, ErrUsage("project name is required")
	}

	path := fmt.Sprintf("/templates/%d/project_constructions.json", templateID)
	req := &CreateProjectFromTemplateRequest{
		Name:        name,
		Description: description,
	}

	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var construction ProjectConstruction
	if err := resp.UnmarshalData(&construction); err != nil {
		return nil, fmt.Errorf("failed to parse project construction: %w", err)
	}

	return &construction, nil
}

// GetConstruction returns the status of a project construction.
func (s *TemplatesService) GetConstruction(ctx context.Context, templateID, constructionID int64) (*ProjectConstruction, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/templates/%d/project_constructions/%d.json", templateID, constructionID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var construction ProjectConstruction
	if err := resp.UnmarshalData(&construction); err != nil {
		return nil, fmt.Errorf("failed to parse project construction: %w", err)
	}

	return &construction, nil
}
