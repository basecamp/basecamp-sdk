package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Gauge represents a gauge (progress indicator) on a project.
type Gauge struct {
	ID                     int64     `json:"id"`
	Title                  string    `json:"title,omitempty"`
	Description            string    `json:"description,omitempty"`
	Enabled                bool      `json:"enabled,omitempty"`
	Status                 string    `json:"status,omitempty"`
	LastNeedleColor        string    `json:"last_needle_color,omitempty"`
	LastNeedlePosition     int32     `json:"last_needle_position,omitempty"`
	PreviousNeedlePosition int32     `json:"previous_needle_position,omitempty"`
	InheritsStatus         bool      `json:"inherits_status,omitempty"`
	VisibleToClients       bool      `json:"visible_to_clients,omitempty"`
	Type                   string    `json:"type,omitempty"`
	URL                    string    `json:"url,omitempty"`
	AppURL                 string    `json:"app_url,omitempty"`
	BookmarkURL            string    `json:"bookmark_url,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// GaugeNeedle represents a single needle (progress update) on a gauge.
type GaugeNeedle struct {
	ID               int64     `json:"id"`
	Title            string    `json:"title,omitempty"`
	Description      string    `json:"description,omitempty"`
	Position         int32     `json:"position,omitempty"`
	Color            string    `json:"color,omitempty"`
	Status           string    `json:"status,omitempty"`
	InheritsStatus   bool      `json:"inherits_status,omitempty"`
	VisibleToClients bool      `json:"visible_to_clients,omitempty"`
	CommentsCount    int32     `json:"comments_count,omitempty"`
	BoostsCount      int32     `json:"boosts_count,omitempty"`
	Type             string    `json:"type,omitempty"`
	URL              string    `json:"url,omitempty"`
	AppURL           string    `json:"app_url,omitempty"`
	BookmarkURL      string    `json:"bookmark_url,omitempty"`
	CommentsURL      string    `json:"comments_url,omitempty"`
	BoostsURL        string    `json:"boosts_url,omitempty"`
	SubscriptionURL  string    `json:"subscription_url,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CreateGaugeNeedleRequest specifies parameters for creating a gauge needle.
type CreateGaugeNeedleRequest struct {
	// Position of the needle (0-100), required.
	Position int32 `json:"position"`
	// Color is the status color: green (default), yellow, or red.
	Color string `json:"color,omitempty"`
	// Description is rich text (HTML) description of the progress update.
	Description string `json:"description,omitempty"`
	// Notify specifies who to notify: "everyone", "working_on", "custom", or omit for nobody.
	Notify string `json:"notify,omitempty"`
	// Subscriptions is an array of people IDs to notify (only used when Notify is "custom").
	Subscriptions []int64 `json:"subscriptions,omitempty"`
}

// UpdateGaugeNeedleRequest specifies parameters for updating a gauge needle.
type UpdateGaugeNeedleRequest struct {
	// Description is rich text (HTML) description.
	Description string `json:"description,omitempty"`
}

// GaugesService handles gauge operations.
type GaugesService struct {
	client *AccountClient
}

// NewGaugesService creates a new GaugesService.
func NewGaugesService(client *AccountClient) *GaugesService {
	return &GaugesService{client: client}
}

// List returns all gauges for the account, following pagination automatically.
func (s *GaugesService) List(ctx context.Context) (result []Gauge, err error) {
	op := OperationInfo{
		Service: "Gauges", Operation: "List",
		ResourceType: "gauge", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListGaugesWithResponse(ctx, s.client.accountID, nil)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var gauges []Gauge
	if err = json.Unmarshal(resp.Body, &gauges); err != nil {
		return nil, fmt.Errorf("failed to parse gauges: %w", err)
	}

	// Follow pagination via Link headers
	rawMore, _, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(gauges), 0)
	if err != nil {
		return nil, err
	}
	for _, raw := range rawMore {
		var g Gauge
		if err = json.Unmarshal(raw, &g); err != nil {
			return nil, fmt.Errorf("failed to parse gauge: %w", err)
		}
		gauges = append(gauges, g)
	}

	return gauges, nil
}

// ListNeedles returns all needles for a project's gauge, following pagination automatically.
func (s *GaugesService) ListNeedles(ctx context.Context, projectID int64) (result []GaugeNeedle, err error) {
	op := OperationInfo{
		Service: "Gauges", Operation: "ListNeedles",
		ResourceType: "gauge_needle", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListGaugeNeedlesWithResponse(ctx, s.client.accountID, projectID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var needles []GaugeNeedle
	if err = json.Unmarshal(resp.Body, &needles); err != nil {
		return nil, fmt.Errorf("failed to parse gauge needles: %w", err)
	}

	// Follow pagination via Link headers
	rawMore, _, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(needles), 0)
	if err != nil {
		return nil, err
	}
	for _, raw := range rawMore {
		var n GaugeNeedle
		if err = json.Unmarshal(raw, &n); err != nil {
			return nil, fmt.Errorf("failed to parse gauge needle: %w", err)
		}
		needles = append(needles, n)
	}

	return needles, nil
}

// GetNeedle returns a single gauge needle by ID.
func (s *GaugesService) GetNeedle(ctx context.Context, needleID int64) (result *GaugeNeedle, err error) {
	op := OperationInfo{
		Service: "Gauges", Operation: "GetNeedle",
		ResourceType: "gauge_needle", IsMutation: false,
		ResourceID: needleID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetGaugeNeedleWithResponse(ctx, s.client.accountID, needleID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var needle GaugeNeedle
	if err = json.Unmarshal(resp.Body, &needle); err != nil {
		return nil, fmt.Errorf("failed to parse gauge needle: %w", err)
	}

	return &needle, nil
}

// CreateNeedle creates a new gauge needle on a project.
func (s *GaugesService) CreateNeedle(ctx context.Context, projectID int64, req *CreateGaugeNeedleRequest) (result *GaugeNeedle, err error) {
	op := OperationInfo{
		Service: "Gauges", Operation: "CreateNeedle",
		ResourceType: "gauge_needle", IsMutation: true,
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
		err = ErrUsage("create needle request is required")
		return nil, err
	}

	body := generated.CreateGaugeNeedleJSONRequestBody{
		GaugeNeedle: generated.GaugeNeedlePayload{
			Position:    req.Position,
			Color:       req.Color,
			Description: req.Description,
		},
		Notify: req.Notify,
	}
	if len(req.Subscriptions) > 0 {
		body.Subscriptions = &req.Subscriptions
	}

	resp, err := s.client.parent.gen.CreateGaugeNeedleWithResponse(ctx, s.client.accountID, projectID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var needle GaugeNeedle
	if err = json.Unmarshal(resp.Body, &needle); err != nil {
		return nil, fmt.Errorf("failed to parse gauge needle: %w", err)
	}

	return &needle, nil
}

// UpdateNeedle updates an existing gauge needle.
func (s *GaugesService) UpdateNeedle(ctx context.Context, needleID int64, req *UpdateGaugeNeedleRequest) (result *GaugeNeedle, err error) {
	op := OperationInfo{
		Service: "Gauges", Operation: "UpdateNeedle",
		ResourceType: "gauge_needle", IsMutation: true,
		ResourceID: needleID,
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
		err = ErrUsage("update needle request is required")
		return nil, err
	}

	body := generated.UpdateGaugeNeedleJSONRequestBody{
		GaugeNeedle: generated.GaugeNeedleUpdatePayload{
			Description: req.Description,
		},
	}

	resp, err := s.client.parent.gen.UpdateGaugeNeedleWithResponse(ctx, s.client.accountID, needleID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var needle GaugeNeedle
	if err = json.Unmarshal(resp.Body, &needle); err != nil {
		return nil, fmt.Errorf("failed to parse gauge needle: %w", err)
	}

	return &needle, nil
}

// DestroyNeedle deletes a gauge needle.
func (s *GaugesService) DestroyNeedle(ctx context.Context, needleID int64) (err error) {
	op := OperationInfo{
		Service: "Gauges", Operation: "DestroyNeedle",
		ResourceType: "gauge_needle", IsMutation: true,
		ResourceID: needleID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DestroyGaugeNeedleWithResponse(ctx, s.client.accountID, needleID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Toggle enables or disables the gauge on a project.
func (s *GaugesService) Toggle(ctx context.Context, projectID int64, enabled bool) (err error) {
	op := OperationInfo{
		Service: "Gauges", Operation: "Toggle",
		ResourceType: "gauge", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	body := generated.ToggleGaugeJSONRequestBody{
		Gauge: generated.GaugeTogglePayload{
			Enabled: enabled,
		},
	}

	resp, err := s.client.parent.gen.ToggleGaugeWithResponse(ctx, s.client.accountID, projectID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}
