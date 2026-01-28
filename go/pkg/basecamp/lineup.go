package basecamp

import (
	"context"
	"fmt"
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
	client *Client
}

// NewLineupService creates a new LineupService.
func NewLineupService(client *Client) *LineupService {
	return &LineupService{client: client}
}

// CreateMarker creates a new marker on the lineup.
// Returns the created marker.
func (s *LineupService) CreateMarker(ctx context.Context, req *CreateMarkerRequest) (*LineupMarker, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Title == "" {
		return nil, ErrUsage("marker title is required")
	}
	if req.StartsOn == "" {
		return nil, ErrUsage("marker starts_on date is required")
	}
	if req.EndsOn == "" {
		return nil, ErrUsage("marker ends_on date is required")
	}

	startsOn, err := types.ParseDate(req.StartsOn)
	if err != nil {
		return nil, ErrUsage("marker starts_on date must be in YYYY-MM-DD format")
	}
	endsOn, err := types.ParseDate(req.EndsOn)
	if err != nil {
		return nil, ErrUsage("marker ends_on date must be in YYYY-MM-DD format")
	}

	body := generated.CreateLineupMarkerJSONRequestBody{
		Title:       req.Title,
		StartsOn:    startsOn,
		EndsOn:      endsOn,
		Color:       req.Color,
		Description: req.Description,
	}

	resp, err := s.client.gen.CreateLineupMarkerWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	marker := lineupMarkerFromGenerated(resp.JSON200.Marker)
	return &marker, nil
}

// UpdateMarker updates an existing marker.
// markerID is the marker ID.
// Returns the updated marker.
func (s *LineupService) UpdateMarker(ctx context.Context, markerID int64, req *UpdateMarkerRequest) (*LineupMarker, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil {
		return nil, ErrUsage("update request is required")
	}

	body := generated.UpdateLineupMarkerJSONRequestBody{
		Color:       req.Color,
		Description: req.Description,
		Title:       req.Title,
	}
	if req.StartsOn != "" {
		startsOn, err := types.ParseDate(req.StartsOn)
		if err != nil {
			return nil, ErrUsage("marker starts_on date must be in YYYY-MM-DD format")
		}
		body.StartsOn = startsOn
	}
	if req.EndsOn != "" {
		endsOn, err := types.ParseDate(req.EndsOn)
		if err != nil {
			return nil, ErrUsage("marker ends_on date must be in YYYY-MM-DD format")
		}
		body.EndsOn = endsOn
	}

	resp, err := s.client.gen.UpdateLineupMarkerWithResponse(ctx, markerID, body)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	marker := lineupMarkerFromGenerated(resp.JSON200.Marker)
	return &marker, nil
}

// DeleteMarker deletes a marker.
// markerID is the marker ID.
func (s *LineupService) DeleteMarker(ctx context.Context, markerID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.DeleteLineupMarkerWithResponse(ctx, markerID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// lineupMarkerFromGenerated converts a generated LineupMarker to our clean type.
func lineupMarkerFromGenerated(gm generated.LineupMarker) LineupMarker {
	m := LineupMarker{
		Status:      gm.Status,
		Color:       gm.Color,
		Title:       gm.Title,
		Description: gm.Description,
		CreatedAt:   gm.CreatedAt,
		UpdatedAt:   gm.UpdatedAt,
		Type:        gm.Type,
		URL:         gm.Url,
		AppURL:      gm.AppUrl,
	}

	if gm.Id != nil {
		m.ID = *gm.Id
	}

	// Convert date fields to strings
	if !gm.StartsOn.IsZero() {
		m.StartsOn = gm.StartsOn.String()
	}
	if !gm.EndsOn.IsZero() {
		m.EndsOn = gm.EndsOn.String()
	}

	if gm.Creator.Id != nil || gm.Creator.Name != "" {
		m.Creator = &Person{
			ID:           derefInt64(gm.Creator.Id),
			Name:         gm.Creator.Name,
			EmailAddress: gm.Creator.EmailAddress,
			AvatarURL:    gm.Creator.AvatarUrl,
			Admin:        gm.Creator.Admin,
			Owner:        gm.Creator.Owner,
		}
	}

	if gm.Parent.Id != nil || gm.Parent.Title != "" {
		m.Parent = &Parent{
			ID:     derefInt64(gm.Parent.Id),
			Title:  gm.Parent.Title,
			Type:   gm.Parent.Type,
			URL:    gm.Parent.Url,
			AppURL: gm.Parent.AppUrl,
		}
	}

	if gm.Bucket.Id != nil || gm.Bucket.Name != "" {
		m.Bucket = &Bucket{
			ID:   derefInt64(gm.Bucket.Id),
			Name: gm.Bucket.Name,
			Type: gm.Bucket.Type,
		}
	}

	return m
}
