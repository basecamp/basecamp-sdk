package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Todoset represents a Basecamp todoset (container for todolists in a project).
// Each project has exactly one todoset in its dock.
type Todoset struct {
	ID                int64     `json:"id"`
	Status            string    `json:"status"`
	VisibleToClients  bool      `json:"visible_to_clients"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	Title             string    `json:"title"`
	InheritsStatus    bool      `json:"inherits_status"`
	Type              string    `json:"type"`
	URL               string    `json:"url"`
	AppURL            string    `json:"app_url"`
	BookmarkURL       string    `json:"bookmark_url"`
	Position          *int      `json:"position,omitempty"`
	Bucket            *Bucket   `json:"bucket,omitempty"`
	Creator           *Person   `json:"creator,omitempty"`
	Name              string    `json:"name"`
	TodolistsCount    int       `json:"todolists_count"`
	TodolistsURL      string    `json:"todolists_url"`
	CompletedRatio    string    `json:"completed_ratio"`
	Completed         bool      `json:"completed"`
	CompletedCount    int       `json:"completed_count"`
	OnScheduleCount   int       `json:"on_schedule_count"`
	OverScheduleCount int       `json:"over_schedule_count"`
	AppTodolistsURL   string    `json:"app_todolists_url"`
}

// TodosetsService handles todoset operations.
type TodosetsService struct {
	client *AccountClient
}

// NewTodosetsService creates a new TodosetsService.
func NewTodosetsService(client *AccountClient) *TodosetsService {
	return &TodosetsService{client: client}
}

// Get returns a todoset by ID.
func (s *TodosetsService) Get(ctx context.Context, todosetID int64) (result *Todoset, err error) {
	op := OperationInfo{
		Service: "Todosets", Operation: "Get",
		ResourceType: "todoset", IsMutation: false,
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

	resp, err := s.client.parent.gen.GetTodosetWithResponse(ctx, s.client.accountID, todosetID)
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

	todoset := todosetFromGenerated(*resp.JSON200)
	return &todoset, nil
}

// todosetFromGenerated converts a generated Todoset to our clean Todoset type.
func todosetFromGenerated(gts generated.Todoset) Todoset {
	ts := Todoset{
		Status:            gts.Status,
		VisibleToClients:  gts.VisibleToClients,
		Title:             gts.Title,
		InheritsStatus:    gts.InheritsStatus,
		Type:              gts.Type,
		URL:               gts.Url,
		AppURL:            gts.AppUrl,
		BookmarkURL:       gts.BookmarkUrl,
		Name:              gts.Name,
		TodolistsCount:    int(gts.TodolistsCount),
		TodolistsURL:      gts.TodolistsUrl,
		CompletedRatio:    gts.CompletedRatio,
		Completed:         gts.Completed,
		CompletedCount:    int(gts.CompletedCount),
		OnScheduleCount:   int(gts.OnScheduleCount),
		OverScheduleCount: int(gts.OverScheduleCount),
		AppTodolistsURL:   gts.AppTodolistsUrl,
		CreatedAt:         gts.CreatedAt,
		UpdatedAt:         gts.UpdatedAt,
	}

	if gts.Id != 0 {
		ts.ID = gts.Id
	}

	if gts.Position != 0 {
		pos := int(gts.Position)
		ts.Position = &pos
	}

	// Convert nested types
	if gts.Bucket.Id != 0 || gts.Bucket.Name != "" {
		ts.Bucket = &Bucket{
			ID:   gts.Bucket.Id,
			Name: gts.Bucket.Name,
			Type: gts.Bucket.Type,
		}
	}

	if gts.Creator.Id != 0 || gts.Creator.Name != "" {
		ts.Creator = &Person{
			ID:           gts.Creator.Id,
			Name:         gts.Creator.Name,
			EmailAddress: gts.Creator.EmailAddress,
			AvatarURL:    gts.Creator.AvatarUrl,
			Admin:        gts.Creator.Admin,
			Owner:        gts.Creator.Owner,
		}
	}

	return ts
}
