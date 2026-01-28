package basecamp

import (
	"context"
	"fmt"
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
}

// CardColumn represents a column in a card table.
type CardColumn struct {
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
	Position         int       `json:"position,omitempty"`
	Color            string    `json:"color,omitempty"`
	Description      string    `json:"description,omitempty"`
	CardsCount       int       `json:"cards_count"`
	CommentCount     int       `json:"comment_count"`
	CardsURL         string    `json:"cards_url,omitempty"`
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
	Subscribers      []Person  `json:"subscribers,omitempty"`
}

// Card represents a card in a card table column.
type Card struct {
	ID                    int64      `json:"id"`
	Status                string     `json:"status"`
	VisibleToClients      bool       `json:"visible_to_clients"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	Title                 string     `json:"title"`
	InheritsStatus        bool       `json:"inherits_status"`
	Type                  string     `json:"type"`
	URL                   string     `json:"url"`
	AppURL                string     `json:"app_url"`
	BookmarkURL           string     `json:"bookmark_url"`
	SubscriptionURL       string     `json:"subscription_url,omitempty"`
	Position              int        `json:"position"`
	Content               string     `json:"content,omitempty"`
	Description           string     `json:"description,omitempty"`
	DueOn                 string     `json:"due_on,omitempty"`
	Completed             bool       `json:"completed"`
	CompletedAt           *time.Time `json:"completed_at,omitempty"`
	CommentsCount         int        `json:"comments_count"`
	CommentsURL           string     `json:"comments_url,omitempty"`
	CommentCount          int        `json:"comment_count"`
	CompletionURL         string     `json:"completion_url,omitempty"`
	Parent                *Parent    `json:"parent,omitempty"`
	Bucket                *Bucket    `json:"bucket,omitempty"`
	Creator               *Person    `json:"creator,omitempty"`
	Completer             *Person    `json:"completer,omitempty"`
	Assignees             []Person   `json:"assignees,omitempty"`
	CompletionSubscribers []Person   `json:"completion_subscribers,omitempty"`
	Steps                 []CardStep `json:"steps,omitempty"`
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
type UpdateCardRequest struct {
	// Title is the card title (optional).
	Title string `json:"title,omitempty"`
	// Content is the card body in HTML (optional).
	Content string `json:"content,omitempty"`
	// DueOn is the due date in ISO 8601 format (optional).
	DueOn string `json:"due_on,omitempty"`
	// AssigneeIDs is a list of person IDs to assign this card to (optional).
	AssigneeIDs []int64 `json:"assignee_ids,omitempty"`
}

// MoveCardRequest specifies the parameters for moving a card.
type MoveCardRequest struct {
	// ColumnID is the destination column ID (required).
	ColumnID int64 `json:"column_id"`
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
	// Position is the position relative to target (optional).
	Position int `json:"position,omitempty"`
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
	// Assignees is a list of person IDs to assign this step to (optional).
	Assignees []int64 `json:"assignees,omitempty"`
}

// UpdateStepRequest specifies the parameters for updating a step.
type UpdateStepRequest struct {
	// Title is the step title (optional).
	Title string `json:"title,omitempty"`
	// DueOn is the due date in ISO 8601 format (optional).
	DueOn string `json:"due_on,omitempty"`
	// Assignees is a list of person IDs to assign this step to (optional).
	Assignees []int64 `json:"assignees,omitempty"`
}

// CardTablesService handles card table operations.
type CardTablesService struct {
	client *Client
}

// NewCardTablesService creates a new CardTablesService.
func NewCardTablesService(client *Client) *CardTablesService {
	return &CardTablesService{client: client}
}

// Get returns a card table by ID.
// bucketID is the project ID, cardTableID is the card table ID.
func (s *CardTablesService) Get(ctx context.Context, bucketID, cardTableID int64) (result *CardTable, err error) {
	op := OperationInfo{
		Service: "CardTables", Operation: "Get",
		ResourceType: "card_table", IsMutation: false,
		BucketID: bucketID, ResourceID: cardTableID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetCardTableWithResponse(ctx, bucketID, cardTableID)
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

	cardTable := cardTableFromGenerated(resp.JSON200.CardTable)
	return &cardTable, nil
}

// CardsService handles card operations.
type CardsService struct {
	client *Client
}

// NewCardsService creates a new CardsService.
func NewCardsService(client *Client) *CardsService {
	return &CardsService{client: client}
}

// List returns all cards in a column.
// bucketID is the project ID, columnID is the column ID.
func (s *CardsService) List(ctx context.Context, bucketID, columnID int64) (result []Card, err error) {
	op := OperationInfo{
		Service: "Cards", Operation: "List",
		ResourceType: "card", IsMutation: false,
		BucketID: bucketID, ResourceID: columnID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.ListCardsWithResponse(ctx, bucketID, columnID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	cards := make([]Card, 0, len(*resp.JSON200))
	for _, gc := range *resp.JSON200 {
		cards = append(cards, cardFromGenerated(gc))
	}
	return cards, nil
}

// Get returns a card by ID.
// bucketID is the project ID, cardID is the card ID.
func (s *CardsService) Get(ctx context.Context, bucketID, cardID int64) (result *Card, err error) {
	op := OperationInfo{
		Service: "Cards", Operation: "Get",
		ResourceType: "card", IsMutation: false,
		BucketID: bucketID, ResourceID: cardID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetCardWithResponse(ctx, bucketID, cardID)
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

	card := cardFromGenerated(resp.JSON200.Card)
	return &card, nil
}

// Create creates a new card in a column.
// bucketID is the project ID, columnID is the column ID.
// Returns the created card.
func (s *CardsService) Create(ctx context.Context, bucketID, columnID int64, req *CreateCardRequest) (result *Card, err error) {
	op := OperationInfo{
		Service: "Cards", Operation: "Create",
		ResourceType: "card", IsMutation: true,
		BucketID: bucketID, ResourceID: columnID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Title == "" {
		err = ErrUsage("card title is required")
		return nil, err
	}

	body := generated.CreateCardJSONRequestBody{
		Title: req.Title,
	}
	if req.Content != "" {
		body.Content = req.Content
	}
	if req.DueOn != "" {
		d, parseErr := types.ParseDate(req.DueOn)
		if parseErr != nil {
			err = ErrUsage("card due_on must be in YYYY-MM-DD format")
			return nil, err
		}
		body.DueOn = d
	}
	if req.Notify {
		body.Notify = req.Notify
	}

	resp, err := s.client.gen.CreateCardWithResponse(ctx, bucketID, columnID, body)
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

	card := cardFromGenerated(resp.JSON200.Card)
	return &card, nil
}

// Update updates an existing card.
// bucketID is the project ID, cardID is the card ID.
// Returns the updated card.
func (s *CardsService) Update(ctx context.Context, bucketID, cardID int64, req *UpdateCardRequest) (result *Card, err error) {
	op := OperationInfo{
		Service: "Cards", Operation: "Update",
		ResourceType: "card", IsMutation: true,
		BucketID: bucketID, ResourceID: cardID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil {
		err = ErrUsage("update request is required")
		return nil, err
	}

	body := generated.UpdateCardJSONRequestBody{}
	if req.Title != "" {
		body.Title = req.Title
	}
	if req.Content != "" {
		body.Content = req.Content
	}
	if req.DueOn != "" {
		d, parseErr := types.ParseDate(req.DueOn)
		if parseErr != nil {
			err = ErrUsage("card due_on must be in YYYY-MM-DD format")
			return nil, err
		}
		body.DueOn = d
	}
	if len(req.AssigneeIDs) > 0 {
		body.AssigneeIds = req.AssigneeIDs
	}

	resp, err := s.client.gen.UpdateCardWithResponse(ctx, bucketID, cardID, body)
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

	card := cardFromGenerated(resp.JSON200.Card)
	return &card, nil
}

// Move moves a card to a different column.
// bucketID is the project ID, cardID is the card ID, columnID is the destination column ID.
func (s *CardsService) Move(ctx context.Context, bucketID, cardID, columnID int64) (err error) {
	op := OperationInfo{
		Service: "Cards", Operation: "Move",
		ResourceType: "card", IsMutation: true,
		BucketID: bucketID, ResourceID: cardID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	body := generated.MoveCardJSONRequestBody{
		ColumnId: columnID,
	}

	resp, err := s.client.gen.MoveCardWithResponse(ctx, bucketID, cardID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Trash moves a card to the trash.
// bucketID is the project ID, cardID is the card ID.
// Trashed cards can be recovered from the trash.
func (s *CardsService) Trash(ctx context.Context, bucketID, cardID int64) (err error) {
	op := OperationInfo{
		Service: "Cards", Operation: "Trash",
		ResourceType: "card", IsMutation: true,
		BucketID: bucketID, ResourceID: cardID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.TrashRecordingWithResponse(ctx, bucketID, cardID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// CardColumnsService handles card column operations.
type CardColumnsService struct {
	client *Client
}

// NewCardColumnsService creates a new CardColumnsService.
func NewCardColumnsService(client *Client) *CardColumnsService {
	return &CardColumnsService{client: client}
}

// Get returns a column by ID.
// bucketID is the project ID, columnID is the column ID.
func (s *CardColumnsService) Get(ctx context.Context, bucketID, columnID int64) (result *CardColumn, err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "Get",
		ResourceType: "card_column", IsMutation: false,
		BucketID: bucketID, ResourceID: columnID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.GetCardColumnWithResponse(ctx, bucketID, columnID)
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

	column := cardColumnFromGenerated(resp.JSON200.Column)
	return &column, nil
}

// Create creates a new column in a card table.
// bucketID is the project ID, cardTableID is the card table ID.
// Returns the created column.
func (s *CardColumnsService) Create(ctx context.Context, bucketID, cardTableID int64, req *CreateColumnRequest) (result *CardColumn, err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "Create",
		ResourceType: "card_column", IsMutation: true,
		BucketID: bucketID, ResourceID: cardTableID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Title == "" {
		err = ErrUsage("column title is required")
		return nil, err
	}

	body := generated.CreateCardColumnJSONRequestBody{
		Title:       req.Title,
		Description: req.Description,
	}

	resp, err := s.client.gen.CreateCardColumnWithResponse(ctx, bucketID, cardTableID, body)
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

	column := cardColumnFromGenerated(resp.JSON200.Column)
	return &column, nil
}

// Update updates an existing column.
// bucketID is the project ID, columnID is the column ID.
// Returns the updated column.
func (s *CardColumnsService) Update(ctx context.Context, bucketID, columnID int64, req *UpdateColumnRequest) (result *CardColumn, err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "Update",
		ResourceType: "card_column", IsMutation: true,
		BucketID: bucketID, ResourceID: columnID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil {
		err = ErrUsage("update request is required")
		return nil, err
	}

	body := generated.UpdateCardColumnJSONRequestBody{
		Title:       req.Title,
		Description: req.Description,
	}

	resp, err := s.client.gen.UpdateCardColumnWithResponse(ctx, bucketID, columnID, body)
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

	column := cardColumnFromGenerated(resp.JSON200.Column)
	return &column, nil
}

// Move moves a column within a card table.
// bucketID is the project ID, cardTableID is the card table ID.
func (s *CardColumnsService) Move(ctx context.Context, bucketID, cardTableID int64, req *MoveColumnRequest) (err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "Move",
		ResourceType: "card_column", IsMutation: true,
		BucketID: bucketID, ResourceID: cardTableID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	if req == nil {
		err = ErrUsage("move request is required")
		return err
	}

	body := generated.MoveCardColumnJSONRequestBody{
		SourceId: req.SourceID,
		TargetId: req.TargetID,
		Position: int32(req.Position),
	}

	resp, err := s.client.gen.MoveCardColumnWithResponse(ctx, bucketID, cardTableID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// SetColor sets the color of a column.
// bucketID is the project ID, columnID is the column ID.
// Valid colors: white, red, orange, yellow, green, blue, aqua, purple, gray, pink, brown.
// Returns the updated column.
func (s *CardColumnsService) SetColor(ctx context.Context, bucketID, columnID int64, color string) (result *CardColumn, err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "SetColor",
		ResourceType: "card_column", IsMutation: true,
		BucketID: bucketID, ResourceID: columnID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if color == "" {
		err = ErrUsage("color is required")
		return nil, err
	}

	body := generated.SetCardColumnColorJSONRequestBody{
		Color: color,
	}

	resp, err := s.client.gen.SetCardColumnColorWithResponse(ctx, bucketID, columnID, body)
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

	column := cardColumnFromGenerated(resp.JSON200.Column)
	return &column, nil
}

// EnableOnHold adds an on-hold section to a column.
// bucketID is the project ID, columnID is the column ID.
// Returns the updated column.
func (s *CardColumnsService) EnableOnHold(ctx context.Context, bucketID, columnID int64) (result *CardColumn, err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "EnableOnHold",
		ResourceType: "card_column", IsMutation: true,
		BucketID: bucketID, ResourceID: columnID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.EnableCardColumnOnHoldWithResponse(ctx, bucketID, columnID)
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

	column := cardColumnFromGenerated(resp.JSON200.Column)
	return &column, nil
}

// DisableOnHold removes the on-hold section from a column.
// bucketID is the project ID, columnID is the column ID.
// Returns the updated column.
func (s *CardColumnsService) DisableOnHold(ctx context.Context, bucketID, columnID int64) (result *CardColumn, err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "DisableOnHold",
		ResourceType: "card_column", IsMutation: true,
		BucketID: bucketID, ResourceID: columnID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.DisableCardColumnOnHoldWithResponse(ctx, bucketID, columnID)
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

	column := cardColumnFromGenerated(resp.JSON200.Column)
	return &column, nil
}

// Watch subscribes the current user to the column.
// bucketID is the project ID, columnID is the column ID.
// Returns the updated subscription information.
func (s *CardColumnsService) Watch(ctx context.Context, bucketID, columnID int64) (result *Subscription, err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "Watch",
		ResourceType: "card_column", IsMutation: true,
		BucketID: bucketID, ResourceID: columnID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.SubscribeWithResponse(ctx, bucketID, columnID)
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

	sub := subscriptionFromGenerated(resp.JSON200.Subscription)
	return &sub, nil
}

// Unwatch unsubscribes the current user from the column.
// bucketID is the project ID, columnID is the column ID.
// Returns nil on success (204 No Content).
func (s *CardColumnsService) Unwatch(ctx context.Context, bucketID, columnID int64) (err error) {
	op := OperationInfo{
		Service: "CardColumns", Operation: "Unwatch",
		ResourceType: "card_column", IsMutation: true,
		BucketID: bucketID, ResourceID: columnID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.UnsubscribeWithResponse(ctx, bucketID, columnID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// CardStepsService handles card step operations.
type CardStepsService struct {
	client *Client
}

// NewCardStepsService creates a new CardStepsService.
func NewCardStepsService(client *Client) *CardStepsService {
	return &CardStepsService{client: client}
}

// Create creates a new step on a card.
// bucketID is the project ID, cardID is the card ID.
// Returns the created step.
func (s *CardStepsService) Create(ctx context.Context, bucketID, cardID int64, req *CreateStepRequest) (result *CardStep, err error) {
	op := OperationInfo{
		Service: "CardSteps", Operation: "Create",
		ResourceType: "card_step", IsMutation: true,
		BucketID: bucketID, ResourceID: cardID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Title == "" {
		err = ErrUsage("step title is required")
		return nil, err
	}

	body := generated.CreateCardStepJSONRequestBody{
		Title:     req.Title,
		Assignees: req.Assignees,
	}
	if req.DueOn != "" {
		d, parseErr := types.ParseDate(req.DueOn)
		if parseErr != nil {
			err = ErrUsage("step due_on must be in YYYY-MM-DD format")
			return nil, err
		}
		body.DueOn = d
	}

	resp, err := s.client.gen.CreateCardStepWithResponse(ctx, bucketID, cardID, body)
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

	step := cardStepFromGenerated(resp.JSON200.Step)
	return &step, nil
}

// Update updates an existing step.
// bucketID is the project ID, stepID is the step ID.
// Returns the updated step.
func (s *CardStepsService) Update(ctx context.Context, bucketID, stepID int64, req *UpdateStepRequest) (result *CardStep, err error) {
	op := OperationInfo{
		Service: "CardSteps", Operation: "Update",
		ResourceType: "card_step", IsMutation: true,
		BucketID: bucketID, ResourceID: stepID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil {
		err = ErrUsage("update request is required")
		return nil, err
	}

	body := generated.UpdateCardStepJSONRequestBody{
		Title:     req.Title,
		Assignees: req.Assignees,
	}
	if req.DueOn != "" {
		d, parseErr := types.ParseDate(req.DueOn)
		if parseErr != nil {
			err = ErrUsage("step due_on must be in YYYY-MM-DD format")
			return nil, err
		}
		body.DueOn = d
	}

	resp, err := s.client.gen.UpdateCardStepWithResponse(ctx, bucketID, stepID, body)
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

	step := cardStepFromGenerated(resp.JSON200.Step)
	return &step, nil
}

// Complete marks a step as completed.
// bucketID is the project ID, stepID is the step ID.
// Returns the updated step.
func (s *CardStepsService) Complete(ctx context.Context, bucketID, stepID int64) (result *CardStep, err error) {
	op := OperationInfo{
		Service: "CardSteps", Operation: "Complete",
		ResourceType: "card_step", IsMutation: true,
		BucketID: bucketID, ResourceID: stepID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.CompleteCardStepWithResponse(ctx, bucketID, stepID)
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

	step := cardStepFromGenerated(resp.JSON200.Step)
	return &step, nil
}

// Uncomplete marks a step as incomplete.
// bucketID is the project ID, stepID is the step ID.
// Returns the updated step.
func (s *CardStepsService) Uncomplete(ctx context.Context, bucketID, stepID int64) (result *CardStep, err error) {
	op := OperationInfo{
		Service: "CardSteps", Operation: "Uncomplete",
		ResourceType: "card_step", IsMutation: true,
		BucketID: bucketID, ResourceID: stepID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return nil, err
	}

	resp, err := s.client.gen.UncompleteCardStepWithResponse(ctx, bucketID, stepID)
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

	step := cardStepFromGenerated(resp.JSON200.Step)
	return &step, nil
}

// Reposition changes the position of a step within a card.
// bucketID is the project ID, cardID is the card ID, stepID is the step ID.
// position is 0-indexed.
func (s *CardStepsService) Reposition(ctx context.Context, bucketID, cardID, stepID int64, position int) (err error) {
	op := OperationInfo{
		Service: "CardSteps", Operation: "Reposition",
		ResourceType: "card_step", IsMutation: true,
		BucketID: bucketID, ResourceID: stepID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	if position < 0 {
		err = ErrUsage("position must be at least 0")
		return err
	}

	body := generated.RepositionCardStepJSONRequestBody{
		SourceId: stepID,
		Position: int32(position),
	}

	resp, err := s.client.gen.RepositionCardStepWithResponse(ctx, bucketID, cardID, body)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// Delete deletes a step (moves it to trash).
// bucketID is the project ID, stepID is the step ID.
func (s *CardStepsService) Delete(ctx context.Context, bucketID, stepID int64) (err error) {
	op := OperationInfo{
		Service: "CardSteps", Operation: "Delete",
		ResourceType: "card_step", IsMutation: true,
		BucketID: bucketID, ResourceID: stepID,
	}
	start := time.Now()
	ctx = s.client.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if err = s.client.RequireAccount(); err != nil {
		return err
	}

	resp, err := s.client.gen.TrashRecordingWithResponse(ctx, bucketID, stepID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
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
		BookmarkURL:      gc.BookmarkUrl,
		SubscriptionURL:  gc.SubscriptionUrl,
		CreatedAt:        gc.CreatedAt,
		UpdatedAt:        gc.UpdatedAt,
	}

	if gc.Id != nil {
		ct.ID = *gc.Id
	}

	if gc.Bucket.Id != nil || gc.Bucket.Name != "" {
		ct.Bucket = &Bucket{
			ID:   derefInt64(gc.Bucket.Id),
			Name: gc.Bucket.Name,
			Type: gc.Bucket.Type,
		}
	}

	if gc.Creator.Id != nil || gc.Creator.Name != "" {
		ct.Creator = &Person{
			ID:           derefInt64(gc.Creator.Id),
			Name:         gc.Creator.Name,
			EmailAddress: gc.Creator.EmailAddress,
			AvatarURL:    gc.Creator.AvatarUrl,
			Admin:        gc.Creator.Admin,
			Owner:        gc.Creator.Owner,
		}
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
		BookmarkURL:      gc.BookmarkUrl,
		Position:         int(gc.Position),
		Color:            gc.Color,
		Description:      gc.Description,
		CardsCount:       int(gc.CardsCount),
		CommentCount:     int(gc.CommentsCount),
		CardsURL:         gc.CardsUrl,
		CreatedAt:        gc.CreatedAt,
		UpdatedAt:        gc.UpdatedAt,
	}

	if gc.Id != nil {
		cc.ID = *gc.Id
	}

	if gc.Parent.Id != nil || gc.Parent.Title != "" {
		cc.Parent = &Parent{
			ID:     derefInt64(gc.Parent.Id),
			Title:  gc.Parent.Title,
			Type:   gc.Parent.Type,
			URL:    gc.Parent.Url,
			AppURL: gc.Parent.AppUrl,
		}
	}

	if gc.Bucket.Id != nil || gc.Bucket.Name != "" {
		cc.Bucket = &Bucket{
			ID:   derefInt64(gc.Bucket.Id),
			Name: gc.Bucket.Name,
			Type: gc.Bucket.Type,
		}
	}

	if gc.Creator.Id != nil || gc.Creator.Name != "" {
		cc.Creator = &Person{
			ID:           derefInt64(gc.Creator.Id),
			Name:         gc.Creator.Name,
			EmailAddress: gc.Creator.EmailAddress,
			AvatarURL:    gc.Creator.AvatarUrl,
			Admin:        gc.Creator.Admin,
			Owner:        gc.Creator.Owner,
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
		BookmarkURL:      gc.BookmarkUrl,
		SubscriptionURL:  gc.SubscriptionUrl,
		Position:         int(gc.Position),
		Content:          gc.Content,
		Description:      gc.Description,
		Completed:        gc.Completed,
		CommentsCount:    int(gc.CommentsCount),
		CommentsURL:      gc.CommentsUrl,
		CompletionURL:    gc.CompletionUrl,
		CreatedAt:        gc.CreatedAt,
		UpdatedAt:        gc.UpdatedAt,
	}

	if gc.Id != nil {
		c.ID = *gc.Id
	}

	// Handle due_on - it's types.Date in generated, string in SDK
	if !gc.DueOn.IsZero() {
		c.DueOn = gc.DueOn.String()
	}

	// Handle completed_at
	if !gc.CompletedAt.IsZero() {
		c.CompletedAt = &gc.CompletedAt
	}

	if gc.Parent.Id != nil || gc.Parent.Title != "" {
		c.Parent = &Parent{
			ID:     derefInt64(gc.Parent.Id),
			Title:  gc.Parent.Title,
			Type:   gc.Parent.Type,
			URL:    gc.Parent.Url,
			AppURL: gc.Parent.AppUrl,
		}
	}

	if gc.Bucket.Id != nil || gc.Bucket.Name != "" {
		c.Bucket = &Bucket{
			ID:   derefInt64(gc.Bucket.Id),
			Name: gc.Bucket.Name,
			Type: gc.Bucket.Type,
		}
	}

	if gc.Creator.Id != nil || gc.Creator.Name != "" {
		c.Creator = &Person{
			ID:           derefInt64(gc.Creator.Id),
			Name:         gc.Creator.Name,
			EmailAddress: gc.Creator.EmailAddress,
			AvatarURL:    gc.Creator.AvatarUrl,
			Admin:        gc.Creator.Admin,
			Owner:        gc.Creator.Owner,
		}
	}

	if gc.Completer.Id != nil || gc.Completer.Name != "" {
		c.Completer = &Person{
			ID:           derefInt64(gc.Completer.Id),
			Name:         gc.Completer.Name,
			EmailAddress: gc.Completer.EmailAddress,
			AvatarURL:    gc.Completer.AvatarUrl,
			Admin:        gc.Completer.Admin,
			Owner:        gc.Completer.Owner,
		}
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
		BookmarkURL:      gs.BookmarkUrl,
		Position:         int(gs.Position),
		Completed:        gs.Completed,
		CreatedAt:        gs.CreatedAt,
		UpdatedAt:        gs.UpdatedAt,
	}

	if gs.Id != nil {
		s.ID = *gs.Id
	}

	// Handle due_on - it's types.Date in generated, string in SDK
	if !gs.DueOn.IsZero() {
		s.DueOn = gs.DueOn.String()
	}

	// Handle completed_at
	if !gs.CompletedAt.IsZero() {
		s.CompletedAt = &gs.CompletedAt
	}

	if gs.Parent.Id != nil || gs.Parent.Title != "" {
		s.Parent = &Parent{
			ID:     derefInt64(gs.Parent.Id),
			Title:  gs.Parent.Title,
			Type:   gs.Parent.Type,
			URL:    gs.Parent.Url,
			AppURL: gs.Parent.AppUrl,
		}
	}

	if gs.Bucket.Id != nil || gs.Bucket.Name != "" {
		s.Bucket = &Bucket{
			ID:   derefInt64(gs.Bucket.Id),
			Name: gs.Bucket.Name,
			Type: gs.Bucket.Type,
		}
	}

	if gs.Creator.Id != nil || gs.Creator.Name != "" {
		s.Creator = &Person{
			ID:           derefInt64(gs.Creator.Id),
			Name:         gs.Creator.Name,
			EmailAddress: gs.Creator.EmailAddress,
			AvatarURL:    gs.Creator.AvatarUrl,
			Admin:        gs.Creator.Admin,
			Owner:        gs.Creator.Owner,
		}
	}

	if gs.Completer.Id != nil || gs.Completer.Name != "" {
		s.Completer = &Person{
			ID:           derefInt64(gs.Completer.Id),
			Name:         gs.Completer.Name,
			EmailAddress: gs.Completer.EmailAddress,
			AvatarURL:    gs.Completer.AvatarUrl,
			Admin:        gs.Completer.Admin,
			Owner:        gs.Completer.Owner,
		}
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
