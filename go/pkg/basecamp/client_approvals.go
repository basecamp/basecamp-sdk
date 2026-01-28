package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
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
	client *AccountClient
}

// NewClientApprovalsService creates a new ClientApprovalsService.
func NewClientApprovalsService(client *AccountClient) *ClientApprovalsService {
	return &ClientApprovalsService{client: client}
}

// List returns all client approvals in a project.
// bucketID is the project ID.
func (s *ClientApprovalsService) List(ctx context.Context, bucketID int64) (result []ClientApproval, err error) {
	op := OperationInfo{
		Service: "ClientApprovals", Operation: "List",
		ResourceType: "client_approval", IsMutation: false,
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

	resp, err := s.client.parent.gen.ListClientApprovalsWithResponse(ctx, s.client.accountID, bucketID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	approvals := make([]ClientApproval, 0, len(*resp.JSON200))
	for _, ga := range *resp.JSON200 {
		approvals = append(approvals, clientApprovalFromGenerated(ga))
	}

	return approvals, nil
}

// Get returns a client approval by ID.
// bucketID is the project ID, approvalID is the client approval ID.
func (s *ClientApprovalsService) Get(ctx context.Context, bucketID, approvalID int64) (result *ClientApproval, err error) {
	op := OperationInfo{
		Service: "ClientApprovals", Operation: "Get",
		ResourceType: "client_approval", IsMutation: false,
		BucketID: bucketID, ResourceID: approvalID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetClientApprovalWithResponse(ctx, s.client.accountID, bucketID, approvalID)
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

	approval := clientApprovalFromGenerated(resp.JSON200.Approval)
	return &approval, nil
}

// clientApprovalFromGenerated converts a generated ClientApproval to our clean type.
func clientApprovalFromGenerated(ga generated.ClientApproval) ClientApproval {
	a := ClientApproval{
		Status:           ga.Status,
		VisibleToClients: ga.VisibleToClients,
		CreatedAt:        ga.CreatedAt,
		UpdatedAt:        ga.UpdatedAt,
		Title:            ga.Title,
		InheritsStatus:   ga.InheritsStatus,
		Type:             ga.Type,
		URL:              ga.Url,
		AppURL:           ga.AppUrl,
		BookmarkURL:      ga.BookmarkUrl,
		SubscriptionURL:  ga.SubscriptionUrl,
		Content:          ga.Content,
		Subject:          ga.Subject,
		RepliesCount:     int(ga.RepliesCount),
		RepliesURL:       ga.RepliesUrl,
		ApprovalStatus:   ga.ApprovalStatus,
	}

	if ga.Id != nil {
		a.ID = *ga.Id
	}

	if !ga.DueOn.IsZero() {
		dueOn := ga.DueOn.String()
		a.DueOn = &dueOn
	}

	if ga.Parent.Id != nil || ga.Parent.Title != "" {
		a.Parent = &Parent{
			ID:     derefInt64(ga.Parent.Id),
			Title:  ga.Parent.Title,
			Type:   ga.Parent.Type,
			URL:    ga.Parent.Url,
			AppURL: ga.Parent.AppUrl,
		}
	}

	if ga.Bucket.Id != nil || ga.Bucket.Name != "" {
		a.Bucket = &Bucket{
			ID:   derefInt64(ga.Bucket.Id),
			Name: ga.Bucket.Name,
			Type: ga.Bucket.Type,
		}
	}

	if ga.Creator.Id != nil || ga.Creator.Name != "" {
		a.Creator = &Person{
			ID:           derefInt64(ga.Creator.Id),
			Name:         ga.Creator.Name,
			EmailAddress: ga.Creator.EmailAddress,
			AvatarURL:    ga.Creator.AvatarUrl,
			Admin:        ga.Creator.Admin,
			Owner:        ga.Creator.Owner,
		}
	}

	if ga.Approver.Id != nil || ga.Approver.Name != "" {
		a.Approver = &Person{
			ID:           derefInt64(ga.Approver.Id),
			Name:         ga.Approver.Name,
			EmailAddress: ga.Approver.EmailAddress,
			AvatarURL:    ga.Approver.AvatarUrl,
			Admin:        ga.Approver.Admin,
			Owner:        ga.Approver.Owner,
		}
	}

	// Convert responses
	if len(ga.Responses) > 0 {
		a.Responses = make([]ClientApprovalResponse, 0, len(ga.Responses))
		for _, gr := range ga.Responses {
			resp := ClientApprovalResponse{
				Status:           gr.Status,
				VisibleToClients: gr.VisibleToClients,
				CreatedAt:        gr.CreatedAt,
				UpdatedAt:        gr.UpdatedAt,
				Title:            gr.Title,
				InheritsStatus:   gr.InheritsStatus,
				Type:             gr.Type,
				AppURL:           gr.AppUrl,
				BookmarkURL:      gr.BookmarkUrl,
				Content:          gr.Content,
				Approved:         gr.Approved,
			}
			if gr.Id != nil {
				resp.ID = *gr.Id
			}
			if gr.Parent.Id != nil || gr.Parent.Title != "" {
				resp.Parent = &Parent{
					ID:     derefInt64(gr.Parent.Id),
					Title:  gr.Parent.Title,
					Type:   gr.Parent.Type,
					URL:    gr.Parent.Url,
					AppURL: gr.Parent.AppUrl,
				}
			}
			if gr.Bucket.Id != nil || gr.Bucket.Name != "" {
				resp.Bucket = &Bucket{
					ID:   derefInt64(gr.Bucket.Id),
					Name: gr.Bucket.Name,
					Type: gr.Bucket.Type,
				}
			}
			if gr.Creator.Id != nil || gr.Creator.Name != "" {
				resp.Creator = &Person{
					ID:           derefInt64(gr.Creator.Id),
					Name:         gr.Creator.Name,
					EmailAddress: gr.Creator.EmailAddress,
					AvatarURL:    gr.Creator.AvatarUrl,
					Admin:        gr.Creator.Admin,
					Owner:        gr.Creator.Owner,
				}
			}
			a.Responses = append(a.Responses, resp)
		}
	}

	return a
}
