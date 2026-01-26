package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
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

	path := fmt.Sprintf("/buckets/%d/webhooks.json", bucketID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	webhooks := make([]Webhook, 0, len(results))
	for _, raw := range results {
		var wh Webhook
		if err := json.Unmarshal(raw, &wh); err != nil {
			return nil, fmt.Errorf("failed to parse webhook: %w", err)
		}
		webhooks = append(webhooks, wh)
	}

	return webhooks, nil
}

// Get returns a webhook by ID.
// bucketID is the project ID, webhookID is the webhook ID.
func (s *WebhooksService) Get(ctx context.Context, bucketID, webhookID int64) (*Webhook, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/webhooks/%d.json", bucketID, webhookID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var webhook Webhook
	if err := resp.UnmarshalData(&webhook); err != nil {
		return nil, fmt.Errorf("failed to parse webhook: %w", err)
	}

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

	path := fmt.Sprintf("/buckets/%d/webhooks.json", bucketID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var webhook Webhook
	if err := resp.UnmarshalData(&webhook); err != nil {
		return nil, fmt.Errorf("failed to parse webhook: %w", err)
	}

	return &webhook, nil
}

// Update updates an existing webhook.
// bucketID is the project ID, webhookID is the webhook ID.
// Returns the updated webhook.
func (s *WebhooksService) Update(ctx context.Context, bucketID, webhookID int64, req *UpdateWebhookRequest) (*Webhook, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/webhooks/%d.json", bucketID, webhookID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var webhook Webhook
	if err := resp.UnmarshalData(&webhook); err != nil {
		return nil, fmt.Errorf("failed to parse webhook: %w", err)
	}

	return &webhook, nil
}

// Delete removes a webhook.
// bucketID is the project ID, webhookID is the webhook ID.
func (s *WebhooksService) Delete(ctx context.Context, bucketID, webhookID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/webhooks/%d.json", bucketID, webhookID)
	_, err := s.client.Delete(ctx, path)
	return err
}
