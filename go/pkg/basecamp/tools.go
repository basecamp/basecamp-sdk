package basecamp

import (
	"context"
	"fmt"
	"time"
)

// Tool represents a dock tool in a Basecamp project.
type Tool struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Title     string    `json:"title"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	Position  *int      `json:"position"`
	URL       string    `json:"url"`
	AppURL    string    `json:"app_url"`
	Bucket    *Bucket   `json:"bucket,omitempty"`
}

// UpdateToolRequest specifies the parameters for updating (renaming) a tool.
type UpdateToolRequest struct {
	// Name is the new name for the tool (required).
	Name string `json:"name"`
}

// ToolsService handles dock tool operations.
type ToolsService struct {
	client *Client
}

// NewToolsService creates a new ToolsService.
func NewToolsService(client *Client) *ToolsService {
	return &ToolsService{client: client}
}

// Get returns a tool by ID.
// bucketID is the project ID, toolID is the tool ID.
func (s *ToolsService) Get(ctx context.Context, bucketID, toolID int64) (*Tool, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/dock/tools/%d.json", bucketID, toolID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var tool Tool
	if err := resp.UnmarshalData(&tool); err != nil {
		return nil, fmt.Errorf("failed to parse tool: %w", err)
	}

	return &tool, nil
}

// Create clones an existing tool to create a new one.
// bucketID is the project ID, sourceToolID is the ID of the tool to clone.
// Returns the newly created tool.
func (s *ToolsService) Create(ctx context.Context, bucketID, sourceToolID int64) (*Tool, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/dock/tools/%d/clone.json", bucketID, sourceToolID)
	resp, err := s.client.Post(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	var tool Tool
	if err := resp.UnmarshalData(&tool); err != nil {
		return nil, fmt.Errorf("failed to parse tool: %w", err)
	}

	return &tool, nil
}

// Update updates (renames) an existing tool.
// bucketID is the project ID, toolID is the tool ID.
// Returns the updated tool.
func (s *ToolsService) Update(ctx context.Context, bucketID, toolID int64, name string) (*Tool, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if name == "" {
		return nil, ErrUsage("tool name is required")
	}

	path := fmt.Sprintf("/buckets/%d/dock/tools/%d.json", bucketID, toolID)
	req := &UpdateToolRequest{Name: name}
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var tool Tool
	if err := resp.UnmarshalData(&tool); err != nil {
		return nil, fmt.Errorf("failed to parse tool: %w", err)
	}

	return &tool, nil
}

// Delete moves a tool to the trash.
// bucketID is the project ID, toolID is the tool ID.
// Trashed tools can be recovered from the trash.
func (s *ToolsService) Delete(ctx context.Context, bucketID, toolID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/dock/tools/%d.json", bucketID, toolID)
	_, err := s.client.Delete(ctx, path)
	return err
}

// Enable enables (shows) a tool on the project dock.
// bucketID is the project ID, toolID is the tool ID.
// The tool will be placed at the end of the dock.
func (s *ToolsService) Enable(ctx context.Context, bucketID, toolID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/dock/tools/%d/position.json", bucketID, toolID)
	_, err := s.client.Post(ctx, path, nil)
	return err
}

// Disable disables (hides) a tool from the project dock.
// bucketID is the project ID, toolID is the tool ID.
// The tool is not deleted, just hidden from the dock.
func (s *ToolsService) Disable(ctx context.Context, bucketID, toolID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/dock/tools/%d/position.json", bucketID, toolID)
	_, err := s.client.Delete(ctx, path)
	return err
}

// Reposition changes the position of a tool on the project dock.
// bucketID is the project ID, toolID is the tool ID.
// position is 1-based (1 = first position on dock).
func (s *ToolsService) Reposition(ctx context.Context, bucketID, toolID int64, position int) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	if position < 1 {
		return ErrUsage("position must be at least 1")
	}

	path := fmt.Sprintf("/buckets/%d/dock/tools/%d/position.json", bucketID, toolID)
	body := map[string]int{"position": position}
	_, err := s.client.Put(ctx, path, body)
	return err
}
