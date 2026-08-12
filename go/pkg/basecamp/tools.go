package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Tool represents a dock tool in a Basecamp project.
//
// The projection is the bare recordings/recording partial: BC3's
// api/docks/tools/show.json.jbuilder renders it and adds nothing.
type Tool struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status"`
	// Type is the tool's recordable type, e.g. "Chat::Transcript", "Todoset", "Vault".
	Type            string `json:"type"`
	URL             string `json:"url"`
	AppURL          string `json:"app_url"`
	BookmarkURL     string `json:"bookmark_url,omitempty"`
	SubscriptionURL string `json:"subscription_url,omitempty"`
	// Position is nil while the tool is disabled (removed from the dock, not
	// deleted). Absence of Position, not Enabled, is the disabled signal.
	Position *int `json:"position"`
	// Parent is nil for a docked tool. It is populated only for a nested
	// recording reachable through this route: the dock-tool lookup scopes by
	// recordable type, so a vault nested inside another vault resolves here
	// and is not docked.
	Parent  *Parent `json:"parent,omitempty"`
	Bucket  *Bucket `json:"bucket,omitempty"`
	Creator *Person `json:"creator,omitempty"`

	// Name is always nil: the tool projection emits no `name` key. The dock
	// array on a project (Project.Dock) carries the tool's slug instead. The
	// pointer is deliberate — a plain string would read as "" for every tool
	// and look like a real answer.
	Name *string `json:"name,omitempty"`

	// Enabled is always nil: no layer of the tool projection emits an
	// `enabled` key. Use Position (see above) or the project's dock array.
	Enabled *bool `json:"enabled,omitempty"`
}

// CreateToolOptions specifies optional parameters for creating a tool.
type CreateToolOptions struct {
	// Title for the new tool. If empty, Basecamp assigns the next available default title for the tool type.
	Title string
	// VisibleToClients sets client visibility at create time (optional, tri-state).
	// nil omits the field so the server applies its own default visibility rule; a
	// non-nil value is sent verbatim, and an explicit false reaches the wire (the
	// pointer distinguishes unset from false). Honored only for tool types that
	// manage their own client visibility (Chat::Transcript, Kanban::Board); every
	// other tool type ignores it and inherits the project default.
	VisibleToClients *bool
}

// UpdateToolRequest specifies the parameters for updating (renaming) a tool.
type UpdateToolRequest struct {
	// Title is the new title for the tool (required).
	Title string `json:"title"`
}

// ToolsService handles dock tool operations.
type ToolsService struct {
	client *AccountClient
}

// NewToolsService creates a new ToolsService.
func NewToolsService(client *AccountClient) *ToolsService {
	return &ToolsService{client: client}
}

// Get returns a tool by ID.
func (s *ToolsService) Get(ctx context.Context, toolID int64) (result *Tool, err error) {
	op := OperationInfo{
		Service: "Tools", Operation: "Get",
		ResourceType: "tool", IsMutation: false,
		ResourceID: toolID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetToolWithResponse(ctx, s.client.accountID, toolID)
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

	tool := toolFromGenerated(*resp.JSON200)
	return &tool, nil
}

// Create adds a tool to the destination bucket.
// toolType is required and must be one of Basecamp's dock tool types:
// "Chat::Transcript", "Inbox", "Kanban::Board", "Message::Board",
// "Questionnaire", "Schedule", "Todoset", or "Vault".
// An optional title can be provided; if empty, Basecamp assigns the next available default title for the tool type.
// Returns the newly created tool.
func (s *ToolsService) Create(ctx context.Context, bucketID int64, toolType string, opts *CreateToolOptions) (result *Tool, err error) {
	op := OperationInfo{
		Service: "Tools", Operation: "Create",
		ResourceType: "tool", IsMutation: true,
		ProjectID: bucketID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if toolType == "" {
		err = ErrUsage("tool type is required")
		return nil, err
	}

	body := generated.CreateToolJSONRequestBody{
		ToolType: toolType,
	}
	if opts != nil {
		if opts.Title != "" {
			body.Title = &opts.Title
		}
		body.VisibleToClients = opts.VisibleToClients
	}

	resp, err := s.client.parent.gen.CreateToolWithResponse(ctx, s.client.accountID, bucketID, body)
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

	tool := toolFromGenerated(*resp.JSON201)
	return &tool, nil
}

// Update updates (renames) an existing tool.
// Returns the updated tool.
func (s *ToolsService) Update(ctx context.Context, toolID int64, title string) (result *Tool, err error) {
	op := OperationInfo{
		Service: "Tools", Operation: "Update",
		ResourceType: "tool", IsMutation: true,
		ResourceID: toolID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if title == "" {
		err = ErrUsage("tool title is required")
		return nil, err
	}

	body := generated.UpdateToolJSONRequestBody{
		Title: title,
	}

	resp, err := s.client.parent.gen.UpdateToolWithResponse(ctx, s.client.accountID, toolID, body)
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

	tool := toolFromGenerated(*resp.JSON200)
	return &tool, nil
}

// Delete moves a tool to the trash.
// Trashed tools can be recovered from the trash.
func (s *ToolsService) Delete(ctx context.Context, toolID int64) (err error) {
	op := OperationInfo{
		Service: "Tools", Operation: "Delete",
		ResourceType: "tool", IsMutation: true,
		ResourceID: toolID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DeleteToolWithResponse(ctx, s.client.accountID, toolID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Enable enables (shows) a tool on the project dock.
// The tool will be placed at the end of the dock.
func (s *ToolsService) Enable(ctx context.Context, toolID int64) (err error) {
	op := OperationInfo{
		Service: "Tools", Operation: "Enable",
		ResourceType: "tool", IsMutation: true,
		ResourceID: toolID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.EnableToolWithResponse(ctx, s.client.accountID, toolID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Disable disables (hides) a tool from the project dock.
// The tool is not deleted, just hidden from the dock.
func (s *ToolsService) Disable(ctx context.Context, toolID int64) (err error) {
	op := OperationInfo{
		Service: "Tools", Operation: "Disable",
		ResourceType: "tool", IsMutation: true,
		ResourceID: toolID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DisableToolWithResponse(ctx, s.client.accountID, toolID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Reposition changes the position of a tool on the project dock.
// position is 1-based (1 = first position on dock).
func (s *ToolsService) Reposition(ctx context.Context, toolID int64, position int) (err error) {
	op := OperationInfo{
		Service: "Tools", Operation: "Reposition",
		ResourceType: "tool", IsMutation: true,
		ResourceID: toolID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if position < 1 {
		err = ErrUsage("position must be at least 1")
		return err
	}

	body := generated.RepositionToolJSONRequestBody{
		Position: int32(position), // #nosec G115 -- position is validated and bounded by API
	}

	resp, err := s.client.parent.gen.RepositionToolWithResponse(ctx, s.client.accountID, toolID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// toolFromGenerated converts a generated Tool to our clean type.
func toolFromGenerated(gt generated.Tool) Tool {
	t := Tool{
		Status:           deref(gt.Status),
		VisibleToClients: gt.VisibleToClients,
		CreatedAt:        gt.CreatedAt,
		UpdatedAt:        gt.UpdatedAt,
		Title:            gt.Title,
		InheritsStatus:   gt.InheritsStatus,
		Type:             gt.Type,
		URL:              deref(gt.Url),
		AppURL:           deref(gt.AppUrl),
		BookmarkURL:      deref(gt.BookmarkUrl),
		SubscriptionURL:  deref(gt.SubscriptionUrl),
		Name:             gt.Name,
		Enabled:          gt.Enabled,
	}

	if gt.Id != 0 {
		t.ID = gt.Id
	}

	if gt.Position != nil {
		pos := int(*gt.Position)
		t.Position = &pos
	}

	if gt.Parent != nil {
		t.Parent = &Parent{
			ID:     gt.Parent.Id,
			Title:  gt.Parent.Title,
			Type:   gt.Parent.Type,
			URL:    gt.Parent.Url,
			AppURL: gt.Parent.AppUrl,
		}
	}

	if gt.Bucket != nil {
		t.Bucket = &Bucket{
			ID:   gt.Bucket.Id,
			Name: gt.Bucket.Name,
			Type: gt.Bucket.Type,
		}
	}

	if gt.Creator.Id != 0 || gt.Creator.Name != "" {
		creator := personFromGenerated(gt.Creator)
		t.Creator = &creator
	}

	return t
}
