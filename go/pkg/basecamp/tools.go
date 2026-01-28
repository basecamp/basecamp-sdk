package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
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
	// Title is the new title for the tool (required).
	Title string `json:"title"`
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
func (s *ToolsService) Get(ctx context.Context, bucketID, toolID int64) (result *Tool, err error) {
	op := OperationInfo{
		Service: "Tools", Operation: "Get",
		ResourceType: "tool", IsMutation: false,
		BucketID: bucketID, ResourceID: toolID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetToolWithResponse(ctx, bucketID, toolID)
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

	tool := toolFromGenerated(resp.JSON200.Tool)
	return &tool, nil
}

// Create clones an existing tool to create a new one.
// bucketID is the project ID, sourceToolID is the ID of the tool to clone.
// Returns the newly created tool.
func (s *ToolsService) Create(ctx context.Context, bucketID, sourceToolID int64) (result *Tool, err error) {
	op := OperationInfo{
		Service: "Tools", Operation: "Create",
		ResourceType: "tool", IsMutation: true,
		BucketID: bucketID, ResourceID: sourceToolID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.CloneToolWithResponse(ctx, bucketID, sourceToolID)
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

	tool := toolFromGenerated(resp.JSON200.Tool)
	return &tool, nil
}

// Update updates (renames) an existing tool.
// bucketID is the project ID, toolID is the tool ID.
// Returns the updated tool.
func (s *ToolsService) Update(ctx context.Context, bucketID, toolID int64, title string) (result *Tool, err error) {
	op := OperationInfo{
		Service: "Tools", Operation: "Update",
		ResourceType: "tool", IsMutation: true,
		BucketID: bucketID, ResourceID: toolID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if title == "" {
		err = ErrUsage("tool title is required")
		return nil, err
	}

	body := generated.UpdateToolJSONRequestBody{
		Title: title,
	}

	resp, err := s.client.gen.UpdateToolWithResponse(ctx, bucketID, toolID, body)
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

	tool := toolFromGenerated(resp.JSON200.Tool)
	return &tool, nil
}

// Delete moves a tool to the trash.
// bucketID is the project ID, toolID is the tool ID.
// Trashed tools can be recovered from the trash.
func (s *ToolsService) Delete(ctx context.Context, bucketID, toolID int64) (err error) {
	op := OperationInfo{
		Service: "Tools", Operation: "Delete",
		ResourceType: "tool", IsMutation: true,
		BucketID: bucketID, ResourceID: toolID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.DeleteToolWithResponse(ctx, bucketID, toolID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Enable enables (shows) a tool on the project dock.
// bucketID is the project ID, toolID is the tool ID.
// The tool will be placed at the end of the dock.
func (s *ToolsService) Enable(ctx context.Context, bucketID, toolID int64) (err error) {
	op := OperationInfo{
		Service: "Tools", Operation: "Enable",
		ResourceType: "tool", IsMutation: true,
		BucketID: bucketID, ResourceID: toolID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.EnableToolWithResponse(ctx, bucketID, toolID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Disable disables (hides) a tool from the project dock.
// bucketID is the project ID, toolID is the tool ID.
// The tool is not deleted, just hidden from the dock.
func (s *ToolsService) Disable(ctx context.Context, bucketID, toolID int64) (err error) {
	op := OperationInfo{
		Service: "Tools", Operation: "Disable",
		ResourceType: "tool", IsMutation: true,
		BucketID: bucketID, ResourceID: toolID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.DisableToolWithResponse(ctx, bucketID, toolID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Reposition changes the position of a tool on the project dock.
// bucketID is the project ID, toolID is the tool ID.
// position is 1-based (1 = first position on dock).
func (s *ToolsService) Reposition(ctx context.Context, bucketID, toolID int64, position int) (err error) {
	op := OperationInfo{
		Service: "Tools", Operation: "Reposition",
		ResourceType: "tool", IsMutation: true,
		BucketID: bucketID, ResourceID: toolID,
	}
	if gater, ok := s.client.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	if position < 1 {
		err = ErrUsage("position must be at least 1")
		return err
	}

	body := generated.RepositionToolJSONRequestBody{
		Position: int32(position),
	}

	resp, err := s.client.gen.RepositionToolWithResponse(ctx, bucketID, toolID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// toolFromGenerated converts a generated Tool to our clean type.
func toolFromGenerated(gt generated.Tool) Tool {
	t := Tool{
		Status:    gt.Status,
		CreatedAt: gt.CreatedAt,
		UpdatedAt: gt.UpdatedAt,
		Title:     gt.Title,
		Name:      gt.Name,
		Enabled:   gt.Enabled,
		URL:       gt.Url,
		AppURL:    gt.AppUrl,
	}

	if gt.Id != nil {
		t.ID = *gt.Id
	}

	if gt.Position != 0 {
		pos := int(gt.Position)
		t.Position = &pos
	}

	if gt.Bucket.Id != nil || gt.Bucket.Name != "" {
		t.Bucket = &Bucket{
			ID:   derefInt64(gt.Bucket.Id),
			Name: gt.Bucket.Name,
			Type: gt.Bucket.Type,
		}
	}

	return t
}
