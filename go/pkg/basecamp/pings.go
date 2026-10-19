package basecamp

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Ping is a private conversation between people, held in its own bucket with
// one Campfire. A Ping's ID is its bucket's ID, so the bucket a Ping's chat
// lines carry is the ID Get takes. The bucket type, and the wire path, is
// "Circle". The Campfire's lines are read and written through
// CampfiresService; CampfireID names it.
type Ping struct {
	// ID is the Ping's id, which is also its bucket's id.
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Title is the Ping's name for the caller: its given name when Named,
	// else the names of the other people in it.
	Title string `json:"title"`
	Named bool   `json:"named"`
	// ImageURL is the Ping's own image; empty unless one was uploaded.
	ImageURL    string `json:"image_url,omitempty"`
	AvatarURL   string `json:"avatar_url"`
	URL         string `json:"url"`
	AppURL      string `json:"app_url"`
	BookmarkURL string `json:"bookmark_url"`
	// RoomURL is the Ping's Campfire.
	RoomURL string `json:"room_url"`
	// EditTitleURL is empty unless the Ping can be renamed: one with more
	// than one other person in it.
	EditTitleURL string    `json:"edit_title_url,omitempty"`
	Links        PingLinks `json:"links"`
	// People are the people in the Ping other than the caller; the caller
	// alone when nobody else is in it.
	People []Person `json:"people"`
}

// PingLinks are the Ping's Campfire endpoints.
type PingLinks struct {
	Lines string `json:"lines"`
	Files string `json:"files"`
}

// CampfireID returns the id of the Ping's Campfire, read from RoomURL
// through the SDK's route table. It is false when RoomURL does not name a
// Campfire.
func (c *Ping) CampfireID() (int64, bool) {
	m := DefaultRouter().MatchAPI(c.RoomURL)
	if m == nil || m.Operation != "GetCampfire" {
		return 0, false
	}
	id, err := strconv.ParseInt(m.Params["campfireId"], 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return id, true
}

// CreatePingRequest specifies the people to ping.
type CreatePingRequest struct {
	// UserIDs are the person ids of the people to ping (required). The caller
	// is always included, and people the caller cannot reach are dropped.
	UserIDs []int64 `json:"user_ids"`
	// NewGroup creates a new group even when a Ping with exactly these
	// people exists. It applies only with two or more other people.
	NewGroup bool `json:"new_group,omitempty"`
}

// PingsService handles Ping operations.
type PingsService struct {
	client *AccountClient
}

// NewPingsService creates a new PingsService.
func NewPingsService(client *AccountClient) *PingsService {
	return &PingsService{client: client}
}

// List returns the caller's active Pings that have a line in them, most
// recently updated first, in a single page. Archived Pings and Pings no one has
// posted in yet are not listed; Get reads either by id. A caller who cannot
// ping gets 403.
func (s *PingsService) List(ctx context.Context) (result []Ping, err error) {
	op := OperationInfo{
		Service: "Pings", Operation: "List",
		ResourceType: "ping", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListPingsWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	pings := []Ping{}
	if resp.JSON200 != nil {
		for _, gp := range *resp.JSON200 {
			pings = append(pings, pingFromGenerated(gp))
		}
	}
	return pings, nil
}

// Get returns a Ping by its id, which is its bucket's id. A bucket that is not
// a Ping the caller is in answers 404; a caller who cannot ping gets 403.
func (s *PingsService) Get(ctx context.Context, pingID int64) (result *Ping, err error) {
	op := OperationInfo{
		Service: "Pings", Operation: "Get",
		ResourceType: "ping", IsMutation: false,
		ResourceID: pingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetPingWithResponse(ctx, s.client.accountID, pingID)
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

	ping := pingFromGenerated(*resp.JSON200)
	return &ping, nil
}

// Create starts a Ping with people, or returns the active one that already
// holds exactly them (the oldest, if several) unless NewGroup is set. A caller
// who cannot ping gets 403, and a Ping with nobody in it who can ping is
// refused with 422.
func (s *PingsService) Create(ctx context.Context, req *CreatePingRequest) (result *Ping, err error) {
	op := OperationInfo{
		Service: "Pings", Operation: "Create",
		ResourceType: "ping", IsMutation: true,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || len(req.UserIDs) == 0 {
		err = ErrUsage("at least one person id is required")
		return nil, err
	}

	body := generated.CreatePingJSONRequestBody{UserIds: req.UserIDs}
	if req.NewGroup {
		body.NewGroup = &req.NewGroup
	}

	resp, err := s.client.parent.gen.CreatePingWithResponse(ctx, s.client.accountID, body)
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

	ping := pingFromGenerated(*resp.JSON201)
	return &ping, nil
}

func pingFromGenerated(gp generated.Ping) Ping {
	p := Ping{
		ID:           gp.Id,
		CreatedAt:    gp.CreatedAt,
		UpdatedAt:    gp.UpdatedAt,
		Title:        gp.Title,
		Named:        gp.Named,
		ImageURL:     deref(gp.ImageUrl),
		AvatarURL:    gp.AvatarUrl,
		URL:          gp.Url,
		AppURL:       gp.AppUrl,
		BookmarkURL:  gp.BookmarkUrl,
		RoomURL:      gp.RoomUrl,
		EditTitleURL: deref(gp.EditTitleUrl),
		Links:        PingLinks{Lines: gp.Links.Lines, Files: gp.Links.Files},
		People:       []Person{},
	}
	for _, person := range gp.People {
		p.People = append(p.People, personFromGenerated(person))
	}
	return p
}
