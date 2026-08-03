package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// WebhookListOptions specifies options for listing webhooks.
type WebhookListOptions struct {
	// Limit is the maximum number of webhooks to return.
	// If 0, returns all. Use -1 for unlimited (same as 0).
	Limit int

	// Page: the page number is ignored -- this endpoint is not paginated
	// server-side -- but any positive value still disables auto-pagination,
	// returning the single response as-is without applying Limit.
	Page int
}

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
//
// Pagination options:
//   - Limit: maximum number of webhooks to return (0 = all, -1 = unlimited)
//   - Page: the page number is ignored (this endpoint is not paginated
//     server-side), but any positive value still disables auto-pagination,
//     returning the single response as-is without applying Limit
//
// The returned WebhookListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *WebhooksService) List(ctx context.Context, bucketID int64, opts *WebhookListOptions) (result *WebhookListResult, err error) {
	op := OperationInfo{
		Service: "Webhooks", Operation: "List",
		ResourceType: "webhook", IsMutation: false,
		ProjectID: bucketID,
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
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var webhooks []Webhook
	if resp.JSON200 != nil {
		for _, gw := range *resp.JSON200 {
			webhooks = append(webhooks, webhookFromGenerated(gw))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		return &WebhookListResult{Webhooks: webhooks, Meta: ListMeta{TotalCount: totalCount, Truncated: hasNextPage(resp.HTTPResponse)}}, nil
	}

	// Determine limit: 0 = all (no limit)
	limit := 0
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0 // unlimited
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	// Check if we already have enough items
	if limit > 0 && len(webhooks) >= limit {
		return &WebhookListResult{Webhooks: webhooks[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(webhooks), limit)}}, nil
	}

	// Follow pagination via Link headers
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(webhooks), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gw generated.Webhook
		if err := json.Unmarshal(raw, &gw); err != nil {
			return nil, fmt.Errorf("failed to parse webhook: %w", err)
		}
		webhooks = append(webhooks, webhookFromGenerated(gw))
	}

	return &WebhookListResult{Webhooks: webhooks, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Get returns a webhook by ID.
func (s *WebhooksService) Get(ctx context.Context, webhookID int64) (result *Webhook, err error) {
	op := OperationInfo{
		Service: "Webhooks", Operation: "Get",
		ResourceType: "webhook", IsMutation: false,
		ResourceID: webhookID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetWebhookWithResponse(ctx, s.client.accountID, webhookID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
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
// Returns the created webhook.
func (s *WebhooksService) Create(ctx context.Context, bucketID int64, req *CreateWebhookRequest) (result *Webhook, err error) {
	op := OperationInfo{
		Service: "Webhooks", Operation: "Create",
		ResourceType: "webhook", IsMutation: true,
		ProjectID: bucketID,
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
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
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
// Returns the updated webhook.
func (s *WebhooksService) Update(ctx context.Context, webhookID int64, req *UpdateWebhookRequest) (result *Webhook, err error) {
	op := OperationInfo{
		Service: "Webhooks", Operation: "Update",
		ResourceType: "webhook", IsMutation: true,
		ResourceID: webhookID,
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

	body := generated.UpdateWebhookJSONRequestBody{}
	if req.PayloadURL != "" {
		body.PayloadUrl = &req.PayloadURL
	}
	// nil means "not addressed" (omitted); a non-nil empty slice is an explicit
	// empty type list and must reach the wire.
	if req.Types != nil {
		body.Types = &req.Types
	}
	if req.Active != nil {
		body.Active = req.Active
	}

	resp, err := s.client.parent.gen.UpdateWebhookWithResponse(ctx, s.client.accountID, webhookID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
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
func (s *WebhooksService) Delete(ctx context.Context, webhookID int64) (err error) {
	op := OperationInfo{
		Service: "Webhooks", Operation: "Delete",
		ResourceType: "webhook", IsMutation: true,
		ResourceID: webhookID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DeleteWebhookWithResponse(ctx, s.client.accountID, webhookID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// webhookFromGenerated converts a generated Webhook to our clean type.
func webhookFromGenerated(gw generated.Webhook) Webhook {
	w := Webhook{
		Active:     deref(gw.Active),
		CreatedAt:  gw.CreatedAt,
		UpdatedAt:  gw.UpdatedAt,
		PayloadURL: gw.PayloadUrl,
		Types:      gw.Types,
		AppURL:     gw.AppUrl,
		URL:        gw.Url,
	}

	if gw.Id != 0 {
		w.ID = gw.Id
	}

	if len(gw.RecentDeliveries) > 0 {
		w.RecentDeliveries = make([]WebhookDelivery, len(gw.RecentDeliveries))
		for i, gd := range gw.RecentDeliveries {
			d := WebhookDelivery{
				CreatedAt: deref(gd.CreatedAt),
			}
			if gd.Request != nil {
				d.Request = WebhookDeliveryRequest{
					Headers: map[string]string(deref(gd.Request.Headers)),
				}
				if gd.Request.Body != nil {
					d.Request.Body = webhookEventFromGenerated(*gd.Request.Body)
				}
			}
			if gd.Response != nil {
				d.Response = WebhookDeliveryResponse{
					Headers: map[string]string(deref(gd.Response.Headers)),
					Code:    int(deref(gd.Response.Code)),
					Message: deref(gd.Response.Message),
				}
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
		Kind: deref(ge.Kind),
	}
	if ge.CreatedAt != nil {
		event.CreatedAt = ge.CreatedAt.Format(time.RFC3339Nano)
	}

	if ge.Id != nil {
		event.ID = *ge.Id
	}

	event.Details = ge.Details

	// Map recording
	if rec := ge.Recording; rec != nil {
		event.Recording = WebhookEventRecording{
			Status:           rec.Status,
			VisibleToClients: rec.VisibleToClients,
			Title:            rec.Title,
			InheritsStatus:   rec.InheritsStatus,
			Type:             rec.Type,
			URL:              rec.Url,
			AppURL:           rec.AppUrl,
			BookmarkURL:      deref(rec.BookmarkUrl),
			Content:          deref(rec.Content),
			CommentsCount:    int(deref(rec.CommentsCount)),
			CommentsURL:      deref(rec.CommentsUrl),
			SubscriptionURL:  deref(rec.SubscriptionUrl),
		}
		if rec.Id != 0 {
			event.Recording.ID = rec.Id
		}
		// Recording created_at/updated_at are @required and generate as VALUE
		// time.Time. There is no pointer to consult, so presence CANNOT be
		// recovered here at all — IsZero is a legacy zero-value heuristic, not a
		// presence signal, and a genuinely present year-1 timestamp is
		// indistinguishable from an absent one. Kept deliberately: unlike the
		// pointer-backed fields above, dropping it would emit
		// "0001-01-01T00:00:00Z" for every recording that omits the field.
		if !rec.CreatedAt.IsZero() {
			event.Recording.CreatedAt = rec.CreatedAt.Format(time.RFC3339Nano)
		}
		if !rec.UpdatedAt.IsZero() {
			event.Recording.UpdatedAt = rec.UpdatedAt.Format(time.RFC3339Nano)
		}
		if rec.Parent != nil {
			event.Recording.Parent = &WebhookEventParent{
				Title:  rec.Parent.Title,
				Type:   rec.Parent.Type,
				URL:    rec.Parent.Url,
				AppURL: rec.Parent.AppUrl,
			}
			event.Recording.Parent.ID = rec.Parent.Id
		}
		if rec.Bucket.Id != 0 {
			event.Recording.Bucket = &WebhookEventBucket{
				Name: rec.Bucket.Name,
				Type: rec.Bucket.Type,
			}
			event.Recording.Bucket.ID = rec.Bucket.Id
		}
		if rec.Creator.Id != 0 {
			p := webhookPersonFromGenerated(rec.Creator)
			event.Recording.Creator = &p
		}
	}

	// Map top-level creator
	if ge.Creator != nil {
		event.Creator = webhookPersonFromGenerated(*ge.Creator)
	}

	// Map copy if present
	// Presence is the pointer: a present-but-sparse copy object is still a copy.
	if ge.Copy != nil {
		c := &WebhookCopy{
			URL:    deref(ge.Copy.Url),
			AppURL: deref(ge.Copy.AppUrl),
			Bucket: WebhookCopyBucket{},
		}
		if ge.Copy.Id != nil {
			c.ID = *ge.Copy.Id
		}
		if ge.Copy.Bucket != nil && ge.Copy.Bucket.Id != nil {
			c.Bucket.ID = *ge.Copy.Bucket.Id
		}
		event.Copy = c
	}

	return event
}

// webhookPersonFromGenerated maps a generated Person to WebhookEventPerson with all fields.
func webhookPersonFromGenerated(gp generated.Person) WebhookEventPerson {
	p := WebhookEventPerson{
		AttachableSGID:      deref(gp.AttachableSgid),
		Name:                gp.Name,
		EmailAddress:        deref(gp.EmailAddress),
		PersonableType:      deref(gp.PersonableType),
		Title:               deref(gp.Title),
		Admin:               deref(gp.Admin),
		Owner:               deref(gp.Owner),
		Client:              deref(gp.Client),
		Employee:            deref(gp.Employee),
		TimeZone:            deref(gp.TimeZone),
		AvatarURL:           deref(gp.AvatarUrl),
		CanManageProjects:   deref(gp.CanManageProjects),
		CanManagePeople:     deref(gp.CanManagePeople),
		CanPing:             deref(gp.CanPing),
		CanAccessTimesheet:  deref(gp.CanAccessTimesheet),
		CanAccessHillCharts: deref(gp.CanAccessHillCharts),
	}
	if gp.Id != 0 {
		p.ID = int64(gp.Id)
	}
	if gp.Bio != nil {
		bio := *gp.Bio
		p.Bio = &bio
	}
	if gp.Location != nil {
		location := *gp.Location
		p.Location = &location
	}
	if gp.CreatedAt != nil {
		p.CreatedAt = gp.CreatedAt.Format(time.RFC3339Nano)
	}
	if gp.UpdatedAt != nil {
		p.UpdatedAt = gp.UpdatedAt.Format(time.RFC3339Nano)
	}
	if gp.Company != nil {
		p.Company = &WebhookEventCompany{
			Name: gp.Company.Name,
		}
		p.Company.ID = gp.Company.Id
	}
	return p
}
