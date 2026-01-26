package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// CardTable represents a Basecamp card table (kanban board).
type CardTable struct {
	ID               int64         `json:"id"`
	Status           string        `json:"status"`
	VisibleToClients bool          `json:"visible_to_clients"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
	Title            string        `json:"title"`
	InheritsStatus   bool          `json:"inherits_status"`
	Type             string        `json:"type"`
	URL              string        `json:"url"`
	AppURL           string        `json:"app_url"`
	BookmarkURL      string        `json:"bookmark_url"`
	SubscriptionURL  string        `json:"subscription_url"`
	Bucket           *Bucket       `json:"bucket,omitempty"`
	Creator          *Person       `json:"creator,omitempty"`
	Subscribers      []Person      `json:"subscribers,omitempty"`
	Lists            []CardColumn  `json:"lists,omitempty"`
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
	ID                     int64      `json:"id"`
	Status                 string     `json:"status"`
	VisibleToClients       bool       `json:"visible_to_clients"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
	Title                  string     `json:"title"`
	InheritsStatus         bool       `json:"inherits_status"`
	Type                   string     `json:"type"`
	URL                    string     `json:"url"`
	AppURL                 string     `json:"app_url"`
	BookmarkURL            string     `json:"bookmark_url"`
	SubscriptionURL        string     `json:"subscription_url,omitempty"`
	Position               int        `json:"position"`
	Content                string     `json:"content,omitempty"`
	Description            string     `json:"description,omitempty"`
	DueOn                  string     `json:"due_on,omitempty"`
	Completed              bool       `json:"completed"`
	CompletedAt            *time.Time `json:"completed_at,omitempty"`
	CommentsCount          int        `json:"comments_count"`
	CommentsURL            string     `json:"comments_url,omitempty"`
	CommentCount           int        `json:"comment_count"`
	CompletionURL          string     `json:"completion_url,omitempty"`
	Parent                 *Parent    `json:"parent,omitempty"`
	Bucket                 *Bucket    `json:"bucket,omitempty"`
	Creator                *Person    `json:"creator,omitempty"`
	Completer              *Person    `json:"completer,omitempty"`
	Assignees              []Person   `json:"assignees,omitempty"`
	CompletionSubscribers  []Person   `json:"completion_subscribers,omitempty"`
	Steps                  []CardStep `json:"steps,omitempty"`
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
	// Assignees is a comma-separated string of user IDs (optional).
	Assignees string `json:"assignees,omitempty"`
}

// UpdateStepRequest specifies the parameters for updating a step.
type UpdateStepRequest struct {
	// Title is the step title (optional).
	Title string `json:"title,omitempty"`
	// DueOn is the due date in ISO 8601 format (optional).
	DueOn string `json:"due_on,omitempty"`
	// Assignees is a comma-separated string of user IDs (optional).
	Assignees string `json:"assignees,omitempty"`
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
func (s *CardTablesService) Get(ctx context.Context, bucketID, cardTableID int64) (*CardTable, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/%d.json", bucketID, cardTableID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var cardTable CardTable
	if err := resp.UnmarshalData(&cardTable); err != nil {
		return nil, fmt.Errorf("failed to parse card table: %w", err)
	}

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
func (s *CardsService) List(ctx context.Context, bucketID, columnID int64) ([]Card, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/lists/%d/cards.json", bucketID, columnID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	cards := make([]Card, 0, len(results))
	for _, raw := range results {
		var c Card
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("failed to parse card: %w", err)
		}
		cards = append(cards, c)
	}

	return cards, nil
}

// Get returns a card by ID.
// bucketID is the project ID, cardID is the card ID.
func (s *CardsService) Get(ctx context.Context, bucketID, cardID int64) (*Card, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/cards/%d.json", bucketID, cardID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var card Card
	if err := resp.UnmarshalData(&card); err != nil {
		return nil, fmt.Errorf("failed to parse card: %w", err)
	}

	return &card, nil
}

// Create creates a new card in a column.
// bucketID is the project ID, columnID is the column ID.
// Returns the created card.
func (s *CardsService) Create(ctx context.Context, bucketID, columnID int64, req *CreateCardRequest) (*Card, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Title == "" {
		return nil, ErrUsage("card title is required")
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/lists/%d/cards.json", bucketID, columnID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var card Card
	if err := resp.UnmarshalData(&card); err != nil {
		return nil, fmt.Errorf("failed to parse card: %w", err)
	}

	return &card, nil
}

// Update updates an existing card.
// bucketID is the project ID, cardID is the card ID.
// Returns the updated card.
func (s *CardsService) Update(ctx context.Context, bucketID, cardID int64, req *UpdateCardRequest) (*Card, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil {
		return nil, ErrUsage("update request is required")
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/cards/%d.json", bucketID, cardID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var card Card
	if err := resp.UnmarshalData(&card); err != nil {
		return nil, fmt.Errorf("failed to parse card: %w", err)
	}

	return &card, nil
}

// Move moves a card to a different column.
// bucketID is the project ID, cardID is the card ID, columnID is the destination column ID.
func (s *CardsService) Move(ctx context.Context, bucketID, cardID, columnID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/cards/%d/moves.json", bucketID, cardID)
	body := MoveCardRequest{ColumnID: columnID}
	_, err := s.client.Post(ctx, path, body)
	return err
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
func (s *CardColumnsService) Get(ctx context.Context, bucketID, columnID int64) (*CardColumn, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/columns/%d.json", bucketID, columnID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var column CardColumn
	if err := resp.UnmarshalData(&column); err != nil {
		return nil, fmt.Errorf("failed to parse column: %w", err)
	}

	return &column, nil
}

// Create creates a new column in a card table.
// bucketID is the project ID, cardTableID is the card table ID.
// Returns the created column.
func (s *CardColumnsService) Create(ctx context.Context, bucketID, cardTableID int64, req *CreateColumnRequest) (*CardColumn, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Title == "" {
		return nil, ErrUsage("column title is required")
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/%d/columns.json", bucketID, cardTableID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var column CardColumn
	if err := resp.UnmarshalData(&column); err != nil {
		return nil, fmt.Errorf("failed to parse column: %w", err)
	}

	return &column, nil
}

// Update updates an existing column.
// bucketID is the project ID, columnID is the column ID.
// Returns the updated column.
func (s *CardColumnsService) Update(ctx context.Context, bucketID, columnID int64, req *UpdateColumnRequest) (*CardColumn, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil {
		return nil, ErrUsage("update request is required")
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/columns/%d.json", bucketID, columnID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var column CardColumn
	if err := resp.UnmarshalData(&column); err != nil {
		return nil, fmt.Errorf("failed to parse column: %w", err)
	}

	return &column, nil
}

// Move moves a column within a card table.
// bucketID is the project ID, cardTableID is the card table ID.
func (s *CardColumnsService) Move(ctx context.Context, bucketID, cardTableID int64, req *MoveColumnRequest) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	if req == nil {
		return ErrUsage("move request is required")
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/%d/moves.json", bucketID, cardTableID)
	_, err := s.client.Post(ctx, path, req)
	return err
}

// SetColor sets the color of a column.
// bucketID is the project ID, columnID is the column ID.
// Valid colors: white, red, orange, yellow, green, blue, aqua, purple, gray, pink, brown.
// Returns the updated column.
func (s *CardColumnsService) SetColor(ctx context.Context, bucketID, columnID int64, color string) (*CardColumn, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if color == "" {
		return nil, ErrUsage("color is required")
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/columns/%d/color.json", bucketID, columnID)
	body := SetColumnColorRequest{Color: color}
	resp, err := s.client.Put(ctx, path, body)
	if err != nil {
		return nil, err
	}

	var column CardColumn
	if err := resp.UnmarshalData(&column); err != nil {
		return nil, fmt.Errorf("failed to parse column: %w", err)
	}

	return &column, nil
}

// EnableOnHold adds an on-hold section to a column.
// bucketID is the project ID, columnID is the column ID.
// Returns the updated column.
func (s *CardColumnsService) EnableOnHold(ctx context.Context, bucketID, columnID int64) (*CardColumn, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/columns/%d/on_hold.json", bucketID, columnID)
	resp, err := s.client.Post(ctx, path, nil)
	if err != nil {
		return nil, err
	}

	var column CardColumn
	if err := resp.UnmarshalData(&column); err != nil {
		return nil, fmt.Errorf("failed to parse column: %w", err)
	}

	return &column, nil
}

// DisableOnHold removes the on-hold section from a column.
// bucketID is the project ID, columnID is the column ID.
// Returns the updated column.
func (s *CardColumnsService) DisableOnHold(ctx context.Context, bucketID, columnID int64) (*CardColumn, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/columns/%d/on_hold.json", bucketID, columnID)
	resp, err := s.client.Delete(ctx, path)
	if err != nil {
		return nil, err
	}

	var column CardColumn
	if err := resp.UnmarshalData(&column); err != nil {
		return nil, fmt.Errorf("failed to parse column: %w", err)
	}

	return &column, nil
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
func (s *CardStepsService) Create(ctx context.Context, bucketID, cardID int64, req *CreateStepRequest) (*CardStep, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.Title == "" {
		return nil, ErrUsage("step title is required")
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/cards/%d/steps.json", bucketID, cardID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var step CardStep
	if err := resp.UnmarshalData(&step); err != nil {
		return nil, fmt.Errorf("failed to parse step: %w", err)
	}

	return &step, nil
}

// Update updates an existing step.
// bucketID is the project ID, stepID is the step ID.
// Returns the updated step.
func (s *CardStepsService) Update(ctx context.Context, bucketID, stepID int64, req *UpdateStepRequest) (*CardStep, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil {
		return nil, ErrUsage("update request is required")
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/steps/%d.json", bucketID, stepID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var step CardStep
	if err := resp.UnmarshalData(&step); err != nil {
		return nil, fmt.Errorf("failed to parse step: %w", err)
	}

	return &step, nil
}

// Complete marks a step as completed.
// bucketID is the project ID, stepID is the step ID.
// Returns the updated step.
func (s *CardStepsService) Complete(ctx context.Context, bucketID, stepID int64) (*CardStep, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/steps/%d/completions.json", bucketID, stepID)
	body := map[string]string{"completion": "on"}
	resp, err := s.client.Put(ctx, path, body)
	if err != nil {
		return nil, err
	}

	var step CardStep
	if err := resp.UnmarshalData(&step); err != nil {
		return nil, fmt.Errorf("failed to parse step: %w", err)
	}

	return &step, nil
}

// Uncomplete marks a step as incomplete.
// bucketID is the project ID, stepID is the step ID.
// Returns the updated step.
func (s *CardStepsService) Uncomplete(ctx context.Context, bucketID, stepID int64) (*CardStep, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/steps/%d/completions.json", bucketID, stepID)
	body := map[string]string{"completion": "off"}
	resp, err := s.client.Put(ctx, path, body)
	if err != nil {
		return nil, err
	}

	var step CardStep
	if err := resp.UnmarshalData(&step); err != nil {
		return nil, fmt.Errorf("failed to parse step: %w", err)
	}

	return &step, nil
}

// Reposition changes the position of a step within a card.
// bucketID is the project ID, cardID is the card ID, stepID is the step ID.
// position is 0-indexed.
func (s *CardStepsService) Reposition(ctx context.Context, bucketID, cardID, stepID int64, position int) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	if position < 0 {
		return ErrUsage("position must be at least 0")
	}

	path := fmt.Sprintf("/buckets/%d/card_tables/cards/%d/positions.json", bucketID, cardID)
	body := map[string]any{"source_id": stepID, "position": position}
	_, err := s.client.Post(ctx, path, body)
	return err
}

// Delete deletes a step (moves it to trash).
// bucketID is the project ID, stepID is the step ID.
func (s *CardStepsService) Delete(ctx context.Context, bucketID, stepID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/recordings/%d/status/trashed.json", bucketID, stepID)
	_, err := s.client.Put(ctx, path, nil)
	return err
}
