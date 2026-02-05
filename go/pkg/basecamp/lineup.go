package basecamp

import (
	"context"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
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
	// Name is the marker name (required).
	Name string `json:"name"`
	// Date is the marker date in YYYY-MM-DD format (required).
	Date string `json:"date"`
}

// UpdateMarkerRequest specifies the parameters for updating a lineup marker.
type UpdateMarkerRequest struct {
	// Name is the marker name (optional).
	Name string `json:"name,omitempty"`
	// Date is the marker date in YYYY-MM-DD format (optional).
	Date string `json:"date,omitempty"`
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

	if req == nil || req.Name == "" {
		err = ErrUsage("marker name is required")
		return err
	}
	if req.Date == "" {
		err = ErrUsage("marker date is required")
		return err
	}

	body := generated.CreateLineupMarkerJSONRequestBody{
		Name: req.Name,
		Date: req.Date,
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
		Name: req.Name,
		Date: req.Date,
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
