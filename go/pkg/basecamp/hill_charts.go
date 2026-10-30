package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// HillChart represents a hill chart for a todoset.
type HillChart struct {
	Enabled bool `json:"enabled"`
	Stale   bool `json:"stale"`
	// UpdatedAt is when the chart last moved. Optional — a chart that has
	// never been updated omits it. A pointer so absence stays distinguishable
	// from a real value: a value-typed time.Time would read as
	// 0001-01-01T00:00:00Z, and `,omitempty` cannot hide that because
	// encoding/json never treats a struct as empty. Mirrors the *time.Time
	// convention used for Card.CompletedAt and EverythingFile.UpdatedAt.
	UpdatedAt      *time.Time     `json:"updated_at,omitempty"`
	AppUpdateURL   string         `json:"app_update_url,omitempty"`
	AppVersionsURL string         `json:"app_versions_url,omitempty"`
	Dots           []HillChartDot `json:"dots,omitempty"`
	// VersionsURL is the API URL of the chart's progress updates (see
	// HillChartsService.ListVersions). Present only when the chart has at
	// least one progress update.
	VersionsURL string `json:"versions_url,omitempty"`
}

// HillChartDot represents a single dot on a hill chart, corresponding to a tracked todolist.
type HillChartDot struct {
	ID       int64  `json:"id"`
	Label    string `json:"label"`
	Color    string `json:"color"`
	Position int    `json:"position"`
	URL      string `json:"url,omitempty"`
	AppURL   string `json:"app_url,omitempty"`
}

// DefaultHillChartVersionLimit is the default number of progress updates to
// return when no limit is specified. Matches the API's maxPageSize of 50.
const DefaultHillChartVersionLimit = 50

// HillChartVersion is a progress update on a hill chart: the note someone
// wrote when they updated the chart, and where each tracked todolist sat on
// the hill at that moment.
type HillChartVersion struct {
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
	BookmarkURL      string    `json:"bookmark_url,omitempty"`
	SubscriptionURL  string    `json:"subscription_url,omitempty"`
	CommentsCount    int       `json:"comments_count"`
	CommentsURL      string    `json:"comments_url,omitempty"`
	BoostsCount      int       `json:"boosts_count,omitempty"`
	BoostsURL        string    `json:"boosts_url,omitempty"`
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
	// Description is the note written with the update, as rich text.
	Description string `json:"description"`
	// DescriptionAttachments holds structured metadata for the downloadable
	// files embedded in Description. @required — the API always sends this
	// array. No omitempty, so the key is never dropped on marshal. See
	// RichTextAttachment.
	DescriptionAttachments []RichTextAttachment `json:"description_attachments"`
	// Dots is where each tracked todolist sat on the hill when the update was
	// made, ordered by position.
	Dots []HillChartVersionDot `json:"dots"`
}

// HillChartVersionDot is a todolist's place on the hill in one progress update.
type HillChartVersionDot struct {
	// ID is the todolist's ID, or nil when the todolist has since been
	// permanently deleted or moved to another project; URL and AppURL are then
	// empty.
	ID *int64 `json:"id"`
	// Label is the todolist's full name at the time of the update.
	Label string `json:"label"`
	Color string `json:"color"`
	// Position is on the same 0–100 scale as HillChartDot.Position.
	Position int    `json:"position"`
	URL      string `json:"url,omitempty"`
	AppURL   string `json:"app_url,omitempty"`
}

// HillChartDotPosition moves one tracked todolist in a progress update.
type HillChartDotPosition struct {
	// ID is the todolist's ID. It must be tracked on the hill chart.
	ID int64 `json:"id"`
	// Position is a whole number from 0 to 100, on the same scale as
	// HillChartDot.Position.
	Position int `json:"position"`
}

// CreateHillChartVersionRequest specifies a progress update to post on a hill
// chart. Every field is optional: an empty request records the chart as it
// stands.
type CreateHillChartVersionRequest struct {
	// Description is a note about the update, as rich text (HTML). Empty
	// omits it.
	Description string `json:"description,omitempty"`
	// Dots moves tracked todolists. Todolists left out keep their current
	// position, so one dot can move without sending the others. To track or
	// untrack a todolist, use HillChartsService.UpdateSettings.
	Dots []HillChartDotPosition `json:"dots,omitempty"`
	// Subscriptions controls who is notified of the update and subscribed to it.
	// nil: field omitted, and nobody else is notified. &[]int64{1,2}: those people.
	Subscriptions *[]int64 `json:"subscriptions,omitempty"`
}

// HillChartVersionListOptions specifies options for listing progress updates.
type HillChartVersionListOptions struct {
	// Limit is the maximum number of progress updates to return.
	// If 0, uses DefaultHillChartVersionLimit (50). Use -1 for unlimited.
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination:
	// exactly one request, no Link rel="next" follow (SPEC §8). A positive
	// Limit still trims that page; the per-operation default limit does not
	// apply to it. Use 0 to paginate through all results up to Limit.
	Page int
}

// HillChartVersionListResult contains the results from listing progress updates.
type HillChartVersionListResult struct {
	// Versions is the list of progress updates returned, newest first.
	Versions []HillChartVersion
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// HillChartsService handles hill chart operations.
type HillChartsService struct {
	client *AccountClient
}

// NewHillChartsService creates a new HillChartsService.
func NewHillChartsService(client *AccountClient) *HillChartsService {
	return &HillChartsService{client: client}
}

// Get returns the hill chart for a todoset.
func (s *HillChartsService) Get(ctx context.Context, todosetID int64) (result *HillChart, err error) {
	op := OperationInfo{
		Service: "HillCharts", Operation: "Get",
		ResourceType: "hill_chart", IsMutation: false,
		ResourceID: todosetID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetHillChartWithResponse(ctx, s.client.accountID, todosetID)
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

	hillChart := hillChartFromGenerated(*resp.JSON200)
	return &hillChart, nil
}

// UpdateSettings tracks or untracks todolists on a hill chart.
// Pass todolist IDs to tracked and/or untracked. Both are optional.
func (s *HillChartsService) UpdateSettings(ctx context.Context, todosetID int64, tracked, untracked []int64) (result *HillChart, err error) {
	op := OperationInfo{
		Service: "HillCharts", Operation: "UpdateSettings",
		ResourceType: "hill_chart", IsMutation: true,
		ResourceID: todosetID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	body := generated.UpdateHillChartSettingsJSONRequestBody{}
	// nil means "not addressed" (omitted); a non-nil empty slice is an
	// explicit empty list and must reach the wire.
	if tracked != nil {
		body.Tracked = &tracked
	}
	if untracked != nil {
		body.Untracked = &untracked
	}

	resp, err := s.client.parent.gen.UpdateHillChartSettingsWithResponse(ctx, s.client.accountID, todosetID, body)
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

	hillChart := hillChartFromGenerated(*resp.JSON200)
	return &hillChart, nil
}

// ListVersions returns the progress updates on a todoset's hill chart, newest
// first. Earlier updates are kept, so this is the chart's history.
//
// By default, returns up to 50 progress updates. Use Limit: -1 for unlimited.
//
// Pagination options:
//   - Limit: maximum number of updates to return (0 = 50, -1 = unlimited)
//   - Page: if positive, fetches only that page and disables auto-pagination
//
// The returned HillChartVersionListResult includes pagination metadata
// (TotalCount from X-Total-Count header) when available.
func (s *HillChartsService) ListVersions(ctx context.Context, todosetID int64, opts *HillChartVersionListOptions) (result *HillChartVersionListResult, err error) {
	op := OperationInfo{
		Service: "HillCharts", Operation: "ListVersions",
		ResourceType: "hill_chart_version", IsMutation: false,
		ResourceID: todosetID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.ListHillChartVersionsParams
	if opts != nil && opts.Page > 0 {
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params = &generated.ListHillChartVersionsParams{Page: page}
	}

	resp, err := s.client.parent.gen.ListHillChartVersionsWithResponse(ctx, s.client.accountID, todosetID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	totalCount := parseTotalCount(resp.HTTPResponse)

	versions := []HillChartVersion{}
	if resp.JSON200 != nil {
		for _, gv := range *resp.JSON200 {
			versions = append(versions, hillChartVersionFromGenerated(gv))
		}
	}

	if opts != nil && opts.Page > 0 {
		keep, truncated := pageCap(len(versions), opts.Limit, resp.HTTPResponse)
		return &HillChartVersionListResult{Versions: versions[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
	}

	limit := DefaultHillChartVersionLimit
	if opts != nil {
		if opts.Limit < 0 {
			limit = 0 // unlimited
		} else if opts.Limit > 0 {
			limit = opts.Limit
		}
	}

	if limit > 0 && len(versions) >= limit {
		return &HillChartVersionListResult{Versions: versions[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(versions), limit)}}, nil
	}

	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(versions), limit)
	if err != nil {
		return nil, err
	}

	for _, raw := range rawMore {
		var gv generated.HillChartVersion
		if err := json.Unmarshal(raw, &gv); err != nil {
			return nil, fmt.Errorf("failed to parse hill chart version: %w", err)
		}
		versions = append(versions, hillChartVersionFromGenerated(gv))
	}

	return &HillChartVersionListResult{Versions: versions, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// GetVersion returns one progress update from a todoset's hill chart.
func (s *HillChartsService) GetVersion(ctx context.Context, todosetID, versionID int64) (result *HillChartVersion, err error) {
	op := OperationInfo{
		Service: "HillCharts", Operation: "GetVersion",
		ResourceType: "hill_chart_version", IsMutation: false,
		ResourceID: versionID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetHillChartVersionWithResponse(ctx, s.client.accountID, todosetID, versionID)
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

	version := hillChartVersionFromGenerated(*resp.JSON200)
	return &version, nil
}

// CreateVersion posts a progress update on a todoset's hill chart: it moves
// the todolists named in req.Dots and saves the update, as updating the hill
// chart in Basecamp does. req may be nil, which records the chart as it
// stands.
//
// Returns the new progress update, including where every tracked todolist now
// sits. The API answers 422 (a validation error, nothing saved) when a
// todolist in Dots appears more than once or isn't tracked on the chart, or
// when the chart tracks no todolists. A position outside 0–100 is refused
// here as a usage error before any request is sent.
func (s *HillChartsService) CreateVersion(ctx context.Context, todosetID int64, req *CreateHillChartVersionRequest) (result *HillChartVersion, err error) {
	op := OperationInfo{
		Service: "HillCharts", Operation: "CreateVersion",
		ResourceType: "hill_chart_version", IsMutation: true,
		ResourceID: todosetID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	body := generated.CreateHillChartVersionJSONRequestBody{}
	if req != nil {
		body.Description = omitzero(req.Description)
		body.Subscriptions = req.Subscriptions
		// nil omits dots; a non-nil slice reaches the wire, even when empty.
		if req.Dots != nil {
			dots := make([]generated.HillChartDotPosition, 0, len(req.Dots))
			for _, d := range req.Dots {
				if d.Position < 0 || d.Position > 100 {
					err = ErrUsage(fmt.Sprintf("hill chart position for todolist %d must be a whole number from 0 to 100, got %d", d.ID, d.Position))
					return nil, err
				}
				dots = append(dots, generated.HillChartDotPosition{
					Id:       d.ID,
					Position: int32(d.Position), // #nosec G115 -- bounds checked above
				})
			}
			body.Dots = &dots
		}
	}

	resp, err := s.client.parent.gen.CreateHillChartVersionWithResponse(ctx, s.client.accountID, todosetID, body)
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

	version := hillChartVersionFromGenerated(*resp.JSON201)
	return &version, nil
}

// hillChartVersionFromGenerated converts a generated HillChartVersion to our clean type.
func hillChartVersionFromGenerated(gv generated.HillChartVersion) HillChartVersion {
	v := HillChartVersion{
		ID:               gv.Id,
		Status:           gv.Status,
		VisibleToClients: gv.VisibleToClients,
		CreatedAt:        gv.CreatedAt,
		UpdatedAt:        gv.UpdatedAt,
		Title:            gv.Title,
		InheritsStatus:   gv.InheritsStatus,
		Type:             gv.Type,
		URL:              gv.Url,
		AppURL:           gv.AppUrl,
		BookmarkURL:      deref(gv.BookmarkUrl),
		SubscriptionURL:  deref(gv.SubscriptionUrl),
		CommentsCount:    int(deref(gv.CommentsCount)),
		CommentsURL:      deref(gv.CommentsUrl),
		BoostsCount:      int(deref(gv.BoostsCount)),
		BoostsURL:        deref(gv.BoostsUrl),
		Description:      deref(gv.Description),
	}

	if gv.Parent.Id != 0 || gv.Parent.Title != "" {
		v.Parent = &Parent{
			ID:     gv.Parent.Id,
			Title:  gv.Parent.Title,
			Type:   gv.Parent.Type,
			URL:    gv.Parent.Url,
			AppURL: gv.Parent.AppUrl,
		}
	}

	if gv.Bucket.Id != 0 || gv.Bucket.Name != "" {
		v.Bucket = &Bucket{
			ID:   gv.Bucket.Id,
			Name: gv.Bucket.Name,
			Type: gv.Bucket.Type,
		}
	}

	if gv.Creator.Id != 0 || gv.Creator.Name != "" {
		creator := personFromGenerated(gv.Creator)
		v.Creator = &creator
	}

	v.DescriptionAttachments = richTextAttachmentsFromGenerated(gv.DescriptionAttachments)

	v.Dots = make([]HillChartVersionDot, 0, len(gv.Dots))
	for _, gd := range gv.Dots {
		v.Dots = append(v.Dots, HillChartVersionDot{
			ID:       gd.Id,
			Label:    gd.Label,
			Color:    gd.Color,
			Position: int(gd.Position),
			URL:      deref(gd.Url),
			AppURL:   deref(gd.AppUrl),
		})
	}

	return v
}

// hillChartFromGenerated converts a generated HillChart to our clean HillChart type.
func hillChartFromGenerated(ghc generated.HillChart) HillChart {
	hc := HillChart{
		Enabled:        ghc.Enabled,
		Stale:          ghc.Stale,
		UpdatedAt:      ghc.UpdatedAt,
		AppUpdateURL:   deref(ghc.AppUpdateUrl),
		VersionsURL:    deref(ghc.VersionsUrl),
		AppVersionsURL: deref(ghc.AppVersionsUrl),
	}

	if len(ghc.Dots) > 0 {
		hc.Dots = make([]HillChartDot, 0, len(ghc.Dots))
		for _, gd := range ghc.Dots {
			hc.Dots = append(hc.Dots, HillChartDot{
				ID:       gd.Id,
				Label:    gd.Label,
				Color:    gd.Color,
				Position: int(gd.Position),
				URL:      deref(gd.Url),
				AppURL:   deref(gd.AppUrl),
			})
		}
	}

	return hc
}
