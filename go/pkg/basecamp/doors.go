package basecamp

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Door is an external link: a dock tool pointing at an outside address (a
// Figma file, a Dropbox folder, a GitHub repository). Basecamp's URLs and
// recording type still call it a door.
type Door struct {
	ID               int64     `json:"id"`
	Status           string    `json:"status"`
	VisibleToClients bool      `json:"visible_to_clients"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Title            string    `json:"title"`
	InheritsStatus   bool      `json:"inherits_status"`
	Type             string    `json:"type"`
	// URL is the external link's OUTSIDE address — not this record's API URL.
	// The door partial renders the shared recording partial and then the
	// door's own url, which overwrites the recording's. AppURL is still this
	// record's Basecamp URL (a redirector to the outside address).
	URL             string `json:"url"`
	AppURL          string `json:"app_url"`
	BookmarkURL     string `json:"bookmark_url,omitempty"`
	SubscriptionURL string `json:"subscription_url,omitempty"`
	// Position is the link's place in the project's External links section.
	Position int     `json:"position,omitempty"`
	Bucket   *Bucket `json:"bucket,omitempty"`
	Creator  *Person `json:"creator,omitempty"`
	// Service is the recognized external service the link points to. Its Code
	// is "other" for a generic link.
	Service DoorService `json:"service"`
	// Description is the rich-text (HTML) description shown beneath the link.
	Description string `json:"description,omitempty"`
}

// DoorListResult is the result of listing a project's external links.
type DoorListResult struct {
	// Doors are the project's active external links, in the order they
	// appear in its External links section.
	Doors []Door `json:"doors"`
}

// CreateDoorRequest is the body for creating an external link.
type CreateDoorRequest struct {
	// Service is the short service identifier (required): "figma",
	// "dropbox", "google_drive", "github", "notion", "trello", "slack",
	// "zoom", … or "other" for a generic link.
	Service string `json:"service"`
	// URL is the address the link points to (required). A value without a
	// scheme is prefixed with https://; an unusable one (javascript:) is a 422.
	URL string `json:"url"`
	// Title is the name shown in the External links section (optional).
	// Omitting it reads back as "Untitled".
	Title string `json:"title,omitempty"`
	// Description is a rich-text (HTML) description (optional).
	Description string `json:"description,omitempty"`
	// VisibleToClients sets whether the project's clients can see the link.
	// nil omits it, and the server then keeps the link PRIVATE to the team,
	// whatever the visibility of the project's other tools. A client caller
	// always creates a client-visible link.
	VisibleToClients *bool `json:"visible_to_clients,omitempty"`
}

// UpdateDoorRequest is the body for updating an external link.
//
// PUT /{accountId}/dock/doors/{doorId} is a PARTIAL update: every field is
// optional, and what is left out keeps its value. When none of Service, URL,
// Title and Description is set, the request carries no door fields at all, so
// it changes only VisibleToClients.
type UpdateDoorRequest struct {
	// Service is the new service identifier. Empty leaves it unchanged.
	Service string `json:"service,omitempty"`
	// URL is the new address. Empty leaves it unchanged.
	URL string `json:"url,omitempty"`
	// Title is the new name. Empty leaves it unchanged.
	Title string `json:"title,omitempty"`
	// Description is the new rich-text (HTML) description. nil leaves it
	// unchanged; a pointer to "" clears it.
	Description *string `json:"description,omitempty"`
	// VisibleToClients sets whether the project's clients can see the link.
	// nil keeps the current visibility.
	VisibleToClients *bool `json:"visible_to_clients,omitempty"`
}

// DoorsService handles external link (door) operations.
type DoorsService struct {
	client *AccountClient
}

// NewDoorsService creates a new DoorsService.
func NewDoorsService(client *AccountClient) *DoorsService {
	return &DoorsService{client: client}
}

// List returns the active external links in a project, in the order they
// appear in its External links section. The list is not paginated. Clients see
// only the links visible to them.
func (s *DoorsService) List(ctx context.Context, bucketID int64) (result *DoorListResult, err error) {
	op := OperationInfo{
		Service: "Doors", Operation: "List",
		ResourceType: "door", IsMutation: false,
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

	resp, err := s.client.parent.gen.ListDoorsWithResponse(ctx, s.client.accountID, bucketID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	doors := []Door{}
	if resp.JSON200 != nil {
		for _, gd := range *resp.JSON200 {
			doors = append(doors, doorFromGenerated(gd))
		}
	}

	return &DoorListResult{Doors: doors}, nil
}

// Get returns an external link by ID. An unknown ID, the ID of a recording
// that is not a door, or a door hidden from a client caller is a not-found
// error.
func (s *DoorsService) Get(ctx context.Context, doorID int64) (result *Door, err error) {
	op := OperationInfo{
		Service: "Doors", Operation: "Get",
		ResourceType: "door", IsMutation: false,
		ResourceID: doorID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetDoorWithResponse(ctx, s.client.accountID, doorID)
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

	door := doorFromGenerated(*resp.JSON200)
	return &door, nil
}

// Create creates an external link at the end of a project's External links
// section. It needs permission to edit the project.
//
// Without req.VisibleToClients the link is private to the team.
func (s *DoorsService) Create(ctx context.Context, bucketID int64, req *CreateDoorRequest) (result *Door, err error) {
	op := OperationInfo{
		Service: "Doors", Operation: "Create",
		ResourceType: "door", IsMutation: true,
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

	if req == nil || req.URL == "" {
		err = ErrUsage("door url is required")
		return nil, err
	}
	if req.Service == "" {
		err = ErrUsage("door service is required")
		return nil, err
	}

	body := generated.CreateDoorJSONRequestBody{
		Door: generated.CreateDoorPayload{
			Service:     req.Service,
			Url:         req.URL,
			Title:       omitzero(req.Title),
			Description: omitzero(req.Description),
		},
		VisibleToClients: req.VisibleToClients,
	}

	resp, err := s.client.parent.gen.CreateDoorWithResponse(ctx, s.client.accountID, bucketID, body)
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

	door := doorFromGenerated(*resp.JSON201)
	return &door, nil
}

// Update changes an external link. It is a partial update: see
// UpdateDoorRequest for what an omitted field means. It needs permission to
// edit the project.
func (s *DoorsService) Update(ctx context.Context, doorID int64, req *UpdateDoorRequest) (result *Door, err error) {
	op := OperationInfo{
		Service: "Doors", Operation: "Update",
		ResourceType: "door", IsMutation: true,
		ResourceID: doorID,
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
		err = ErrUsage("door update request is required")
		return nil, err
	}

	body := generated.UpdateDoorJSONRequestBody{
		VisibleToClients: req.VisibleToClients,
	}
	if req.Service != "" || req.URL != "" || req.Title != "" || req.Description != nil {
		body.Door = &generated.UpdateDoorPayload{
			Service:     omitzero(req.Service),
			Url:         omitzero(req.URL),
			Title:       omitzero(req.Title),
			Description: req.Description,
		}
	}

	resp, err := s.client.parent.gen.UpdateDoorWithResponse(ctx, s.client.accountID, doorID, body)
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

	door := doorFromGenerated(*resp.JSON200)
	return &door, nil
}

// Delete deletes an external link. Its status becomes "deleted" and it leaves
// the project's External links section. It does not go to the trash: Basecamp
// permanently destroys it about five days later. It needs permission to edit
// the project.
func (s *DoorsService) Delete(ctx context.Context, doorID int64) (err error) {
	op := OperationInfo{
		Service: "Doors", Operation: "Delete",
		ResourceType: "door", IsMutation: true,
		ResourceID: doorID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DeleteDoorWithResponse(ctx, s.client.accountID, doorID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// doorFromGenerated converts a generated Door to our clean type.
func doorFromGenerated(gd generated.Door) Door {
	d := Door{
		ID:               gd.Id,
		Status:           gd.Status,
		VisibleToClients: gd.VisibleToClients,
		CreatedAt:        gd.CreatedAt,
		UpdatedAt:        gd.UpdatedAt,
		Title:            gd.Title,
		InheritsStatus:   gd.InheritsStatus,
		Type:             gd.Type,
		URL:              gd.Url,
		AppURL:           gd.AppUrl,
		BookmarkURL:      deref(gd.BookmarkUrl),
		SubscriptionURL:  deref(gd.SubscriptionUrl),
		Position:         int(deref(gd.Position)),
		Description:      deref(gd.Description),
		Service: DoorService{
			Name:           deref(gd.Service.Name),
			ExampleURL:     deref(gd.Service.ExampleUrl),
			Code:           deref(gd.Service.Code),
			ValidPatterns:  slices.Clone(gd.Service.ValidPatterns),
			SupportingText: deref(gd.Service.SupportingText),
		},
	}

	if gd.Bucket.Id != 0 || gd.Bucket.Name != "" {
		d.Bucket = &Bucket{
			ID:   gd.Bucket.Id,
			Name: gd.Bucket.Name,
			Type: gd.Bucket.Type,
		}
	}

	if gd.Creator.Id != 0 || gd.Creator.Name != "" {
		creator := personFromGenerated(gd.Creator)
		d.Creator = &creator
	}

	return d
}
