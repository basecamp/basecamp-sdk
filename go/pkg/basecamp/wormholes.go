package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Wormhole links a card table to a column on another card table, enabling cards
// to move across projects. A wormhole's id is a valid columnID for
// CardsService.Move: moving a card onto a wormhole teleports it to the
// destination column.
//
// The wormhole carries the full recording representation of its source board, so
// URL, AppURL, and Parent point at the source. DestinationURL is the only field
// identifying the destination column, and is nil when the wormhole is unlinked.
type Wormhole struct {
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
	// Color is the wormhole color, or nil when unset. Always emitted on the wire.
	Color *string `json:"color,omitempty"`
	// Linked is true only while the destination column, its board, and its bucket
	// are all active; it becomes false once the destination is unlinked.
	Linked bool `json:"linked"`
	// DestinationURL is the URL of the destination column, or nil when unlinked.
	DestinationURL *string `json:"destination_url,omitempty"`
	Parent         *Parent `json:"parent,omitempty"`
	Bucket         *Bucket `json:"bucket,omitempty"`
	Creator        *Person `json:"creator,omitempty"`
}

// WormholesService handles card-table wormhole operations.
type WormholesService struct {
	client *AccountClient
}

// NewWormholesService creates a new WormholesService.
func NewWormholesService(client *AccountClient) *WormholesService {
	return &WormholesService{client: client}
}

// Create links a card table to a column on another accessible card table.
//
// destinationRecordingID is the id of a column on the destination card table.
// A card table may hold at most four wormholes; exceeding that limit returns a
// validation error. An invalid, inaccessible, inactive, or same-board
// destination returns a not-found error. Returns the newly created wormhole.
func (s *WormholesService) Create(ctx context.Context, projectID, cardTableID, destinationRecordingID int64) (result *Wormhole, err error) {
	op := OperationInfo{
		Service: "Wormholes", Operation: "Create",
		ResourceType: "wormhole", IsMutation: true,
		ResourceID: cardTableID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if projectID == 0 {
		err = ErrUsage("project ID is required")
		return nil, err
	}
	if cardTableID == 0 {
		err = ErrUsage("card table ID is required")
		return nil, err
	}
	if destinationRecordingID == 0 {
		err = ErrUsage("destination recording ID is required")
		return nil, err
	}

	body := generated.CreateWormholeJSONRequestBody{
		DestinationRecordingId: destinationRecordingID,
	}

	resp, err := s.client.parent.gen.CreateWormholeWithResponse(ctx, s.client.accountID, projectID, cardTableID, body)
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

	wormhole := wormholeFromGenerated(*resp.JSON201)
	return &wormhole, nil
}

// Update points an existing wormhole at a new destination column.
//
// destinationRecordingID is the id of a column on another accessible card table.
// Returns the updated wormhole.
func (s *WormholesService) Update(ctx context.Context, projectID, wormholeID, destinationRecordingID int64) (result *Wormhole, err error) {
	op := OperationInfo{
		Service: "Wormholes", Operation: "Update",
		ResourceType: "wormhole", IsMutation: true,
		ResourceID: wormholeID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if projectID == 0 {
		err = ErrUsage("project ID is required")
		return nil, err
	}
	if wormholeID == 0 {
		err = ErrUsage("wormhole ID is required")
		return nil, err
	}
	if destinationRecordingID == 0 {
		err = ErrUsage("destination recording ID is required")
		return nil, err
	}

	body := generated.UpdateWormholeJSONRequestBody{
		DestinationRecordingId: destinationRecordingID,
	}

	resp, err := s.client.parent.gen.UpdateWormholeWithResponse(ctx, s.client.accountID, projectID, wormholeID, body)
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

	wormhole := wormholeFromGenerated(*resp.JSON200)
	return &wormhole, nil
}

// Delete removes a wormhole from a card table.
func (s *WormholesService) Delete(ctx context.Context, projectID, wormholeID int64) (err error) {
	op := OperationInfo{
		Service: "Wormholes", Operation: "Delete",
		ResourceType: "wormhole", IsMutation: true,
		ResourceID: wormholeID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if projectID == 0 {
		err = ErrUsage("project ID is required")
		return err
	}
	if wormholeID == 0 {
		err = ErrUsage("wormhole ID is required")
		return err
	}

	resp, err := s.client.parent.gen.DeleteWormholeWithResponse(ctx, s.client.accountID, projectID, wormholeID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// wormholeFromGenerated converts a generated Wormhole to our clean Wormhole type.
func wormholeFromGenerated(gw generated.Wormhole) Wormhole {
	w := Wormhole{
		Status:           gw.Status,
		VisibleToClients: gw.VisibleToClients,
		Title:            gw.Title,
		InheritsStatus:   gw.InheritsStatus,
		Type:             gw.Type,
		URL:              gw.Url,
		AppURL:           gw.AppUrl,
		BookmarkURL:      gw.BookmarkUrl,
		Linked:           gw.Linked,
		CreatedAt:        gw.CreatedAt,
		UpdatedAt:        gw.UpdatedAt,
	}

	if gw.Id != 0 {
		w.ID = gw.Id
	}

	// color and destination_url are modeled as nullable strings (x-go-type
	// "*string"), so the generated fields are already *string — nil when unset,
	// set otherwise. Copy the value so the clean type doesn't alias gw.
	if gw.Color != nil {
		s := *gw.Color
		w.Color = &s
	}
	if gw.DestinationUrl != nil {
		s := *gw.DestinationUrl
		w.DestinationURL = &s
	}

	if gw.Parent.Id != 0 || gw.Parent.Title != "" {
		w.Parent = &Parent{
			ID:     gw.Parent.Id,
			Title:  gw.Parent.Title,
			Type:   gw.Parent.Type,
			URL:    gw.Parent.Url,
			AppURL: gw.Parent.AppUrl,
		}
	}

	if gw.Bucket.Id != 0 || gw.Bucket.Name != "" {
		w.Bucket = &Bucket{
			ID:   gw.Bucket.Id,
			Name: gw.Bucket.Name,
			Type: gw.Bucket.Type,
		}
	}

	if gw.Creator.Id != 0 || gw.Creator.Name != "" {
		creator := personFromGenerated(gw.Creator)
		w.Creator = &creator
	}

	return w
}
