package basecamp

import (
	"context"
	"fmt"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// Campfire represents a Basecamp Campfire (real-time chat room).
type Campfire struct {
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
	LinesURL         string    `json:"lines_url"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
}

// CampfireLine represents a message in a Campfire chat.
type CampfireLine struct {
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
	Content          string    `json:"content"`
	Parent           *Parent   `json:"parent,omitempty"`
	Bucket           *Bucket   `json:"bucket,omitempty"`
	Creator          *Person   `json:"creator,omitempty"`
}

// CreateCampfireLineRequest specifies the parameters for creating a campfire line.
type CreateCampfireLineRequest struct {
	// Content is the plain text message body (required).
	Content string `json:"content"`
}

// Chatbot represents a Basecamp chatbot integration.
type Chatbot struct {
	ID          int64     `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ServiceName string    `json:"service_name"`
	CommandURL  string    `json:"command_url,omitempty"`
	URL         string    `json:"url"`
	AppURL      string    `json:"app_url"`
	LinesURL    string    `json:"lines_url"`
}

// CreateChatbotRequest specifies the parameters for creating a chatbot.
type CreateChatbotRequest struct {
	// ServiceName is the chatbot name used to invoke queries and commands (required).
	// No spaces, emoji or non-word characters are allowed.
	ServiceName string `json:"service_name"`
	// CommandURL is the HTTPS URL that Basecamp should call when the bot is addressed (optional).
	CommandURL string `json:"command_url,omitempty"`
}

// UpdateChatbotRequest specifies the parameters for updating a chatbot.
type UpdateChatbotRequest struct {
	// ServiceName is the chatbot name used to invoke queries and commands (required).
	// No spaces, emoji or non-word characters are allowed.
	ServiceName string `json:"service_name"`
	// CommandURL is the HTTPS URL that Basecamp should call when the bot is addressed (optional).
	CommandURL string `json:"command_url,omitempty"`
}

// CampfiresService handles campfire operations.
type CampfiresService struct {
	client *AccountClient
}

// NewCampfiresService creates a new CampfiresService.
func NewCampfiresService(client *AccountClient) *CampfiresService {
	return &CampfiresService{client: client}
}

// List returns all campfires across the account.
func (s *CampfiresService) List(ctx context.Context) (result []Campfire, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "List",
		ResourceType: "campfire", IsMutation: false,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.ListCampfiresWithResponse(ctx, s.client.accountID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	campfires := make([]Campfire, 0, len(*resp.JSON200))
	for _, gc := range *resp.JSON200 {
		campfires = append(campfires, campfireFromGenerated(gc))
	}
	return campfires, nil
}

// Get returns a campfire by ID.
// bucketID is the project ID, campfireID is the campfire ID.
func (s *CampfiresService) Get(ctx context.Context, bucketID, campfireID int64) (result *Campfire, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "Get",
		ResourceType: "campfire", IsMutation: false,
		BucketID: bucketID, ResourceID: campfireID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.GetCampfireWithResponse(ctx, s.client.accountID, bucketID, campfireID)
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

	campfire := campfireFromGenerated(resp.JSON200.Campfire)
	return &campfire, nil
}

// ListLines returns all lines (messages) in a campfire.
// bucketID is the project ID, campfireID is the campfire ID.
func (s *CampfiresService) ListLines(ctx context.Context, bucketID, campfireID int64) (result []CampfireLine, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "ListLines",
		ResourceType: "campfire_line", IsMutation: false,
		BucketID: bucketID, ResourceID: campfireID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.ListCampfireLinesWithResponse(ctx, s.client.accountID, bucketID, campfireID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	lines := make([]CampfireLine, 0, len(*resp.JSON200))
	for _, gl := range *resp.JSON200 {
		lines = append(lines, campfireLineFromGenerated(gl))
	}
	return lines, nil
}

// GetLine returns a single line (message) from a campfire.
// bucketID is the project ID, campfireID is the campfire ID, lineID is the line ID.
func (s *CampfiresService) GetLine(ctx context.Context, bucketID, campfireID, lineID int64) (result *CampfireLine, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "GetLine",
		ResourceType: "campfire_line", IsMutation: false,
		BucketID: bucketID, ResourceID: lineID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.GetCampfireLineWithResponse(ctx, s.client.accountID, bucketID, campfireID, lineID)
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

	line := campfireLineFromGenerated(resp.JSON200.Line)
	return &line, nil
}

// CreateLine creates a new line (message) in a campfire.
// bucketID is the project ID, campfireID is the campfire ID.
// Returns the created line.
func (s *CampfiresService) CreateLine(ctx context.Context, bucketID, campfireID int64, content string) (result *CampfireLine, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "CreateLine",
		ResourceType: "campfire_line", IsMutation: true,
		BucketID: bucketID, ResourceID: campfireID,
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
		err = ErrUsage("campfire line content is required")
		return nil, err
	}

	body := generated.CreateCampfireLineJSONRequestBody{
		Content: content,
	}

	resp, err := s.client.gen.CreateCampfireLineWithResponse(ctx, s.client.accountID, bucketID, campfireID, body)
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

	line := campfireLineFromGenerated(resp.JSON200.Line)
	return &line, nil
}

// DeleteLine deletes a line (message) from a campfire.
// bucketID is the project ID, campfireID is the campfire ID, lineID is the line ID.
func (s *CampfiresService) DeleteLine(ctx context.Context, bucketID, campfireID, lineID int64) (err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "DeleteLine",
		ResourceType: "campfire_line", IsMutation: true,
		BucketID: bucketID, ResourceID: lineID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.DeleteCampfireLineWithResponse(ctx, s.client.accountID, bucketID, campfireID, lineID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// ListChatbots returns all chatbots for a campfire.
// bucketID is the project ID, campfireID is the campfire ID.
// Note: Chatbots are account-wide but with basecamp-specific callback URLs.
func (s *CampfiresService) ListChatbots(ctx context.Context, bucketID, campfireID int64) (result []Chatbot, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "ListChatbots",
		ResourceType: "chatbot", IsMutation: false,
		BucketID: bucketID, ResourceID: campfireID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.ListChatbotsWithResponse(ctx, s.client.accountID, bucketID, campfireID)
	if err != nil {
		return nil, err
	}
	if err = checkResponse(resp.HTTPResponse); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}

	chatbots := make([]Chatbot, 0, len(*resp.JSON200))
	for _, gc := range *resp.JSON200 {
		chatbots = append(chatbots, chatbotFromGenerated(gc))
	}
	return chatbots, nil
}

// GetChatbot returns a chatbot by ID.
// bucketID is the project ID, campfireID is the campfire ID, chatbotID is the chatbot ID.
func (s *CampfiresService) GetChatbot(ctx context.Context, bucketID, campfireID, chatbotID int64) (result *Chatbot, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "GetChatbot",
		ResourceType: "chatbot", IsMutation: false,
		BucketID: bucketID, ResourceID: chatbotID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.GetChatbotWithResponse(ctx, s.client.accountID, bucketID, campfireID, chatbotID)
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

	chatbot := chatbotFromGenerated(resp.JSON200.Chatbot)
	return &chatbot, nil
}

// CreateChatbot creates a new chatbot for a campfire.
// bucketID is the project ID, campfireID is the campfire ID.
// Note: Chatbots are account-wide and can only be managed by administrators.
// Returns the created chatbot with its lines_url for posting.
func (s *CampfiresService) CreateChatbot(ctx context.Context, bucketID, campfireID int64, req *CreateChatbotRequest) (result *Chatbot, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "CreateChatbot",
		ResourceType: "chatbot", IsMutation: true,
		BucketID: bucketID, ResourceID: campfireID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.ServiceName == "" {
		err = ErrUsage("chatbot service_name is required")
		return nil, err
	}

	body := generated.CreateChatbotJSONRequestBody{
		ServiceName: req.ServiceName,
	}
	if req.CommandURL != "" {
		body.CommandUrl = req.CommandURL
	}

	resp, err := s.client.gen.CreateChatbotWithResponse(ctx, s.client.accountID, bucketID, campfireID, body)
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

	chatbot := chatbotFromGenerated(resp.JSON200.Chatbot)
	return &chatbot, nil
}

// UpdateChatbot updates an existing chatbot.
// bucketID is the project ID, campfireID is the campfire ID, chatbotID is the chatbot ID.
// Note: Updates to chatbots are account-wide.
// Returns the updated chatbot.
func (s *CampfiresService) UpdateChatbot(ctx context.Context, bucketID, campfireID, chatbotID int64, req *UpdateChatbotRequest) (result *Chatbot, err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "UpdateChatbot",
		ResourceType: "chatbot", IsMutation: true,
		BucketID: bucketID, ResourceID: chatbotID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	if req == nil || req.ServiceName == "" {
		err = ErrUsage("chatbot service_name is required")
		return nil, err
	}

	body := generated.UpdateChatbotJSONRequestBody{
		ServiceName: req.ServiceName,
	}
	if req.CommandURL != "" {
		body.CommandUrl = req.CommandURL
	}

	resp, err := s.client.gen.UpdateChatbotWithResponse(ctx, s.client.accountID, bucketID, campfireID, chatbotID, body)
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

	chatbot := chatbotFromGenerated(resp.JSON200.Chatbot)
	return &chatbot, nil
}

// DeleteChatbot deletes a chatbot.
// bucketID is the project ID, campfireID is the campfire ID, chatbotID is the chatbot ID.
// Note: Deleting a chatbot removes it from the entire account.
func (s *CampfiresService) DeleteChatbot(ctx context.Context, bucketID, campfireID, chatbotID int64) (err error) {
	op := OperationInfo{
		Service: "Campfires", Operation: "DeleteChatbot",
		ResourceType: "chatbot", IsMutation: true,
		BucketID: bucketID, ResourceID: chatbotID,
	}
	if gater, ok := s.client.parent.hooks.(GatingHooks); ok {
		if ctx, err = gater.OnOperationGate(ctx, op); err != nil {
			return
		}
	}
	start := time.Now()
	ctx = s.client.parent.hooks.OnOperationStart(ctx, op)
	defer func() { s.client.parent.hooks.OnOperationEnd(ctx, op, err, time.Since(start)) }()

	resp, err := s.client.gen.DeleteChatbotWithResponse(ctx, s.client.accountID, bucketID, campfireID, chatbotID)
	if err != nil {
		return err
	}
	return checkResponse(resp.HTTPResponse)
}

// campfireFromGenerated converts a generated Campfire to our clean Campfire type.
func campfireFromGenerated(gc generated.Campfire) Campfire {
	c := Campfire{
		Status:           gc.Status,
		VisibleToClients: gc.VisibleToClients,
		Title:            gc.Title,
		InheritsStatus:   gc.InheritsStatus,
		Type:             gc.Type,
		URL:              gc.Url,
		AppURL:           gc.AppUrl,
		LinesURL:         gc.LinesUrl,
		CreatedAt:        gc.CreatedAt,
		UpdatedAt:        gc.UpdatedAt,
	}

	if gc.Id != nil {
		c.ID = *gc.Id
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

	return c
}

// campfireLineFromGenerated converts a generated CampfireLine to our clean CampfireLine type.
func campfireLineFromGenerated(gl generated.CampfireLine) CampfireLine {
	l := CampfireLine{
		Status:           gl.Status,
		VisibleToClients: gl.VisibleToClients,
		Title:            gl.Title,
		InheritsStatus:   gl.InheritsStatus,
		Type:             gl.Type,
		URL:              gl.Url,
		AppURL:           gl.AppUrl,
		Content:          gl.Content,
		CreatedAt:        gl.CreatedAt,
		UpdatedAt:        gl.UpdatedAt,
	}

	if gl.Id != nil {
		l.ID = *gl.Id
	}

	if gl.Parent.Id != nil || gl.Parent.Title != "" {
		l.Parent = &Parent{
			ID:     derefInt64(gl.Parent.Id),
			Title:  gl.Parent.Title,
			Type:   gl.Parent.Type,
			URL:    gl.Parent.Url,
			AppURL: gl.Parent.AppUrl,
		}
	}

	if gl.Bucket.Id != nil || gl.Bucket.Name != "" {
		l.Bucket = &Bucket{
			ID:   derefInt64(gl.Bucket.Id),
			Name: gl.Bucket.Name,
			Type: gl.Bucket.Type,
		}
	}

	if gl.Creator.Id != nil || gl.Creator.Name != "" {
		l.Creator = &Person{
			ID:           derefInt64(gl.Creator.Id),
			Name:         gl.Creator.Name,
			EmailAddress: gl.Creator.EmailAddress,
			AvatarURL:    gl.Creator.AvatarUrl,
			Admin:        gl.Creator.Admin,
			Owner:        gl.Creator.Owner,
		}
	}

	return l
}

// chatbotFromGenerated converts a generated Chatbot to our clean Chatbot type.
func chatbotFromGenerated(gc generated.Chatbot) Chatbot {
	c := Chatbot{
		ServiceName: gc.ServiceName,
		CommandURL:  gc.CommandUrl,
		URL:         gc.Url,
		AppURL:      gc.AppUrl,
		LinesURL:    gc.LinesUrl,
		CreatedAt:   gc.CreatedAt,
		UpdatedAt:   gc.UpdatedAt,
	}

	if gc.Id != nil {
		c.ID = *gc.Id
	}

	return c
}
