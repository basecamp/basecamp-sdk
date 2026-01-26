package basecamp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
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
	client *Client
}

// NewCampfiresService creates a new CampfiresService.
func NewCampfiresService(client *Client) *CampfiresService {
	return &CampfiresService{client: client}
}

// List returns all campfires across the account.
func (s *CampfiresService) List(ctx context.Context) ([]Campfire, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	results, err := s.client.GetAll(ctx, "/chats.json")
	if err != nil {
		return nil, err
	}

	campfires := make([]Campfire, 0, len(results))
	for _, raw := range results {
		var c Campfire
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, fmt.Errorf("failed to parse campfire: %w", err)
		}
		campfires = append(campfires, c)
	}

	return campfires, nil
}

// Get returns a campfire by ID.
// bucketID is the project ID, campfireID is the campfire ID.
func (s *CampfiresService) Get(ctx context.Context, bucketID, campfireID int64) (*Campfire, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/chats/%d.json", bucketID, campfireID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var campfire Campfire
	if err := resp.UnmarshalData(&campfire); err != nil {
		return nil, fmt.Errorf("failed to parse campfire: %w", err)
	}

	return &campfire, nil
}

// ListLines returns all lines (messages) in a campfire.
// bucketID is the project ID, campfireID is the campfire ID.
func (s *CampfiresService) ListLines(ctx context.Context, bucketID, campfireID int64) ([]CampfireLine, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/chats/%d/lines.json", bucketID, campfireID)
	results, err := s.client.GetAll(ctx, path)
	if err != nil {
		return nil, err
	}

	lines := make([]CampfireLine, 0, len(results))
	for _, raw := range results {
		var l CampfireLine
		if err := json.Unmarshal(raw, &l); err != nil {
			return nil, fmt.Errorf("failed to parse campfire line: %w", err)
		}
		lines = append(lines, l)
	}

	return lines, nil
}

// GetLine returns a single line (message) from a campfire.
// bucketID is the project ID, campfireID is the campfire ID, lineID is the line ID.
func (s *CampfiresService) GetLine(ctx context.Context, bucketID, campfireID, lineID int64) (*CampfireLine, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/chats/%d/lines/%d.json", bucketID, campfireID, lineID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var line CampfireLine
	if err := resp.UnmarshalData(&line); err != nil {
		return nil, fmt.Errorf("failed to parse campfire line: %w", err)
	}

	return &line, nil
}

// CreateLine creates a new line (message) in a campfire.
// bucketID is the project ID, campfireID is the campfire ID.
// Returns the created line.
func (s *CampfiresService) CreateLine(ctx context.Context, bucketID, campfireID int64, content string) (*CampfireLine, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if content == "" {
		return nil, ErrUsage("campfire line content is required")
	}

	req := &CreateCampfireLineRequest{Content: content}
	path := fmt.Sprintf("/buckets/%d/chats/%d/lines.json", bucketID, campfireID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var line CampfireLine
	if err := resp.UnmarshalData(&line); err != nil {
		return nil, fmt.Errorf("failed to parse campfire line: %w", err)
	}

	return &line, nil
}

// DeleteLine deletes a line (message) from a campfire.
// bucketID is the project ID, campfireID is the campfire ID, lineID is the line ID.
func (s *CampfiresService) DeleteLine(ctx context.Context, bucketID, campfireID, lineID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/chats/%d/lines/%d.json", bucketID, campfireID, lineID)
	_, err := s.client.Delete(ctx, path)
	return err
}

// ListChatbots returns all chatbots for a campfire.
// bucketID is the project ID, campfireID is the campfire ID.
// Note: Chatbots are account-wide but with basecamp-specific callback URLs.
func (s *CampfiresService) ListChatbots(ctx context.Context, bucketID, campfireID int64) ([]Chatbot, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/chats/%d/integrations.json", bucketID, campfireID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var chatbots []Chatbot
	if err := resp.UnmarshalData(&chatbots); err != nil {
		return nil, fmt.Errorf("failed to parse chatbots: %w", err)
	}

	return chatbots, nil
}

// GetChatbot returns a chatbot by ID.
// bucketID is the project ID, campfireID is the campfire ID, chatbotID is the chatbot ID.
func (s *CampfiresService) GetChatbot(ctx context.Context, bucketID, campfireID, chatbotID int64) (*Chatbot, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/buckets/%d/chats/%d/integrations/%d.json", bucketID, campfireID, chatbotID)
	resp, err := s.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var chatbot Chatbot
	if err := resp.UnmarshalData(&chatbot); err != nil {
		return nil, fmt.Errorf("failed to parse chatbot: %w", err)
	}

	return &chatbot, nil
}

// CreateChatbot creates a new chatbot for a campfire.
// bucketID is the project ID, campfireID is the campfire ID.
// Note: Chatbots are account-wide and can only be managed by administrators.
// Returns the created chatbot with its lines_url for posting.
func (s *CampfiresService) CreateChatbot(ctx context.Context, bucketID, campfireID int64, req *CreateChatbotRequest) (*Chatbot, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.ServiceName == "" {
		return nil, ErrUsage("chatbot service_name is required")
	}

	path := fmt.Sprintf("/buckets/%d/chats/%d/integrations.json", bucketID, campfireID)
	resp, err := s.client.Post(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var chatbot Chatbot
	if err := resp.UnmarshalData(&chatbot); err != nil {
		return nil, fmt.Errorf("failed to parse chatbot: %w", err)
	}

	return &chatbot, nil
}

// UpdateChatbot updates an existing chatbot.
// bucketID is the project ID, campfireID is the campfire ID, chatbotID is the chatbot ID.
// Note: Updates to chatbots are account-wide.
// Returns the updated chatbot.
func (s *CampfiresService) UpdateChatbot(ctx context.Context, bucketID, campfireID, chatbotID int64, req *UpdateChatbotRequest) (*Chatbot, error) {
	if err := s.client.RequireAccount(); err != nil {
		return nil, err
	}

	if req == nil || req.ServiceName == "" {
		return nil, ErrUsage("chatbot service_name is required")
	}

	path := fmt.Sprintf("/buckets/%d/chats/%d/integrations/%d.json", bucketID, campfireID, chatbotID)
	resp, err := s.client.Put(ctx, path, req)
	if err != nil {
		return nil, err
	}

	var chatbot Chatbot
	if err := resp.UnmarshalData(&chatbot); err != nil {
		return nil, fmt.Errorf("failed to parse chatbot: %w", err)
	}

	return &chatbot, nil
}

// DeleteChatbot deletes a chatbot.
// bucketID is the project ID, campfireID is the campfire ID, chatbotID is the chatbot ID.
// Note: Deleting a chatbot removes it from the entire account.
func (s *CampfiresService) DeleteChatbot(ctx context.Context, bucketID, campfireID, chatbotID int64) error {
	if err := s.client.RequireAccount(); err != nil {
		return err
	}

	path := fmt.Sprintf("/buckets/%d/chats/%d/integrations/%d.json", bucketID, campfireID, chatbotID)
	_, err := s.client.Delete(ctx, path)
	return err
}
