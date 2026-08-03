package basecamp

import (
	"context"
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

// hillChartFromGenerated converts a generated HillChart to our clean HillChart type.
func hillChartFromGenerated(ghc generated.HillChart) HillChart {
	hc := HillChart{
		Enabled:        ghc.Enabled,
		Stale:          ghc.Stale,
		UpdatedAt:      ghc.UpdatedAt,
		AppUpdateURL:   deref(ghc.AppUpdateUrl),
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
