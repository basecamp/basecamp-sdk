package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
	"github.com/basecamp/basecamp-sdk/go/pkg/types"
)

// CardTable represents a Basecamp card table (kanban board).
type CardTable struct {
	ID               int64        `json:"id"`
	Status           string       `json:"status"`
	VisibleToClients bool         `json:"visible_to_clients"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
	Title            string       `json:"title"`
	InheritsStatus   bool         `json:"inherits_status"`
	Type             string       `json:"type"`
	URL              string       `json:"url"`
	AppURL           string       `json:"app_url"`
	BookmarkURL      string       `json:"bookmark_url"`
	SubscriptionURL  string       `json:"subscription_url"`
	Bucket           *Bucket      `json:"bucket,omitempty"`
	Creator          *Person      `json:"creator,omitempty"`
	Subscribers      []Person     `json:"subscribers,omitempty"`
	Lists            []CardColumn `json:"lists,omitempty"`
	Wormholes        []Wormhole   `json:"wormholes,omitempty"`
}

// CardColumn represents a column in a card table.
type CardColumn struct {
	ID               int64             `json:"id"`
	Status           string            `json:"status"`
	VisibleToClients bool              `json:"visible_to_clients"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	Title            string            `json:"title"`
	InheritsStatus   bool              `json:"inherits_status"`
	Type             string            `json:"type"`
	URL              string            `json:"url"`
	AppURL           string            `json:"app_url"`
	BookmarkURL      string            `json:"bookmark_url"`
	Position         int               `json:"position,omitempty"`
	Color            string            `json:"color,omitempty"`
	Description      string            `json:"description,omitempty"`
	CardsCount       int               `json:"cards_count"`
	CommentCount     int               `json:"comment_count"`
	CommentsCount    int               `json:"comments_count,omitempty"`
	CardsURL         string            `json:"cards_url,omitempty"`
	OnHold           *CardColumnOnHold `json:"on_hold,omitempty"`
	Parent           *Parent           `json:"parent,omitempty"`
	Bucket           *Bucket           `json:"bucket,omitempty"`
	Creator          *Person           `json:"creator,omitempty"`
	Subscribers      []Person          `json:"subscribers,omitempty"`
}

// CardColumnOnHold represents the on-hold section of a card column.
type CardColumnOnHold struct {
	ID             int64     `json:"id"`
	Status         string    `json:"status"`
	InheritsStatus bool      `json:"inherits_status"`
	Title          string    `json:"title"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	CardsCount     int       `json:"cards_count"`
	CardsURL       string    `json:"cards_url"`
}

// Card represents a card in a card table column.
type Card struct {
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
	SubscriptionURL  string    `json:"subscription_url,omitempty"`
	Position         int       `json:"position"`
	Content          string    `json:"content,omitempty"`
	Description      string    `json:"description,omitempty"`
	// DescriptionAttachments holds structured metadata for the downloadable files
	// embedded in the rich text Description. @required — the API always sends this
	// array (empty when the description has no inline files). No omitempty, so on
	// marshal a non-nil slice emits its elements ([] when empty) and a nil
	// slice emits null; the key is never
	// dropped. Decode distinguishes a server-sent [] (non-nil) from nil. See
	// RichTextAttachment.
	DescriptionAttachments []RichTextAttachment `json:"description_attachments"`
	DueOn                  string               `json:"due_on,omitempty"`
	Completed              bool                 `json:"completed"`
	CompletedAt            *time.Time           `json:"completed_at,omitempty"`
	CommentsCount          int                  `json:"comments_count"`
	BoostsCount            int                  `json:"boosts_count"`
	BoostsURL              string               `json:"boosts_url,omitempty"`
	CommentsURL            string               `json:"comments_url,omitempty"`
	CommentCount           int                  `json:"comment_count"`
	CompletionURL          string               `json:"completion_url,omitempty"`
	Parent                 *Parent              `json:"parent,omitempty"`
	Bucket                 *Bucket              `json:"bucket,omitempty"`
	Creator                *Person              `json:"creator,omitempty"`
	Completer              *Person              `json:"completer,omitempty"`
	Assignees              []Person             `json:"assignees,omitempty"`
	CompletionSubscribers  []Person             `json:"completion_subscribers,omitempty"`
	Steps                  []CardStep           `json:"steps,omitempty"`
}

// CardStep represents a step (checklist item) on a card.
type CardStep struct {
	ID               int64      `json:"id"`
	Status           string     `json:"status"`
	VisibleToClients bool       `json:"visible_to_clients"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Title            string     `json:"title"`
	InheritsStatus   bool       `json:"inherits_status"`
	Type             string     `json:"type"`
	URL              string     `json:"url"`
	AppURL           string     `json:"app_url"`
	BookmarkURL      string     `json:"bookmark_url"`
	CompletionURL    string     `json:"completion_url,omitempty"`
	Position         int        `json:"position"`
	DueOn            string     `json:"due_on,omitempty"`
	Completed        bool       `json:"completed"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	Parent           *Parent    `json:"parent,omitempty"`
	Bucket           *Bucket    `json:"bucket,omitempty"`
	Creator          *Person    `json:"creator,omitempty"`
	Completer        *Person    `json:"completer,omitempty"`
	Assignees        []Person   `json:"assignees,omitempty"`
}

// CreateCardRequest specifies the parameters for creating a card.
type CreateCardRequest struct {
	// Title is the card title (required).
	Title string `json:"title"`
	// Content is the card body in HTML (optional).
	Content string `json:"content,omitempty"`
	// DueOn is the due date in ISO 8601 format (optional).
	DueOn string `json:"due_on,omitempty"`
	// Notify when true, will notify assignees (optional).
	Notify bool `json:"notify,omitempty"`
}

// UpdateCardRequest specifies the parameters for updating a card.
//
// Every field is presence-bearing: a nil pointer means "leave this alone", so
// Update can tell "the caller did not mention the due date" from "the caller
// wants the due date cleared". That distinction is what makes Update
// merge-safe — see its doc comment for why BC3 forces the issue.
type UpdateCardRequest struct {
	// Title is the card title. Nil leaves it unchanged.
	Title *string `json:"title,omitempty"`
	// Content is the card body in HTML. Nil leaves it unchanged; a pointer to
	// the empty string clears it.
	Content *string `json:"content,omitempty"`
	// DueOn is the due date in YYYY-MM-DD form. Nil leaves the existing due
	// date in place; a pointer to the empty string clears it; a pointer to a
	// date sets it.
	DueOn *string `json:"due_on,omitempty"`
	// AssigneeIDs is a list of person IDs to assign this card to. Nil leaves
	// the assignees unchanged; an empty non-nil slice clears them.
	//
	// Note that BC3 filters incoming assignee IDs through reachable_people, so
	// echoing back an id belonging to someone who has since lost board access
	// silently unassigns them. Update therefore never resends assignees the
	// caller did not ask for.
	AssigneeIDs []int64 `json:"assignee_ids,omitempty"`
}

// MoveCardRequest specifies the parameters for moving a card.
type MoveCardRequest struct {
	// ColumnID is the destination column ID (required).
	ColumnID int64 `json:"column_id"`
}

// CardListOptions specifies options for listing cards.
type CardListOptions struct {
	// Limit is the maximum number of cards to return.
	// If 0 (default), returns all cards. Use a positive value to cap results.
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination:
	// exactly one request, no Link rel="next" follow (SPEC §8). A positive
	// Limit still trims that page; the per-operation default limit does not
	// apply to it. Use 0 to paginate through all results up to Limit.
	Page int
}

// CardListResult contains the results from listing cards.
type CardListResult struct {
	// Cards is the list of cards returned.
	Cards []Card
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// CreateColumnRequest specifies the parameters for creating a column.
type CreateColumnRequest struct {
	// Title is the column title (required).
	Title string `json:"title"`
	// Description is the column description (optional).
	Description string `json:"description,omitempty"`
}

// UpdateColumnRequest specifies the parameters for updating a column.
type UpdateColumnRequest struct {
	// Title is the column title (optional).
	Title string `json:"title,omitempty"`
	// Description is the column description (optional).
	Description string `json:"description,omitempty"`
}

// MoveColumnRequest specifies the parameters for moving a column.
type MoveColumnRequest struct {
	// SourceID is the column ID to move (required).
	SourceID int64 `json:"source_id"`
	// TargetID is the column ID to move relative to (required).
	TargetID int64 `json:"target_id"`
	// Position is the zero-indexed position within the destination column.
	// BC3 documents it as a required parameter and its own example sends 0,
	// so it is always transmitted — including the zero value.
	Position int `json:"position"`
}

// SetColumnColorRequest specifies the parameters for changing a column color.
type SetColumnColorRequest struct {
	// Color is the column color. Valid values: white, red, orange, yellow,
	// green, blue, aqua, purple, gray, pink, brown (required).
	Color string `json:"color"`
}

// CreateStepRequest specifies the parameters for creating a step.
type CreateStepRequest struct {
	// Title is the step title (required).
	Title string `json:"title"`
	// DueOn is the due date in ISO 8601 format (optional).
	DueOn string `json:"due_on,omitempty"`
	// AssigneeIDs is a list of person IDs to assign this step to (optional).
	AssigneeIDs []int64 `json:"assignee_ids,omitempty"`
}

// UpdateStepRequest specifies the parameters for updating a step.
//
// DueOn is presence-bearing, matching UpdateCardRequest: nil leaves the due
// date alone, a pointer to the empty string clears it, and a pointer to a date
// sets it. BC3 became presence-aware on step updates in basecamp/bc3#12521 —
// before that an omitted due_on was indistinguishable from a clear on the
// wire, so the pointer would have bought nothing (basecamp/basecamp-cli#604).
// Use Ptr to build one: Ptr(""), Ptr("2026-08-14").
type UpdateStepRequest struct {
	// Title is the step title. Empty leaves it unchanged — BC3 made title
	// optional on update in basecamp/bc3#12521.
	Title string `json:"title,omitempty"`
	// DueOn is the due date in ISO 8601 format. Nil leaves it unchanged; a
	// pointer to "" clears it.
	DueOn *string `json:"due_on,omitempty"`
	// AssigneeIDs is a list of person IDs to assign this step to. Nil leaves
	// assignees unchanged; a non-nil empty slice removes everyone.
	AssigneeIDs []int64 `json:"assignee_ids,omitempty"`
}

// CardTablesService handles card table operations.
type CardTablesService struct {
	client *AccountClient
}

// NewCardTablesService creates a new CardTablesService.
func NewCardTablesService(client *AccountClient) *CardTablesService {
	return &CardTablesService{client: client}
}

// Get returns a card table by ID.
func (s *CardTablesService) Get(ctx context.Context, cardTableID int64) (result *CardTable, err error) {
	op := OperationInfo{
		Service: "CardTables", Operation: "Get",
		ResourceType: "card_table", IsMutation: false,
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

	resp, err := s.client.parent.gen.GetCardTableWithResponse(ctx, s.client.accountID, cardTableID)
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

	cardTable := cardTableFromGenerated(*resp.JSON200)
	return &cardTable, nil
}

// CardsService handles card operations.
type CardsService struct {
	client *AccountClient
}

// NewCardsService creates a new CardsService.
func NewCardsService(client *AccountClient) *CardsService {
	return &CardsService{client: client}
}

// List returns all cards in a column.
//
// By default, returns all cards (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of cards to return (0 = all)
//   - Page: if positive, fetches only that page and disables auto-pagination
//
// The returned CardListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *CardsService) List(ctx context.Context, columnID int64, opts *CardListOptions) (result *CardListResult, err error) {
	op := OperationInfo{
		Service: "Cards", Operation: "List",
		ResourceType: "card", IsMutation: false,
		ResourceID: columnID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	var params *generated.ListCardsParams
	if opts != nil && opts.Page > 0 {
		var page *int32
		if page, err = pageParam(opts.Page); err != nil {
			return nil, err
		}
		params = &generated.ListCardsParams{Page: page}
	}

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListCardsWithResponse(ctx, s.client.accountID, columnID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var cards []Card
	if resp.JSON200 != nil {
		for _, gc := range *resp.JSON200 {
			cards = append(cards, cardFromGenerated(gc))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		keep, truncated := pageCap(len(cards), opts.Limit, resp.HTTPResponse)
		return &CardListResult{Cards: cards[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
	}

	// Determine limit: 0 = all (default for cards), >0 = specific limit
	limit := 0 // default to all for cards (per-column, typically small)
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(cards) >= limit {
		return &CardListResult{Cards: cards[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(cards), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(cards), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gc generated.Card
		if err := json.Unmarshal(raw, &gc); err != nil {
			return nil, fmt.Errorf("failed to parse card: %w", err)
		}
		cards = append(cards, cardFromGenerated(gc))
	}

	return &CardListResult{Cards: cards, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Get returns a card by ID.
func (s *CardsService) Get(ctx context.Context, cardID int64) (result *Card, err error) {
	op := OperationInfo{
		Service: "Cards", Operation: "Get",
		ResourceType: "card", IsMutation: false,
		ResourceID: cardID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetCardWithResponse(ctx, s.client.accountID, cardID)
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

	card := cardFromGenerated(*resp.JSON200)
	return &card, nil
}

// Create creates a new card in a column.
// Returns the created card.
func (s *CardsService) Create(ctx context.Context, columnID int64, req *CreateCardRequest) (result *Card, err error) {
	op := OperationInfo{
		Service: "Cards", Operation: "Create",
		ResourceType: "card", IsMutation: true,
		ResourceID: columnID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.Title == "" {
		err = ErrUsage("card title is required")
		return nil, err
	}

	body := generated.CreateCardJSONRequestBody{
		Title: req.Title,
	}
	if req.Content != "" {
		body.Content = &req.Content
	}
	if req.DueOn != "" {
		d, parseErr := types.ParseDate(req.DueOn)
		if parseErr != nil {
			err = ErrUsage("card due_on must be in YYYY-MM-DD format")
			return nil, err
		}
		body.DueOn = &d
	}
	if req.Notify {
		body.Notify = &req.Notify
	}

	resp, err := s.client.parent.gen.CreateCardWithResponse(ctx, s.client.accountID, columnID, body)
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

	card := cardFromGenerated(*resp.JSON201)
	return &card, nil
}

// Update updates a card without disturbing fields the caller did not mention.
//
// BC3 is presence-aware on the JSON representation (basecamp/bc3#12521): an
// omitted key is left unchanged, so sending only what the caller addressed is
// already the merge-safe thing to do. Update is therefore a single PUT.
//
// It costs one request and has no read-modify-write race. Earlier releases
// fetched the card first and resent the existing due date, because BC3 built
// its update params as `{ due_on: nil }.merge(card_params)` and any body
// omitting due_on erased the date. That default is gone for JSON callers, so
// the preservation GET is gone with it.
//
// To clear a due date, set DueOn to a pointer to the empty string; leaving it
// nil means "leave the due date alone". Update deliberately does not resend
// anything the caller did not set: BC3 filters assignee IDs through
// reachable_people, so echoing back assignees would unassign anyone who has
// since lost board access.
func (s *CardsService) Update(ctx context.Context, cardID int64, req *UpdateCardRequest) (result *Card, err error) {
	return s.UpdateVerbatim(ctx, cardID, req)
}

// UpdateVerbatim sends exactly the fields the caller set, in a single PUT.
//
// This is the raw API behaviour. Since BC3 became presence-aware
// (basecamp/bc3#12521) it is also what Update does — an omitted key is left
// unchanged server-side, so there is nothing left for a composite to defend
// against. The two are kept distinct because the names are load-bearing in the
// generated surface, not because they differ.
//
// Set DueOn to a pointer to the empty string to clear the date deliberately;
// that is spelled `"due_on": ""` on the wire.
func (s *CardsService) UpdateVerbatim(ctx context.Context, cardID int64, req *UpdateCardRequest) (result *Card, err error) {
	op := OperationInfo{
		Service: "Cards", Operation: "UpdateVerbatim",
		ResourceType: "card", IsMutation: true,
		ResourceID: cardID,
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
		err = ErrUsage("update request is required")
		return nil, err
	}

	body := map[string]any{}
	if req.Title != nil {
		body["title"] = *req.Title
	}
	if req.Content != nil {
		body["content"] = *req.Content
	}
	if req.DueOn != nil {
		// A pointer to the empty string is an explicit clear, and it goes on the
		// wire AS the empty string. BC3 blank-casts "" to nil on the date
		// attribute (basecamp/bc3#12521 pins this by test), so "" clears.
		// Omission would NOT: since that change an absent due_on means "leave
		// it alone". Sending {"due_on": null} would violate the body-compaction
		// rule in SPEC section 18, and five of the six SDKs strip nulls before
		// the wire anyway, so "" is the only clear encoding every SDK can
		// express identically.
		if *req.DueOn != "" {
			if _, parseErr := types.ParseDate(*req.DueOn); parseErr != nil {
				err = ErrUsage("card due_on must be in YYYY-MM-DD format")
				return nil, err
			}
		}
		body["due_on"] = *req.DueOn
	}
	if req.AssigneeIDs != nil {
		body["assignee_ids"] = req.AssigneeIDs
	}

	bodyReader, err := marshalBody(body)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.parent.gen.UpdateCardWithBodyWithResponse(ctx, s.client.accountID, cardID, "application/json", bodyReader)
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

	card := cardFromGenerated(*resp.JSON200)
	return &card, nil
}

// MoveCardOptions specifies optional parameters for moving a card.
type MoveCardOptions struct {
	// Position is the 1-indexed position within the destination column.
	// When zero, defaults to 1 (top of column).
	Position int32 `json:"position,omitempty"`
}

// Move moves a card to a different column, optionally at a specific position.
//
// A wormhole id is a valid columnID: passing one teleports the card to the
// destination column on another card table, the only way to move a card across
// projects. Discover a card table's wormholes via CardTables().Get; manage them
// with Wormholes().
func (s *CardsService) Move(ctx context.Context, cardID, columnID int64, opts *MoveCardOptions) (err error) {
	op := OperationInfo{
		Service: "Cards", Operation: "Move",
		ResourceType: "card", IsMutation: true,
		ResourceID: cardID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	body := generated.MoveCardJSONRequestBody{
		ColumnId: columnID,
	}
	if opts != nil && opts.Position > 0 {
		body.Position = &opts.Position
	}

	resp, err := s.client.parent.gen.MoveCardWithResponse(ctx, s.client.accountID, cardID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Trash moves a card to the trash.
// Trashed cards can be recovered from the trash.
func (s *CardsService) Trash(ctx context.Context, cardID int64) (err error) {
	op := OperationInfo{
		Service: "Cards", Operation: "Trash",
		ResourceType: "card", IsMutation: true,
		ResourceID: cardID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.TrashRecordingWithResponse(ctx, s.client.accountID, cardID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// CardColumnsService handles card column operations.
type CardColumnsService struct {
	client *AccountClient
}

// NewCardColumnsService creates a new CardColumnsService.
func NewCardColumnsService(client *AccountClient) *CardColumnsService {
	return &CardColumnsService{client: client}
}

// Get returns a column by ID.
func (s *CardColumnsService) Get(ctx context.Context, columnID int64) (result *CardColumn, err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "Get",
		ResourceType: "card_column", IsMutation: false,
		ResourceID: columnID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetCardColumnWithResponse(ctx, s.client.accountID, columnID)
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

	column := cardColumnFromGenerated(*resp.JSON200)
	return &column, nil
}

// Create creates a new column in a card table.
// Returns the created column.
func (s *CardColumnsService) Create(ctx context.Context, cardTableID int64, req *CreateColumnRequest) (result *CardColumn, err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "Create",
		ResourceType: "card_column", IsMutation: true,
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

	if req == nil || req.Title == "" {
		err = ErrUsage("column title is required")
		return nil, err
	}

	body := generated.CreateCardColumnJSONRequestBody{
		Title:       req.Title,
		Description: omitzero(req.Description),
	}

	resp, err := s.client.parent.gen.CreateCardColumnWithResponse(ctx, s.client.accountID, cardTableID, body)
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

	column := cardColumnFromGenerated(*resp.JSON201)
	return &column, nil
}

// Update updates an existing column.
// Returns the updated column.
func (s *CardColumnsService) Update(ctx context.Context, columnID int64, req *UpdateColumnRequest) (result *CardColumn, err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "Update",
		ResourceType: "card_column", IsMutation: true,
		ResourceID: columnID,
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
		err = ErrUsage("update request is required")
		return nil, err
	}

	body := generated.UpdateCardColumnJSONRequestBody{}
	if req.Title != "" {
		body.Title = &req.Title
	}
	if req.Description != "" {
		body.Description = &req.Description
	}

	resp, err := s.client.parent.gen.UpdateCardColumnWithResponse(ctx, s.client.accountID, columnID, body)
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

	column := cardColumnFromGenerated(*resp.JSON200)
	return &column, nil
}

// Move moves a column within a card table.
func (s *CardColumnsService) Move(ctx context.Context, cardTableID int64, req *MoveColumnRequest) (err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "Move",
		ResourceType: "card_column", IsMutation: true,
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

	if req == nil {
		err = ErrUsage("move request is required")
		return err
	}

	// Range-checked rather than blind-converted: Position is a plain int, and
	// a value past int32 would wrap to a negative column index on the wire.
	if req.Position < 0 || req.Position > math.MaxInt32 {
		err = ErrUsage("position must be between 0 and 2147483647")
		return err
	}

	body := generated.MoveCardColumnJSONRequestBody{
		SourceId: req.SourceID,
		TargetId: req.TargetID,
		// Always sent: position 0 is the documented first slot, not "unset".
		Position: ptr(int32(req.Position)),
	}

	resp, err := s.client.parent.gen.MoveCardColumnWithResponse(ctx, s.client.accountID, cardTableID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// SetColor sets the color of a column.
// Valid colors: white, red, orange, yellow, green, blue, aqua, purple, gray, pink, brown.
// Returns the updated column.
func (s *CardColumnsService) SetColor(ctx context.Context, bucketID, columnID int64, color string) (result *CardColumn, err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "SetColor",
		ResourceType: "card_column", IsMutation: true,
		ProjectID:  bucketID,
		ResourceID: columnID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if color == "" {
		err = ErrUsage("color is required")
		return nil, err
	}

	body := generated.SetCardColumnColorJSONRequestBody{
		Color: color,
	}

	resp, err := s.client.parent.gen.SetCardColumnColorWithResponse(ctx, s.client.accountID, bucketID, columnID, body)
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

	column := cardColumnFromGenerated(*resp.JSON200)
	return &column, nil
}

// EnableOnHold adds an on-hold section to a column.
// Returns the updated column.
func (s *CardColumnsService) EnableOnHold(ctx context.Context, bucketID, columnID int64) (result *CardColumn, err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "EnableOnHold",
		ResourceType: "card_column", IsMutation: true,
		ProjectID:  bucketID,
		ResourceID: columnID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.EnableCardColumnOnHoldWithResponse(ctx, s.client.accountID, bucketID, columnID)
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

	column := cardColumnFromGenerated(*resp.JSON200)
	return &column, nil
}

// DisableOnHold removes the on-hold section from a column.
// Returns the updated column.
func (s *CardColumnsService) DisableOnHold(ctx context.Context, bucketID, columnID int64) (result *CardColumn, err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "DisableOnHold",
		ResourceType: "card_column", IsMutation: true,
		ProjectID:  bucketID,
		ResourceID: columnID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DisableCardColumnOnHoldWithResponse(ctx, s.client.accountID, bucketID, columnID)
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

	column := cardColumnFromGenerated(*resp.JSON200)
	return &column, nil
}

// Watch subscribes the current user to the column.
// Returns the updated subscription information.
func (s *CardColumnsService) Watch(ctx context.Context, columnID int64) (result *Subscription, err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "Watch",
		ResourceType: "card_column", IsMutation: true,
		ResourceID: columnID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.SubscribeWithResponse(ctx, s.client.accountID, columnID)
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

	sub := subscriptionFromGenerated(*resp.JSON200)
	return &sub, nil
}

// Unwatch unsubscribes the current user from the column.
// Returns nil on success (204 No Content).
func (s *CardColumnsService) Unwatch(ctx context.Context, columnID int64) (err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "Unwatch",
		ResourceType: "card_column", IsMutation: true,
		ResourceID: columnID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.UnsubscribeWithResponse(ctx, s.client.accountID, columnID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Subscribe subscribes the current user to the column, watching it for changes.
//
// Subscribe uses the card-table-specific subscription endpoint
// (card_tables/lists/{columnID}/subscription), which returns no content.
// Watch is the same action through the generic recording subscription
// endpoint and returns the resulting subscription details.
// Returns nil on success (204 No Content).
func (s *CardColumnsService) Subscribe(ctx context.Context, columnID int64) (err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "Subscribe",
		ResourceType: "card_column", IsMutation: true,
		ResourceID: columnID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.SubscribeToCardColumnWithResponse(ctx, s.client.accountID, columnID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Unsubscribe unsubscribes the current user from the column, no longer
// watching it for changes.
//
// Unsubscribe uses the card-table-specific subscription endpoint
// (card_tables/lists/{columnID}/subscription); Unwatch is the same action
// through the generic recording subscription endpoint.
// Returns nil on success (204 No Content).
func (s *CardColumnsService) Unsubscribe(ctx context.Context, columnID int64) (err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "Unsubscribe",
		ResourceType: "card_column", IsMutation: true,
		ResourceID: columnID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.UnsubscribeFromCardColumnWithResponse(ctx, s.client.accountID, columnID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// CardStepsService handles card step operations.
type CardStepsService struct {
	client *AccountClient
}

// NewCardStepsService creates a new CardStepsService.
func NewCardStepsService(client *AccountClient) *CardStepsService {
	return &CardStepsService{client: client}
}

// Get retrieves a card step by ID.
func (s *CardStepsService) Get(ctx context.Context, stepID int64) (result *CardStep, err error) {
	op := OperationInfo{
		Service: "CardSteps", Operation: "Get",
		ResourceType: "card_step", IsMutation: false,
		ResourceID: stepID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetCardStepWithResponse(ctx, s.client.accountID, stepID)
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

	step := cardStepFromGenerated(*resp.JSON200)
	return &step, nil
}

// Create creates a new step on a card.
// Returns the created step.
func (s *CardStepsService) Create(ctx context.Context, cardID int64, req *CreateStepRequest) (result *CardStep, err error) {
	op := OperationInfo{
		Service: "CardSteps", Operation: "Create",
		ResourceType: "card_step", IsMutation: true,
		ResourceID: cardID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.Title == "" {
		err = ErrUsage("step title is required")
		return nil, err
	}

	body := generated.CreateCardStepJSONRequestBody{
		Title: req.Title,
	}
	// nil means "not addressed" (omitted); a non-nil empty slice is an explicit
	// empty assignee list and must reach the wire.
	if req.AssigneeIDs != nil {
		body.AssigneeIds = &req.AssigneeIDs
	}
	if req.DueOn != "" {
		d, parseErr := types.ParseDate(req.DueOn)
		if parseErr != nil {
			err = ErrUsage("step due_on must be in YYYY-MM-DD format")
			return nil, err
		}
		body.DueOn = &d
	}

	resp, err := s.client.parent.gen.CreateCardStepWithResponse(ctx, s.client.accountID, cardID, body)
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

	step := cardStepFromGenerated(*resp.JSON201)
	return &step, nil
}

// Update updates an existing step.
// Returns the updated step.
func (s *CardStepsService) Update(ctx context.Context, stepID int64, req *UpdateStepRequest) (result *CardStep, err error) {
	op := OperationInfo{
		Service: "CardSteps", Operation: "Update",
		ResourceType: "card_step", IsMutation: true,
		ResourceID: stepID,
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
		err = ErrUsage("update request is required")
		return nil, err
	}

	body := map[string]any{}
	if req.Title != "" {
		body["title"] = req.Title
	}
	if req.AssigneeIDs != nil {
		body["assignee_ids"] = req.AssigneeIDs
	}
	if req.DueOn != nil {
		// Same encoding as cards: a pointer to the empty string is an explicit
		// clear and goes on the wire as "", which BC3 blank-casts to nil.
		// Omission means "leave it alone" since basecamp/bc3#12521.
		if *req.DueOn != "" {
			if _, parseErr := types.ParseDate(*req.DueOn); parseErr != nil {
				err = ErrUsage("step due_on must be in YYYY-MM-DD format")
				return nil, err
			}
		}
		body["due_on"] = *req.DueOn
	}

	bodyReader, err := marshalBody(body)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.parent.gen.UpdateCardStepWithBodyWithResponse(ctx, s.client.accountID, stepID, "application/json", bodyReader)
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

	step := cardStepFromGenerated(*resp.JSON200)
	return &step, nil
}

// Complete marks a step as completed.
// Returns the updated step.
func (s *CardStepsService) Complete(ctx context.Context, stepID int64) (result *CardStep, err error) {
	op := OperationInfo{
		Service: "CardSteps", Operation: "Complete",
		ResourceType: "card_step", IsMutation: true,
		ResourceID: stepID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	body := generated.SetCardStepCompletionJSONRequestBody{Completion: "on"}
	resp, err := s.client.parent.gen.SetCardStepCompletionWithResponse(ctx, s.client.accountID, stepID, body)
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

	step := cardStepFromGenerated(*resp.JSON200)
	return &step, nil
}

// Uncomplete marks a step as incomplete.
// Returns the updated step.
func (s *CardStepsService) Uncomplete(ctx context.Context, stepID int64) (result *CardStep, err error) {
	op := OperationInfo{
		Service: "CardSteps", Operation: "Uncomplete",
		ResourceType: "card_step", IsMutation: true,
		ResourceID: stepID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	body := generated.SetCardStepCompletionJSONRequestBody{Completion: ""}
	resp, err := s.client.parent.gen.SetCardStepCompletionWithResponse(ctx, s.client.accountID, stepID, body)
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

	step := cardStepFromGenerated(*resp.JSON200)
	return &step, nil
}

// Reposition changes the position of a step within a card.
// position is 0-indexed.
func (s *CardStepsService) Reposition(ctx context.Context, cardID, stepID int64, position int) (err error) {
	op := OperationInfo{
		Service: "CardSteps", Operation: "Reposition",
		ResourceType: "card_step", IsMutation: true,
		ResourceID: stepID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if position < 0 {
		err = ErrUsage("position must be at least 0")
		return err
	}

	body := generated.RepositionCardStepJSONRequestBody{
		SourceId: stepID,
		Position: int32(position), // #nosec G115 -- position is validated and bounded by API
	}

	resp, err := s.client.parent.gen.RepositionCardStepWithResponse(ctx, s.client.accountID, cardID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Delete deletes a step (moves it to trash).
func (s *CardStepsService) Delete(ctx context.Context, stepID int64) (err error) {
	op := OperationInfo{
		Service: "CardSteps", Operation: "Delete",
		ResourceType: "card_step", IsMutation: true,
		ResourceID: stepID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.TrashRecordingWithResponse(ctx, s.client.accountID, stepID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// cardTableFromGenerated converts a generated CardTable to our clean CardTable type.
func cardTableFromGenerated(gc generated.CardTable) CardTable {
	ct := CardTable{
		Status:           gc.Status,
		VisibleToClients: gc.VisibleToClients,
		Title:            gc.Title,
		InheritsStatus:   gc.InheritsStatus,
		Type:             gc.Type,
		URL:              gc.Url,
		AppURL:           gc.AppUrl,
		BookmarkURL:      deref(gc.BookmarkUrl),
		SubscriptionURL:  deref(gc.SubscriptionUrl),
		CreatedAt:        gc.CreatedAt,
		UpdatedAt:        gc.UpdatedAt,
	}

	if gc.Id != 0 {
		ct.ID = gc.Id
	}

	if gc.Bucket.Id != 0 || gc.Bucket.Name != "" {
		ct.Bucket = &Bucket{
			ID:   gc.Bucket.Id,
			Name: gc.Bucket.Name,
			Type: gc.Bucket.Type,
		}
	}

	if gc.Creator.Id != 0 || gc.Creator.Name != "" {
		creator := personFromGenerated(gc.Creator)
		ct.Creator = &creator
	}

	if len(gc.Subscribers) > 0 {
		ct.Subscribers = make([]Person, 0, len(gc.Subscribers))
		for _, gs := range gc.Subscribers {
			ct.Subscribers = append(ct.Subscribers, personFromGenerated(gs))
		}
	}

	if len(gc.Lists) > 0 {
		ct.Lists = make([]CardColumn, 0, len(gc.Lists))
		for _, gl := range gc.Lists {
			ct.Lists = append(ct.Lists, cardColumnFromGenerated(gl))
		}
	}

	if len(gc.Wormholes) > 0 {
		ct.Wormholes = make([]Wormhole, 0, len(gc.Wormholes))
		for _, gw := range gc.Wormholes {
			ct.Wormholes = append(ct.Wormholes, wormholeFromGenerated(gw))
		}
	}

	return ct
}

// cardColumnFromGenerated converts a generated CardColumn to our clean CardColumn type.
func cardColumnFromGenerated(gc generated.CardColumn) CardColumn {
	cc := CardColumn{
		Status:           gc.Status,
		VisibleToClients: gc.VisibleToClients,
		Title:            gc.Title,
		InheritsStatus:   gc.InheritsStatus,
		Type:             gc.Type,
		URL:              gc.Url,
		AppURL:           gc.AppUrl,
		BookmarkURL:      deref(gc.BookmarkUrl),
		Position:         int(deref(gc.Position)),
		Color:            deref(gc.Color),
		Description:      deref(gc.Description),
		CardsCount:       int(deref(gc.CardsCount)),
		CommentCount:     int(deref(gc.CommentsCount)),
		CommentsCount:    int(deref(gc.CommentsCount)),
		CardsURL:         deref(gc.CardsUrl),
		CreatedAt:        gc.CreatedAt,
		UpdatedAt:        gc.UpdatedAt,
	}

	if gc.Id != 0 {
		cc.ID = gc.Id
	}

	if gc.Parent.Id != 0 || gc.Parent.Title != "" {
		cc.Parent = &Parent{
			ID:     gc.Parent.Id,
			Title:  gc.Parent.Title,
			Type:   gc.Parent.Type,
			URL:    gc.Parent.Url,
			AppURL: gc.Parent.AppUrl,
		}
	}

	if gc.Bucket.Id != 0 || gc.Bucket.Name != "" {
		cc.Bucket = &Bucket{
			ID:   gc.Bucket.Id,
			Name: gc.Bucket.Name,
			Type: gc.Bucket.Type,
		}
	}

	if gc.Creator.Id != 0 || gc.Creator.Name != "" {
		creator := personFromGenerated(gc.Creator)
		cc.Creator = &creator
	}

	if gc.OnHold != nil {
		cc.OnHold = &CardColumnOnHold{
			ID:             gc.OnHold.Id,
			Status:         gc.OnHold.Status,
			InheritsStatus: gc.OnHold.InheritsStatus,
			Title:          gc.OnHold.Title,
			CreatedAt:      gc.OnHold.CreatedAt,
			UpdatedAt:      gc.OnHold.UpdatedAt,
			CardsCount:     int(gc.OnHold.CardsCount),
			CardsURL:       gc.OnHold.CardsUrl,
		}
	}

	if len(gc.Subscribers) > 0 {
		cc.Subscribers = make([]Person, 0, len(gc.Subscribers))
		for _, gs := range gc.Subscribers {
			cc.Subscribers = append(cc.Subscribers, personFromGenerated(gs))
		}
	}

	return cc
}

// cardFromGenerated converts a generated Card to our clean Card type.
func cardFromGenerated(gc generated.Card) Card {
	c := Card{
		Status:           gc.Status,
		VisibleToClients: gc.VisibleToClients,
		Title:            gc.Title,
		InheritsStatus:   gc.InheritsStatus,
		Type:             gc.Type,
		URL:              gc.Url,
		AppURL:           gc.AppUrl,
		BookmarkURL:      deref(gc.BookmarkUrl),
		SubscriptionURL:  deref(gc.SubscriptionUrl),
		Position:         int(deref(gc.Position)),
		Content:          deref(gc.Content),
		Description:      deref(gc.Description),
		Completed:        deref(gc.Completed),
		CommentsCount:    int(deref(gc.CommentsCount)),
		BoostsCount:      int(deref(gc.BoostsCount)),
		BoostsURL:        deref(gc.BoostsUrl),
		CommentsURL:      deref(gc.CommentsUrl),
		CompletionURL:    deref(gc.CompletionUrl),
		CreatedAt:        gc.CreatedAt,
		UpdatedAt:        gc.UpdatedAt,
	}

	if gc.Id != 0 {
		c.ID = gc.Id
	}

	// Handle due_on - it's types.Date in generated, string in SDK
	if gc.DueOn != nil && !gc.DueOn.IsZero() {
		c.DueOn = gc.DueOn.String()
	}

	// Handle completed_at
	if gc.CompletedAt != nil {
		c.CompletedAt = gc.CompletedAt
	}

	if gc.Parent.Id != 0 || gc.Parent.Title != "" {
		c.Parent = &Parent{
			ID:     gc.Parent.Id,
			Title:  gc.Parent.Title,
			Type:   gc.Parent.Type,
			URL:    gc.Parent.Url,
			AppURL: gc.Parent.AppUrl,
		}
	}

	if gc.Bucket.Id != 0 || gc.Bucket.Name != "" {
		c.Bucket = &Bucket{
			ID:   gc.Bucket.Id,
			Name: gc.Bucket.Name,
			Type: gc.Bucket.Type,
		}
	}

	if gc.Creator.Id != 0 || gc.Creator.Name != "" {
		creator := personFromGenerated(gc.Creator)
		c.Creator = &creator
	}

	if gc.Completer != nil {
		completer := personFromGenerated(*gc.Completer)
		c.Completer = &completer
	}

	if len(gc.Assignees) > 0 {
		c.Assignees = make([]Person, 0, len(gc.Assignees))
		for _, ga := range gc.Assignees {
			c.Assignees = append(c.Assignees, personFromGenerated(ga))
		}
	}

	if len(gc.CompletionSubscribers) > 0 {
		c.CompletionSubscribers = make([]Person, 0, len(gc.CompletionSubscribers))
		for _, gs := range gc.CompletionSubscribers {
			c.CompletionSubscribers = append(c.CompletionSubscribers, personFromGenerated(gs))
		}
	}

	if len(gc.Steps) > 0 {
		c.Steps = make([]CardStep, 0, len(gc.Steps))
		for _, gs := range gc.Steps {
			c.Steps = append(c.Steps, cardStepFromGenerated(gs))
		}
	}

	c.DescriptionAttachments = richTextAttachmentsFromGenerated(gc.DescriptionAttachments)

	return c
}

// cardStepFromGenerated converts a generated CardStep to our clean CardStep type.
func cardStepFromGenerated(gs generated.CardStep) CardStep {
	s := CardStep{
		Status:           gs.Status,
		VisibleToClients: gs.VisibleToClients,
		Title:            gs.Title,
		InheritsStatus:   gs.InheritsStatus,
		Type:             gs.Type,
		URL:              gs.Url,
		AppURL:           gs.AppUrl,
		BookmarkURL:      deref(gs.BookmarkUrl),
		CompletionURL:    deref(gs.CompletionUrl),
		Position:         int(deref(gs.Position)),
		Completed:        deref(gs.Completed),
		CreatedAt:        gs.CreatedAt,
		UpdatedAt:        gs.UpdatedAt,
	}

	if gs.Id != 0 {
		s.ID = gs.Id
	}

	// Handle due_on - it's types.Date in generated, string in SDK
	if gs.DueOn != nil && !gs.DueOn.IsZero() {
		s.DueOn = gs.DueOn.String()
	}

	// Handle completed_at
	if gs.CompletedAt != nil {
		s.CompletedAt = gs.CompletedAt
	}

	if gs.Parent.Id != 0 || gs.Parent.Title != "" {
		s.Parent = &Parent{
			ID:     gs.Parent.Id,
			Title:  gs.Parent.Title,
			Type:   gs.Parent.Type,
			URL:    gs.Parent.Url,
			AppURL: gs.Parent.AppUrl,
		}
	}

	if gs.Bucket.Id != 0 || gs.Bucket.Name != "" {
		s.Bucket = &Bucket{
			ID:   gs.Bucket.Id,
			Name: gs.Bucket.Name,
			Type: gs.Bucket.Type,
		}
	}

	if gs.Creator.Id != 0 || gs.Creator.Name != "" {
		creator := personFromGenerated(gs.Creator)
		s.Creator = &creator
	}

	if gs.Completer != nil {
		completer := personFromGenerated(*gs.Completer)
		s.Completer = &completer
	}

	if len(gs.Assignees) > 0 {
		s.Assignees = make([]Person, 0, len(gs.Assignees))
		for _, ga := range gs.Assignees {
			s.Assignees = append(s.Assignees, personFromGenerated(ga))
		}
	}

	return s
}

// personFromGenerated is defined in people.go

// subscriptionFromGenerated is defined in subscriptions.go
