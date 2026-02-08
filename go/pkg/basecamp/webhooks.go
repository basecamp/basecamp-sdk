package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Webhook represents a Basecamp webhook subscription.
type Webhook struct {
	ID               int64             `json:"id"`
	Active           bool              `json:"active"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	PayloadURL       string            `json:"payload_url"`
	Types            []string          `json:"types"`
	AppURL           string            `json:"app_url,omitempty"`
	URL              string            `json:"url,omitempty"`
	RecentDeliveries []WebhookDelivery `json:"recent_deliveries,omitempty"`
}

// WebhookDelivery represents a recent delivery attempt for a webhook.
type WebhookDelivery struct {
	ID        int64                   `json:"id"`
	CreatedAt time.Time               `json:"created_at"`
	Request   WebhookDeliveryRequest  `json:"request"`
	Response  WebhookDeliveryResponse `json:"response"`
}

// WebhookDeliveryRequest contains the outbound request details.
type WebhookDeliveryRequest struct {
	Headers map[string]string `json:"headers"`
	Body    WebhookEvent      `json:"body"`
}

// WebhookDeliveryResponse contains the response from the webhook endpoint.
type WebhookDeliveryResponse struct {
	Headers map[string]string `json:"headers"`
	Code    int               `json:"code"`
	Message string            `json:"message"`
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

// WebhookListResult contains the results from listing webhooks.
type WebhookListResult struct {
	// Webhooks is the list of webhooks returned.
	Webhooks []Webhook
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// WebhooksService handles webhook operations.
type WebhooksService struct {
	client *AccountClient
}

// NewWebhooksService creates a new WebhooksService.
func NewWebhooksService(client *AccountClient) *WebhooksService {
	return &WebhooksService{client: client}
}

// List returns all webhooks for a project (bucket).
// bucketID is the project ID.
//
// The returned WebhookListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *WebhooksService) List(ctx context.Context, bucketID int64) (result *WebhookListResult, err error) {
	op := OperationInfo{
		Service: "Webhooks", Operation: "List",
		ResourceType: "webhook", IsMutation: false,
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

	resp, err := s.client.parent.gen.ListWebhooksWithResponse(ctx, s.client.accountID, bucketID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header
	totalCount := parseTotalCount(resp.HTTPResponse)

	if resp.JSON200 == nil {
		return &WebhookListResult{Webhooks: nil, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	webhooks := make([]Webhook, 0, len(*resp.JSON200))
	for _, gw := range *resp.JSON200 {
		webhooks = append(webhooks, webhookFromGenerated(gw))
	}

	return &WebhookListResult{Webhooks: webhooks, Meta: ListMeta{TotalCount: totalCount}}, nil
}

// Get returns a webhook by ID.
// bucketID is the project ID, webhookID is the webhook ID.
func (s *WebhooksService) Get(ctx context.Context, bucketID, webhookID int64) (result *Webhook, err error) {
	op := OperationInfo{
		Service: "Webhooks", Operation: "Get",
		ResourceType: "webhook", IsMutation: false,
		BucketID: bucketID, ResourceID: webhookID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetWebhookWithResponse(ctx, s.client.accountID, bucketID, webhookID)
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

	webhook := webhookFromGenerated(*resp.JSON200)
	return &webhook, nil
}

// Create creates a new webhook for a project (bucket).
// bucketID is the project ID.
// Returns the created webhook.
func (s *WebhooksService) Create(ctx context.Context, bucketID int64, req *CreateWebhookRequest) (result *Webhook, err error) {
	op := OperationInfo{
		Service: "Webhooks", Operation: "Create",
		ResourceType: "webhook", IsMutation: true,
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

	if req == nil {
		err = ErrUsage("webhook request is required")
		return nil, err
	}

	if req.PayloadURL == "" {
		err = ErrUsage("webhook payload_url is required")
		return nil, err
	}
	if err = requireHTTPS(req.PayloadURL); err != nil {
		err = ErrUsage("webhook payload_url must use HTTPS")
		return nil, err
	}
	if len(req.Types) == 0 {
		err = ErrUsage("webhook types are required")
		return nil, err
	}

	body := generated.CreateWebhookJSONRequestBody{
		PayloadUrl: req.PayloadURL,
		Types:      req.Types,
		Active:     req.Active,
	}

	resp, err := s.client.parent.gen.CreateWebhookWithResponse(ctx, s.client.accountID, bucketID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	webhook := webhookFromGenerated(*resp.JSON201)
	return &webhook, nil
}

// Update updates an existing webhook.
// bucketID is the project ID, webhookID is the webhook ID.
// Returns the updated webhook.
func (s *WebhooksService) Update(ctx context.Context, bucketID, webhookID int64, req *UpdateWebhookRequest) (result *Webhook, err error) {
	op := OperationInfo{
		Service: "Webhooks", Operation: "Update",
		ResourceType: "webhook", IsMutation: true,
		BucketID: bucketID, ResourceID: webhookID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req.PayloadURL != "" {
		if err = requireHTTPS(req.PayloadURL); err != nil {
			err = ErrUsage("webhook payload_url must use HTTPS")
			return nil, err
		}
	}

	body := generated.UpdateWebhookJSONRequestBody{
		PayloadUrl: req.PayloadURL,
		Types:      req.Types,
		Active:     req.Active,
	}

	resp, err := s.client.parent.gen.UpdateWebhookWithResponse(ctx, s.client.accountID, bucketID, webhookID, body)
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

	webhook := webhookFromGenerated(*resp.JSON200)
	return &webhook, nil
}

// Delete removes a webhook.
// bucketID is the project ID, webhookID is the webhook ID.
func (s *WebhooksService) Delete(ctx context.Context, bucketID, webhookID int64) (err error) {
	op := OperationInfo{
		Service: "Webhooks", Operation: "Delete",
		ResourceType: "webhook", IsMutation: true,
		BucketID: bucketID, ResourceID: webhookID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DeleteWebhookWithResponse(ctx, s.client.accountID, bucketID, webhookID)
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

	if len(gw.RecentDeliveries) > 0 {
		w.RecentDeliveries = make([]WebhookDelivery, len(gw.RecentDeliveries))
		for i, gd := range gw.RecentDeliveries {
			d := WebhookDelivery{
				CreatedAt: gd.CreatedAt,
				Request: WebhookDeliveryRequest{
					Headers: map[string]string(gd.Request.Headers),
					Body:    webhookEventFromGenerated(gd.Request.Body),
				},
				Response: WebhookDeliveryResponse{
					Headers: map[string]string(gd.Response.Headers),
					Code:    int(gd.Response.Code),
					Message: gd.Response.Message,
				},
			}
			if gd.Id != nil {
				d.ID = *gd.Id
			}
			w.RecentDeliveries[i] = d
		}
	}

	return w
}

// webhookEventFromGenerated converts a generated WebhookEvent to our clean type.
func webhookEventFromGenerated(ge generated.WebhookEvent) WebhookEvent {
	event := WebhookEvent{
		Kind:      ge.Kind,
		CreatedAt: ge.CreatedAt.Format(time.RFC3339Nano),
	}

	if ge.Id != nil {
		event.ID = *ge.Id
	}

	if ge.Details != nil {
		if m, ok := ge.Details.(map[string]interface{}); ok {
			event.Details = m
		}
	}

	// Map recording
	rec := &ge.Recording
	event.Recording = WebhookEventRecording{
		Status:           rec.Status,
		VisibleToClients: rec.VisibleToClients,
		Title:            rec.Title,
		InheritsStatus:   rec.InheritsStatus,
		Type:             rec.Type,
		URL:              rec.Url,
		AppURL:           rec.AppUrl,
		BookmarkURL:      rec.BookmarkUrl,
		Content:          rec.Content,
		CommentsCount:    int(rec.CommentsCount),
		CommentsURL:      rec.CommentsUrl,
		SubscriptionURL:  rec.SubscriptionUrl,
	}
	if rec.Id != nil {
		event.Recording.ID = *rec.Id
	}
	if !rec.CreatedAt.IsZero() {
		event.Recording.CreatedAt = rec.CreatedAt.Format(time.RFC3339Nano)
	}
	if !rec.UpdatedAt.IsZero() {
		event.Recording.UpdatedAt = rec.UpdatedAt.Format(time.RFC3339Nano)
	}
	if rec.Parent.Id != nil {
		event.Recording.Parent = &WebhookEventParent{
			Title:  rec.Parent.Title,
			Type:   rec.Parent.Type,
			URL:    rec.Parent.Url,
			AppURL: rec.Parent.AppUrl,
		}
		event.Recording.Parent.ID = *rec.Parent.Id
	}
	if rec.Bucket.Id != nil {
		event.Recording.Bucket = &WebhookEventBucket{
			Name: rec.Bucket.Name,
			Type: rec.Bucket.Type,
		}
		event.Recording.Bucket.ID = *rec.Bucket.Id
	}
	if rec.Creator.Id != nil {
		c := &WebhookEventPerson{
			Name:         rec.Creator.Name,
			EmailAddress: rec.Creator.EmailAddress,
		}
		c.ID = *rec.Creator.Id
		event.Recording.Creator = c
	}

	// Map top-level creator
	if ge.Creator.Id != nil {
		event.Creator = WebhookEventPerson{
			Name:         ge.Creator.Name,
			EmailAddress: ge.Creator.EmailAddress,
		}
		event.Creator.ID = *ge.Creator.Id
	}

	// Map copy if present
	if ge.Copy.Url != "" || (ge.Copy.Id != nil && *ge.Copy.Id != 0) {
		c := &WebhookCopy{
			URL:    ge.Copy.Url,
			AppURL: ge.Copy.AppUrl,
			Bucket: WebhookCopyBucket{},
		}
		if ge.Copy.Id != nil {
			c.ID = *ge.Copy.Id
		}
		if ge.Copy.Bucket.Id != nil {
			c.Bucket.ID = *ge.Copy.Bucket.Id
		}
		event.Copy = c
	}

	return event
}
