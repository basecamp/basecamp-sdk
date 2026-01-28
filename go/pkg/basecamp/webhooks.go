package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Webhook represents a Basecamp webhook subscription.
type Webhook struct {
	ID         int64     `json:"id"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	PayloadURL string    `json:"payload_url"`
	Types      []string  `json:"types"`
	AppURL     string    `json:"app_url,omitempty"`
	URL        string    `json:"url,omitempty"`
}

// CreateWebhookRequest specifies the parameters for creating a webhook.
type CreateWebhookRequest struct {
	// PayloadURL is the URL to receive webhook payloads (required).
	PayloadURL string `json:"payload_url"`
	// Types is a list of event types to subscribe to (required).
	// Example: ["Todo", "Todolist", "Comment"]
	Types []string `json:"types"`
	// Active indicates whether the webhook is active (default: true).
	Active *bool `json:"active,omitempty"`
}

// UpdateWebhookRequest specifies the parameters for updating a webhook.
type UpdateWebhookRequest struct {
	// PayloadURL is the URL to receive webhook payloads.
	PayloadURL string `json:"payload_url,omitempty"`
	// Types is a list of event types to subscribe to.
	Types []string `json:"types,omitempty"`
	// Active indicates whether the webhook is active.
	Active *bool `json:"active,omitempty"`
}

// WebhooksService handles webhook operations.
type WebhooksService struct {
	client *Client
}

// NewWebhooksService creates a new WebhooksService.
func NewWebhooksService(client *Client) *WebhooksService {
	return &WebhooksService{client: client}
}

// List returns all webhooks for a project (bucket).
// bucketID is the project ID.
func (s *WebhooksService) List(ctx context.Context, bucketID int64) ([]Webhook, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.ListWebhooksWithResponse(ctx, bucketID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	webhooks := make([]Webhook, 0, len(resp.JSON200.Webhooks))
	for _, gw := range resp.JSON200.Webhooks {
		webhooks = append(webhooks, webhookFromGenerated(gw))
	}

	return webhooks, nil
}

// Get returns a webhook by ID.
// bucketID is the project ID, webhookID is the webhook ID.
func (s *WebhooksService) Get(ctx context.Context, bucketID, webhookID int64) (*Webhook, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetWebhookWithResponse(ctx, bucketID, webhookID)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	webhook := webhookFromGenerated(resp.JSON200.Webhook)
	return &webhook, nil
}

// Create creates a new webhook for a project (bucket).
// bucketID is the project ID.
// Returns the created webhook.
func (s *WebhooksService) Create(ctx context.Context, bucketID int64, req *CreateWebhookRequest) (*Webhook, error) {
	if req == nil {
		return nil, ErrUsage("webhook request is required")
	}

	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req.PayloadURL == "" {
		return nil, ErrUsage("webhook payload_url is required")
	}
	if len(req.Types) == 0 {
		return nil, ErrUsage("webhook types are required")
	}

	body := generated.CreateWebhookJSONRequestBody{
		PayloadUrl: req.PayloadURL,
		Types:      req.Types,
		Active:     req.Active != nil && *req.Active,
	}

	resp, err := s.client.gen.CreateWebhookWithResponse(ctx, bucketID, body)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	webhook := webhookFromGenerated(resp.JSON200.Webhook)
	return &webhook, nil
}

// Update updates an existing webhook.
// bucketID is the project ID, webhookID is the webhook ID.
// Returns the updated webhook.
func (s *WebhooksService) Update(ctx context.Context, bucketID, webhookID int64, req *UpdateWebhookRequest) (*Webhook, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	body := generated.UpdateWebhookJSONRequestBody{
		PayloadUrl: req.PayloadURL,
		Types:      req.Types,
		Active:     req.Active != nil && *req.Active,
	}

	resp, err := s.client.gen.UpdateWebhookWithResponse(ctx, bucketID, webhookID, body)
	if err != nil {
		return nil, err
	}
	if err := checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected empty response")
	}

	webhook := webhookFromGenerated(resp.JSON200.Webhook)
	return &webhook, nil
}

// Delete removes a webhook.
// bucketID is the project ID, webhookID is the webhook ID.
func (s *WebhooksService) Delete(ctx context.Context, bucketID, webhookID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.DeleteWebhookWithResponse(ctx, bucketID, webhookID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// webhookFromGenerated converts a generated Webhook to our clean type.
func webhookFromGenerated(gw generated.Webhook) Webhook {
	w := Webhook{
		Active:     gw.Active,
		CreatedAt:  gw.CreatedAt,
		UpdatedAt:  gw.UpdatedAt,
		PayloadURL: gw.PayloadUrl,
		Types:      gw.Types,
		AppURL:     gw.AppUrl,
		URL:        gw.Url,
	}

	if gw.Id != nil {
		w.ID = *gw.Id
	}

	return w
}
