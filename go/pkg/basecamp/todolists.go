package basecamp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Note: Todolists default to fetching all (no limit) since they are structural
// indices, not high-volume content. Use Limit to cap results if needed.

// Todolist represents a Basecamp todolist — or a group inside one. There is
// only this type: BC3 has no group model, so a group is a Todolist whose
// parent is a Todolist, rendered through the same jbuilder and reporting
// Type "Todolist".
//
// Discriminate structurally, never on Type:
//
//   - GroupsURL non-empty  → a to-do list; Parent is a Todoset.
//   - GroupPositionURL non-empty → a group; Parent is a Todolist.
//
// Exactly one of the two is present on any response.
type Todolist struct {
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
	BoostsCount      int       `json:"boosts_count,omitempty"`
	BoostsURL        string    `json:"boosts_url,omitempty"`
	SubscriptionURL  string    `json:"subscription_url"`
	// BubbleUpURL is the URL of the Bubble Up record for this recording. Always
	// present: todolists/_todolist.json.jbuilder renders the shared recording
	// partial with bubbleupable: true unconditionally, and every list, show, and
	// group path renders that partial.
	BubbleUpURL   string  `json:"bubble_up_url"`
	CommentsCount int     `json:"comments_count"`
	CommentsURL   string  `json:"comments_url"`
	Position      int     `json:"position"`
	Parent        *Parent `json:"parent,omitempty"`
	Bucket        *Bucket `json:"bucket,omitempty"`
	Creator       *Person `json:"creator,omitempty"`
	Description   string  `json:"description"`
	// DescriptionAttachments holds structured metadata for the downloadable files
	// embedded in the rich text Description. @required — the API always sends this
	// array (empty when the description has no inline files). No omitempty, so on
	// marshal a non-nil slice emits its elements ([] when empty) and a nil
	// slice emits null; the key is never
	// dropped. Decode distinguishes a server-sent [] (non-nil) from nil. See
	// RichTextAttachment.
	DescriptionAttachments []RichTextAttachment `json:"description_attachments"`
	Completed              bool                 `json:"completed"`
	CompletedRatio         string               `json:"completed_ratio"`
	Name                   string               `json:"name"`
	TodosURL               string               `json:"todos_url"`
	// GroupsURL is the API URL for this list's groups. Present only when Parent
	// is a Todoset — i.e. this is a to-do list, not a group. Mutually exclusive
	// with GroupPositionURL.
	GroupsURL string `json:"groups_url"`
	// GroupPositionURL is the API URL for repositioning this group within its
	// parent list. Present only when Parent is a Todolist — i.e. this is a
	// group. Mutually exclusive with GroupsURL.
	GroupPositionURL string `json:"group_position_url"`
	// AppTodosURL is the in-app (non-API) URL for this list's todos, alongside
	// the API-host TodosURL.
	AppTodosURL string `json:"app_todos_url"`
	// Color is one of BC3's recording colors (white, red, orange, yellow,
	// green, blue, aqua, purple, gray, pink, brown). The key is always emitted
	// but its value is null when unset, which decodes to "".
	Color string `json:"color"`
	// CommentsAppURL is the in-app (non-API) URL for this recording's comments,
	// alongside the API-host CommentsURL.
	CommentsAppURL string `json:"comments_app_url"`
}

// TodolistListOptions specifies options for listing todolists.
type TodolistListOptions struct {
	// Status filters by status: "archived" or "trashed".
	// Empty returns active todolists.
	Status string

	// Limit is the maximum number of todolists to return.
	// If 0 (default), returns all todolists. Use a positive value to cap results.
	Limit int

	// Page, if positive, fetches only that page and disables auto-pagination:
	// exactly one request, no Link rel="next" follow (SPEC §8). A positive
	// Limit still trims that page; the per-operation default limit does not
	// apply to it. Use 0 to paginate through all results up to Limit.
	Page int
}

// TodolistListResult contains the results from listing todolists.
type TodolistListResult struct {
	// Todolists is the list of todolists returned.
	Todolists []Todolist
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// CreateTodolistRequest specifies the parameters for creating a todolist.
type CreateTodolistRequest struct {
	// Name is the todolist name (required).
	Name string `json:"name"`
	// Description is an optional description (can include HTML).
	Description string `json:"description,omitempty"`
	// VisibleToClients sets client visibility at create time (optional, tri-state).
	// nil omits the field so the server applies its own default visibility rule; a
	// non-nil value is sent verbatim, and an explicit false reaches the wire (the
	// pointer distinguishes unset from false).
	VisibleToClients *bool `json:"visible_to_clients,omitempty"`
}

// UpdateTodolistRequest specifies the fields to set when updating a todolist.
// Zero-value fields are left untouched (see TodolistsService.Update).
type UpdateTodolistRequest struct {
	// Name is the todolist name.
	Name string `json:"name,omitempty"`
	// Description is an optional description (can include HTML).
	Description string `json:"description,omitempty"`
}

// ReplaceTodolistRequest specifies the new complete representation of a
// todolist for TodolistsService.Replace. Omitted fields are cleared
// server-side: BC3's TodolistsController#update rebuilds the recordable from
// only the permitted params, so a missing description erases the description.
type ReplaceTodolistRequest struct {
	// Name is the todolist name (required). It is always sent — the server
	// presence-validates it, so omitting it is a 422, not a preserve.
	Name string `json:"name"`
	// Description is an optional description (can include HTML). Omitting it
	// clears the description.
	Description string `json:"description,omitempty"`
}

// TodolistFields holds a todolist's full writable state for
// TodolistsService.Update and TodolistsService.Edit. The whole struct is PUT
// back to the server, so clearing a field means setting it empty ("") —
// there is no third state. The writable set is exactly {name, description}.
type TodolistFields struct {
	// Name is the todolist name (required; the server rejects an empty one).
	Name string
	// Description is a description (can include HTML). "" clears it.
	Description string
}

// fieldsFromTodolist derives the full writable state from a fetched todolist.
//
// Classification is by origin, not by value. The same empty name is a caller
// error when the caller set it and a malformed response when it came off the
// wire, so each provenance is checked where it is unambiguous: here for the
// response, fullBody for the caller. Name is presence-validated server-side, so
// a todolist that comes back without one is malformed — not an empty value to
// preserve, and emphatically not something to write back over the real name on
// a full-replace endpoint.
//
// body is the raw GET payload the decoded todolist came from, and it is a
// parameter rather than an afterthought so that no caller can lift writable
// state without it: the description guard below is unimplementable from the
// struct alone. See requireDescription.
func fieldsFromTodolist(tl *Todolist, body []byte) (*TodolistFields, error) {
	if tl.Name == "" {
		// Structured, and statusless by SPEC §6: the transport succeeded, so no
		// HTTP status describes this, and re-requesting cannot repair a
		// malformed body. A bare wrapped error would give callers nothing to
		// branch on and would not carry the hint.
		return nil, &Error{
			Code:    CodeAPI,
			Message: fmt.Sprintf("todolist %d came back with an empty name", tl.ID),
			Hint:    "The name is presence-validated server-side, so this is a malformed response, not a value to preserve. Use Replace to write the record deliberately.",
		}
	}
	if err := requireDescription(body, tl.ID); err != nil {
		return nil, err
	}
	return &TodolistFields{
		Name:        tl.Name,
		Description: tl.Description,
	}, nil
}

// requireDescription refuses a response whose description key is absent or
// null, before the merge-safe write reads a "" that was never there.
//
// Presence and non-emptiness are two different claims. Since #544 description
// is @required and never null — BC3's format_api_content funnels a blank rich
// text through call_pipeline, which returns "" rather than nil — so an absent
// key and an explicit null are both malformed, while a present "" is the
// ordinary state of a description-less list and must round-trip untouched.
//
// Go cannot make that distinction from the decoded value. generated.Todolist
// carries Description as a plain string, so absent, null and a real "" all
// unmarshal to "" and are indistinguishable at that layer — Swift's Codable
// and kotlinx.serialization reject the missing member during decoding, and
// encoding/json does not. The distinction survives only in the raw response
// bytes, which the GET already has in hand (GetTodolistOrGroupResponse.Body)
// and threads through fieldsFromTodolist. This second decode is therefore not
// a redundant one: it reads presence, which the first decode discarded.
//
// The failure is api_error and statusless for the same reason the empty-name
// one is: the transport succeeded and nothing the caller passed is at fault.
func requireDescription(body []byte, id int64) error {
	malformed := func(what string) error {
		return &Error{
			Code:    CodeAPI,
			Message: fmt.Sprintf("todolist %d came back %s", id, what),
			Hint:    "The description is required and never null on the wire, so this is a malformed response, not an empty value to preserve. Update and Edit PUT the full writable state back, so reading it as empty would erase the real description. Use Replace to write the record deliberately.",
		}
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return malformed("with a body that is not a JSON object")
	}
	raw, ok := fields["description"]
	if !ok {
		return malformed("without a description")
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return malformed("with a null description")
	}
	return nil
}

// fullBody serializes the complete writable state for the replace transport:
// name and description are always sent (the empty description included, so
// clears survive the PUT — "" is how a clear is expressed, never JSON null).
func (f *TodolistFields) fullBody() (generated.UpdateTodolistOrGroupJSONRequestBody, error) {
	if f.Name == "" {
		return generated.UpdateTodolistOrGroupJSONRequestBody{}, ErrUsage("todolist name is required")
	}
	return generated.UpdateTodolistOrGroupJSONRequestBody{
		Name:        f.Name,
		Description: &f.Description,
	}, nil
}

// TodolistsService handles todolist operations.
type TodolistsService struct {
	client *AccountClient
}

// NewTodolistsService creates a new TodolistsService.
func NewTodolistsService(client *AccountClient) *TodolistsService {
	return &TodolistsService{client: client}
}

// List returns todolists in a todoset.
//
// By default, returns all todolists (no limit). Use Limit to cap results.
//
// Pagination options:
//   - Limit: maximum number of todolists to return (0 = all)
//   - Page: if positive, fetches only that page and disables auto-pagination
//
// The returned TodolistListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *TodolistsService) List(ctx context.Context, todosetID int64, opts *TodolistListOptions) (result *TodolistListResult, err error) {
	op := OperationInfo{
		Service: "Todolists", Operation: "List",
		ResourceType: "todolist", IsMutation: false,
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

	// Build params for generated client
	params := &generated.ListTodolistsParams{}
	if opts != nil {
		if opts.Status != "" {
			params.Status = &opts.Status
		}
		if opts.Page > 0 {
			var page *int32
			if page, err = pageParam(opts.Page); err != nil {
				return nil, err
			}
			params.Page = page
		}
	}

	// Call generated client for first page (spec-conformant - no manual path construction)
	resp, err := s.client.parent.gen.ListTodolistsWithResponse(ctx, s.client.accountID, todosetID, params)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, err
	}

	// Capture total count from X-Total-Count header (first page only)
	totalCount := parseTotalCount(resp.HTTPResponse)

	// Parse first page
	var todolists []Todolist
	if resp.JSON200 != nil {
		for _, gtl := range *resp.JSON200 {
			todolists = append(todolists, todolistFromGenerated(gtl))
		}
	}

	// Handle single page fetch (--page flag)
	if opts != nil && opts.Page > 0 {
		keep, truncated := pageCap(len(todolists), opts.Limit, resp.HTTPResponse)
		return &TodolistListResult{Todolists: todolists[:keep], Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
	}

	// Determine limit: 0 = all (default for todolists), >0 = specific limit
	limit := 0 // default to all for todolists (structural index, not high-volume)
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}

	// Check if we already have enough items
	if limit > 0 && len(todolists) >= limit {
		return &TodolistListResult{Todolists: todolists[:limit], Meta: ListMeta{TotalCount: totalCount, Truncated: isFirstPageTruncated(resp.HTTPResponse, len(todolists), limit)}}, nil
	}

	// Follow pagination via Link headers (uses absolute URLs from API, no path construction)
	rawMore, truncated, err := s.client.parent.followPagination(ctx, resp.HTTPResponse, len(todolists), limit)
	if err != nil {
		return nil, err
	}

	// Parse additional pages
	for _, raw := range rawMore {
		var gtl generated.Todolist
		if err := json.Unmarshal(raw, &gtl); err != nil {
			return nil, fmt.Errorf("failed to parse todolist: %w", err)
		}
		todolists = append(todolists, todolistFromGenerated(gtl))
	}

	return &TodolistListResult{Todolists: todolists, Meta: ListMeta{TotalCount: totalCount, Truncated: truncated}}, nil
}

// Get returns a todolist by ID.
func (s *TodolistsService) Get(ctx context.Context, todolistID int64) (*Todolist, error) {
	todolist, _, err := s.getWithBody(ctx, todolistID)
	return todolist, err
}

// getWithBody is Get, plus the raw response payload the todolist decoded from.
//
// Update and Edit need both: the decoded struct for the values and the bytes
// for what the struct cannot express. Since #544 generated.Todolist declares
// Description as a plain string, so an absent key, an explicit null and a real
// "" all arrive as "" — see requireDescription. Get itself drops the bytes,
// keeping the public read surface unchanged; nothing else about the request
// differs, so hooks observe one Todolists.Get either way.
func (s *TodolistsService) getWithBody(ctx context.Context, todolistID int64) (result *Todolist, body []byte, err error) {
	op := OperationInfo{
		Service: "Todolists", Operation: "Get",
		ResourceType: "todolist", IsMutation: false,
		ResourceID: todolistID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetTodolistOrGroupWithResponse(ctx, s.client.accountID, todolistID)
	if err != nil {
		return nil, nil, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return nil, nil, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, nil, err
	}

	// One flat shape (#544): GetTodolistOrGroupResponseContent is generated.Todolist
	// itself, so the generated parser's decode is the decode — the bytes returned
	// alongside it are not a second decode of the values but the only remaining
	// record of which keys the server actually sent. A group answers here too and
	// lands in the same struct.
	todolist := todolistFromGenerated(*resp.JSON200)
	return &todolist, resp.Body, nil
}

// Create creates a new todolist in a todoset.
// Returns the created todolist.
func (s *TodolistsService) Create(ctx context.Context, todosetID int64, req *CreateTodolistRequest) (result *Todolist, err error) {
	op := OperationInfo{
		Service: "Todolists", Operation: "Create",
		ResourceType: "todolist", IsMutation: true,
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

	if req.Name == "" {
		err = ErrUsage("todolist name is required")
		return nil, err
	}

	body := generated.CreateTodolistJSONRequestBody{
		Name:             req.Name,
		Description:      omitzero(req.Description),
		VisibleToClients: req.VisibleToClients,
	}

	resp, err := s.client.parent.gen.CreateTodolistWithResponse(ctx, s.client.accountID, todosetID, body)
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

	todolist := todolistFromGenerated(*resp.JSON201)
	return &todolist, nil
}

// Update sets the given fields on a todolist and preserves everything else:
// it GETs the current todolist, overlays the explicitly-set request fields,
// and PUTs the full representation back. A zero-value field is untouched,
// guaranteed. Strings cannot be cleared through Update — use Edit or Replace
// to clear.
//
// Hooks observe the two wire operations (Todolists.Get then
// Todolists.Replace), not a synthetic composite.
//
// Update is read-modify-write, not atomic: there is no conditional-update
// signal on this endpoint, so a concurrent write between the GET and PUT is
// overwritten — last write wins for the whole representation. The window is
// one round-trip. Use Replace to overwrite deliberately.
// Returns the updated todolist.
func (s *TodolistsService) Update(ctx context.Context, todolistID int64, req *UpdateTodolistRequest) (*Todolist, error) {
	if req == nil {
		return nil, ErrUsage("update request is required")
	}

	current, body, err := s.getWithBody(ctx, todolistID)
	if err != nil {
		return nil, err
	}

	fields, err := fieldsFromTodolist(current, body)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		fields.Name = req.Name
	}
	if req.Description != "" {
		fields.Description = req.Description
	}

	return s.replaceTodolist(ctx, todolistID, fields.fullBody)
}

// Edit applies a read-modify-write closure to a todolist: it GETs the current
// todolist, hands fn the full writable representation, and PUTs the whole
// thing back. Clearing a field means setting it empty ("") — an untouched
// field keeps its current value. If fn returns an error, the edit aborts and
// nothing is written.
//
// Hooks observe the two wire operations (Todolists.Get then
// Todolists.Replace), not a synthetic composite.
//
// Edit is read-modify-write, not atomic: there is no conditional-update
// signal on this endpoint, so a concurrent write between the GET and PUT is
// overwritten — last write wins for the whole representation. The window is
// one round-trip. Use Replace to overwrite deliberately.
// Returns the updated todolist.
func (s *TodolistsService) Edit(ctx context.Context, todolistID int64, fn func(*TodolistFields) error) (*Todolist, error) {
	if fn == nil {
		return nil, ErrUsage("edit function is required")
	}

	current, body, err := s.getWithBody(ctx, todolistID)
	if err != nil {
		return nil, err
	}

	fields, err := fieldsFromTodolist(current, body)
	if err != nil {
		return nil, err
	}
	if err := fn(fields); err != nil {
		return nil, err
	}

	return s.replaceTodolist(ctx, todolistID, fields.fullBody)
}

// Replace sends the request verbatim as the todolist's new complete
// representation — the server's native PUT semantics. No GET is issued, and
// any field omitted from the request is cleared server-side (a missing
// description clears it). BC3's TodolistsController#update rebuilds the
// recordable from only the permitted params, so omission is destructive by
// design. Name is required. Use Update or Edit to preserve unspecified
// fields.
// Returns the updated todolist.
func (s *TodolistsService) Replace(ctx context.Context, todolistID int64, req *ReplaceTodolistRequest) (*Todolist, error) {
	return s.replaceTodolist(ctx, todolistID, func() (generated.UpdateTodolistOrGroupJSONRequestBody, error) {
		if req == nil {
			return generated.UpdateTodolistOrGroupJSONRequestBody{}, ErrUsage("replace request is required")
		}
		if req.Name == "" {
			return generated.UpdateTodolistOrGroupJSONRequestBody{}, ErrUsage("todolist name is required")
		}
		body := generated.UpdateTodolistOrGroupJSONRequestBody{Name: req.Name}
		if req.Description != "" {
			body.Description = &req.Description
		}
		return body, nil
	})
}

// replaceTodolist is the single transport for the UpdateTodolistOrGroup wire
// operation as TodolistsService issues it, shared by Replace, Update, and
// Edit. It pins the Todolists.Replace hook identity and projects the todolist
// shape; the envelope and the one generated-client call site live in
// replaceTodolistOrGroup.
func (s *TodolistsService) replaceTodolist(ctx context.Context, todolistID int64, buildBody func() (generated.UpdateTodolistOrGroupJSONRequestBody, error)) (*Todolist, error) {
	op := OperationInfo{
		Service: "Todolists", Operation: "Replace",
		ResourceType: "todolist", IsMutation: true,
		ResourceID: todolistID,
	}

	gtl, err := replaceTodolistOrGroup(ctx, s.client, op, todolistID, buildBody)
	if err != nil {
		return nil, err
	}

	todolist := todolistFromGenerated(gtl)
	return &todolist, nil
}

// replaceTodolistOrGroup is the one call site for the polymorphic
// UpdateTodolistOrGroup wire operation (PUT /{accountId}/todolists/{id}).
// TodolistsService and TodolistGroupsService both route their writes through
// it: the endpoint accepts either shape, so only the hook identity and the
// decoding differ. It owns the hook envelope, and buildBody runs inside that
// envelope so usage errors are observable to hooks.
//
// The writable set reaches the wire verbatim: an empty description arrives
// present-and-empty (that is how a clear is expressed) through a non-nil
// pointer to "", which `omitempty` preserves — it tests pointer nil-ness,
// not pointee emptiness.
//
// Returns the decoded generated shape. Since #544 that is one flat
// generated.Todolist for both variants, so each caller only chooses which
// wrapper name to project it under.
func replaceTodolistOrGroup(ctx context.Context, client *AccountClient, op OperationInfo, id int64, buildBody func() (generated.UpdateTodolistOrGroupJSONRequestBody, error)) (gtl generated.Todolist, err error) {
	if gater, ok := client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return generated.Todolist{}, err
		}
	}
	start := time.Now()
	ctx = client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	body, err := buildBody()
	if err != nil {
		return generated.Todolist{}, err
	}

	resp, err := client.parent.gen.UpdateTodolistOrGroupWithResponse(ctx, client.accountID, id, body)
	if err != nil {
		return generated.Todolist{}, err
	}
	if err = checkResponse(resp.HTTPResponse, resp.Body); err != nil {
		return generated.Todolist{}, err
	}
	if resp.JSON200 == nil {
		err = fmt.Errorf("unexpected empty response")
		return generated.Todolist{}, err
	}

	return *resp.JSON200, nil
}

// Trash moves a todolist to the trash.
// Trashed todolists can be recovered from the trash.
func (s *TodolistsService) Trash(ctx context.Context, todolistID int64) (err error) {
	op := OperationInfo{
		Service: "Todolists", Operation: "Trash",
		ResourceType: "todolist", IsMutation: true,
		ResourceID: todolistID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.TrashRecordingWithResponse(ctx, s.client.accountID, todolistID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// Reposition moves a todolist to a new position within its todoset.
// position is the 1-based index among the todolists the caller can see; the
// server translates it relative to loose todos and hidden completed lists and
// shifts the sibling lists to make room.
func (s *TodolistsService) Reposition(ctx context.Context, todolistID int64, position int) (err error) {
	op := OperationInfo{
		Service: "Todolists", Operation: "Reposition",
		ResourceType: "todolist", IsMutation: true,
		ResourceID: todolistID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if position < 1 {
		err = ErrUsage("position must be at least 1")
		return err
	}
	if position > math.MaxInt32 {
		err = ErrUsage("position is out of range")
		return err
	}

	body := generated.RepositionTodolistJSONRequestBody{
		Position: int32(position), // #nosec G115 -- bounded above by the MaxInt32 guard
	}

	resp, err := s.client.parent.gen.RepositionTodolistWithResponse(ctx, s.client.accountID, todolistID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse, resp.Body)
}

// todolistFromGenerated converts a generated Todolist to our clean Todolist
// type. It is variant-agnostic: a group decodes into generated.Todolist just
// as a to-do list does, so both arrive here and the structural discriminator
// (GroupsURL vs GroupPositionURL) simply carries through.
func todolistFromGenerated(gtl generated.Todolist) Todolist {
	tl := Todolist{
		Status:           gtl.Status,
		VisibleToClients: gtl.VisibleToClients,
		Title:            gtl.Title,
		InheritsStatus:   gtl.InheritsStatus,
		Type:             gtl.Type,
		URL:              gtl.Url,
		AppURL:           gtl.AppUrl,
		BookmarkURL:      deref(gtl.BookmarkUrl),
		BoostsCount:      int(deref(gtl.BoostsCount)),
		BoostsURL:        deref(gtl.BoostsUrl),
		SubscriptionURL:  deref(gtl.SubscriptionUrl),
		BubbleUpURL:      gtl.BubbleUpUrl,
		CommentsCount:    int(deref(gtl.CommentsCount)),
		CommentsURL:      deref(gtl.CommentsUrl),
		// CommentsAppURL is @required and never null on the wire — the jbuilder
		// emits it from a route helper, which returns a String or raises. No
		// deref; it is a plain string.
		CommentsAppURL: gtl.CommentsAppUrl,
		Position:       int(deref(gtl.Position)),
		// Description is @required and never null on the wire:
		// format_api_content funnels a blank rich text through call_pipeline,
		// which returns "" rather than nil. No deref — it is a plain string.
		Description:      gtl.Description,
		Completed:        deref(gtl.Completed),
		CompletedRatio:   deref(gtl.CompletedRatio),
		Name:             gtl.Name,
		TodosURL:         deref(gtl.TodosUrl),
		GroupsURL:        deref(gtl.GroupsUrl),
		GroupPositionURL: deref(gtl.GroupPositionUrl),
		// Color's key is always emitted but its value is null when unset, so a
		// nil pointer and an unset color are the same thing: "".
		Color:       deref(gtl.Color),
		AppTodosURL: deref(gtl.AppTodosUrl),
		CreatedAt:   gtl.CreatedAt,
		UpdatedAt:   gtl.UpdatedAt,
	}

	if gtl.Id != 0 {
		tl.ID = gtl.Id
	}

	// Convert nested types
	if gtl.Parent.Id != 0 || gtl.Parent.Title != "" {
		tl.Parent = &Parent{
			ID:     gtl.Parent.Id,
			Title:  gtl.Parent.Title,
			Type:   gtl.Parent.Type,
			URL:    gtl.Parent.Url,
			AppURL: gtl.Parent.AppUrl,
		}
	}

	if gtl.Bucket.Id != 0 || gtl.Bucket.Name != "" {
		tl.Bucket = &Bucket{
			ID:   gtl.Bucket.Id,
			Name: gtl.Bucket.Name,
			Type: gtl.Bucket.Type,
		}
	}

	if gtl.Creator.Id != 0 || gtl.Creator.Name != "" {
		creator := personFromGenerated(gtl.Creator)
		tl.Creator = &creator
	}

	tl.DescriptionAttachments = richTextAttachmentsFromGenerated(gtl.DescriptionAttachments)

	return tl
}
