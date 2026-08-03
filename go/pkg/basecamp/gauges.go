package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Gauge represents a gauge (progress indicator) on a project.
type Gauge struct {
	ID          int64  `json:"id"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	// DescriptionAttachments holds structured metadata for the downloadable
	// files embedded in the rich text Description. Optional: the API renders
	// this array only when the gauge has needles, so it is absent for a
	// needle-less gauge. Optional and non-nullable; modeled as a pointer to a
	// slice with omitempty so all three wire states round-trip faithfully — nil
	// pointer (absent) is omitted, a non-nil pointer to an empty slice
	// re-encodes as [], and a populated one as the list. Decodes directly
	// (RichTextAttachment.UnmarshalJSON runs per element). See RichTextAttachment.
	DescriptionAttachments *[]RichTextAttachment `json:"description_attachments,omitempty"`
	Enabled                bool                  `json:"enabled,omitempty"`
	Status                 string                `json:"status,omitempty"`
	LastNeedleColor        string                `json:"last_needle_color,omitempty"`
	LastNeedlePosition     int32                 `json:"last_needle_position,omitempty"`
	PreviousNeedlePosition int32                 `json:"previous_needle_position,omitempty"`
	InheritsStatus         bool                  `json:"inherits_status,omitempty"`
	VisibleToClients       bool                  `json:"visible_to_clients,omitempty"`
	Type                   string                `json:"type,omitempty"`
	URL                    string                `json:"url,omitempty"`
	AppURL                 string                `json:"app_url,omitempty"`
	BookmarkURL            string                `json:"bookmark_url,omitempty"`
	Creator                *Person               `json:"creator,omitempty"`
	Bucket                 *Bucket               `json:"bucket,omitempty"`
	CreatedAt              time.Time             `json:"created_at"`
	UpdatedAt              time.Time             `json:"updated_at"`
}

// GaugeNeedle represents a single needle (progress update) on a gauge.
type GaugeNeedle struct {
	ID          int64  `json:"id"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	// DescriptionAttachments holds structured metadata for the downloadable files
	// embedded in the rich text Description. @required — the API always sends this
	// array (empty when the description has no inline files). No omitempty, so on
	// marshal a non-nil slice emits its elements ([] when empty) and a nil
	// slice emits null; the key is never
	// dropped. Decode distinguishes a server-sent [] (non-nil) from nil. See
	// RichTextAttachment.
	DescriptionAttachments []RichTextAttachment `json:"description_attachments"`
	Position               int32                `json:"position,omitempty"`
	Color                  string               `json:"color,omitempty"`
	Status                 string               `json:"status,omitempty"`
	InheritsStatus         bool                 `json:"inherits_status,omitempty"`
	VisibleToClients       bool                 `json:"visible_to_clients,omitempty"`
	CommentsCount          int32                `json:"comments_count,omitempty"`
	BoostsCount            int32                `json:"boosts_count,omitempty"`
	Type                   string               `json:"type,omitempty"`
	URL                    string               `json:"url,omitempty"`
	AppURL                 string               `json:"app_url,omitempty"`
	BookmarkURL            string               `json:"bookmark_url,omitempty"`
	CommentsURL            string               `json:"comments_url,omitempty"`
	BoostsURL              string               `json:"boosts_url,omitempty"`
	SubscriptionURL        string               `json:"subscription_url,omitempty"`
	Creator                *Person              `json:"creator,omitempty"`
	Bucket                 *Bucket              `json:"bucket,omitempty"`
	Parent                 *Parent              `json:"parent,omitempty"`
	CreatedAt              time.Time            `json:"created_at"`
	UpdatedAt              time.Time            `json:"updated_at"`
}

// CreateGaugeNeedleRequest specifies parameters for creating a gauge needle.
type CreateGaugeNeedleRequest struct {
	// Position of the needle (0-100), required.
	Position int32 `json:"position"`
	// Color is the status color: green (default), yellow, or red.
	Color string `json:"color,omitempty"`
	// Description is rich text (HTML) description of the progress update.
	Description string `json:"description,omitempty"`
	// Notify specifies who to notify: "everyone", "working_on", "custom", or omit for nobody.
	Notify string `json:"notify,omitempty"`
	// Subscriptions is an array of people IDs to notify (only used when Notify is "custom").
	Subscriptions []int64 `json:"subscriptions,omitempty"`
}

// UpdateGaugeNeedleRequest specifies parameters for updating a gauge needle.
type UpdateGaugeNeedleRequest struct {
	// Description is rich text (HTML) description. Tri-state: nil leaves the
	// existing description untouched, a pointer to "" clears it, and any other
	// value replaces it. A plain string could not express "clear" — the empty
	// value was indistinguishable from "not provided".
	Description *string `json:"description,omitempty"`
}

// GaugeListOptions specifies pagination for listing gauges.
type GaugeListOptions struct {
	// Limit is the maximum number of gauges to return.
	// If 0 (default), returns all gauges.
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination:
	// exactly one request, no Link rel="next" follow (SPEC §8). A positive
	// Limit still trims that page; the per-operation default limit does not
	// apply to it. Use 0 to paginate through all results up to Limit.
	Page int
}

// GaugeListResult contains the results from listing gauges.
type GaugeListResult struct {
	// Gauges is the list of gauges returned.
	Gauges []Gauge
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// GaugeNeedleListOptions specifies pagination for listing gauge needles.
type GaugeNeedleListOptions struct {
	// Limit is the maximum number of needles to return.
	// If 0 (default), returns all needles.
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination:
	// exactly one request, no Link rel="next" follow (SPEC §8). A positive
	// Limit still trims that page; the per-operation default limit does not
	// apply to it. Use 0 to paginate through all results up to Limit.
	Page int
}

// GaugeNeedleListResult contains the results from listing gauge needles.
type GaugeNeedleListResult struct {
	// Needles is the list of gauge needles returned.
	Needles []GaugeNeedle
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// GaugesService handles gauge operations.
type GaugesService struct {
	client *AccountClient
}

// NewGaugesService creates a new GaugesService.
func NewGaugesService(client *AccountClient) *GaugesService {
	return &GaugesService{client: client}
}

// decodeGaugePayload normalizes a raw gauge/needle response body and unmarshals
// it onto v. Gauge and GaugeNeedle embed *Person under "creator", and BC3 may
// serialize a person id as a JSON string; Person.ID is a plain int64, so the
// body is run through normalizeEmbeddedPeopleJSON first to coerce those ids.
// Normalization failures fall back to the raw body so a malformed-but-decodable
// payload is not lost.
func decodeGaugePayload(data []byte, v any) error {
	normalized, normalizeErr := normalizeEmbeddedPeopleJSON(data)
	if normalizeErr != nil {
		normalized = data
	}
	return json.Unmarshal(normalized, v)
}

// List returns gauges for the account, following pagination automatically.
// A positive opts.Page returns exactly that page in a single request.
func (s *GaugesService) List(ctx context.Context, opts *GaugeListOptions) (result *GaugeListResult, err error) {
	op := OperationInfo{
		Service: "Gauges", Operation: "List",
		ResourceType: "gauge", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.ListGaugesParams
	if opts != nil && opts.Page > 0 {
		params = &generated.ListGaugesParams{}
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params.Page = page
	}

	resp, err := s.client.parent.gen.ListGaugesWithResponse(ctx, s.client.accountID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var gauges []Gauge
	if err = decodeGaugePayload(resp.Body, &gauges); err != nil {
		return nil, fmt.Errorf("failed to parse gauges: %w", err)
	}

	totalCount := parseTotalCount(resp.HTTPResponse)
	if opts != nil && opts.Page > 0 {
		keep, truncated := pageCap(len(gauges), opts.Limit, resp.HTTPResponse)
		return &GaugeListResult{Gauges: gauges[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
	}

	limit := 0
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}
	if limit > 0 && len(gauges) >= limit {
		return &GaugeListResult{Gauges: gauges[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(gauges), limit)}}, nil
	}

	// Follow pagination via Link headers
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(gauges), limit)
	if err != nil {
		return nil, err
	}
	for _, raw := range rawMore {
		var g Gauge
		if err = decodeGaugePayload(raw, &g); err != nil {
			return nil, fmt.Errorf("failed to parse gauge: %w", err)
		}
		gauges = append(gauges, g)
	}

	return &GaugeListResult{Gauges: gauges, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// ListNeedles returns needles for a project's gauge, following pagination
// automatically. A positive opts.Page returns exactly that page in a single
// request.
func (s *GaugesService) ListNeedles(ctx context.Context, projectID int64, opts *GaugeNeedleListOptions) (result *GaugeNeedleListResult, err error) {
	op := OperationInfo{
		Service: "Gauges", Operation: "ListNeedles",
		ResourceType: "gauge_needle", IsMutation: false,
		ProjectID: projectID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.ListGaugeNeedlesParams
	if opts != nil && opts.Page > 0 {
		params = &generated.ListGaugeNeedlesParams{}
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params.Page = page
	}

	resp, err := s.client.parent.gen.ListGaugeNeedlesWithResponse(ctx, s.client.accountID, projectID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var needles []GaugeNeedle
	if err = decodeGaugePayload(resp.Body, &needles); err != nil {
		return nil, fmt.Errorf("failed to parse gauge needles: %w", err)
	}

	totalCount := parseTotalCount(resp.HTTPResponse)
	if opts != nil && opts.Page > 0 {
		keep, truncated := pageCap(len(needles), opts.Limit, resp.HTTPResponse)
		return &GaugeNeedleListResult{Needles: needles[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
	}

	limit := 0
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}
	if limit > 0 && len(needles) >= limit {
		return &GaugeNeedleListResult{Needles: needles[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(needles), limit)}}, nil
	}

	// Follow pagination via Link headers
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(needles), limit)
	if err != nil {
		return nil, err
	}
	for _, raw := range rawMore {
		var n GaugeNeedle
		if err = decodeGaugePayload(raw, &n); err != nil {
			return nil, fmt.Errorf("failed to parse gauge needle: %w", err)
		}
		needles = append(needles, n)
	}

	return &GaugeNeedleListResult{Needles: needles, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// GetNeedle returns a single gauge needle by ID.
func (s *GaugesService) GetNeedle(ctx context.Context, needleID int64) (result *GaugeNeedle, err error) {
	op := OperationInfo{
		Service: "Gauges", Operation: "GetNeedle",
		ResourceType: "gauge_needle", IsMutation: false,
		ResourceID: needleID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetGaugeNeedleWithResponse(ctx, s.client.accountID, needleID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var needle GaugeNeedle
	if err = decodeGaugePayload(resp.Body, &needle); err != nil {
		return nil, fmt.Errorf("failed to parse gauge needle: %w", err)
	}

	return &needle, nil
}

// CreateNeedle creates a new gauge needle on a project.
func (s *GaugesService) CreateNeedle(ctx context.Context, projectID int64, req *CreateGaugeNeedleRequest) (result *GaugeNeedle, err error) {
	op := OperationInfo{
		Service: "Gauges", Operation: "CreateNeedle",
		ResourceType: "gauge_needle", IsMutation: true,
		ProjectID: projectID,
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
		err = ErrUsage("create needle request is required")
		return nil, err
	}

	body := generated.CreateGaugeNeedleJSONRequestBody{
		GaugeNeedle: generated.GaugeNeedlePayload{
			Position:    req.Position,
			Color:       omitzero(req.Color),
			Description: omitzero(req.Description),
		},
		Notify: omitzero(req.Notify),
	}
	// nil means "not addressed" (omitted); a non-nil empty slice is an explicit
	// empty subscriber list and must reach the wire.
	if req.Subscriptions != nil {
		body.Subscriptions = &req.Subscriptions
	}

	resp, err := s.client.parent.gen.CreateGaugeNeedleWithResponse(ctx, s.client.accountID, projectID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var needle GaugeNeedle
	if err = decodeGaugePayload(resp.Body, &needle); err != nil {
		return nil, fmt.Errorf("failed to parse gauge needle: %w", err)
	}

	return &needle, nil
}

// UpdateNeedle updates an existing gauge needle.
func (s *GaugesService) UpdateNeedle(ctx context.Context, needleID int64, req *UpdateGaugeNeedleRequest) (result *GaugeNeedle, err error) {
	op := OperationInfo{
		Service: "Gauges", Operation: "UpdateNeedle",
		ResourceType: "gauge_needle", IsMutation: true,
		ResourceID: needleID,
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
		err = ErrUsage("update needle request is required")
		return nil, err
	}

	body := generated.UpdateGaugeNeedleJSONRequestBody{
		GaugeNeedle: &generated.GaugeNeedleUpdatePayload{
			Description: req.Description,
		},
	}

	resp, err := s.client.parent.gen.UpdateGaugeNeedleWithResponse(ctx, s.client.accountID, needleID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	var needle GaugeNeedle
	if err = decodeGaugePayload(resp.Body, &needle); err != nil {
		return nil, fmt.Errorf("failed to parse gauge needle: %w", err)
	}

	return &needle, nil
}

// DestroyNeedle deletes a gauge needle.
func (s *GaugesService) DestroyNeedle(ctx context.Context, needleID int64) (err error) {
	op := OperationInfo{
		Service: "Gauges", Operation: "DestroyNeedle",
		ResourceType: "gauge_needle", IsMutation: true,
		ResourceID: needleID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DestroyGaugeNeedleWithResponse(ctx, s.client.accountID, needleID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Toggle enables or disables the gauge on a project.
func (s *GaugesService) Toggle(ctx context.Context, projectID int64, enabled bool) (err error) {
	op := OperationInfo{
		Service: "Gauges", Operation: "Toggle",
		ResourceType: "gauge", IsMutation: true,
		ProjectID: projectID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	body := generated.ToggleGaugeJSONRequestBody{
		Gauge: generated.GaugeTogglePayload{
			Enabled: enabled,
		},
	}

	resp, err := s.client.parent.gen.ToggleGaugeWithResponse(ctx, s.client.accountID, projectID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}
