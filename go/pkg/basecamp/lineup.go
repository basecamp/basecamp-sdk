package basecamp

import (
	"context"
	"fmt"
	"time"
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

	path := "/lineup/markers.json"
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var marker LineupMarker
	if err := resp.UnmarshalData(&marker); err != nil {
		return nil, fmt.Errorf("failed to parse marker: %w", err)
	}

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

	path := fmt.Sprintf("/lineup/markers/%d.json", markerID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var marker LineupMarker
	if err := resp.UnmarshalData(&marker); err != nil {
		return nil, fmt.Errorf("failed to parse marker: %w", err)
	}

	return &marker, nil
}

// DeleteMarker deletes a marker.
// markerID is the marker ID.
func (s *LineupService) DeleteMarker(ctx context.Context, markerID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/lineup/markers/%d.json", markerID)
	_, err := s.client.Delete(ctx, path)
	return err
}
