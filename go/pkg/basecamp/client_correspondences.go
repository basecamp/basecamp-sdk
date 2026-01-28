package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
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

	resp, err := s.client.gen.ListClientCorrespondencesWithResponse(ctx, bucketID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	correspondences := make([]ClientCorrespondence, 0, len(resp.JSON200.Correspondences))
	for _, gc := range resp.JSON200.Correspondences {
		correspondences = append(correspondences, clientCorrespondenceFromGenerated(gc))
	}

	return correspondences, nil
}

// Get returns a client correspondence by ID.
// bucketID is the project ID, correspondenceID is the client correspondence ID.
func (s *ClientCorrespondencesService) Get(ctx context.Context, bucketID, correspondenceID int64) (*ClientCorrespondence, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetClientCorrespondenceWithResponse(ctx, bucketID, correspondenceID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	correspondence := clientCorrespondenceFromGenerated(resp.JSON200.Correspondence)
	return &correspondence, nil
}

// clientCorrespondenceFromGenerated converts a generated ClientCorrespondence to our clean type.
func clientCorrespondenceFromGenerated(gc generated.ClientCorrespondence) ClientCorrespondence {
	c := ClientCorrespondence{
		Status:           gc.Status,
		VisibleToClients: gc.VisibleToClients,
		CreatedAt:        gc.CreatedAt,
		UpdatedAt:        gc.UpdatedAt,
		Title:            gc.Title,
		InheritsStatus:   gc.InheritsStatus,
		Type:             gc.Type,
		URL:              gc.Url,
		AppURL:           gc.AppUrl,
		BookmarkURL:      gc.BookmarkUrl,
		SubscriptionURL:  gc.SubscriptionUrl,
		Content:          gc.Content,
		Subject:          gc.Subject,
		RepliesCount:     int(gc.RepliesCount),
		RepliesURL:       gc.RepliesUrl,
	}

	if gc.Id != nil {
		c.ID = *gc.Id
	}

	if gc.Parent.Id != nil || gc.Parent.Title != "" {
		c.Parent = &Parent{
			ID:     derefInt64(gc.Parent.Id),
			Title:  gc.Parent.Title,
			Type:   gc.Parent.Type,
			URL:    gc.Parent.Url,
			AppURL: gc.Parent.AppUrl,
		}
	}

	if gc.Bucket.Id != nil || gc.Bucket.Name != "" {
		c.Bucket = &Bucket{
			ID:   derefInt64(gc.Bucket.Id),
			Name: gc.Bucket.Name,
			Type: gc.Bucket.Type,
		}
	}

	if gc.Creator.Id != nil || gc.Creator.Name != "" {
		c.Creator = &Person{
			ID:           derefInt64(gc.Creator.Id),
			Name:         gc.Creator.Name,
			EmailAddress: gc.Creator.EmailAddress,
			AvatarURL:    gc.Creator.AvatarUrl,
			Admin:        gc.Creator.Admin,
			Owner:        gc.Creator.Owner,
		}
	}

	return c
}
