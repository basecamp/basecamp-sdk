package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Boost represents a Basecamp boost (emoji reaction) on a recording.
type Boost struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	Booster   *Person   `json:"booster,omitempty"`
	Recording *Parent   `json:"recording,omitempty"`
}

// BoostListResult contains the results from listing boosts.
type BoostListResult struct {
	// Boosts is the list of boosts returned.
	Boosts []Boost
	// Meta contains pagination metadata (total count, etc.).
	Meta ListMeta
}

// BoostsService handles boost operations.
type BoostsService struct {
	client *AccountClient
}

// NewBoostsService creates a new BoostsService.
func NewBoostsService(client *AccountClient) *BoostsService {
	return &BoostsService{client: client}
}

// ListRecording returns all boosts on a recording.
//
// The returned BoostListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *BoostsService) ListRecording(ctx context.Context, recordingID int64) (result *BoostListResult, err error) {
	op := OperationInfo{
		Service: "Boosts", Operation: "ListRecording",
		ResourceType: "boost", IsMutation: false,
		ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListRecordingBoostsWithResponse(ctx, s.client.accountID, recordingID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}

	totalCount := parseTotalCount(resp.HTTPResponse)

	if resp.JSON200 == nil {
		return &BoostListResult{Boosts: nil, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	boosts := make([]Boost, 0, len(*resp.JSON200))
	for _, gb := range *resp.JSON200 {
		boosts = append(boosts, boostFromGenerated(gb))
	}
	return &BoostListResult{Boosts: boosts, Meta: ListMeta{TotalCount: totalCount}}, nil
}

// ListEvent returns all boosts on a specific event within a recording.
//
// The returned BoostListResult includes pagination metadata (TotalCount from
// X-Total-Count header) when available.
func (s *BoostsService) ListEvent(ctx context.Context, recordingID, eventID int64) (result *BoostListResult, err error) {
	op := OperationInfo{
		Service: "Boosts", Operation: "ListEvent",
		ResourceType: "boost", IsMutation: false,
		ResourceID: eventID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.ListEventBoostsWithResponse(ctx, s.client.accountID, recordingID, eventID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}

	totalCount := parseTotalCount(resp.HTTPResponse)

	if resp.JSON200 == nil {
		return &BoostListResult{Boosts: nil, Meta: ListMeta{TotalCount: totalCount}}, nil
	}

	boosts := make([]Boost, 0, len(*resp.JSON200))
	for _, gb := range *resp.JSON200 {
		boosts = append(boosts, boostFromGenerated(gb))
	}
	return &BoostListResult{Boosts: boosts, Meta: ListMeta{TotalCount: totalCount}}, nil
}

// Get returns a boost by ID.
func (s *BoostsService) Get(ctx context.Context, boostID int64) (result *Boost, err error) {
	op := OperationInfo{
		Service: "Boosts", Operation: "Get",
		ResourceType: "boost", IsMutation: false,
		ResourceID: boostID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.GetBoostWithResponse(ctx, s.client.accountID, boostID)
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

	boost := boostFromGenerated(*resp.JSON200)
	return &boost, nil
}

// CreateRecording creates a boost on a recording.
// content is the emoji content for the boost.
// Returns the created boost.
func (s *BoostsService) CreateRecording(ctx context.Context, recordingID int64, content string) (result *Boost, err error) {
	op := OperationInfo{
		Service: "Boosts", Operation: "CreateRecording",
		ResourceType: "boost", IsMutation: true,
		ResourceID: recordingID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if content == "" {
		err = ErrUsage("boost content is required")
		return nil, err
	}

	body := generated.CreateRecordingBoostJSONRequestBody{
		Content: content,
	}

	resp, err := s.client.parent.gen.CreateRecordingBoostWithResponse(ctx, s.client.accountID, recordingID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	boost := boostFromGenerated(*resp.JSON201)
	return &boost, nil
}

// CreateEvent creates a boost on a specific event within a recording.
// content is the emoji content for the boost.
// Returns the created boost.
func (s *BoostsService) CreateEvent(ctx context.Context, recordingID, eventID int64, content string) (result *Boost, err error) {
	op := OperationInfo{
		Service: "Boosts", Operation: "CreateEvent",
		ResourceType: "boost", IsMutation: true,
		ResourceID: eventID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if content == "" {
		err = ErrUsage("boost content is required")
		return nil, err
	}

	body := generated.CreateEventBoostJSONRequestBody{
		Content: content,
	}

	resp, err := s.client.parent.gen.CreateEventBoostWithResponse(ctx, s.client.accountID, recordingID, eventID, body)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		err = fmt.Errorf("unexpected empty response")
		return nil, err
	}

	boost := boostFromGenerated(*resp.JSON201)
	return &boost, nil
}

// Delete deletes a boost.
func (s *BoostsService) Delete(ctx context.Context, boostID int64) (err error) {
	op := OperationInfo{
		Service: "Boosts", Operation: "Delete",
		ResourceType: "boost", IsMutation: true,
		ResourceID: boostID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.parent.gen.DeleteBoostWithResponse(ctx, s.client.accountID, boostID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// boostFromGenerated converts a generated Boost to our clean Boost type.
func boostFromGenerated(gb generated.Boost) Boost {
	b := Boost{
		Content:   gb.Content,
		CreatedAt: gb.CreatedAt,
	}

	b.ID = derefInt64(gb.Id)

	if derefInt64(gb.Booster.Id) != 0 || gb.Booster.Name != "" {
		b.Booster = &Person{
			ID:           derefInt64(gb.Booster.Id),
			Name:         gb.Booster.Name,
			EmailAddress: gb.Booster.EmailAddress,
			AvatarURL:    gb.Booster.AvatarUrl,
			Admin:        gb.Booster.Admin,
			Owner:        gb.Booster.Owner,
		}
	}

	if derefInt64(gb.Recording.Id) != 0 || gb.Recording.Title != "" {
		b.Recording = &Parent{
			ID:     derefInt64(gb.Recording.Id),
			Title:  gb.Recording.Title,
			Type:   gb.Recording.Type,
			URL:    gb.Recording.Url,
			AppURL: gb.Recording.AppUrl,
		}
	}

	return b
}
