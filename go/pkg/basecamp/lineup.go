package basecamp

import (
	"context"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
	"github.com/basecamp/basecamp-sdk/go/pkg/types"
)

// LineupMarker represents a marker on the Basecamp Lineup.
type LineupMarker struct {
	ID          int64     `json:"id"`
	Status      string    `json:"status"`
	Color       string    `json:"color"`
	Title       string    `json:"title"`
	StartsOn    string    `json:"starts_on"`
	EndsOn      string    `json:"ends_on"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Type        string    `json:"type"`
	URL         string    `json:"url"`
	AppURL      string    `json:"app_url"`
	Creator     *Person   `json:"creator,omitempty"`
	Parent      *Parent   `json:"parent,omitempty"`
	Bucket      *Bucket   `json:"bucket,omitempty"`
}

// CreateMarkerRequest specifies the parameters for creating a lineup marker.
type CreateMarkerRequest struct {
	// Title is the marker title (required).
	Title string `json:"title"`
	// StartsOn is the start date in YYYY-MM-DD format (required).
	StartsOn string `json:"starts_on"`
	// EndsOn is the end date in YYYY-MM-DD format (required).
	EndsOn string `json:"ends_on"`
	// Color is the marker color (optional).
	Color string `json:"color,omitempty"`
	// Description is the marker description in HTML (optional).
	Description string `json:"description,omitempty"`
}

// UpdateMarkerRequest specifies the parameters for updating a lineup marker.
type UpdateMarkerRequest struct {
	// Title is the marker title (optional).
	Title string `json:"title,omitempty"`
	// StartsOn is the start date in YYYY-MM-DD format (optional).
	StartsOn string `json:"starts_on,omitempty"`
	// EndsOn is the end date in YYYY-MM-DD format (optional).
	EndsOn string `json:"ends_on,omitempty"`
	// Color is the marker color (optional).
	Color string `json:"color,omitempty"`
	// Description is the marker description in HTML (optional).
	Description string `json:"description,omitempty"`
}

// LineupService handles lineup marker operations.
type LineupService struct {
	client *AccountClient
}

// NewLineupService creates a new LineupService.
func NewLineupService(client *AccountClient) *LineupService {
	return &LineupService{client: client}
}

// CreateMarker creates a new marker on the lineup.
func (s *LineupService) CreateMarker(ctx context.Context, req *CreateMarkerRequest) (err error) {
	op := OperationInfo{
		Service: "Lineup", Operation: "CreateMarker",
		ResourceType: "lineup_marker", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.Title == "" {
		err = ErrUsage("marker title is required")
		return err
	}
	if req.StartsOn == "" {
		err = ErrUsage("marker starts_on date is required")
		return err
	}
	if req.EndsOn == "" {
		err = ErrUsage("marker ends_on date is required")
		return err
	}

	startsOn, parseErr := types.ParseDate(req.StartsOn)
	if parseErr != nil {
		err = ErrUsage("marker starts_on date must be in YYYY-MM-DD format")
		return err
	}
	endsOn, parseErr := types.ParseDate(req.EndsOn)
	if parseErr != nil {
		err = ErrUsage("marker ends_on date must be in YYYY-MM-DD format")
		return err
	}

	body := generated.CreateLineupMarkerJSONRequestBody{
		Title:       req.Title,
		StartsOn:    startsOn,
		EndsOn:      endsOn,
		Color:       req.Color,
		Description: req.Description,
	}

	resp, err := s.client.parent.gen.CreateLineupMarkerWithResponse(ctx, s.client.accountID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// UpdateMarker updates an existing marker.
// markerID is the marker ID.
func (s *LineupService) UpdateMarker(ctx context.Context, markerID int64, req *UpdateMarkerRequest) (err error) {
	op := OperationInfo{
		Service: "Lineup", Operation: "UpdateMarker",
		ResourceType: "lineup_marker", IsMutation: true,
		ResourceID: markerID,
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
		err = ErrUsage("update request is required")
		return err
	}

	body := generated.UpdateLineupMarkerJSONRequestBody{
		Color:       req.Color,
		Description: req.Description,
		Title:       req.Title,
	}
	if req.StartsOn != "" {
		startsOn, parseErr := types.ParseDate(req.StartsOn)
		if parseErr != nil {
			err = ErrUsage("marker starts_on date must be in YYYY-MM-DD format")
			return err
		}
		body.StartsOn = startsOn
	}
	if req.EndsOn != "" {
		endsOn, parseErr := types.ParseDate(req.EndsOn)
		if parseErr != nil {
			err = ErrUsage("marker ends_on date must be in YYYY-MM-DD format")
			return err
		}
		body.EndsOn = endsOn
	}

	resp, err := s.client.parent.gen.UpdateLineupMarkerWithResponse(ctx, s.client.accountID, markerID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// DeleteMarker deletes a marker.
// markerID is the marker ID.
func (s *LineupService) DeleteMarker(ctx context.Context, markerID int64) (err error) {
	op := OperationInfo{
		Service: "Lineup", Operation: "DeleteMarker",
		ResourceType: "lineup_marker", IsMutation: true,
		ResourceID: markerID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DeleteLineupMarkerWithResponse(ctx, s.client.accountID, markerID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Note: lineupMarkerFromGenerated was removed because CreateLineupMarker and
// UpdateLineupMarker now return 204 No Content with no response body.
