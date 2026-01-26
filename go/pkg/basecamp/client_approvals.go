package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ClientApproval represents a Basecamp client approval request.
type ClientApproval struct {
	ID               int64                    `json:"id"`
	Status           string                   `json:"status"`
	VisibleToClients bool                     `json:"visible_to_clients"`
	CreatedAt        time.Time                `json:"created_at"`
	UpdatedAt        time.Time                `json:"updated_at"`
	Title            string                   `json:"title"`
	InheritsStatus   bool                     `json:"inherits_status"`
	Type             string                   `json:"type"`
	URL              string                   `json:"url"`
	AppURL           string                   `json:"app_url"`
	BookmarkURL      string                   `json:"bookmark_url"`
	SubscriptionURL  string                   `json:"subscription_url"`
	Parent           *Parent                  `json:"parent,omitempty"`
	Bucket           *Bucket                  `json:"bucket,omitempty"`
	Creator          *Person                  `json:"creator,omitempty"`
	Content          string                   `json:"content"`
	Subject          string                   `json:"subject"`
	DueOn            *string                  `json:"due_on,omitempty"`
	RepliesCount     int                      `json:"replies_count"`
	RepliesURL       string                   `json:"replies_url"`
	ApprovalStatus   string                   `json:"approval_status"`
	Approver         *Person                  `json:"approver,omitempty"`
	Responses        []ClientApprovalResponse `json:"responses,omitempty"`
}

// ClientApprovalResponse represents a response to a client approval.
type ClientApprovalResponse struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status"`
	Type             string    `json:"type"`
	AppURL           string    `json:"app_url"`
	BookmarkURL      string    `json:"bookmark_url"`
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
	Content          string    `json:"content"`
	Approved         bool      `json:"approved"`
}

// ClientApprovalsService handles client approval operations.
type ClientApprovalsService struct {
	client *Client
}

// NewClientApprovalsService creates a new ClientApprovalsService.
func NewClientApprovalsService(client *Client) *ClientApprovalsService {
	return &ClientApprovalsService{client: client}
}

// List returns all client approvals in a project.
// bucketID is the project ID.
func (s *ClientApprovalsService) List(ctx context.Context, bucketID int64) ([]ClientApproval, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/client/approvals.json", bucketID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	approvals := make([]ClientApproval, 0, len(results))
	for _, raw := range results {
		var a ClientApproval
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, fmt.Errorf("failed to parse client approval: %w", err)
		}
		approvals = append(approvals, a)
	}

	return approvals, nil
}

// Get returns a client approval by ID.
// bucketID is the project ID, approvalID is the client approval ID.
func (s *ClientApprovalsService) Get(ctx context.Context, bucketID, approvalID int64) (*ClientApproval, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/client/approvals/%d.json", bucketID, approvalID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var approval ClientApproval
	if err := resp.UnmarshalData(&approval); err != nil {
		return nil, fmt.Errorf("failed to parse client approval: %w", err)
	}

	return &approval, nil
}
