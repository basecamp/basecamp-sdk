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

// ClientCorrespondenceListResult contains the results from listing client correspondences.
type ClientCorrespondenceListResult struct {
	// Correspondences is the list of client correspondences returned.
	Correspondences []ClientCorrespondence
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// ClientCorrespondencesService handles client correspondence operations.
type ClientCorrespondencesService struct {
	client *AccountClient
}

// NewClientCorrespondencesService creates a new ClientCorrespondencesService.
func NewClientCorrespondencesService(client *AccountClient) *ClientCorrespondencesService {
	return &ClientCorrespondencesService{client: client}
}

// List returns all client correspondences in a project.
// bucketID is the project ID.
//
// The returned ClientCorrespondenceListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *ClientCorrespondencesService) List(ctx context.Context, bucketID int64) (result *ClientCorrespondenceListResult, err error) {
	op := OperationInfo{
		Service: "ClientCorrespondences", Operation: "List",
		ResourceType: "client_correspondence", IsMutation: false,
		BucketID: bucketID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListClientCorrespondencesWithResponse(ctx, s.client.accountID, bucketID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header
	totalCount := parseTotalCount(resp.HTTPResponse)

	if resp.JSON200 == nil {
		return &ClientCorrespondenceListResult{Correspondences: nil, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	correspondences := make([]ClientCorrespondence, 0, len(*resp.JSON200))
	for _, gc := range *resp.JSON200 {
		correspondences = append(correspondences, clientCorrespondenceFromGenerated(gc))
	}

	return &ClientCorrespondenceListResult{Correspondences: correspondences, Meta: ListMeta{TotalCount: totalCount}}, nil
}

// Get returns a client correspondence by ID.
// bucketID is the project ID, correspondenceID is the client correspondence ID.
func (s *ClientCorrespondencesService) Get(ctx context.Context, bucketID, correspondenceID int64) (result *ClientCorrespondence, err error) {
	op := OperationInfo{
		Service: "ClientCorrespondences", Operation: "Get",
		ResourceType: "client_correspondence", IsMutation: false,
		BucketID: bucketID, ResourceID: correspondenceID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetClientCorrespondenceWithResponse(ctx, s.client.accountID, bucketID, correspondenceID)
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

	correspondence := clientCorrespondenceFromGenerated(*resp.JSON200)
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

	if gc.Id != 0 {
		c.ID = gc.Id
	}

	if gc.Parent.Id != 0 || gc.Parent.Title != "" {
		c.Parent = &Parent{
			ID:     gc.Parent.Id,
			Title:  gc.Parent.Title,
			Type:   gc.Parent.Type,
			URL:    gc.Parent.Url,
			AppURL: gc.Parent.AppUrl,
		}
	}

	if gc.Bucket.Id != 0 || gc.Bucket.Name != "" {
		c.Bucket = &Bucket{
			ID:   gc.Bucket.Id,
			Name: gc.Bucket.Name,
			Type: gc.Bucket.Type,
		}
	}

	if gc.Creator.Id != 0 || gc.Creator.Name != "" {
		c.Creator = &Person{
			ID:           gc.Creator.Id,
			Name:         gc.Creator.Name,
			EmailAddress: gc.Creator.EmailAddress,
			AvatarURL:    gc.Creator.AvatarUrl,
			Admin:        gc.Creator.Admin,
			Owner:        gc.Creator.Owner,
		}
	}

	return c
}
