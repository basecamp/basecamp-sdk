package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ClientCorrespondence represents a Basecamp client correspondence (message to clients).
type ClientCorrespondence struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status"`
	Type             string    `json:"type"`
	URL              string    `json:"url"`
	AppURL           string    `json:"app_url"`
	BookmarkURL      string    `json:"bookmark_url"`
	SubscriptionURL  string    `json:"subscription_url"`
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
	Content          string    `json:"content"`
	Subject          string    `json:"subject"`
	RepliesCount     int       `json:"replies_count"`
	RepliesURL       string    `json:"replies_url"`
}

// ClientCorrespondencesService handles client correspondence operations.
type ClientCorrespondencesService struct {
	client *Client
}

// NewClientCorrespondencesService creates a new ClientCorrespondencesService.
func NewClientCorrespondencesService(client *Client) *ClientCorrespondencesService {
	return &ClientCorrespondencesService{client: client}
}

// List returns all client correspondences in a project.
// bucketID is the project ID.
func (s *ClientCorrespondencesService) List(ctx context.Context, bucketID int64) ([]ClientCorrespondence, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/client/correspondences.json", bucketID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	correspondences := make([]ClientCorrespondence, 0, len(results))
	for _, raw := range results {
		var c ClientCorrespondence
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("failed to parse client correspondence: %w", err)
		}
		correspondences = append(correspondences, c)
	}

	return correspondences, nil
}

// Get returns a client correspondence by ID.
// bucketID is the project ID, correspondenceID is the client correspondence ID.
func (s *ClientCorrespondencesService) Get(ctx context.Context, bucketID, correspondenceID int64) (*ClientCorrespondence, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/client/correspondences/%d.json", bucketID, correspondenceID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var correspondence ClientCorrespondence
	if err := resp.UnmarshalData(&correspondence); err != nil {
		return nil, fmt.Errorf("failed to parse client correspondence: %w", err)
	}

	return &correspondence, nil
}
